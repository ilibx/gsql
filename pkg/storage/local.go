package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
