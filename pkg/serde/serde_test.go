package serde

import (
	"context"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

func TestDecodeEncodeCSV(t *testing.T) {
	ctx := context.Background()
	columns := []catalog.ColumnDef{
		{Name: "col1", Type: "text"},
		{Name: "col2", Type: "text"},
	}
	csvData := "a1,b1\na2,b2\n"
	opts := SerdeOptions{
		Delimiter: ',',
		Quote:     '"',
	}

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

	var buf strings.Builder
	if err := Encode(ctx, "csv", rows, columns, &buf, opts); err != nil {
		t.Fatalf("Encode CSV: %v", err)
	}
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(csvData) {
		t.Fatalf("encoded CSV mismatch:\nwant: %q\n got: %q", strings.TrimSpace(csvData), strings.TrimSpace(buf.String()))
	}
}

func TestDecodeEncodeJSON(t *testing.T) {
	ctx := context.Background()
	columns := []catalog.ColumnDef{
		{Name: "col1", Type: "text"},
		{Name: "col2", Type: "text"},
	}
	jsonData := `{"col1":"a1","col2":"b1"}
{"col1":"a2","col2":"b2"}
`
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

	var buf strings.Builder
	if err := Encode(ctx, "json", rows, columns, &buf, opts); err != nil {
		t.Fatalf("Encode JSON: %v", err)
	}
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
