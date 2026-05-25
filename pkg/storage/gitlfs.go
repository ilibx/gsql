package storage

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
)

// Git LFS storage constants
const (
	gitLFSPathKey = "git_lfs_path"
	gitLFSRepoKey = "git_lfs_repo"
)

// Git LFS pointer file format
type lfsPointer struct {
	Version   string
	OID       string
	Size      int64
	Algorithm string
}

type gitlfsStorage struct {
	local        Storage
	basePath     string
	lfsCachePath string
}

func newGitLFSStorage(tbl *catalog.Table) (Storage, error) {
	repoPath := tbl.Option(gitLFSRepoKey, "")
	lfsPath := tbl.Option(gitLFSPathKey, "")
	path := tbl.Option("path", "")
	repo := tbl.Option("repo", "")
	if path != "" {
		lfsPath = path
	}
	if repo != "" {
		repoPath = repo
	}
	if repoPath == "" && lfsPath == "" {
		return nil, fmt.Errorf("missing git_lfs_repo or git_lfs_path for table %s", tbl.Name)
	}
	var sourcePath, lfsCachePath string
	if repoPath != "" {
		sourcePath = repoPath
		lfsCachePath = filepath.Join(repoPath, ".git", "lfs", "objects")
	} else {
		sourcePath = lfsPath
		lfsCachePath = lfsPath
	}
	return &gitlfsStorage{
		local:        NewLocalStorage(sourcePath),
		basePath:     sourcePath,
		lfsCachePath: lfsCachePath,
	}, nil
}

func (s *gitlfsStorage) resolvePath(name string) string {
	name = filepath.Clean(name)
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(s.basePath, name)
}

// --- Storage interface delegation ---

func (s *gitlfsStorage) Open(ctx context.Context, name string) (File, error) {
	f, err := s.local.Open(ctx, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if isLFSPointer(data) {
		ptr, err := parseLFSPointer(data)
		if err != nil {
			return nil, err
		}
		lfsPath := filepath.Join(s.lfsCachePath, ptr.OID[:2], ptr.OID[2:])
		resolved, err := os.ReadFile(lfsPath)
		if err != nil {
			return nil, err
		}
		return &gitlfsReadFile{r: bytes.NewReader(resolved)}, nil
	}
	return &gitlfsReadFile{r: bytes.NewReader(data)}, nil
}

func (s *gitlfsStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	return s.local.Stat(ctx, name)
}

func (s *gitlfsStorage) Glob(ctx context.Context, pattern string) ([]string, error) {
	return s.local.Glob(ctx, pattern)
}

func (s *gitlfsStorage) List(ctx context.Context, path string) ([]string, error) {
	return s.local.List(ctx, path)
}

func (s *gitlfsStorage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	return s.local.Mkdir(ctx, name, perm)
}

func (s *gitlfsStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	return s.local.MkdirAll(ctx, name, perm)
}

func (s *gitlfsStorage) Remove(ctx context.Context, name string) error {
	return s.local.Remove(ctx, name)
}

func (s *gitlfsStorage) RemoveAll(ctx context.Context, name string) error {
	return s.local.RemoveAll(ctx, name)
}

func (s *gitlfsStorage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	resolved := s.resolvePath(name)
	if err := os.MkdirAll(filepath.Dir(resolved), perm); err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	oid := hex.EncodeToString(hash[:])
	lfsPath := filepath.Join(s.lfsCachePath, oid[:2], oid[2:])
	if err := os.MkdirAll(filepath.Dir(lfsPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(lfsPath, data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(resolved, []byte(formatLFSPointer(oid, int64(len(data)))), perm)
}

func (s *gitlfsStorage) Rename(ctx context.Context, oldName, newName string) error {
	return s.local.Rename(ctx, oldName, newName)
}

func (s *gitlfsStorage) Create(ctx context.Context, name string) (File, error) {
	resolved := s.resolvePath(name)
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return nil, err
	}
	return &gitlfsWriteFile{
		buf:          &bytes.Buffer{},
		resolvedPath: resolved,
		lfsCachePath: s.lfsCachePath,
	}, nil
}

func (s *gitlfsStorage) Exists(ctx context.Context, name string) bool {
	return s.local.Exists(ctx, name)
}

func (s *gitlfsStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// --- File types ---

type gitlfsReadFile struct {
	baseFile
	r *bytes.Reader
}

func (f *gitlfsReadFile) Read(p []byte) (int, error)     { return f.r.Read(p) }
func (f *gitlfsReadFile) ReadAt(p []byte, off int64) (int, error) { return f.r.ReadAt(p, off) }
func (f *gitlfsReadFile) Seek(offset int64, whence int) (int64, error) { return f.r.Seek(offset, whence) }
func (f *gitlfsReadFile) Close() error                   { return nil }
func (f *gitlfsReadFile) String() string                 { return "" }

type gitlfsWriteFile struct {
	baseFile
	buf          *bytes.Buffer
	resolvedPath string
	lfsCachePath string
	closed       bool
}

func (f *gitlfsWriteFile) Write(p []byte) (int, error) {
	if f.closed {
		return 0, fmt.Errorf("gitlfs: write on closed file")
	}
	return f.buf.Write(p)
}

func (f *gitlfsWriteFile) Read(p []byte) (int, error) {
	return f.buf.Read(p)
}

func (f *gitlfsWriteFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	data := f.buf.Bytes()
	hash := sha256.Sum256(data)
	oid := hex.EncodeToString(hash[:])
	// Store data in LFS object cache
	lfsPath := filepath.Join(f.lfsCachePath, oid[:2], oid[2:])
	if err := os.MkdirAll(filepath.Dir(lfsPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(lfsPath, data, 0o644); err != nil {
		return err
	}
	// Write pointer file at the target path
	return os.WriteFile(f.resolvedPath, []byte(formatLFSPointer(oid, int64(len(data)))), 0o644)
}

func (f *gitlfsWriteFile) String() string {
	return f.resolvedPath
}

// Helper functions

func isLFSPointer(content []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "version ") {
			return true
		}
	}
	return false
}

func parseLFSPointer(content []byte) (*lfsPointer, error) {
	pointer := &lfsPointer{
		Algorithm: "sha256",
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "version":
			pointer.Version = value
		case "oid":
			if subParts := strings.SplitN(value, ":", 2); len(subParts) == 2 {
				pointer.Algorithm = subParts[0]
				pointer.OID = subParts[1]
			}
		case "size":
			fmt.Sscanf(value, "%d", &pointer.Size)
		}
	}

	if pointer.OID == "" {
		return nil, fmt.Errorf("invalid LFS pointer: missing oid")
	}

	return pointer, nil
}

func formatLFSPointer(oid string, size int64) string {
	return fmt.Sprintf("version https://git-lfs.github.com/spec/v1\noid sha256:%s\nsize %d\n", oid, size)
}
