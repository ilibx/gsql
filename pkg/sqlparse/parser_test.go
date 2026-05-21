package sqlparse

import (
	"testing"
)

func TestParseCreateTable(t *testing.T) {
	sql := `CREATE TABLE users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/data/users'
);`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	createStmt, ok := stmts[0].(*CreateTableStmt)
	if !ok {
		t.Fatalf("expected CreateTableStmt, got %T", stmts[0])
	}
	if createStmt.Name != "users" {
		t.Errorf("expected table name users, got %s", createStmt.Name)
	}
	if len(createStmt.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(createStmt.Columns))
	}
	if createStmt.WithOptions["format"] != "csv" {
		t.Errorf("expected format=csv, got %s", createStmt.WithOptions["format"])
	}
}

func TestParseSelectWithCTE(t *testing.T) {
	sql := `WITH recent_users AS (
  SELECT id, name, email FROM users WHERE name = 'alice'
)
SELECT id, name FROM recent_users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.Table != "recent_users" {
		t.Errorf("expected result table recent_users, got %s", selectStmt.Query.Table)
	}
	if len(selectStmt.Query.CTEs) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(selectStmt.Query.CTEs))
	}
	if selectStmt.Query.CTEs[0].Name != "recent_users" {
		t.Errorf("expected CTE name recent_users, got %s", selectStmt.Query.CTEs[0].Name)
	}
	if selectStmt.Query.CTEs[0].Query.Where == nil {
		t.Fatal("expected CTE where expression")
	}
}

func TestParseSelectWhereComparison(t *testing.T) {
	sql := `SELECT id, name FROM users WHERE age >= 30;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.Table != "users" {
		t.Errorf("expected table users, got %s", selectStmt.Query.Table)
	}
	cmp, ok := selectStmt.Query.Where.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected ComparisonExpr, got %T", selectStmt.Query.Where)
	}
	if cmp.Column != "age" || cmp.Operator != ">=" || cmp.Value != "30" {
		t.Errorf("unexpected comparison expression: %#v", cmp)
	}
}

func TestParseSelectWhereLike(t *testing.T) {
	sql := `SELECT id, name FROM users WHERE email LIKE '%@example.com';`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	cmp, ok := selectStmt.Query.Where.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected ComparisonExpr, got %T", selectStmt.Query.Where)
	}
	if cmp.Column != "email" || cmp.Operator != "LIKE" || cmp.Value != "%@example.com" {
		t.Errorf("unexpected comparison expression: %#v", cmp)
	}
}
