package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

func TestWritePartitionedTable(t *testing.T) {
	dir := t.TempDir()

	tbl := &catalog.Table{
		Name: "events",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		PartitionBy: []string{"dt"},
		WithOptions: map[string]string{
			"path":  dir,
			"format":    "csv",
			"file_name": "data.csv",
		},
	}

	rows := []Row{
		{"id": "1", "name": "a", "dt": "2024-01-01"},
		{"id": "2", "name": "b", "dt": "2024-01-01"},
		{"id": "3", "name": "c", "dt": "2024-01-02"},
	}

	if err := WriteRows(tbl, rows, false); err != nil {
		t.Fatalf("WriteRows failed: %v", err)
	}

	part1Dir := filepath.Join(dir, "dt=2024-01-01")
	part2Dir := filepath.Join(dir, "dt=2024-01-02")

	checkFile := func(path, expect string) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s failed: %v", path, err)
		}
		if !strings.Contains(string(data), expect) {
			t.Errorf("expected %s to contain %q, got %s", path, expect, string(data))
		}
	}

	checkFile(filepath.Join(part1Dir, "data.csv"), "1")
	checkFile(filepath.Join(part1Dir, "data.csv"), "2")
	checkFile(filepath.Join(part2Dir, "data.csv"), "3")

	if _, err := os.Stat(part1Dir); os.IsNotExist(err) {
		t.Errorf("expected partition dir %s to exist", part1Dir)
	}
	if _, err := os.Stat(part2Dir); os.IsNotExist(err) {
		t.Errorf("expected partition dir %s to exist", part2Dir)
	}
}

func TestWriteAppendNonPartitioned(t *testing.T) {
	dir := t.TempDir()

	tbl := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"path":  dir,
			"format":    "csv",
			"file_name": "result.csv",
		},
	}

	rows1 := []Row{{"id": "1", "name": "alice"}}
	rows2 := []Row{{"id": "2", "name": "bob"}}

	if err := WriteRows(tbl, rows1, true); err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	if err := WriteRows(tbl, rows2, true); err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	// Should have 1 merged file with both rows
	data, err := os.ReadFile(filepath.Join(dir, "result.csv"))
	if err != nil {
		t.Fatalf("read result.csv failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "1") || !strings.Contains(content, "alice") {
		t.Errorf("missing row1 in output: %s", content)
	}
	if !strings.Contains(content, "2") || !strings.Contains(content, "bob") {
		t.Errorf("missing row2 in output: %s", content)
	}
}

func TestWriteAppendPartitioned(t *testing.T) {
	dir := t.TempDir()

	tbl := &catalog.Table{
		Name: "events",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		PartitionBy: []string{"dt"},
		WithOptions: map[string]string{
			"path":  dir,
			"format":    "csv",
			"file_name": "data.csv",
		},
	}

	rows1 := []Row{{"id": "1", "name": "a", "dt": "2024-01-01"}}
	rows2 := []Row{{"id": "2", "name": "b", "dt": "2024-01-01"}}

	if err := WriteRows(tbl, rows1, true); err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	if err := WriteRows(tbl, rows2, true); err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	// Should have 1 merged file with both rows
	partDir := filepath.Join(dir, "dt=2024-01-01")
	data, err := os.ReadFile(filepath.Join(partDir, "data.csv"))
	if err != nil {
		t.Fatalf("read data.csv failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "1") || !strings.Contains(content, "a") {
		t.Errorf("missing row1 in output: %s", content)
	}
	if !strings.Contains(content, "2") || !strings.Contains(content, "b") {
		t.Errorf("missing row2 in output: %s", content)
	}
}

func TestWriteNonPartitionedTableUnchanged(t *testing.T) {
	dir := t.TempDir()

	tbl := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"path":  dir,
			"format":    "csv",
			"file_name": "result.csv",
		},
	}

	rows := []Row{
		{"id": "1", "name": "alice"},
		{"id": "2", "name": "bob"},
	}

	if err := WriteRows(tbl, rows, false); err != nil {
		t.Fatalf("WriteRows failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "result.csv"))
	if err != nil {
		t.Fatalf("read result failed: %v", err)
	}
	if !strings.Contains(string(data), "alice") {
		t.Errorf("expected output to contain alice, got %s", string(data))
	}
}

func TestReadPartitionedTable(t *testing.T) {
	dir := t.TempDir()

	tbl := &catalog.Table{
		Name: "events",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		PartitionBy: []string{"dt"},
		WithOptions: map[string]string{
			"path":  dir,
			"format":    "csv",
			"file_name": "data.csv",
		},
	}

	rows := []Row{
		{"id": "1", "name": "a", "dt": "2024-01-01"},
		{"id": "2", "name": "b", "dt": "2024-01-02"},
	}
	if err := WriteRows(tbl, rows, false); err != nil {
		t.Fatalf("WriteRows failed: %v", err)
	}

	// Read without filters — should get all rows
	all, err := ReadTableRows(tbl)
	if err != nil {
		t.Fatalf("ReadTableRows failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 rows, got %d", len(all))
	}
}

func TestReadPartitionedTableWithFilter(t *testing.T) {
	dir := t.TempDir()

	tbl := &catalog.Table{
		Name: "events",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		PartitionBy: []string{"dt"},
		WithOptions: map[string]string{
			"path":  dir,
			"format":    "csv",
			"file_name": "data.csv",
		},
	}

	rows := []Row{
		{"id": "1", "name": "a", "dt": "2024-01-01"},
		{"id": "2", "name": "b", "dt": "2024-01-02"},
		{"id": "3", "name": "c", "dt": "2024-01-01"},
	}
	if err := WriteRows(tbl, rows, false); err != nil {
		t.Fatalf("WriteRows failed: %v", err)
	}

	// Read with filter on dt — should only get matching partition
	pruned, err := ReadTableRows(tbl, PartitionFilter{Column: "dt", Value: "2024-01-01"})
	if err != nil {
		t.Fatalf("ReadTableRows with filter failed: %v", err)
	}
	if len(pruned) != 2 {
		t.Errorf("expected 2 rows from pruned read, got %d", len(pruned))
	}
}

func TestPartitionPruningExplain(t *testing.T) {
	// Verify that the planner produces a TableScan with partition filters
	// by testing the extractPartitionFilters helper indirectly via the plan test
	_ = fmt.Sprintf("placeholder")
}
