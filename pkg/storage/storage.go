package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"
)

type File interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	io.Writer
	fmt.Stringer

	// Truncate 清空/截断文件到 size 大小
	Truncate(ctx context.Context, size int64) error

	CopyTo(ctx context.Context, file File) error

	// Stat 获取文件属性
	Stat(ctx context.Context) (FileInfo, error)

	// Touch 更新上次变更信息
	Touch(ctx context.Context) error
}

type FileInfo interface {
	// LastModified 上次变更时间
	LastModified() *time.Time

	// Size 文件大小
	Size() uint64

	// Path 访问路径
	Path() string

	// Url 访问路径
	Url() string

	// Name 文件名称
	Name() string

	// IsDir 是否为目录
	IsDir() bool

	// Metadata 访问元数据
	Metadata() map[string]string
}

type Storage interface {

	// Open 打开文件
	Open(ctx context.Context, name string) (File, error)

	// Stat 查看文件信息
	Stat(ctx context.Context, name string) (FileInfo, error)

	// Glob 按文件 pattern 查找文件清单
	Glob(ctx context.Context, pattern string) ([]string, error)

	// List 获取文件清单
	List(ctx context.Context, path string) ([]string, error)

	// Mkdir 创建目录
	Mkdir(ctx context.Context, name string, perm fs.FileMode) error

	// MkdirAll 创建目录
	MkdirAll(ctx context.Context, name string, perm fs.FileMode) error

	// Remove 删除文件
	Remove(ctx context.Context, name string) error

	// RemoveAll 删除所有文件
	RemoveAll(ctx context.Context, name string) error

	// WriteFile 写入文件
	WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error

	// Rename 重命名
	Rename(ctx context.Context, oldName, newName string) error

	// Create 流式写入（返回可写文件句柄）
	Create(ctx context.Context, name string) (File, error)

	// Exists 是否存在
	Exists(ctx context.Context, name string) bool

	// Join 合并目录
	Join(elem ...string) string
}

// NewLocalStorage returns a Storage implementation backed by the local filesystem.
func NewLocalStorage(root string) Storage {
	return &localStorage{root: filepath.Clean(root)}
}

type Row = serde.Row

type localStorage struct {
	root string
}

type localFile struct {
	*os.File
	path string
}

type localFileInfo struct {
	info os.FileInfo
	path string
}

func (s *localStorage) resolvePath(name string) string {
	name = filepath.Clean(name)
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(s.root, name)
}

func (s *localStorage) relPath(path string) string {
	if s.root == "" {
		return path
	}
	rel, err := filepath.Rel(s.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

func (s *localStorage) Open(ctx context.Context, name string) (File, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	path := s.resolvePath(name)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f, path: path}, nil
}

func (s *localStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	path := s.resolvePath(name)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &localFileInfo{info: info, path: path}, nil
}

func (s *localStorage) Glob(ctx context.Context, pattern string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if filepath.IsAbs(pattern) {
		matches, err := filepath.Glob(pattern)
		return matches, err
	}
	matches, err := filepath.Glob(filepath.Join(s.root, pattern))
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		result = append(result, s.relPath(m))
	}
	return result, nil
}

func (s *localStorage) List(ctx context.Context, path string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	resolved := s.resolvePath(path)
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if path == "" || path == "." {
			result = append(result, entry.Name())
		} else {
			result = append(result, filepath.Join(path, entry.Name()))
		}
	}
	return result, nil
}

func (s *localStorage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return os.Mkdir(s.resolvePath(name), perm)
}

func (s *localStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return os.MkdirAll(s.resolvePath(name), perm)
}

func (s *localStorage) Remove(ctx context.Context, name string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return os.Remove(s.resolvePath(name))
}

func (s *localStorage) RemoveAll(ctx context.Context, name string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return os.RemoveAll(s.resolvePath(name))
}

func (s *localStorage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	path := s.resolvePath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func (s *localStorage) Rename(ctx context.Context, oldName, newName string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return os.Rename(s.resolvePath(oldName), s.resolvePath(newName))
}

func (s *localStorage) Create(ctx context.Context, name string) (File, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	path := s.resolvePath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f, path: path}, nil
}

