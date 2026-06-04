package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/crypto/ssh"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/tunnel"
)

// Remote storage option keys (without type prefix)
const (
	urlKey  = "url"
	userKey = "user"
	passKey = "pass"
	pathKey = "path"
)

const (
	dialTimeout   = 10 * time.Second
	remoteTimeout = 30 * time.Second
)

// parseRemoteURL extracts host, port, and path from a URL like ftp://host:port/path or sftp://host/path.
func parseRemoteURL(rawURL string, defaultPort string) (host, port, path string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	host = u.Hostname()
	if host == "" {
		return "", "", "", fmt.Errorf("missing host in URL %q", rawURL)
	}
	port = u.Port()
	if port == "" {
		port = defaultPort
	}
	path = strings.TrimPrefix(u.Path, "/")
	return host, port, path, nil
}

// ---------------------------------------------------------------------------
// FTP Storage
// ---------------------------------------------------------------------------

type ftpStorage struct {
	addr        string
	user        string
	pass        string
	root        string
	tunnelClose func()
}

func newFTPStorage(tbl *catalog.Table) (Storage, error) {
	rawURL := tbl.Option(urlKey, "")
	host := tbl.Option("host", "")
	port := tbl.Option("port", "21")
	root := tbl.Option("path", "")
	user := tbl.Option("username", "")
	pass := tbl.Option("password", "")

	if rawURL != "" {
		parsedHost, parsedPort, parsedPath, err := parseRemoteURL(rawURL, "21")
		if err != nil {
			return nil, err
		}
		if host == "" {
			host = parsedHost
		}
		if port == "21" || port == "" {
			port = parsedPort
		}
		if root == "" {
			root = parsedPath
		}
		if u, err := url.Parse(rawURL); err == nil && u.User != nil {
			if user == "" {
				user = u.User.Username()
			}
			if pass == "" {
				pass, _ = u.User.Password()
			}
		}
	}
	if host == "" {
		return nil, fmt.Errorf("missing host for FTP table %s", tbl.Name)
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	var tunnelClose func()

	if sshCfg := tunnel.OptionsFromMap(tbl.WithOptions); sshCfg != nil {
		portInt := 21
		if p, err := strconv.Atoi(port); err == nil {
			portInt = p
		}
		localAddr, closeFn, err := tunnel.Dial(sshCfg, host, portInt)
		if err != nil {
			return nil, fmt.Errorf("ssh tunnel: %w", err)
		}
		tunnelClose = closeFn
		addr = localAddr
	}

	return &ftpStorage{
		addr:        addr,
		user:        user,
		pass:        pass,
		root:        strings.TrimSuffix(root, "/"),
		tunnelClose: tunnelClose,
	}, nil
}

func (s *ftpStorage) Close() error {
	if s.tunnelClose != nil {
		s.tunnelClose()
		s.tunnelClose = nil
	}
	return nil
}

func (s *ftpStorage) dial() (*ftp.ServerConn, error) {
	conn, err := ftp.DialTimeout(s.addr, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to FTP %s: %w", s.addr, err)
	}
	if s.user != "" {
		if err := conn.Login(s.user, s.pass); err != nil {
			conn.Quit()
			return nil, fmt.Errorf("failed to login to FTP: %w", err)
		}
	}
	return conn, nil
}

func (s *ftpStorage) resolve(name string) string {
	if s.root == "" {
		return name
	}
	return s.root + "/" + name
}

func (s *ftpStorage) Open(ctx context.Context, name string) (File, error) {
	conn, err := s.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	filePath := s.resolve(name)
	if s.root != "" {
		if err := conn.ChangeDir(s.root); err != nil {
			return nil, fmt.Errorf("failed to change to directory %s: %w", s.root, err)
		}
	}
	resp, err := conn.Retr(filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file %s: %w", filePath, err)
	}
	defer resp.Close()

	data, err := io.ReadAll(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return &ftpReadFile{baseFile: baseFile{}, reader: bytes.NewReader(data), name: name}, nil
}

func (s *ftpStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	conn, err := s.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	if s.root != "" {
		if err := conn.ChangeDir(s.root); err != nil {
			return nil, err
		}
	}
	entries, err := conn.List(".")
	if err != nil {
		return nil, err
	}
	baseName := filepath.Base(name)
	for _, e := range entries {
		if e.Name == baseName {
			return &baseFileInfo{
				name:  e.Name,
				path:  name,
				size:  uint64(e.Size),
				isDir: e.Type == ftp.EntryTypeFolder,
			}, nil
		}
	}
	return nil, fmt.Errorf("file %s not found", name)
}

func (s *ftpStorage) Glob(ctx context.Context, pattern string) ([]string, error) {
	conn, err := s.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	if s.root != "" {
		if err := conn.ChangeDir(s.root); err != nil {
			return nil, err
		}
	}
	entries, err := conn.List(".")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if e.Type != ftp.EntryTypeFile {
			continue
		}
		if matchPattern(e.Name, pattern) {
			result = append(result, e.Name)
		}
	}
	return result, nil
}

func (s *ftpStorage) List(ctx context.Context, dirPath string) ([]string, error) {
	conn, err := s.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	path := s.resolve(dirPath)
	if path != "" {
		if err := conn.ChangeDir(path); err != nil {
			return nil, err
		}
	} else if s.root != "" {
		if err := conn.ChangeDir(s.root); err != nil {
			return nil, err
		}
	}
	entries, err := conn.List(".")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		entry := e.Name
		if dirPath == "" || dirPath == "." {
			result = append(result, entry)
		} else {
			result = append(result, filepath.Join(dirPath, entry))
		}
	}
	return result, nil
}

