package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"
)

func tempFileName(ext string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return "append_" + hex.EncodeToString(b) + ext
}

type TableReader interface {
	ReadAll() ([]Row, error)
}

type PartitionFilter struct {
	Column   string
	Operator string
	Value    string
}

type partitionFilterCond struct {
	Operator string
	Value    string
}

// partitionFile describes a data file with its inferred partition values.
type partitionFile struct {
	Path            string
	PartitionValues map[string]string
}

func ReadTableRows(tbl *catalog.Table, filters ...PartitionFilter) ([]Row, error) {
	store, err := GetStorage(tbl)
	if err != nil {
		return nil, err
	}
	return readTable(store, tbl, filters)
}

func readTable(store Storage, tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	format := strings.ToLower(tbl.Option("format", ""))
	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	pattern := tbl.Option("file_pattern", "")
	if pattern == "" {
		pattern = tbl.Option("file_name", "")
	}
	if pattern == "" {
		switch format {
		case "csv":
			pattern = "*.csv"
		case "json":
			pattern = "*.json"
		case "excel", "xlsx":
			pattern = "*.xlsx"
		default:
			pattern = "*"
		}
	}

	var partitions []partitionFile
	var err error
	isPartitioned := len(tbl.PartitionBy) > 0
	if isPartitioned {
		bareFormat := tbl.Option("partition_format", "") == "value"
		partitions, err = resolvePartitionPaths(store, ".", pattern, tbl.PartitionBy, filters, bareFormat)
		if err != nil {
			return nil, err
		}
		if len(partitions) == 0 {
			return nil, fmt.Errorf("no files found for table %s matching partition filters", tbl.Name)
		}
	} else {
		var paths []string
		paths, err = resolveLocalPaths(store, pattern)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no files found for table %s", tbl.Name)
		}
		partitions = make([]partitionFile, len(paths))
		for i, p := range paths {
			partitions[i] = partitionFile{Path: p}
		}
	}

	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(partitions))
	csvOpts := serde.NewCSVOptions(tbl)
	for _, pf := range partitions {
		go func(pf partitionFile) {
			var fileRows []Row
			var readErr error
			switch format {
			case "csv":
				fileRows, readErr = readCSV(store, pf.Path, tbl.Columns, csvOpts)
			case "json":
				fileRows, readErr = readJSON(store, pf.Path, tbl.Columns)
			case "excel", "xlsx":
				fileRows, readErr = readExcel(store, pf.Path, tbl.Columns, csvOpts)
			default:
				resultCh <- fileResult{err: fmt.Errorf("unsupported format %q", format)}
				return
			}
			if readErr != nil {
				resultCh <- fileResult{err: readErr}
				return
			}
			if len(pf.PartitionValues) > 0 {
				for i := range fileRows {
					for k, v := range pf.PartitionValues {
						fileRows[i][k] = v
					}
				}
			}
			resultCh <- fileResult{rows: fileRows}
		}(pf)
	}

	var rows []Row
	for i := 0; i < len(partitions); i++ {
		result := <-resultCh
		if result.err != nil {
			return nil, result.err
		}
		rows = append(rows, result.rows...)
	}
	return rows, nil
}

