package serde

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/xuri/excelize/v2"
)

type Row map[string]string

type CSVOptions struct {
	Delimiter       rune // field separator (default: ',')
	SkipHeaderLines int  // lines to skip at file start (default: 0)
	Quote           rune // quote character (default: '"')
	Escape          rune // escape character (default: '"')
	IncludeHeader   bool // write column names as first row
}

func NewCSVOptions(tbl *catalog.Table) CSVOptions {
	opts := CSVOptions{
		Delimiter: ',',
		Quote:     '"',
		Escape:    '"',
	}
	if d := tbl.Option("delimiter", ""); d != "" {
		opts.Delimiter = []rune(d)[0]
	}
	if s := tbl.Option("skip_lines", ""); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			opts.SkipHeaderLines = n
		}
	}
	if q := tbl.Option("quote_char", ""); q != "" {
		opts.Quote = []rune(q)[0]
	}
	if e := tbl.Option("escape_char", ""); e != "" {
		opts.Escape = []rune(e)[0]
	}
	if h := tbl.Option("include_header", ""); h != "" {
		opts.IncludeHeader = h == "true" || h == "1" || h == "yes"
	}
	return opts
}

func Decode(ctx context.Context, format string, r io.Reader, columns []catalog.ColumnDef, opts CSVOptions) ([]Row, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	switch strings.ToLower(format) {
	case "csv", "":
		return decodeCSV(r, columns, opts)
	case "json":
		return decodeJSON(r, columns)
	case "excel", "xlsx":
		return decodeExcel(r, columns, opts)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func Encode(ctx context.Context, format string, rows []Row, columns []catalog.ColumnDef, w io.Writer, opts CSVOptions) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch strings.ToLower(format) {
	case "csv", "":
		return encodeCSV(w, rows, columns, opts)
	case "json":
		return encodeJSON(w, rows)
	case "excel", "xlsx":
		return encodeExcel(w, rows, columns, opts)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func decodeCSV(r io.Reader, columns []catalog.ColumnDef, opts CSVOptions) ([]Row, error) {
	reader := csv.NewReader(bufio.NewReader(r))
	reader.Comma = opts.Delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	for i := 0; i < opts.SkipHeaderLines; i++ {
		if _, err := reader.Read(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

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

func decodeJSON(r io.Reader, columns []catalog.ColumnDef) ([]Row, error) {
	scanner := bufio.NewScanner(r)
	var rows []Row
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

func encodeCSV(w io.Writer, rows []Row, columns []catalog.ColumnDef, opts CSVOptions) error {
	writer := csv.NewWriter(w)
	writer.Comma = opts.Delimiter
	defer writer.Flush()

	if opts.IncludeHeader {
		header := make([]string, len(columns))
		for i, col := range columns {
			header[i] = col.Name
		}
		if err := writer.Write(header); err != nil {
			return err
		}
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
	return writer.Error()
}

func encodeJSON(w io.Writer, rows []Row) error {
	encoder := json.NewEncoder(w)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func decodeExcel(r io.Reader, columns []catalog.ColumnDef, opts CSVOptions) ([]Row, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	var result []Row
	for idx, row := range rows {
		if idx < opts.SkipHeaderLines {
			continue
		}
		r := make(Row)
		for i, col := range columns {
			if i < len(row) {
				r[col.Name] = row[i]
			} else {
				r[col.Name] = ""
			}
		}
		result = append(result, r)
	}
	return result, nil
}

func encodeExcel(w io.Writer, rows []Row, columns []catalog.ColumnDef, opts CSVOptions) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	rowOffset := 0
	if opts.IncludeHeader {
		for j, col := range columns {
			cell, _ := excelize.CoordinatesToCellName(j+1, 1)
			f.SetCellValue(sheet, cell, col.Name)
		}
		rowOffset = 1
	}
	for i, row := range rows {
		for j, col := range columns {
			cell, err := excelize.CoordinatesToCellName(j+1, i+1+rowOffset)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell, row[col.Name]); err != nil {
				return err
			}
		}
	}
	return f.Write(w)
}
