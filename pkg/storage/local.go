package storage

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
)

func tempFilePattern(outputFile string) string {
	if ext := filepath.Ext(outputFile); ext != "" {
		return "append_*" + ext
	}
	return "append_*"
}

type Row map[string]string

type TableReader interface {
	ReadAll() ([]Row, error)
}

type PartitionFilter struct {
	Column string
	Value  string
}

func ReadTableRows(tbl *catalog.Table, filters ...PartitionFilter) ([]Row, error) {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		return readLocalTable(tbl, filters)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func readLocalTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	location := tbl.Option("location", "")
	if location == "" {
		return nil, fmt.Errorf("missing location for table %s", tbl.Name)
	}

	format := strings.ToLower(tbl.Option("format", ""))
	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	pattern := tbl.Option("file_pattern", "*")

	var paths []string
	var err error
	if len(tbl.PartitionBy) > 0 {
		paths, err = resolvePartitionPaths(location, pattern, tbl.PartitionBy, filters)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no files found for table %s at %s matching partition filters", tbl.Name, location)
		}
	} else {
		paths, err = resolveLocalPaths(location, pattern)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no files found for table %s at %s", tbl.Name, location)
		}
	}

	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(paths))
	for _, path := range paths {
		go func(path string) {
			switch format {
			case "csv":
				fileRows, err := readCSV(path, tbl.Columns)
				resultCh <- fileResult{rows: fileRows, err: err}
			case "json":
				fileRows, err := readJSON(path, tbl.Columns)
				resultCh <- fileResult{rows: fileRows, err: err}
			default:
				resultCh <- fileResult{err: fmt.Errorf("unsupported format %q", format)}
			}
		}(path)
	}

	var rows []Row
	for i := 0; i < len(paths); i++ {
		result := <-resultCh
		if result.err != nil {
			return nil, result.err
		}
		rows = append(rows, result.rows...)
	}
	return rows, nil
}

func resolvePartitionPaths(location, pattern string, partitionCols []string, filters []PartitionFilter) ([]string, error) {
	filterMap := make(map[string]string)
	for _, f := range filters {
		filterMap[f.Column] = f.Value
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
			prefix := col + "="
			for _, e := range entries {
				if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
					continue
				}
				val := strings.TrimPrefix(e.Name(), prefix)
				if filterVal, ok := filterMap[col]; ok && val != filterVal {
					continue
				}
				next = append(next, filepath.Join(dir, e.Name()))
			}
		}
		if len(next) == 0 {
			return nil, nil
		}
		dirs = next
	}

	// At the last partition level, list files matching pattern
	var files []string
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	return files, nil
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
		return filepath.Glob(globPath)
	}

	return []string{location}, nil
}

func readCSV(path string, columns []catalog.ColumnDef) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.TrimLeadingSpace = true

	var rows []Row
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		row := make(Row)
		for i, col := range columns {
			var value string
			if i < len(record) {
				value = record[i]
			}
			row[col.Name] = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readJSON(path string, columns []catalog.ColumnDef) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var rows []Row
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rowMap := make(map[string]any)
		if err := json.Unmarshal([]byte(line), &rowMap); err != nil {
			return nil, err
		}
		row := make(Row)
		for _, col := range columns {
			if value, ok := rowMap[col.Name]; ok {
				row[col.Name] = fmt.Sprint(value)
			} else {
				row[col.Name] = ""
			}
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func WriteRows(tbl *catalog.Table, rows []Row, appendMode bool) error {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		return writeLocalTable(tbl, rows, appendMode)
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

	if len(tbl.PartitionBy) > 0 {
		return writePartitionedTable(location, outputFile, format, tbl, rows, appendMode)
	}
	if err := os.MkdirAll(location, 0o755); err != nil {
		return err
	}

	outputPath := filepath.Join(location, outputFile)
	if appendMode {
		f, err := os.CreateTemp(location, tempFilePattern(outputFile))
		if err != nil {
			return err
		}
		f.Close()
		outputPath = f.Name()
	}

	switch format {
	case "csv":
		return writeCSV(outputPath, tbl.Columns, rows)
	case "json":
		return writeJSON(outputPath, rows)
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}
}

func writePartitionedTable(location, outputFile, format string, tbl *catalog.Table, rows []Row, appendMode bool) error {
	partitionCols := tbl.PartitionBy

	partitions := make(map[string][]Row)

	for _, row := range rows {
		var kvPairs []string
		for _, pc := range partitionCols {
			kvPairs = append(kvPairs, fmt.Sprintf("%s=%s", pc, row[pc]))
		}
		key := strings.Join(kvPairs, "/")
		partitions[key] = append(partitions[key], row)
	}

	for key, partRows := range partitions {
		partDir := filepath.Join(location, key)
		if err := os.MkdirAll(partDir, 0o755); err != nil {
			return err
		}
		outPath := filepath.Join(partDir, outputFile)
		if appendMode {
			f, err := os.CreateTemp(partDir, tempFilePattern(outputFile))
			if err != nil {
				return err
			}
			f.Close()
			outPath = f.Name()
		}
		switch format {
		case "csv":
			if err := writeCSV(outPath, tbl.Columns, partRows); err != nil {
				return err
			}
		case "json":
			if err := writeJSON(outPath, partRows); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported write format %q", format)
		}
	}
	return nil
}

func writeCSV(path string, columns []catalog.ColumnDef, rows []Row) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			record[i] = row[col.Name]
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, rows []Row) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
