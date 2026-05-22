package storage

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// readGitLFSTable reads data from Git LFS
func readGitLFSTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	repoPath := tbl.Option(gitLFSRepoKey, "")
	lfsPath := tbl.Option(gitLFSPathKey, "")

	if repoPath == "" && lfsPath == "" {
		return nil, fmt.Errorf("missing git_lfs_repo or git_lfs_path for table %s", tbl.Name)
	}

	pattern := tbl.Option("file_pattern", "*")
	format := strings.ToLower(tbl.Option("format", ""))

	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	// Determine LFS object storage path
	var lfsCachePath string
	if repoPath != "" {
		lfsCachePath = filepath.Join(repoPath, ".git", "lfs", "objects")
	} else {
		lfsCachePath = lfsPath
	}

	// List files that match the pattern
	var fileNames []string
	if entries, err := os.ReadDir(lfsPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if matchPattern(entry.Name(), pattern) {
				fileNames = append(fileNames, entry.Name())
			}
		}
	}

	if len(fileNames) == 0 {
		return nil, fmt.Errorf("no files found in Git LFS at %s", lfsPath)
	}

	// Read files in parallel
	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(fileNames))
	for _, fileName := range fileNames {
		go func(fileName string) {
			filePath := filepath.Join(lfsPath, fileName)
			data, err := readGitLFSFile(filePath, lfsCachePath)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to read Git LFS file %s: %w", fileName, err)}
				return
			}

			switch format {
			case "csv":
				rows, err := readCSVFromBytes(data, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			case "json":
				rows, err := readJSONFromBytes(data, tbl.Columns)
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

// writeGitLFSTable writes data to Git LFS
func writeGitLFSTable(tbl *catalog.Table, rows []Row, appendMode bool) error {
	repoPath := tbl.Option(gitLFSRepoKey, "")
	lfsPath := tbl.Option(gitLFSPathKey, "")

	if repoPath == "" && lfsPath == "" {
		return fmt.Errorf("missing git_lfs_repo or git_lfs_path for table %s", tbl.Name)
	}

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

	// Determine LFS object storage path and working directory
	var lfsCachePath, workDir string
	if repoPath != "" {
		lfsCachePath = filepath.Join(repoPath, ".git", "lfs", "objects")
		workDir = repoPath
	} else {
		lfsCachePath = lfsPath
		workDir = filepath.Dir(lfsPath)
	}

	// Ensure directories exist
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}

	// Write Git LFS file
	outPath := filepath.Join(workDir, fileName)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", outPath, err)
	}

	// Store actual data in LFS cache
	hash := sha256.Sum256(data)
	oid := hex.EncodeToString(hash[:])
	lfsObjPath := filepath.Join(lfsCachePath, oid[:2], oid[2:])

	if err := os.MkdirAll(filepath.Dir(lfsObjPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(lfsObjPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write LFS object %s: %w", lfsObjPath, err)
	}

	// Write pointer file
	pointer := formatLFSPointer(oid, int64(len(data)))
	pointerPath := filepath.Join(workDir, fileName+".pointer")
	if err := os.WriteFile(pointerPath, []byte(pointer), 0o644); err != nil {
		return fmt.Errorf("failed to write LFS pointer %s: %w", pointerPath, err)
	}

	return nil
}

// Helper functions

func readGitLFSFile(filePath, lfsCachePath string) ([]byte, error) {
	// Try to read as LFS pointer file first
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Check if it's an LFS pointer file
	if isLFSPointer(content) {
		pointer, err := parseLFSPointer(content)
		if err != nil {
			return nil, err
		}

		// Read from LFS cache
		lfsObjPath := filepath.Join(lfsCachePath, pointer.OID[:2], pointer.OID[2:])
		return os.ReadFile(lfsObjPath)
	}

	// If not a pointer file, assume it's the actual data
	return content, nil
}

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
