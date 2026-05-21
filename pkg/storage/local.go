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

type Row map[string]string

type TableReader interface {
	ReadAll() ([]Row, error)
}

func ReadTableRows(tbl *catalog.Table) ([]Row, error) {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		return readLocalTable(tbl)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func readLocalTable(tbl *catalog.Table) ([]Row, error) {
	location := tbl.Option("location", "")
	if location == "" {
		return nil, fmt.Errorf("missing location for table %s", tbl.Name)
	}

	pattern := tbl.Option("file_pattern", "*")
	paths, err := resolveLocalPaths(location, pattern)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no files found for table %s at %s", tbl.Name, location)
	}

	format := strings.ToLower(tbl.Option("format", ""))
	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
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

func WriteRows(tbl *catalog.Table, rows []Row) error {
	storageType := strings.ToLower(tbl.Option("storage", "local"))
	switch storageType {
	case "", "local":
		return writeLocalTable(tbl, rows)
	default:
		return fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func writeLocalTable(tbl *catalog.Table, rows []Row) error {
	location := tbl.Option("location", "")
	if location == "" {
		return fmt.Errorf("missing location for table %s", tbl.Name)
	}
	outputFile := tbl.Option("file_name", "result.csv")
	outputPath := filepath.Join(location, outputFile)

	if err := os.MkdirAll(location, 0o755); err != nil {
		return err
	}

	format := strings.ToLower(tbl.Option("format", "csv"))
	switch format {
	case "csv":
		return writeCSV(outputPath, tbl.Columns, rows)
	case "json":
		return writeJSON(outputPath, rows)
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}
}

func writeCSV(path string, columns []catalog.ColumnDef, rows []Row) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := make([]string, len(columns))
	for i, col := range columns {
		header[i] = col.Name
	}
	if err := writer.Write(header); err != nil {
		return err
	}

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