func (s *ftpStorage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()
	return conn.MakeDir(s.resolve(name))
}

func (s *ftpStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	parts := strings.Split(strings.Trim(s.resolve(name), "/"), "/")
	current := ""
	for _, p := range parts {
		current += "/" + p
		if err := conn.MakeDir(current); err != nil {
			// ignore "already exists" errors
		}
	}
	return nil
}

func (s *ftpStorage) Remove(ctx context.Context, name string) error {
	conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()
	return conn.Delete(s.resolve(name))
}

func (s *ftpStorage) RemoveAll(ctx context.Context, name string) error {
	// FTP doesn't have recursive delete; try Delete and ignore errors
	conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()
	conn.Delete(s.resolve(name))
	return nil
}

func (s *ftpStorage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	filePath := s.resolve(name)
	if s.root != "" {
		if err := conn.ChangeDir(s.root); err != nil {
			return err
		}
	}
	return conn.Stor(filepath.Base(filePath), bytes.NewReader(data))
}

func (s *ftpStorage) Rename(ctx context.Context, oldName, newName string) error {
	conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()
	return conn.Rename(s.resolve(oldName), s.resolve(newName))
}

func (s *ftpStorage) Create(ctx context.Context, name string) (File, error) {
	return &ftpWriteFile{baseFile: baseFile{}, store: s, name: name, buf: &bytes.Buffer{}}, nil
}

func (s *ftpStorage) Exists(ctx context.Context, name string) bool {
	conn, err := s.dial()
	if err != nil {
		return false
	}
	defer conn.Quit()

	if s.root != "" {
		conn.ChangeDir(s.root)
	}
	entries, err := conn.List(".")
	if err != nil {
		return false
	}
	baseName := filepath.Base(name)
	for _, e := range entries {
		if e.Name == baseName {
			return true
		}
	}
	return false
}

func (s *ftpStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

type ftpReadFile struct {
	baseFile
	reader *bytes.Reader
	name   string
}

func (f *ftpReadFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *ftpReadFile) Close() error                { return nil }
func (f *ftpReadFile) String() string              { return f.name }

type ftpWriteFile struct {
	baseFile
	store *ftpStorage
	name  string
	buf   *bytes.Buffer
}

func (f *ftpWriteFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *ftpWriteFile) Close() error {
	conn, err := f.store.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()
	filePath := f.store.resolve(f.name)
	if f.store.root != "" {
		if err := conn.ChangeDir(f.store.root); err != nil {
			return err
		}
	}
	return conn.Stor(filepath.Base(filePath), bytes.NewReader(f.buf.Bytes()))
}
func (f *ftpWriteFile) String() string { return f.name }

// ---------------------------------------------------------------------------
// SFTP Storage
// ---------------------------------------------------------------------------

type sftpStorage struct {
	addr   string
	user   string
	pass   string
	root   string
	sshCfg *ssh.ClientConfig
}

func newSFTPStorage(tbl *catalog.Table) (Storage, error) {
	rawURL := tbl.Option(urlKey, "")
	host := tbl.Option("host", "")
	port := tbl.Option("port", "22")
	root := tbl.Option("path", "")
	user := tbl.Option("username", "")
	pass := tbl.Option("password", "")

	if rawURL != "" {
		parsedHost, parsedPort, parsedPath, err := parseRemoteURL(rawURL, "22")
		if err != nil {
			return nil, err
		}
		if host == "" {
			host = parsedHost
		}
		if port == "22" || port == "" {
			port = parsedPort
		}
		if root == "" {
			root = parsedPath
		}
		if u, err := url.Parse(rawURL); err == nil && u.User != nil {
			if user == "" {
				user = u.User.Username()
			}
			if pass == "" {
				pass, _ = u.User.Password()
			}
		}
	}
	if host == "" {
		return nil, fmt.Errorf("missing host for SFTP table %s", tbl.Name)
	}
	sshCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         dialTimeout,
	}
	return &sftpStorage{
		addr:   fmt.Sprintf("%s:%s", host, port),
		user:   user,
		pass:   pass,
		root:   strings.TrimSuffix(root, "/"),
		sshCfg: sshCfg,
	}, nil
}

