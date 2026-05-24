package storage

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStorageBasicFileOps(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStorage(dir)
	ctx := context.Background()

	const fileName = "subdir/test.txt"
	data := []byte("hello world")

	if err := store.WriteFile(ctx, fileName, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if !store.Exists(ctx, fileName) {
		t.Fatalf("expected file to exist")
	}

	info, err := store.Stat(ctx, fileName)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Fatalf("unexpected file name: %s", info.Name())
	}
	if info.Size() != uint64(len(data)) {
		t.Fatalf("unexpected size: %d", info.Size())
	}
	if info.IsDir() {
		t.Fatalf("expected file, got dir")
	}

	f, err := store.Open(ctx, fileName)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	read, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(read) != string(data) {
		t.Fatalf("unexpected content: %s", string(read))
	}

	if err := store.Rename(ctx, fileName, "subdir/renamed.txt"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if store.Exists(ctx, fileName) {
		t.Fatalf("old file still exists after rename")
	}
	if !store.Exists(ctx, "subdir/renamed.txt") {
		t.Fatalf("renamed file should exist")
	}

	matches, err := store.Glob(ctx, "subdir/*.txt")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 1 || matches[0] != filepath.ToSlash("subdir/renamed.txt") {
		t.Fatalf("unexpected glob result: %v", matches)
	}

	entries, err := store.List(ctx, "subdir")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 || entries[0] != filepath.Join("subdir", "renamed.txt") {
		t.Fatalf("unexpected list result: %v", entries)
	}

	file, err := store.Open(ctx, "subdir/renamed.txt")
	if err != nil {
		t.Fatalf("Open renamed file failed: %v", err)
	}
	defer file.Close()

	if err := file.Touch(ctx); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}
	if info2, err := file.Stat(ctx); err != nil {
		t.Fatalf("file Stat failed: %v", err)
	} else if info2.LastModified() == nil || time.Since(*info2.LastModified()) > time.Minute {
		t.Fatalf("unexpected last modified time: %v", info2.LastModified())
	}

	if err := file.Truncate(ctx, 5); err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	truncated, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read after truncate failed: %v", err)
	}
	if string(truncated) != "hello" {
		t.Fatalf("unexpected truncated content: %q", string(truncated))
	}

	copyDst, err := store.Create(ctx, "subdir/copy.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek start failed: %v", err)
	}
	if err := file.CopyTo(ctx, copyDst); err != nil {
		t.Fatalf("CopyTo failed: %v", err)
	}
	copyDst.Close()

	if !store.Exists(ctx, "subdir/copy.txt") {
		t.Fatalf("copy target should exist")
	}

	if err := store.Remove(ctx, "subdir/renamed.txt"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if err := store.RemoveAll(ctx, "subdir"); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}
	if store.Exists(ctx, "subdir/copy.txt") {
		t.Fatalf("expected subdir copy file removed")
	}
}

func TestLocalStorageMkdirAll(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStorage(dir)
	ctx := context.Background()

	nested := "nested/a/b"
	if err := store.MkdirAll(ctx, nested, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	if !store.Exists(ctx, nested) {
		t.Fatalf("expected nested directory to exist")
	}
}
