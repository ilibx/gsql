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
