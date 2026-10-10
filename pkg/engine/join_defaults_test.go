package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

// runSQL parses sql and executes every non-SELECT statement.
func runSQL(t *testing.T, eng *Engine, sql string) {
	t.Helper()
	p := parser.NewParser()
	stmts, err := p.Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v\nsql: %s", err, sql)
	}
	for _, stmt := range stmts {
		if _, ok := stmt.(*parser.SelectStmt); ok {
			continue
		}
		if err := eng.Execute(stmt); err != nil {
			t.Fatalf("execute failed: %v\nsql: %s", err, sql)
		}
	}
}

// querySQL parses sql and returns the rows of the first SELECT.
func querySQL(t *testing.T, eng *Engine, sql string) []storage.Row {
	t.Helper()
	p := parser.NewParser()
	stmts, err := p.Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v\nsql: %s", err, sql)
	}
	for _, stmt := range stmts {
		if s, ok := stmt.(*parser.SelectStmt); ok {
			rows, err := eng.ExecuteSelect(s.Query)
			if err != nil {
				t.Fatalf("select failed: %v\nsql: %s", err, sql)
			}
			return rows
		}
	}
	t.Fatalf("no SELECT in: %s", sql)
	return nil
}

// joinDefaultsFixture builds left(id INT, name STRING, amount INT) and
// right(lid INT, name STRING, ramount INT, note STRING) so every join type
// can verify that an unmatched side is filled with type-based defaults.
func joinDefaultsFixture(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte("1,alice,10\n2,bob,20\n3,carol,30\n"), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte("1,alice_r,5,ok\n2,bob_r,6,ok\n4,dave_r,7,ok\n"), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)
	left := &catalog.Table{
		Name: "ldef_left",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "amount", Type: "INT"},
		},
		WithOptions: map[string]string{"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv"},
	}
	right := &catalog.Table{
		Name: "ldef_right",
		Columns: []catalog.ColumnDef{
			{Name: "lid", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "ramount", Type: "INT"},
			{Name: "note", Type: "STRING"},
		},
		WithOptions: map[string]string{"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv"},
	}
	if err := cat.CreateTable(left); err != nil {
		t.Fatalf("create left table failed: %v", err)
	}
	if err := cat.CreateTable(right); err != nil {
		t.Fatalf("create right table failed: %v", err)
	}
	return eng
}

// An unmatched left row must carry the right side's column defaults:
// STRING -> "", INT -> "0"; the conflicting "name" column stays readable
// under the right-side qualifier.
func TestJoinLeftUnmatchedDefaults(t *testing.T) {
	eng := joinDefaultsFixture(t)
	rows := querySQL(t, eng, "SELECT s.name, s.amount, r.name, r.ramount, r.note FROM ldef_left s LEFT JOIN ldef_right r ON r.lid = s.id;")
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	var carol storage.Row
	for _, r := range rows {
		if r["name"] == "carol" {
			carol = r
		}
	}
	if carol == nil {
		t.Fatalf("unmatched left row carol missing: %+v", rows)
	}
	// "r.name" collides with the left "name" so it stays qualified;
	// "ramount"/"note" are unambiguous and de-qualified in the final rows
	if carol["r.name"] != "" {
		t.Errorf("expected r.name='' (STRING default), got %q", carol["r.name"])
	}
	if carol["ramount"] != "0" {
		t.Errorf("expected ramount='0' (INT default), got %q", carol["ramount"])
	}
	if carol["note"] != "" {
		t.Errorf("expected note='' (STRING default), got %q", carol["note"])
	}
	if carol["amount"] != "30" {
		t.Errorf("expected left amount=30 preserved, got %q", carol["amount"])
	}
	// matched rows keep their real right-side values
	for _, r := range rows {
		if r["name"] == "alice" {
			if r["r.name"] != "alice_r" || r["ramount"] != "5" || r["note"] != "ok" {
				t.Errorf("matched row lost right-side values: %+v", r)
			}
		}
	}
}

// String defaults fill with "" which the engine treats as NULL, so
// "p.col IS NULL" is true for an unmatched row; numeric defaults are real
// values ("0") and are therefore not null.
func TestJoinUnmatchedDefaultsNullSemantics(t *testing.T) {
	eng := joinDefaultsFixture(t)
	rows := querySQL(t, eng, "SELECT s.name FROM ldef_left s LEFT JOIN ldef_right r ON r.lid = s.id WHERE r.note IS NULL;")
	if len(rows) != 1 || rows[0]["name"] != "carol" {
		t.Errorf("expected only carol for r.note IS NULL, got %+v", rows)
	}

	rows = querySQL(t, eng, "SELECT s.name FROM ldef_left s LEFT JOIN ldef_right r ON r.lid = s.id WHERE r.ramount IS NULL;")
	if len(rows) != 0 {
		t.Errorf("expected no rows (INT default '0' is not null), got %+v", rows)
	}
}

