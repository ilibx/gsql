package serde

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

// dummyFormat is a test format that treats each line as a value for the first column.
type dummyFormat struct{}

// Decode implements SerdeFormat.Decode for dummy format.
func (d *dummyFormat) Decode(ctx context.Context, r io.Reader, columns []catalog.ColumnDef, opts SerdeOptions) ([]Row, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	scanner := bufio.NewScanner(r)
	var rows []Row
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		row := make(Row)
		if len(columns) > 0 {
			row[columns[0].Name] = line
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

// Encode implements SerdeFormat.Encode for dummy format.
func (d *dummyFormat) Encode(ctx context.Context, rows []Row, columns []catalog.ColumnDef, w io.Writer, opts SerdeOptions) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, row := range rows {
		if len(columns) > 0 {
			if _, err := fmt.Fprintln(w, row[columns[0].Name]); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintln(w, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// TestDecodeEncodeCSV tests that CSV format still works after the refactor.
func TestDecodeEncodeCSV(t *testing.T) {
	ctx := context.Background()
	// Define a simple schema
	columns := []catalog.ColumnDef{
		{Name: "col1", Type: "text"},
		{Name: "col2", Type: "text"},
	}
	// CSV data
	csvData := "a1,b1\na2,b2\n"
	// Use default CSV options
	opts := SerdeOptions{
		"csv.delimiter": ",",
		"csv.quote_char": `"`}

	rows, err := Decode(ctx, "csv", strings.NewReader(csvData), columns, opts)
	if err != nil {
		t.Fatalf("Decode CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["col1"] != "a1" || rows[0]["col2"] != "b1" {
		t.Fatalf("expected first row a1,b1, got %v", rows[0])
	}
	if rows[1]["col1"] != "a2" || rows[1]["col2"] != "b2" {
		t.Fatalf("expected second row a2,b2, got %v", rows[1])
	}

	// Encode back to CSV
	var buf strings.Builder
	if err := Encode(ctx, "csv", rows, columns, &buf, opts); err != nil {
		t.Fatalf("Encode CSV: %v", err)
	}
	// Note: the encoded CSV may not have a trailing newline, but we can compare trimmed
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(csvData) {
		t.Fatalf("encoded CSV mismatch:\nwant: %q\n got: %q", strings.TrimSpace(csvData), strings.TrimSpace(buf.String()))
	}
}

// TestDecodeEncodeJSON tests that JSON format still works after the refactor.
func TestDecodeEncodeJSON(t *testing.T) {
	ctx := context.Background()
	columns := []catalog.ColumnDef{
		{Name: "col1", Type: "text"},
		{Name: "col2", Type: "text"},
	}
	// JSON lines
	jsonData := `{"col1":"a1","col2":"b1"}
{"col1":"a2","col2":"b2"}
`
	// For JSON, the SerdeOptions are not used, but we still need to pass a valid opts.
	opts := SerdeOptions{}
	rows, err := Decode(ctx, "json", strings.NewReader(jsonData), columns, opts)
	if err != nil {
		t.Fatalf("Decode JSON: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["col1"] != "a1" || rows[0]["col2"] != "b1" {
		t.Fatalf("expected first row a1,b1, got %v", rows[0])
	}
	if rows[1]["col1"] != "a2" || rows[1]["col2"] != "b2" {
		t.Fatalf("expected second row a2,b2, got %v", rows[1])
	}

	// Encode back to JSON
	var buf strings.Builder
	if err := Encode(ctx, "json", rows, columns, &buf, opts); err != nil {
		t.Fatalf("Encode JSON: %v", err)
	}
	// Compare line by line, ignoring trailing newline differences
	wantLines := strings.Split(strings.TrimSpace(jsonData), "\n")
	gotLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(wantLines) != len(gotLines) {
		t.Fatalf("encoded JSON line count mismatch: want %d, got %d", len(wantLines), len(gotLines))
	}
	for i := range wantLines {
		if wantLines[i] != gotLines[i] {
			t.Fatalf("encoded JSON line %d mismatch: want %q, got %q", i, wantLines[i], gotLines[i])
		}
	}
}

// TestRegisterFormat tests that a new format can be registered and used.
func TestRegisterFormat(t *testing.T) {
	// Register the dummy format
	RegisterFormat("dummy", &dummyFormat{})

	ctx := context.Background()
	columns := []catalog.ColumnDef{
		{Name: "value", Type: "text"},
	}
	// Data for dummy format: one value per line
	dummyData := "hello\nworld\n"
	opts := SerdeOptions{}
	rows, err := Decode(ctx, "dummy", strings.NewReader(dummyData), columns, opts)
	if err != nil {
		t.Fatalf("Decode dummy: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["value"] != "hello" {
		t.Fatalf("expected first row value hello, got %v", rows[0])
	}
	if rows[1]["value"] != "world" {
		t.Fatalf("expected second row value world, got %v", rows[1])
	}

	// Encode back to dummy format
	var buf strings.Builder
	if err := Encode(ctx, "dummy", rows, columns, &buf, opts); err != nil {
		t.Fatalf("Encode dummy: %v", err)
	}
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(dummyData) {
		t.Fatalf("encoded dummy mismatch:\nwant: %q\n got: %q", strings.TrimSpace(dummyData), strings.TrimSpace(buf.String()))
	}
}

// TestUnsupportedFormat ensures that an unsupported format returns an error.
func TestUnsupportedFormat(t *testing.T) {
	ctx := context.Background()
	columns := []catalog.ColumnDef{}
	opts := SerdeOptions{}
	_, err := Decode(ctx, "xml", strings.NewReader(""), columns, opts)
	if err == nil {
		t.Fatalf("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
	err = Encode(ctx, "xml", []Row{}, columns, &strings.Builder{}, opts)
	if err == nil {
		t.Fatalf("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}