func (s *sftpStorage) newClient() (*sftp.Client, error) {
	sshClient, err := ssh.Dial("tcp", s.addr, s.sshCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via SSH: %w", err)
	}
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	return client, nil
}

func (s *sftpStorage) resolve(name string) string {
	if s.root == "" {
		return name
	}
	return s.root + "/" + name
}

func (s *sftpStorage) Open(ctx context.Context, name string) (File, error) {
	client, err := s.newClient()
	if err != nil {
		return nil, err
	}
	filePath := s.resolve(name)
	f, err := client.Open(filePath)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to open SFTP file %s: %w", filePath, err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	client.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read SFTP file %s: %w", filePath, err)
	}
	return &sftpReadFile{baseFile: baseFile{}, reader: bytes.NewReader(data), name: name}, nil
}

func (s *sftpStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	client, err := s.newClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	filePath := s.resolve(name)
	info, err := client.Stat(filePath)
	if err != nil {
		return nil, err
	}
	return &baseFileInfo{
		name:  filepath.Base(name),
		path:  name,
		size:  uint64(info.Size()),
		isDir: info.IsDir(),
	}, nil
}

func (s *sftpStorage) Glob(ctx context.Context, pattern string) ([]string, error) {
	client, err := s.newClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	dir := filepath.Dir(s.resolve("."))
	entries, err := client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if matchPattern(e.Name(), pattern) {
			result = append(result, e.Name())
		}
	}
	return result, nil
}

func (s *sftpStorage) List(ctx context.Context, dirPath string) ([]string, error) {
	client, err := s.newClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	path := s.resolve(dirPath)
	entries, err := client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if dirPath == "" || dirPath == "." {
			result = append(result, e.Name())
		} else {
			result = append(result, filepath.Join(dirPath, e.Name()))
		}
	}
	return result, nil
}

func (s *sftpStorage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Mkdir(s.resolve(name))
}

func (s *sftpStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.MkdirAll(s.resolve(name))
}

func (s *sftpStorage) Remove(ctx context.Context, name string) error {
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Remove(s.resolve(name))
}

func (s *sftpStorage) RemoveAll(ctx context.Context, name string) error {
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.RemoveDirectory(s.resolve(name))
}

func (s *sftpStorage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	filePath := s.resolve(name)
	f, err := client.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s *sftpStorage) Rename(ctx context.Context, oldName, newName string) error {
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Rename(s.resolve(oldName), s.resolve(newName))
}

func (s *sftpStorage) Create(ctx context.Context, name string) (File, error) {
	return &sftpWriteFile{
		baseFile: baseFile{},
		store:    s,
		name:     name,
		buf:      &bytes.Buffer{},
	}, nil
}

func (s *sftpStorage) Exists(ctx context.Context, name string) bool {
	client, err := s.newClient()
	if err != nil {
		return false
	}
	defer client.Close()
	_, err = client.Stat(s.resolve(name))
	return err == nil
}

