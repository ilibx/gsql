package storage

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/crypto/ssh"

	"github.com/ilibx/gsql/pkg/catalog"
)

// FTP/SFTP/WebDAV storage constants
const (
	ftpHostKey     = "ftp_host"
	ftpPortKey     = "ftp_port"
	ftpUserKey     = "ftp_user"
	ftpPassKey     = "ftp_pass"
	ftpPathKey     = "ftp_path"
	sftpHostKey    = "sftp_host"
	sftpPortKey    = "sftp_port"
	sftpUserKey    = "sftp_user"
	sftpPassKey    = "sftp_pass"
	sftpPathKey    = "sftp_path"
	webdavURLKey   = "webdav_url"
	webdavUserKey  = "webdav_user"
	webdavPassKey  = "webdav_pass"
	webdavPathKey  = "webdav_path"
	dialTimeout    = 10 * time.Second
	remoteTimeout  = 30 * time.Second
)

// readFTPTable reads data from FTP
func readFTPTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	host := tbl.Option(ftpHostKey, "")
	if host == "" {
		return nil, fmt.Errorf("missing ftp_host for table %s", tbl.Name)
	}

	port := tbl.Option(ftpPortKey, "21")
	user := tbl.Option(ftpUserKey, "")
	pass := tbl.Option(ftpPassKey, "")
	path := tbl.Option(ftpPathKey, "")
	pattern := tbl.Option("file_pattern", "*")
	format := strings.ToLower(tbl.Option("format", ""))

	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	// Connect to FTP with timeout
	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := ftp.DialTimeout(addr, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to FTP %s: %w", addr, err)
	}
	defer conn.Quit()

	if user != "" {
		if err := conn.Login(user, pass); err != nil {
			return nil, fmt.Errorf("failed to login to FTP: %w", err)
		}
	}

	// List files
	if path != "" {
		if err := conn.ChangeDir(path); err != nil {
			return nil, fmt.Errorf("failed to change to directory %s: %w", path, err)
		}
	}

	entries, err := conn.List(".")
	if err != nil {
		return nil, fmt.Errorf("failed to list FTP directory: %w", err)
	}

	var fileNames []string
	for _, entry := range entries {
		// entry.Type: 0 = file, 1 = directory, 2 = symlink
		// Only process files (type 0)
		if entry.Type != ftp.EntryTypeFile {
			continue
		}
		if matchPattern(entry.Name, pattern) {
			fileNames = append(fileNames, entry.Name)
		}
	}

	if len(fileNames) == 0 {
		return nil, fmt.Errorf("no files found on FTP at %s", path)
	}

	// Read files in parallel
	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(fileNames))
	for _, fileName := range fileNames {
		go func(fileName string) {
			conn2, err := ftp.DialTimeout(addr, dialTimeout)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to connect to FTP: %w", err)}
				return
			}
			defer conn2.Quit()

			if user != "" {
				if err := conn2.Login(user, pass); err != nil {
					resultCh <- fileResult{err: fmt.Errorf("failed to login to FTP: %w", err)}
					return
				}
			}

			if path != "" {
				if err := conn2.ChangeDir(path); err != nil {
					resultCh <- fileResult{err: fmt.Errorf("failed to change to directory %s: %w", path, err)}
					return
				}
			}

			resp, err := conn2.Retr(fileName)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to retrieve file %s: %w", fileName, err)}
				return
			}
			defer resp.Close()

			body, err := io.ReadAll(resp)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to read file %s: %w", fileName, err)}
				return
			}

			switch format {
			case "csv":
				rows, err := readCSVFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			case "json":
				rows, err := readJSONFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			default:
				resultCh <- fileResult{err: fmt.Errorf("unsupported format %q", format)}
			}
		}(fileName)
	}

	var rows []Row
	for i := 0; i < len(fileNames); i++ {
		result := <-resultCh
		if result.err != nil {
			return nil, result.err
		}
		rows = append(rows, result.rows...)
	}

	return rows, nil
}

