package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/parser"
)

// A declared DEFAULT fills CSV rows that do not provide the column; a
// present-but-empty cell stays empty (explicit value, not a missing one).
func TestColumnDefaultCSVMissingFields(t *testing.T) {
	dir := t.TempDir()
	csv := "1,alice,alice@example.com,\n2,bob\n"
	if err := os.WriteFile(filepath.Join(dir, "data.csv"), []byte(csv), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)
	table := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "email", Type: "STRING", Default: "none@default", HasDefault: true},
			{Name: "city", Type: "STRING", Default: "unknown", HasDefault: true},
		},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "data.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := eng.ExecuteSelect(&parser.SelectQuery{
		Columns: []string{"id", "name", "email", "city"},
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// row 1 provides email but an explicit empty city cell
	if rows[0]["email"] != "alice@example.com" {
		t.Errorf("expected provided email kept, got %q", rows[0]["email"])
	}
	if rows[0]["city"] != "" {
		t.Errorf("expected explicit empty city to stay empty, got %q", rows[0]["city"])
	}
	// row 2 is short: email and city come from their DEFAULTs
	if rows[1]["email"] != "none@default" {
		t.Errorf("expected email default, got %q", rows[1]["email"])
	}
	if rows[1]["city"] != "unknown" {
		t.Errorf("expected city default, got %q", rows[1]["city"])
	}
}

// INSERT ... VALUES accepts rows shorter than the table when every missing
// column declares a DEFAULT; too many values or a missing value without a
// DEFAULT is still a column count mismatch.
func TestColumnDefaultInsertValues(t *testing.T) {
	dir := t.TempDir()
	cat := catalog.NewCatalog()
	eng := NewEngine(cat)
	table := &catalog.Table{
		Name: "metrics",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "tag", Type: "STRING"},
			{Name: "score", Type: "INT", Default: "7", HasDefault: true},
		},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_name": "out.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	runSQL(t, eng, `INSERT INTO metrics VALUES (1, 'y');`)
	data, err := os.ReadFile(filepath.Join(dir, "out.csv"))
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	if !strings.Contains(string(data), "1,y,7") {
		t.Errorf("expected short VALUES row padded with score default, got:\n%s", string(data))
	}

	// missing value for a column without DEFAULT -> mismatch error
	err = runSQLError(t, eng, `INSERT INTO metrics VALUES (2);`)
	if err == nil || !strings.Contains(err.Error(), "no DEFAULT") {
		t.Errorf("expected 'no DEFAULT' mismatch error, got %v", err)
	}
	// more values than columns -> mismatch error
	err = runSQLError(t, eng, `INSERT INTO metrics VALUES (3, 'a', 1, 2);`)
	if err == nil || !strings.Contains(err.Error(), "column count mismatch") {
		t.Errorf("expected column count mismatch error, got %v", err)
	}
}

// INSERT ... SELECT with fewer SELECT columns than target columns fills the
// uncovered target columns with their DEFAULTs.
func TestColumnDefaultInsertSelect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "src.csv"), []byte("1\n2\n"), 0o644); err != nil {
		t.Fatalf("write src csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)
	src := &catalog.Table{
		Name:    "src_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "src.csv",
		},
	}
	tgt := &catalog.Table{
		Name: "tgt_t",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "tag", Type: "STRING", Default: "auto", HasDefault: true},
		},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_name": "tgt.csv",
		},
	}
	for _, tbl := range []*catalog.Table{src, tgt} {
		if err := cat.CreateTable(tbl); err != nil {
			t.Fatalf("create table %s failed: %v", tbl.Name, err)
		}
	}

	runSQL(t, eng, `INSERT OVERWRITE tgt_t SELECT id FROM src_t;`)
	data, err := os.ReadFile(filepath.Join(dir, "tgt.csv"))
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	for _, want := range []string{"1,auto", "2,auto"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, string(data))
		}
	}
}

// An outer join fills an unmatched side with the declared column DEFAULT
// when present, falling back to the type-based default otherwise.
func TestJoinFillUsesDeclaredDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte("1,alice\n2,bob\n"), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte("1,7,ok,5\n"), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)
	left := &catalog.Table{
		Name: "dleft",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	right := &catalog.Table{
		Name: "dright",
		Columns: []catalog.ColumnDef{
			{Name: "lid", Type: "INT"},
			// declared DEFAULT wins over the type default ("0")
			{Name: "ramount", Type: "INT", Default: "-7", HasDefault: true},
			{Name: "note", Type: "STRING", Default: "missing", HasDefault: true},
			// no declared DEFAULT -> type default applies
			{Name: "code", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	for _, tbl := range []*catalog.Table{left, right} {
		if err := cat.CreateTable(tbl); err != nil {
			t.Fatalf("create table %s failed: %v", tbl.Name, err)
		}
	}

	rows := querySQL(t, eng, "SELECT s.name, r.ramount, r.note, r.code FROM dleft s LEFT JOIN dright r ON r.lid = s.id;")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, r := range rows {
		switch r["name"] {
		case "alice": // matched: real values
			if r["ramount"] != "7" || r["note"] != "ok" || r["code"] != "5" {
				t.Errorf("matched row should keep right values, got %+v", r)
			}
		case "bob": // unmatched: defaults
			if r["ramount"] != "-7" {
				t.Errorf("expected declared default -7, got %q", r["ramount"])
			}
			if r["note"] != "missing" {
				t.Errorf("expected declared default 'missing', got %q", r["note"])
			}
			if r["code"] != "0" {
				t.Errorf("expected type default '0' for code, got %q", r["code"])
			}
		}
	}
}

// runSQLError parses and executes a single statement, returning the error.
func runSQLError(t *testing.T, eng *Engine, sql string) error {
	t.Helper()
	p := parser.NewParser()
	stmts, err := p.Parse(sql)
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if err := eng.Execute(stmt); err != nil {
			return err
		}
	}
	return nil
}