func (s *localStorage) Exists(ctx context.Context, name string) bool {
	if ctx.Err() != nil {
		return false
	}
	_, err := os.Stat(s.resolvePath(name))
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

func (s *localStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (f *localFile) Truncate(ctx context.Context, size int64) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return os.Truncate(f.path, size)
}

func (f *localFile) CopyTo(ctx context.Context, file File) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err := io.Copy(file, f)
	return err
}

func (f *localFile) Stat(ctx context.Context) (FileInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &localFileInfo{info: info, path: f.path}, nil
}

func (f *localFile) Touch(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	now := time.Now()
	return os.Chtimes(f.path, now, now)
}

func (f *localFile) String() string {
	return f.path
}

func (fi *localFileInfo) LastModified() *time.Time {
	mod := fi.info.ModTime()
	return &mod
}

func (fi *localFileInfo) Size() uint64 {
	return uint64(fi.info.Size())
}

func (fi *localFileInfo) Path() string {
	return fi.path
}

func (fi *localFileInfo) Url() string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(fi.path)}
	return u.String()
}

func (fi *localFileInfo) Name() string {
	return fi.info.Name()
}

func (fi *localFileInfo) IsDir() bool {
	return fi.info.IsDir()
}

func (fi *localFileInfo) Metadata() map[string]string {
	return map[string]string{}
}

// GetStorage returns a Storage implementation for the given table.
func GetStorage(tbl *catalog.Table) (Storage, error) {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		location := tbl.Option("location", "")
		if location == "" {
			rawURL := tbl.Option("url", "")
			if rawURL != "" {
				if parsed, err := url.Parse(rawURL); err == nil && parsed.Scheme == "local" {
					location = parsed.Path
				}
			}
			if location == "" {
				return nil, fmt.Errorf("missing location for table %s", tbl.Name)
			}
		}
		return NewLocalStorage(location), nil
	case "s3":
		return newS3Storage(tbl)
	case "ftp":
		return newFTPStorage(tbl)
	case "sftp":
		return newSFTPStorage(tbl)
	case "webdav":
		return newWebDAVStorage(tbl)
	case "git-lfs", "gitlfs":
		return newGitLFSStorage(tbl)
	case "lark":
		return newLarkStorage(tbl)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", storageType)
	}
}

// baseFile provides stub implementations for File interface methods.
// Backend-specific file types embed this and override the methods they support.
type baseFile struct{}

func (f *baseFile) Read(p []byte) (int, error)              { return 0, fmt.Errorf("Read not supported") }
func (f *baseFile) ReadAt(p []byte, off int64) (int, error) { return 0, fmt.Errorf("ReadAt not supported") }
func (f *baseFile) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("Seek not supported")
}
func (f *baseFile) Close() error                             { return fmt.Errorf("Close not supported") }
func (f *baseFile) Write(p []byte) (int, error)              { return 0, fmt.Errorf("Write not supported") }
func (f *baseFile) Truncate(ctx context.Context, size int64) error { return fmt.Errorf("Truncate not supported") }
func (f *baseFile) CopyTo(ctx context.Context, file File) error    { return fmt.Errorf("CopyTo not supported") }
func (f *baseFile) Stat(ctx context.Context) (FileInfo, error)     { return nil, fmt.Errorf("Stat not supported") }
func (f *baseFile) Touch(ctx context.Context) error                 { return fmt.Errorf("Touch not supported") }
func (f *baseFile) String() string                                 { return "" }

// baseFileInfo provides a reusable FileInfo implementation.
type baseFileInfo struct {
	name     string
	path     string
	size     uint64
	modTime  time.Time
	isDir    bool
	metadata map[string]string
}

func (fi *baseFileInfo) LastModified() *time.Time     { return &fi.modTime }
func (fi *baseFileInfo) Size() uint64                  { return fi.size }
func (fi *baseFileInfo) Path() string                  { return fi.path }
func (fi *baseFileInfo) Url() string                   { return fi.path }
func (fi *baseFileInfo) Name() string                  { return fi.name }
func (fi *baseFileInfo) IsDir() bool                   { return fi.isDir }
func (fi *baseFileInfo) Metadata() map[string]string   { return fi.metadata }