// readSFTPTable reads data from SFTP
func readSFTPTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	host := tbl.Option(sftpHostKey, "")
	if host == "" {
		return nil, fmt.Errorf("missing sftp_host for table %s", tbl.Name)
	}

	port := tbl.Option(sftpPortKey, "22")
	user := tbl.Option(sftpUserKey, "")
	pass := tbl.Option(sftpPassKey, "")
	path := tbl.Option(sftpPathKey, "")
	pattern := tbl.Option("file_pattern", "*")
	format := strings.ToLower(tbl.Option("format", ""))

	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         dialTimeout,
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	entries, err := sftpClient.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list SFTP directory %s: %w", path, err)
	}

	var fileNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if matchPattern(entry.Name(), pattern) {
			fileNames = append(fileNames, entry.Name())
		}
	}

	if len(fileNames) == 0 {
		return nil, fmt.Errorf("no files found on SFTP at %s", path)
	}

	// Read files in parallel
	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(fileNames))
	for _, fileName := range fileNames {
		go func(fileName string) {
			client2, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to connect via SSH: %w", err)}
				return
			}
			defer client2.Close()

			sftpClient2, err := sftp.NewClient(client2)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to create SFTP client: %w", err)}
				return
			}
			defer sftpClient2.Close()

			filePath := filepath.Join(path, fileName)
			file, err := sftpClient2.Open(filePath)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to open file %s: %w", filePath, err)}
				return
			}
			defer file.Close()

			body, err := io.ReadAll(file)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to read file %s: %w", filePath, err)}
				return
			}

			switch format {
			case "csv":
				rows, err := readCSVFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			case "json":
				rows, err := readJSONFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			default:
				resultCh <- fileResult{err: fmt.Errorf("unsupported format %q", format)}
			}
		}(fileName)
	}

	var rows []Row
	for i := 0; i < len(fileNames); i++ {
		result := <-resultCh
		if result.err != nil {
			return nil, result.err
		}
		rows = append(rows, result.rows...)
	}

	return rows, nil
}

// readWebDAVTable reads data from WebDAV
func readWebDAVTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	url := tbl.Option(webdavURLKey, "")
	if url == "" {
		return nil, fmt.Errorf("missing webdav_url for table %s", tbl.Name)
	}

	user := tbl.Option(webdavUserKey, "")
	pass := tbl.Option(webdavPassKey, "")
	path := tbl.Option(webdavPathKey, "")
	pattern := tbl.Option("file_pattern", "*")
	format := strings.ToLower(tbl.Option("format", ""))

	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	client := gowebdav.NewClient(url, user, pass)

	// List files
	entries, err := client.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list WebDAV directory %s: %w", path, err)
	}

	var fileNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if matchPattern(entry.Name(), pattern) {
			fileNames = append(fileNames, entry.Name())
		}
	}

	if len(fileNames) == 0 {
		return nil, fmt.Errorf("no files found on WebDAV at %s", path)
	}

	// Read files in parallel
	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(fileNames))
	for _, fileName := range fileNames {
		go func(fileName string) {
			filePath := filepath.Join(path, fileName)
			body, err := client.Read(filePath)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to read file %s: %w", filePath, err)}
				return
			}

			switch format {
			case "csv":
				rows, err := readCSVFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			case "json":
				rows, err := readJSONFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			default:
				resultCh <- fileResult{err: fmt.Errorf("unsupported format %q", format)}
			}
		}(fileName)
	}

	var rows []Row
	for i := 0; i < len(fileNames); i++ {
		result := <-resultCh
		if result.err != nil {
			return nil, result.err
		}
		rows = append(rows, result.rows...)
	}

	return rows, nil
}