func resolvePartitionPaths(store Storage, location, pattern string, partitionCols []string, filters []PartitionFilter, bareFormatOpt bool) ([]partitionFile, error) {
	ctx := context.Background()
	filterMap := make(map[string][]partitionFilterCond)
	for _, f := range filters {
		filterMap[f.Column] = append(filterMap[f.Column], partitionFilterCond{Operator: f.Operator, Value: f.Value})
	}

	// Auto-detect partition format: check if first partition level uses col=value or bare directories
	useBareFormat := bareFormatOpt
	if !useBareFormat {
		if firstEntries, err := store.List(ctx, location); err == nil && len(partitionCols) > 0 {
			hasColEq := false
			hasBareDir := false
			prefix := partitionCols[0] + "="
			for _, entry := range firstEntries {
				leaf := filepath.Base(entry)
				info, err := store.Stat(ctx, entry)
				if err != nil || !info.IsDir() {
					continue
				}
				if strings.HasPrefix(leaf, prefix) {
					hasColEq = true
				} else {
					hasBareDir = true
				}
			}
			// Only use bare format if there are no col=value dirs
			useBareFormat = !hasColEq && hasBareDir
		}
	}

	dirs := []string{location}
	for _, col := range partitionCols {
		var next []string
		for _, dir := range dirs {
			entries, err := store.List(ctx, dir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				info, err := store.Stat(ctx, entry)
				if err != nil || !info.IsDir() {
					continue
				}

				// Extract leaf name (the current partition level)
				var leaf string
				if dir == "." || dir == "" {
					leaf = entry
				} else {
					if rel, err := filepath.Rel(dir, entry); err == nil && !strings.Contains(rel, string(filepath.Separator)) {
						leaf = rel
					} else {
						continue
					}
				}

				var val string
				if useBareFormat {
					val = leaf
				} else {
					prefix := col + "="
					if !strings.HasPrefix(leaf, prefix) {
						continue
					}
					val = strings.TrimPrefix(leaf, prefix)
				}

				// Check all filters for this column
				if filters, ok := filterMap[col]; ok {
					if !matchPartitionValue(val, filters) {
						continue
					}
				}
				next = append(next, entry)
			}
		}
		if len(next) == 0 {
			return nil, nil
		}
		dirs = next
	}

	var files []partitionFile
	for _, dir := range dirs {
		matches, err := store.Glob(ctx, store.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			pf := partitionFile{Path: match, PartitionValues: make(map[string]string)}
			// Extract partition values from directory path relative to location
			if rel, err := filepath.Rel(location, filepath.Dir(match)); err == nil {
				parts := strings.Split(rel, string(filepath.Separator))
				for i, col := range partitionCols {
					if i < len(parts) {
						val := parts[i]
						if !useBareFormat {
							// col=value format: strip the "col=" prefix
							if _, after, found := strings.Cut(val, "="); found {
								val = after
							}
						}
						pf.PartitionValues[col] = val
					}
				}
			}
			files = append(files, pf)
		}
	}
	return files, nil
}

func matchPartitionValue(val string, conds []partitionFilterCond) bool {
	for _, c := range conds {
		op := c.Operator
		if op == "" {
			op = "="
		}
		switch op {
		case "=":
			if val != c.Value {
				return false
			}
		case "!=":
			if val == c.Value {
				return false
			}
		case ">":
			if !(val > c.Value) {
				return false
			}
		case "<":
			if !(val < c.Value) {
				return false
			}
		case ">=":
			if !(val >= c.Value) {
				return false
			}
		case "<=":
			if !(val <= c.Value) {
				return false
			}
		}
	}
	return true
}

func resolveLocalPaths(store Storage, pattern string) ([]string, error) {
	ctx := context.Background()

	// If the pattern contains glob characters, use it directly
	if strings.ContainsAny(pattern, "*?[]") {
		matches, err := store.Glob(ctx, pattern)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, m := range matches {
			info, err := store.Stat(ctx, m)
			if err == nil && !info.IsDir() {
				files = append(files, m)
			}
		}
		return files, nil
	}

	// pattern is a specific file name
	info, err := store.Stat(ctx, pattern)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		// pattern is a directory — list all files inside
		entries, err := store.List(ctx, pattern)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, entry := range entries {
			info, err := store.Stat(ctx, entry)
			if err == nil && !info.IsDir() {
				files = append(files, entry)
			}
		}
		return files, nil
	}
	return []string{pattern}, nil
}

func readCSV(store Storage, path string, columns []catalog.ColumnDef, opts serde.CSVOptions) ([]Row, error) {
	ctx := context.Background()
	file, err := store.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return serde.Decode(ctx, "csv", file, columns, opts)
}

func readJSON(store Storage, path string, columns []catalog.ColumnDef) ([]Row, error) {
	ctx := context.Background()
	file, err := store.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return serde.Decode(ctx, "json", file, columns, serde.CSVOptions{})
}

func readExcel(store Storage, path string, columns []catalog.ColumnDef, csvOpts serde.CSVOptions) ([]Row, error) {
	ctx := context.Background()
	file, err := store.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return serde.Decode(ctx, "excel", file, columns, csvOpts)
}

func WriteRows(tbl *catalog.Table, rows []Row, appendMode bool) error {
	store, err := GetStorage(tbl)
	if err != nil {
		return err
	}
	return writeTable(store, tbl, rows, appendMode)
}

