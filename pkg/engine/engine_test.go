package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/sqlparse"
)

func TestEngineCreateTable(t *testing.T) {
	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	stmt := &sqlparse.CreateTableStmt{
		Name: "users",
		Columns: []sqlparse.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{"storage": "local"},
	}
	if err := engine.Execute(stmt); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if _, ok := cat.GetTable("users"); !ok {
		t.Fatalf("expected table users to exist")
	}
}

func TestEngineSelectFromLocalCSV(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,alice@example.com\n2,bob,bob@example.com\n"
	filePath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(filePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	table := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "email", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"location":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&sqlparse.SelectQuery{
		Columns: []string{"id", "name"},
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" {
		t.Errorf("expected alice, got %s", rows[0]["name"])
	}
}

func TestEngineSelectWithCTE(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,alice@example.com\n2,bob,bob@example.com\n"
	filePath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(filePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	table := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "email", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"location":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	query := `WITH recent_users AS (
  SELECT id, name, email FROM users WHERE name = 'alice'
)
SELECT id, name FROM recent_users`

	stmts, err := sqlparse.NewParser().Parse(query)
	if err != nil {
		t.Fatalf("parse query failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*sqlparse.SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	rows, err := engine.executeSelect(selectStmt.Query)
	if err != nil {
		t.Fatalf("execute select with CTE failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" {
		t.Errorf("expected alice, got %s", rows[0]["name"])
	}
}

func TestEngineSelectWithLikePredicate(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,alice@example.com\n2,bob,bob@example.com\n"
	filePath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(filePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	table := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "email", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"location":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&sqlparse.SelectQuery{
		Columns: []string{"id", "name"},
		Table:   "users",
		Where:   &sqlparse.ComparisonExpr{Column: "email", Operator: "LIKE", Value: "%@example.com"},
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" {
		t.Errorf("expected alice, got %s", rows[0]["name"])
	}
}

func TestEngineInsertOverwriteTable(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,alice@example.com\n2,bob,bob@example.com\n"
	sourcePath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(sourcePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	sourceTable := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "email", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"location":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(sourceTable); err != nil {
		t.Fatalf("create source table failed: %v", err)
	}

	targetDir := filepath.Join(dir, "out")
	targetTable := &catalog.Table{
		Name: "result_users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":   "local",
			"format":    "csv",
			"location":  targetDir,
			"file_name": "result.csv",
		},
	}
	if err := cat.CreateTable(targetTable); err != nil {
		t.Fatalf("create target table failed: %v", err)
	}

	stmt := &sqlparse.InsertOverwriteStmt{
		TableName: "result_users",
		Query: &sqlparse.SelectQuery{
			Columns: []string{"id", "name"},
			Table:   "users",
		},
	}
	if err := engine.Execute(stmt); err != nil {
		t.Fatalf("execute insert overwrite failed: %v", err)
	}

	outputPath := filepath.Join(targetDir, "result.csv")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	if !strings.Contains(string(data), "alice") {
		t.Errorf("expected output to contain alice, got %s", string(data))
	}
}