// An unmatched right row must carry the left side's column defaults.
func TestJoinRightUnmatchedDefaults(t *testing.T) {
	eng := joinDefaultsFixture(t)
	rows := querySQL(t, eng, "SELECT s.id, s.name, s.amount, r.name FROM ldef_left s RIGHT JOIN ldef_right r ON r.lid = s.id;")
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	var dave storage.Row
	for _, r := range rows {
		if r["r.name"] == "dave_r" {
			dave = r
		}
	}
	if dave == nil {
		t.Fatalf("unmatched right row dave_r missing: %+v", rows)
	}
	if dave["id"] != "0" {
		t.Errorf("expected s.id='0' (INT default), got %q", dave["id"])
	}
	if dave["name"] != "" {
		t.Errorf("expected s.name='' (STRING default), got %q", dave["name"])
	}
	if dave["amount"] != "0" {
		t.Errorf("expected s.amount='0' (INT default), got %q", dave["amount"])
	}
}

// FULL join fills both directions: left-only rows get right defaults,
// right-only rows get left defaults, matched rows are untouched.
func TestJoinFullUnmatchedDefaults(t *testing.T) {
	eng := joinDefaultsFixture(t)
	rows := querySQL(t, eng, "SELECT s.name, s.amount, r.name, r.ramount, r.note FROM ldef_left s FULL JOIN ldef_right r ON r.lid = s.id;")
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	for _, r := range rows {
		switch r["name"] {
		case "carol": // left-only: right side filled with defaults
			if r["r.name"] != "" || r["ramount"] != "0" || r["note"] != "" {
				t.Errorf("left-only row missing right defaults: %+v", r)
			}
		case "": // right-only (dave_r): left side filled with defaults
			if r["r.name"] != "dave_r" {
				t.Errorf("right-only row lost right values: %+v", r)
			}
			if r["amount"] != "0" {
				t.Errorf("right-only row expected s.amount='0' (INT default), got %q", r["amount"])
			}
		default: // matched: real values on both sides
			if r["r.name"] == "" {
				t.Errorf("matched row lost right name: %+v", r)
			}
		}
	}
}

// INSERT OVERWRITE must map each SELECT column to the row key that actually
// holds its value. With aliases like "r.input AS p_input", the old
// stripped-name alias lookup collided with the unaliased "s.input" and wrote
// the right-side value into both target columns.
func TestInsertOverwriteJoinColumnAliasCollision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte("r1,100\nr2,200\n"), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte("r1,111\nr2,222\n"), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)
	left := &catalog.Table{
		Name: "ial_left",
		Columns: []catalog.ColumnDef{
			{Name: "request_id", Type: "STRING"},
			{Name: "input", Type: "INT"},
		},
		WithOptions: map[string]string{"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv"},
	}
	right := &catalog.Table{
		Name: "ial_right",
		Columns: []catalog.ColumnDef{
			{Name: "request_id", Type: "STRING"},
			{Name: "input", Type: "INT"},
		},
		WithOptions: map[string]string{"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv"},
	}
	outDir := filepath.Join(dir, "out")
	out := &catalog.Table{
		Name: "ial_out",
		Columns: []catalog.ColumnDef{
			{Name: "request_id", Type: "STRING"},
			{Name: "input", Type: "INT"},
			{Name: "p_input", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": outDir, "file_name": "out.csv",
		},
	}
	for _, tbl := range []*catalog.Table{left, right, out} {
		if err := cat.CreateTable(tbl); err != nil {
			t.Fatalf("create table %s failed: %v", tbl.Name, err)
		}
	}

	runSQL(t, eng, `INSERT OVERWRITE ial_out
SELECT s.request_id, s.input, r.input AS p_input
FROM ial_left s
JOIN ial_right r ON r.request_id = s.request_id;`)

	data, err := os.ReadFile(filepath.Join(outDir, "out.csv"))
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	got := string(data)
	for _, want := range []string{"r1,100,111", "r2,200,222"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}
