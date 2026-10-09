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

type SerdeOptions struct {
	Delimiter       rune   // field separator (default: ',')
	SkipHeaderLines int    // lines to skip at file start (default: 0)
	Quote           rune   // quote character (default: '"')
	Escape          rune   // escape character (default: '"')
	IncludeHeader   bool   // write column names as first row
	SheetName       string // excel sheet name (default: first sheet on read, "Sheet1" on write)
}

func NewSerdeOptions(tbl *catalog.Table) SerdeOptions {
	opts := SerdeOptions{
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
	if s := tbl.Option("sheet", ""); s != "" {
		opts.SheetName = s
	}
	return opts
}

// Decoder is a streaming row decoder. Next returns io.EOF when the input
// is exhausted. Formats that cannot be streamed (excel) are buffered on
// creation and replayed from memory.
type Decoder interface {
	Next() (Row, error)
	Close() error
}

// NewDecoder creates a streaming decoder for the given format.
func NewDecoder(format string, r io.Reader, columns []catalog.ColumnDef, opts SerdeOptions) (Decoder, error) {
	switch strings.ToLower(format) {
	case "csv", "":
		return newCSVDecoder(r, columns, opts), nil
	case "json":
		return newJSONDecoder(r, columns), nil
	case "excel", "xlsx":
		rows, err := decodeExcel(r, columns, opts)
		if err != nil {
			return nil, err
		}
		return &sliceDecoder{rows: rows}, nil
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func Decode(ctx context.Context, format string, r io.Reader, columns []catalog.ColumnDef, opts SerdeOptions) ([]Row, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	dec, err := NewDecoder(format, r, columns, opts)
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	var rows []Row
	for {
		row, err := dec.Next()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
}

func Encode(ctx context.Context, format string, rows []Row, columns []catalog.ColumnDef, w io.Writer, opts SerdeOptions) error {
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

type csvDecoder struct {
	reader  *csv.Reader
	columns []catalog.ColumnDef
	skip    int
	done    bool
}

func newCSVDecoder(r io.Reader, columns []catalog.ColumnDef, opts SerdeOptions) *csvDecoder {
	reader := csv.NewReader(bufio.NewReader(r))
	reader.Comma = opts.Delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	return &csvDecoder{reader: reader, columns: columns, skip: opts.SkipHeaderLines}
}

func (d *csvDecoder) Next() (Row, error) {
	if d.done {
		return nil, io.EOF
	}
	for d.skip > 0 {
		d.skip--
		if _, err := d.reader.Read(); err != nil {
			if err == io.EOF {
				d.done = true
				return nil, io.EOF
			}
			return nil, err
		}
	}
	record, err := d.reader.Read()
	if err != nil {
		if err == io.EOF {
			d.done = true
			return nil, io.EOF
		}
		return nil, err
	}
	row := make(Row)
	for i, col := range d.columns {
		if i < len(record) {
			row[col.Name] = record[i]
		}
	}
	return row, nil
}

func (d *csvDecoder) Close() error { return nil }

type jsonDecoder struct {
	scanner *bufio.Scanner
	columns []catalog.ColumnDef
}

func newJSONDecoder(r io.Reader, columns []catalog.ColumnDef) *jsonDecoder {
	return &jsonDecoder{scanner: bufio.NewScanner(r), columns: columns}
}

func (d *jsonDecoder) Next() (Row, error) {
	for d.scanner.Scan() {
		line := strings.TrimSpace(d.scanner.Text())
		if line == "" {
			continue
		}
		rowMap := make(map[string]any)
		if err := json.Unmarshal([]byte(line), &rowMap); err != nil {
			return nil, err
		}
		row := make(Row)
		for _, col := range d.columns {
			if value, ok := rowMap[col.Name]; ok {
				row[col.Name] = fmt.Sprint(value)
			} else {
				row[col.Name] = ""
			}
		}
		return row, nil
	}
	if err := d.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func (d *jsonDecoder) Close() error { return nil }

// sliceDecoder replays a fully buffered row set; used by formats that
// cannot be streamed (excel).
type sliceDecoder struct {
	rows []Row
	pos  int
}

func (d *sliceDecoder) Next() (Row, error) {
	if d.pos >= len(d.rows) {
		return nil, io.EOF
	}
	row := d.rows[d.pos]
	d.pos++
	return row, nil
}

func (d *sliceDecoder) Close() error { return nil }

func encodeCSV(w io.Writer, rows []Row, columns []catalog.ColumnDef, opts SerdeOptions) error {
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

func decodeExcel(r io.Reader, columns []catalog.ColumnDef, opts SerdeOptions) ([]Row, error) {
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
	if opts.SheetName != "" {
		sheet = opts.SheetName
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	var result []Row
	for idx, row := range rows {
		if idx < opts.SkipHeaderLines {
			continue
		}
		if opts.IncludeHeader && idx == opts.SkipHeaderLines {
			continue // skip header row for xlsx read
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

func encodeExcel(w io.Writer, rows []Row, columns []catalog.ColumnDef, opts SerdeOptions) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	if opts.SheetName != "" {
		f.SetSheetName("Sheet1", opts.SheetName)
		sheet = opts.SheetName
	}
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