// writeFTPTable writes data to FTP
func writeFTPTable(tbl *catalog.Table, rows []Row, appendMode bool) error {
	host := tbl.Option(ftpHostKey, "")
	if host == "" {
		return fmt.Errorf("missing ftp_host for table %s", tbl.Name)
	}

	port := tbl.Option(ftpPortKey, "21")
	user := tbl.Option(ftpUserKey, "")
	pass := tbl.Option(ftpPassKey, "")
	path := tbl.Option(ftpPathKey, "")
	fileName := tbl.Option("file_name", "result.csv")
	format := strings.ToLower(tbl.Option("format", "csv"))

	if appendMode {
		fileName = fmt.Sprintf("append_%d_%s", len(rows), fileName)
	}

	// Generate data
	var data []byte
	switch format {
	case "csv":
		buf := &bytes.Buffer{}
		if err := writeCSVToBuffer(buf, tbl.Columns, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	case "json":
		buf := &bytes.Buffer{}
		if err := writeJSONToBuffer(buf, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}

	// Connect to FTP
	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := ftp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to FTP %s: %w", addr, err)
	}
	defer conn.Quit()

	if user != "" {
		if err := conn.Login(user, pass); err != nil {
			return fmt.Errorf("failed to login to FTP: %w", err)
		}
	}

	if path != "" {
		if err := conn.ChangeDir(path); err != nil {
			return fmt.Errorf("failed to change to directory %s: %w", path, err)
		}
	}

	// Upload file
	if err := conn.Stor(fileName, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to store file %s on FTP: %w", fileName, err)
	}

	return nil
}

// writeSFTPTable writes data to SFTP
func writeSFTPTable(tbl *catalog.Table, rows []Row, appendMode bool) error {
	host := tbl.Option(sftpHostKey, "")
	if host == "" {
		return fmt.Errorf("missing sftp_host for table %s", tbl.Name)
	}

	port := tbl.Option(sftpPortKey, "22")
	user := tbl.Option(sftpUserKey, "")
	pass := tbl.Option(sftpPassKey, "")
	path := tbl.Option(sftpPathKey, "")
	fileName := tbl.Option("file_name", "result.csv")
	format := strings.ToLower(tbl.Option("format", "csv"))

	if appendMode {
		fileName = fmt.Sprintf("append_%d_%s", len(rows), fileName)
	}

	// Generate data
	var data []byte
	switch format {
	case "csv":
		buf := &bytes.Buffer{}
		if err := writeCSVToBuffer(buf, tbl.Columns, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	case "json":
		buf := &bytes.Buffer{}
		if err := writeJSONToBuffer(buf, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	filePath := filepath.Join(path, fileName)
	file, err := sftpClient.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s on SFTP: %w", filePath, err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write to file %s on SFTP: %w", filePath, err)
	}

	return nil
}

// writeWebDAVTable writes data to WebDAV
func writeWebDAVTable(tbl *catalog.Table, rows []Row, appendMode bool) error {
	url := tbl.Option(webdavURLKey, "")
	if url == "" {
		return fmt.Errorf("missing webdav_url for table %s", tbl.Name)
	}

	user := tbl.Option(webdavUserKey, "")
	pass := tbl.Option(webdavPassKey, "")
	path := tbl.Option(webdavPathKey, "")
	fileName := tbl.Option("file_name", "result.csv")
	format := strings.ToLower(tbl.Option("format", "csv"))

	if appendMode {
		fileName = fmt.Sprintf("append_%d_%s", len(rows), fileName)
	}

	// Generate data
	var data []byte
	switch format {
	case "csv":
		buf := &bytes.Buffer{}
		if err := writeCSVToBuffer(buf, tbl.Columns, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	case "json":
		buf := &bytes.Buffer{}
		if err := writeJSONToBuffer(buf, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}

	client := gowebdav.NewClient(url, user, pass)
	filePath := filepath.Join(path, fileName)

	if err := client.Write(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s to WebDAV: %w", filePath, err)
	}

	return nil
}
