package storage

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
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

	Truncate(ctx context.Context, size int64) error
	CopyTo(ctx context.Context, file File) error
	Stat(ctx context.Context) (FileInfo, error)
	Touch(ctx context.Context) error
}

type FileInfo interface {
	LastModified() *time.Time
	Size() uint64
	Path() string
	Url() string
	Name() string
	IsDir() bool
	Metadata() map[string]string
}

type Storage interface {
	Open(ctx context.Context, name string) (File, error)
	Stat(ctx context.Context, name string) (FileInfo, error)
	Glob(ctx context.Context, pattern string) ([]string, error)
	List(ctx context.Context, path string) ([]string, error)
	Mkdir(ctx context.Context, name string, perm fs.FileMode) error
	MkdirAll(ctx context.Context, name string, perm fs.FileMode) error
	Remove(ctx context.Context, name string) error
	RemoveAll(ctx context.Context, name string) error
	WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error
	Rename(ctx context.Context, oldName, newName string) error
	Create(ctx context.Context, name string) (File, error)
	Exists(ctx context.Context, name string) bool
	Join(elem ...string) string
}

func NewLocalStorage(root string) Storage {
	return &localStorage{root: filepath.Clean(root)}
}

type Row = serde.Row

func GetStorage(tbl *catalog.Table) (Storage, error) {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		path := tbl.Option("path", "")
		if path == "" {
			rawURL := tbl.Option("url", "")
			if rawURL != "" {
				if parsed, err := url.Parse(rawURL); err == nil && parsed.Scheme == "local" {
					path = parsed.Path
				}
			}
			if path == "" {
				return nil, fmt.Errorf("missing path for table %s", tbl.Name)
			}
		}
		return NewLocalStorage(path), nil
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

type baseFileInfo struct {
	name     string
	path     string
	size     uint64
	modTime  time.Time
	isDir    bool
	metadata map[string]string
}

func (fi *baseFileInfo) LastModified() *time.Time   { return &fi.modTime }
func (fi *baseFileInfo) Size() uint64                { return fi.size }
func (fi *baseFileInfo) Path() string                { return fi.path }
func (fi *baseFileInfo) Url() string                 { return fi.path }
func (fi *baseFileInfo) Name() string                { return fi.name }
func (fi *baseFileInfo) IsDir() bool                 { return fi.isDir }
func (fi *baseFileInfo) Metadata() map[string]string { return fi.metadata }
