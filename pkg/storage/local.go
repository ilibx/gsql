package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"
)

func tempFilePattern(outputFile string) string {
	if ext := filepath.Ext(outputFile); ext != "" {
		return "append_*" + ext
	}
	return "append_*"
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
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		return readLocalTable(tbl, filters)
	case "s3":
		return readS3Table(tbl, filters)
	case "ftp":
		return readFTPTable(tbl, filters)
	case "sftp":
		return readSFTPTable(tbl, filters)
	case "webdav":
		return readWebDAVTable(tbl, filters)
	case "git-lfs", "gitlfs":
		return readGitLFSTable(tbl, filters)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func readLocalTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	// Try to get location from either 'location' or URL
	location := tbl.Option("location", "")
	if location == "" {
		// Check if URL is provided
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

	format := strings.ToLower(tbl.Option("format", ""))
	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	pattern := tbl.Option("file_pattern", "")
	if pattern == "" {
		pattern = tbl.Option("file_name", "*")
	}

	var partitions []partitionFile
	var err error
		isPartitioned := len(tbl.PartitionBy) > 0
		if isPartitioned {
			bareFormat := tbl.Option("partition_format", "") == "value"
			partitions, err = resolvePartitionPaths(location, pattern, tbl.PartitionBy, filters, bareFormat)
		if err != nil {
			return nil, err
		}
		if len(partitions) == 0 {
			return nil, fmt.Errorf("no files found for table %s at %s matching partition filters", tbl.Name, location)
		}
	} else {
		var paths []string
		paths, err = resolveLocalPaths(location, pattern)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no files found for table %s at %s", tbl.Name, location)
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
				fileRows, readErr = readCSV(pf.Path, tbl.Columns, csvOpts)
			case "json":
				fileRows, readErr = readJSON(pf.Path, tbl.Columns)
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

func resolvePartitionPaths(location, pattern string, partitionCols []string, filters []PartitionFilter, bareFormatOpt bool) ([]partitionFile, error) {
	filterMap := make(map[string][]partitionFilterCond)
	for _, f := range filters {
		filterMap[f.Column] = append(filterMap[f.Column], partitionFilterCond{Operator: f.Operator, Value: f.Value})
	}

    // Auto-detect partition format: check if first partition level uses col=value or bare directories
    // If partition_format option is explicitly set, use that; otherwise auto-detect.
    useBareFormat := bareFormatOpt
    if !useBareFormat {
        if firstEntries, err := os.ReadDir(location); err == nil && len(partitionCols) > 0 {
            hasColEq := false
            hasBareDir := false
            prefix := partitionCols[0] + "="
            for _, e := range firstEntries {
                if !e.IsDir() {
                    continue
                }
                if strings.HasPrefix(e.Name(), prefix) {
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
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				var val string
				if useBareFormat {
					val = e.Name()
				} else {
					prefix := col + "="
					if !strings.HasPrefix(e.Name(), prefix) {
						continue
					}
					val = strings.TrimPrefix(e.Name(), prefix)
				}

				// Check all filters for this column
				if filters, ok := filterMap[col]; ok {
					if !matchPartitionValue(val, filters) {
						continue
					}
				}
				next = append(next, filepath.Join(dir, e.Name()))
			}
		}
		if len(next) == 0 {
			return nil, nil
		}
		dirs = next
	}

	var files []partitionFile
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
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

func resolveLocalPaths(location, pattern string) ([]string, error) {
	if strings.ContainsAny(location, "*?[]") {
		return filepath.Glob(location)
	}

	info, err := os.Stat(location)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		globPath := filepath.Join(location, pattern)
		matches, err := filepath.Glob(globPath)
		if err != nil {
			return nil, err
		}
		// filter out directories
		var files []string
		for _, m := range matches {
			if fi, statErr := os.Stat(m); statErr == nil && !fi.IsDir() {
				files = append(files, m)
			}
		}
		return files, nil
	}

	return []string{location}, nil
}

func readCSV(path string, columns []catalog.ColumnDef, opts serde.CSVOptions) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return serde.Decode(context.Background(), "csv", file, columns, opts)
}

func readJSON(path string, columns []catalog.ColumnDef) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return serde.Decode(context.Background(), "json", file, columns, serde.CSVOptions{})
}

func WriteRows(tbl *catalog.Table, rows []Row, appendMode bool) error {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		return writeLocalTable(tbl, rows, appendMode)
	case "s3":
		return writeS3Table(tbl, rows, appendMode)
	case "ftp":
		return writeFTPTable(tbl, rows, appendMode)
	case "sftp":
		return writeSFTPTable(tbl, rows, appendMode)
	case "webdav":
		return writeWebDAVTable(tbl, rows, appendMode)
	case "git-lfs", "gitlfs":
		return writeGitLFSTable(tbl, rows, appendMode)
	default:
		return fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func writeLocalTable(tbl *catalog.Table, rows []Row, appendMode bool) error {
	location := tbl.Option("location", "")
	if location == "" {
		return fmt.Errorf("missing location for table %s", tbl.Name)
	}
	outputFile := tbl.Option("file_name", "result.csv")
	format := strings.ToLower(tbl.Option("format", "csv"))

	csvOpts := serde.NewCSVOptions(tbl)

	if len(tbl.PartitionBy) > 0 {
		return writePartitionedTable(location, outputFile, format, tbl, rows, appendMode, csvOpts)
	}
	if err := os.MkdirAll(location, 0o755); err != nil {
		return err
	}

	outputPath := filepath.Join(location, outputFile)
	var existingRows []Row
	if appendMode {
		// Read existing rows if file exists
		if _, err := os.Stat(outputPath); err == nil {
			existingRows, err = readCSV(outputPath, tbl.Columns, csvOpts)
			if err != nil {
				return fmt.Errorf("failed to read existing CSV: %w", err)
			}
			fmt.Printf("-- read %d existing rows from %s\n", len(existingRows), outputPath)
		} else {
			fmt.Printf("-- no existing file %s, creating new\n", outputPath)
		}
		f, err := os.CreateTemp(location, tempFilePattern(outputFile))
		if err != nil {
			return err
		}
		f.Close()
		outputPath = f.Name()
	}

	// Use atomic write: write to temp file, then rename
	tmpPath := outputPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if err := serde.Encode(context.Background(), format, rows, tbl.Columns, file, csvOpts); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	if appendMode {
		// Remove the original file and rename the temp file to the original file
		if err := os.Remove(filepath.Join(location, outputFile)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove original file: %w", err)
		}
		if err := os.Rename(outputPath, filepath.Join(location, outputFile)); err != nil {
			return fmt.Errorf("failed to rename temp file to original: %w", err)
		}
	}
	return nil
}

func writePartitionedTable(location, outputFile, format string, tbl *catalog.Table, rows []Row, appendMode bool, csvOpts serde.CSVOptions) error {
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
		partDir := filepath.Join(location, key)
		if err := os.MkdirAll(partDir, 0o755); err != nil {
			return err
		}
		var outPath string
		if appendMode {
			f, err := os.CreateTemp(partDir, tempFilePattern(outputFile))
			if err != nil {
				return err
			}
			f.Close()
			outPath = f.Name()
		} else {
			outPath = filepath.Join(partDir, outputFile)
		}

		// Atomic write: write to temp, then rename
		tmpPath := outPath + ".tmp"
		file, err := os.Create(tmpPath)
		if err != nil {
			return err
		}
		if err := serde.Encode(context.Background(), format, partRows, tbl.Columns, file, csvOpts); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return err
		}
		if err := file.Close(); err != nil {
			os.Remove(tmpPath)
			return err
		}
		if err := os.Rename(tmpPath, outPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to rename temp file: %w", err)
		}
	}
	return nil
}
