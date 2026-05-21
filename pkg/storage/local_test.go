package storage

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/ilibx/gsql/pkg/catalog"
)

func TestReadLocalTableCSV(t *testing.T) {
    dir := t.TempDir()
    data := "1,alice,alice@example.com\n2,bob,bob@example.com\n"
    filePath := filepath.Join(dir, "users.csv")
    if err := os.WriteFile(filePath, []byte(data), 0o644); err != nil {
        t.Fatalf("write temp csv failed: %v", err)
    }

    tbl := &catalog.Table{
        Name: "users",
        Columns: []catalog.ColumnDef{
            {Name: "id", Type: "INT"},
            {Name: "name", Type: "STRING"},
            {Name: "email", Type: "STRING"},
        },
        WithOptions: map[string]string{
            "storage":     "local",
            "format":      "csv",
            "location":    dir,
            "file_pattern": "*.csv",
        },
    }

    rows, err := ReadTableRows(tbl)
    if err != nil {
        t.Fatalf("read table rows failed: %v", err)
    }
    if len(rows) != 2 {
        t.Fatalf("expected 2 rows, got %d", len(rows))
    }
    if rows[0]["name"] != "alice" {
        t.Errorf("expected alice, got %s", rows[0]["name"])
    }
}
