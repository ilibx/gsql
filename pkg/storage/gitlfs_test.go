package storage

import (
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

func TestGitLFSStorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_gitlfs_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "git-lfs",
			"git_lfs_repo": "/path/to/repo",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "git-lfs" {
		t.Errorf("expected storage type 'git-lfs', got %q", storageType)
	}

	if repoPath := tbl.Option("git_lfs_repo", ""); repoPath != "/path/to/repo" {
		t.Errorf("expected git_lfs_repo '/path/to/repo', got %q", repoPath)
	}
}

func TestGitLFSAlternativeStorageType(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_gitlfs_alt",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":       "gitlfs",
			"git_lfs_path":  "/lfs/cache",
			"format":        "csv",
			"file_pattern":  "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "gitlfs" {
		t.Errorf("expected storage type 'gitlfs', got %q", storageType)
	}
}

func TestLFSPointerParsing(t *testing.T) {
	pointerContent := []byte(`version https://git-lfs.github.com/spec/v1
oid sha256:1234567890abcdef
size 12345
`)

	if !isLFSPointer(pointerContent) {
		t.Error("expected isLFSPointer to return true")
	}

	pointer, err := parseLFSPointer(pointerContent)
	if err != nil {
		t.Fatalf("failed to parse LFS pointer: %v", err)
	}

	if pointer.Version != "https://git-lfs.github.com/spec/v1" {
		t.Errorf("expected version 'https://git-lfs.github.com/spec/v1', got %q", pointer.Version)
	}

	if pointer.OID != "1234567890abcdef" {
		t.Errorf("expected OID '1234567890abcdef', got %q", pointer.OID)
	}

	if pointer.Size != 12345 {
		t.Errorf("expected size 12345, got %d", pointer.Size)
	}
}

func TestLFSPointerFormatting(t *testing.T) {
	oid := "1234567890abcdef"
	size := int64(12345)

	pointer := formatLFSPointer(oid, size)

	if pointer != "version https://git-lfs.github.com/spec/v1\noid sha256:1234567890abcdef\nsize 12345\n" {
		t.Errorf("unexpected pointer format: %q", pointer)
	}
}

func TestNotLFSPointer(t *testing.T) {
	normalContent := []byte(`id,name
1,alice
2,bob
`)

	if isLFSPointer(normalContent) {
		t.Error("expected isLFSPointer to return false for normal CSV content")
	}
}