func (s *sftpStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

type sftpReadFile struct {
	baseFile
	reader *bytes.Reader
	name   string
}

func (f *sftpReadFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *sftpReadFile) Close() error                { return nil }
func (f *sftpReadFile) String() string              { return f.name }

type sftpWriteFile struct {
	baseFile
	store *sftpStorage
	name  string
	buf   *bytes.Buffer
}

func (f *sftpWriteFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *sftpWriteFile) Close() error {
	client, err := f.store.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	filePath := f.store.resolve(f.name)
	out, err := client.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.Write(f.buf.Bytes())
	return err
}
func (f *sftpWriteFile) String() string { return f.name }

// ---------------------------------------------------------------------------
// WebDAV Storage
// ---------------------------------------------------------------------------

type webdavStorage struct {
	client *gowebdav.Client
	root   string
}

func newWebDAVStorage(tbl *catalog.Table) (Storage, error) {
	rawURL := tbl.Option(urlKey, "")
	webdavURL := tbl.Option("url", "")
	user := tbl.Option("username", "")
	pass := tbl.Option("password", "")
	root := tbl.Option("path", "")

	if rawURL != "" {
		if u, err := url.Parse(rawURL); err == nil {
			if webdavURL == "" {
				webdavURL = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
			}
			if root == "" {
				root = u.Path
			}
			if u.User != nil {
				if user == "" {
					user = u.User.Username()
				}
				if pass == "" {
					pass, _ = u.User.Password()
				}
			}
		}
	}
	if webdavURL == "" {
		webdavURL = rawURL
	}
	if webdavURL == "" {
		return nil, fmt.Errorf("missing url for WebDAV table %s", tbl.Name)
	}
	client := gowebdav.NewClient(webdavURL, user, pass)
	return &webdavStorage{
		client: client,
		root:   strings.TrimSuffix(root, "/"),
	}, nil
}

func (s *webdavStorage) resolve(name string) string {
	if s.root == "" {
		return name
	}
	return s.root + "/" + name
}

func (s *webdavStorage) Open(ctx context.Context, name string) (File, error) {
	data, err := s.client.Read(s.resolve(name))
	if err != nil {
		return nil, fmt.Errorf("failed to read WebDAV file %s: %w", name, err)
	}
	return &webdavReadFile{baseFile: baseFile{}, reader: bytes.NewReader(data), name: name}, nil
}

func (s *webdavStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	dir := filepath.Dir(s.resolve(name))
	entries, err := s.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	baseName := filepath.Base(name)
	for _, e := range entries {
		if e.Name() == baseName {
			return &baseFileInfo{
				name:  e.Name(),
				path:  name,
				size:  uint64(e.Size()),
				isDir: e.IsDir(),
			}, nil
		}
	}
	return nil, fmt.Errorf("file %s not found", name)
}

func (s *webdavStorage) Glob(ctx context.Context, pattern string) ([]string, error) {
	entries, err := s.client.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if matchPattern(e.Name(), pattern) {
			result = append(result, e.Name())
		}
	}
	return result, nil
}

func (s *webdavStorage) List(ctx context.Context, dirPath string) ([]string, error) {
	path := s.resolve(dirPath)
	entries, err := s.client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if dirPath == "" || dirPath == "." {
			result = append(result, e.Name())
		} else {
			result = append(result, filepath.Join(dirPath, e.Name()))
		}
	}
	return result, nil
}

func (s *webdavStorage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	return s.client.Mkdir(s.resolve(name), perm)
}

func (s *webdavStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	return s.client.MkdirAll(s.resolve(name), perm)
}

func (s *webdavStorage) Remove(ctx context.Context, name string) error {
	return s.client.Remove(s.resolve(name))
}

func (s *webdavStorage) RemoveAll(ctx context.Context, name string) error {
	return s.client.RemoveAll(s.resolve(name))
}

func (s *webdavStorage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	return s.client.Write(s.resolve(name), data, perm)
}

func (s *webdavStorage) Rename(ctx context.Context, oldName, newName string) error {
	return s.client.Rename(s.resolve(oldName), s.resolve(newName), true)
}

func (s *webdavStorage) Create(ctx context.Context, name string) (File, error) {
	return &webdavWriteFile{
		baseFile: baseFile{},
		client:   s.client,
		path:     s.resolve(name),
		buf:      &bytes.Buffer{},
	}, nil
}

func (s *webdavStorage) Exists(ctx context.Context, name string) bool {
	dir := filepath.Dir(s.resolve(name))
	entries, err := s.client.ReadDir(dir)
	if err != nil {
		return false
	}
	baseName := filepath.Base(name)
	for _, e := range entries {
		if e.Name() == baseName {
			return true
		}
	}
	return false
}

func (s *webdavStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

type webdavReadFile struct {
	baseFile
	reader *bytes.Reader
	name   string
}

func (f *webdavReadFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *webdavReadFile) Close() error                { return nil }
func (f *webdavReadFile) String() string              { return f.name }

type webdavWriteFile struct {
	baseFile
	client *gowebdav.Client
	path   string
	buf    *bytes.Buffer
}

func (f *webdavWriteFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *webdavWriteFile) Close() error {
	return f.client.Write(f.path, f.buf.Bytes(), 0644)
}
func (f *webdavWriteFile) String() string { return f.path }