func writeTable(store Storage, tbl *catalog.Table, rows []Row, appendMode bool) error {
	ctx := context.Background()
	format := strings.ToLower(tbl.Option("format", "csv"))
	defaultName := "result.csv"
	if format == "excel" || format == "xlsx" {
		defaultName = "result.xlsx"
	}
	outputFile := tbl.Option("file_name", defaultName)

	csvOpts := serde.NewCSVOptions(tbl)

	if len(tbl.PartitionBy) > 0 {
		return writePartitionedTable(store, outputFile, format, tbl, rows, appendMode, csvOpts)
	}
	if err := store.MkdirAll(ctx, ".", 0o755); err != nil {
		return err
	}

	if appendMode {
		// Read existing file and merge with new rows
		existing, err := store.Open(ctx, outputFile)
		if err == nil {
			existingRows, derr := serde.Decode(ctx, format, existing, tbl.Columns, csvOpts)
			existing.Close()
			if derr == nil {
				rows = append(existingRows, rows...)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("append open existing file failed: %w", err)
		}
		// Write combined data directly to output file
		var buf bytes.Buffer
		if err := serde.Encode(ctx, format, rows, tbl.Columns, &buf, csvOpts); err != nil {
			return fmt.Errorf("append encode failed: %w", err)
		}
		return store.WriteFile(ctx, outputFile, buf.Bytes(), 0o644)
	}

	// Atomic write: write to temp file, then rename
	outputPath := outputFile
	tmpPath := outputPath + ".tmp"
	file, err := store.Create(ctx, tmpPath)
	if err != nil {
		return err
	}
	if err := serde.Encode(ctx, format, rows, tbl.Columns, file, csvOpts); err != nil {
		file.Close()
		store.Remove(ctx, tmpPath)
		return err
	}
	if err := file.Close(); err != nil {
		store.Remove(ctx, tmpPath)
		return err
	}

	if err := store.Rename(ctx, tmpPath, outputPath); err != nil {
		store.Remove(ctx, tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	return nil
}

func writePartitionedTable(store Storage, outputFile, format string, tbl *catalog.Table, rows []Row, appendMode bool, csvOpts serde.CSVOptions) error {
	ctx := context.Background()
	partitionCols := tbl.PartitionBy

	partitions := make(map[string][]Row)

	barePartition := tbl.Option("partition_format", "") == "value"

	for _, row := range rows {
		var kvPairs []string
		for _, pc := range partitionCols {
			if barePartition {
				kvPairs = append(kvPairs, fmt.Sprintf("%s", row[pc]))
			} else {
				kvPairs = append(kvPairs, fmt.Sprintf("%s=%s", pc, row[pc]))
			}
		}
		key := strings.Join(kvPairs, "/")
		partitions[key] = append(partitions[key], row)
	}

	for key, partRows := range partitions {
		if err := store.MkdirAll(ctx, key, 0o755); err != nil {
			return err
		}
		partPath := store.Join(key, outputFile)

		if appendMode {
			// Read existing partition file and merge with new rows
			existing, err := store.Open(ctx, partPath)
			if err == nil {
				existingRows, derr := serde.Decode(ctx, format, existing, tbl.Columns, csvOpts)
				existing.Close()
				if derr == nil {
					partRows = append(existingRows, partRows...)
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("append open existing file failed: %w", err)
			}
			var buf bytes.Buffer
			if err := serde.Encode(ctx, format, partRows, tbl.Columns, &buf, csvOpts); err != nil {
				return fmt.Errorf("append encode failed: %w", err)
			}
			if err := store.WriteFile(ctx, partPath, buf.Bytes(), 0o644); err != nil {
				return err
			}
			continue
		}

		// Atomic write: write to temp, then rename
		tmpPath := partPath + ".tmp"
		file, err := store.Create(ctx, tmpPath)
		if err != nil {
			return err
		}
		if err := serde.Encode(ctx, format, partRows, tbl.Columns, file, csvOpts); err != nil {
			file.Close()
			store.Remove(ctx, tmpPath)
			return err
		}
		if err := file.Close(); err != nil {
			store.Remove(ctx, tmpPath)
			return err
		}
		if err := store.Rename(ctx, tmpPath, partPath); err != nil {
			store.Remove(ctx, tmpPath)
			return fmt.Errorf("failed to rename temp file: %w", err)
		}
	}
	return nil
}
