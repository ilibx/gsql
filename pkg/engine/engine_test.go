package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/parser"
)

func TestEngineCreateTable(t *testing.T) {
	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	stmt := &parser.CreateTableStmt{
		Name: "users",
		Columns: []parser.ColumnDef{
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
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
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
			"path":     dir,
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

	stmts, err := parser.NewParser().Parse(query)
	if err != nil {
		t.Fatalf("parse query failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*parser.SelectStmt)
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
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name"},
		Table:   "users",
		Where:   &parser.ComparisonExpr{Column: "email", Operator: "LIKE", Value: "%@example.com"},
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

func TestEngineSelectWithAndOrWhere(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,charlie,35\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name"},
		Table:   "users",
		Where: &parser.LogicalExpr{
			Left:     &parser.ComparisonExpr{Column: "age", Operator: ">=", Value: "30"},
			Operator: "AND",
			Right:    &parser.ComparisonExpr{Column: "name", Operator: "!=", Value: "bob"},
		},
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["name"] != "charlie" {
		t.Errorf("expected charlie, got %s", rows[0]["name"])
	}
}

func TestEngineSelectGroupByCount(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,alice,35\n4,bob,40\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:    []string{"name", "COUNT(*)"},
		Table:      "users",
		GroupBy:    []string{"name"},
		Aggregates: []parser.AggregateExpr{{FuncName: "COUNT", Column: "*"}},
	})
	if err != nil {
		t.Fatalf("execute select with group by failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" || rows[0]["COUNT(*)"] != "2" {
		t.Errorf("expected alice count=2, got %s count=%s", rows[0]["name"], rows[0]["COUNT(*)"])
	}
	if rows[1]["name"] != "bob" || rows[1]["COUNT(*)"] != "2" {
		t.Errorf("expected bob count=2, got %s count=%s", rows[1]["name"], rows[1]["COUNT(*)"])
	}
}

func TestEngineSelectAggregateSumAvg(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,charlie,35\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:    []string{"SUM(age)", "AVG(age)"},
		Table:      "users",
		Aggregates: []parser.AggregateExpr{{FuncName: "SUM", Column: "age"}, {FuncName: "AVG", Column: "age"}},
	})
	if err != nil {
		t.Fatalf("execute select with aggregates failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["SUM(age)"] != "90" {
		t.Errorf("expected SUM=90, got %s", rows[0]["SUM(age)"])
	}
	if rows[0]["AVG(age)"] != "30" {
		t.Errorf("expected AVG=30, got %s", rows[0]["AVG(age)"])
	}
}

func TestEngineSelectGroupByWithFilter(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,alice,35\n4,bob,40\n5,charlie,45\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:    []string{"name", "COUNT(*)"},
		Table:      "users",
		Where:      &parser.ComparisonExpr{Column: "age", Operator: ">=", Value: "30"},
		GroupBy:    []string{"name"},
		Aggregates: []parser.AggregateExpr{{FuncName: "COUNT", Column: "*"}},
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(rows))
	}
	countMap := make(map[string]string)
	for _, row := range rows {
		countMap[row["name"]] = row["COUNT(*)"]
	}
	if countMap["alice"] != "1" || countMap["bob"] != "2" || countMap["charlie"] != "1" {
		t.Errorf("unexpected counts: %v", countMap)
	}
}

func TestEngineSelectHaving(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,alice,35\n4,bob,40\n5,charlie,45\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:    []string{"name", "COUNT(*)"},
		Table:      "users",
		GroupBy:    []string{"name"},
		Aggregates: []parser.AggregateExpr{{FuncName: "COUNT", Column: "*"}},
		Having:     &parser.ComparisonExpr{Column: "COUNT(*)", Operator: ">", Value: "1"},
	})
	if err != nil {
		t.Fatalf("execute select with HAVING failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 groups after HAVING, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" || rows[0]["COUNT(*)"] != "2" {
		t.Errorf("expected alice count=2, got %s count=%s", rows[0]["name"], rows[0]["COUNT(*)"])
	}
	if rows[1]["name"] != "bob" || rows[1]["COUNT(*)"] != "2" {
		t.Errorf("expected bob count=2, got %s count=%s", rows[1]["name"], rows[1]["COUNT(*)"])
	}
}

func TestEngineSelectFromSubquery(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,charlie,35\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name"},
		FromSubquery: &parser.SelectQuery{
			Columns: []string{"id", "name", "age"},
			Table:   "users",
		},
		FromAlias: "sub",
	})
	if err != nil {
		t.Fatalf("execute select from subquery failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" {
		t.Errorf("expected alice, got %s", rows[0]["name"])
	}
}

func TestEngineSelectSubqueryWithWhere(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,charlie,35\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "*.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name"},
		FromSubquery: &parser.SelectQuery{
			Columns: []string{"id", "name", "age"},
			Table:   "users",
			Where:   &parser.ComparisonExpr{Column: "age", Operator: ">=", Value: "30"},
		},
		FromAlias: "sub",
	})
	if err != nil {
		t.Fatalf("execute select from subquery with where failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "bob" {
		t.Errorf("expected bob, got %s", rows[0]["name"])
	}
	if rows[1]["name"] != "charlie" {
		t.Errorf("expected charlie, got %s", rows[1]["name"])
	}
}

func TestEngineJoin(t *testing.T) {
	dir := t.TempDir()
	usersCSV := "1,alice,25\n2,bob,30\n"
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte(usersCSV), 0o644); err != nil {
		t.Fatalf("write users csv failed: %v", err)
	}
	ordersCSV := "1,1,100\n2,2,200\n3,1,300\n"
	if err := os.WriteFile(filepath.Join(dir, "orders.csv"), []byte(ordersCSV), 0o644); err != nil {
		t.Fatalf("write orders csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	usersTable := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(usersTable); err != nil {
		t.Fatalf("create users table failed: %v", err)
	}

	ordersTable := &catalog.Table{
		Name: "orders",
		Columns: []catalog.ColumnDef{
			{Name: "order_id", Type: "INT"},
			{Name: "user_id", Type: "INT"},
			{Name: "amount", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "orders.csv",
		},
	}
	if err := cat.CreateTable(ordersTable); err != nil {
		t.Fatalf("create orders table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"name", "amount"},
		Table:   "users",
		Joins: []parser.JoinClause{
			{RightTable: "orders", LeftColumn: "id", RightColumn: "user_id"},
		},
	})
	if err != nil {
		t.Fatalf("execute join failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows from join, got %d", len(rows))
	}
	hasAlice100 := false
	hasBob200 := false
	hasAlice300 := false
	for _, row := range rows {
		if row["name"] == "alice" && row["amount"] == "100" {
			hasAlice100 = true
		}
		if row["name"] == "bob" && row["amount"] == "200" {
			hasBob200 = true
		}
		if row["name"] == "alice" && row["amount"] == "300" {
			hasAlice300 = true
		}
	}
	if !hasAlice100 {
		t.Error("expected alice-100 row")
	}
	if !hasBob200 {
		t.Error("expected bob-200 row")
	}
	if !hasAlice300 {
		t.Error("expected alice-300 row")
	}
}

func TestEngineLeftJoin(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,alice\n2,bob\n3,carol\n"
	csvRight := "1,100\n2,200\n4,400\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "user_id", Type: "INT"}, {Name: "amount", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("left join returns all left rows", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "id", RightColumn: "user_id", JoinType: "LEFT"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("LEFT JOIN failed: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows (2 matched + 1 unmatched left), got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found["carol"] != "" {
			t.Errorf("expected carol='' (unmatched), got %q", found["carol"])
		}
	})

	t.Run("left join with reversed ON columns", func(t *testing.T) {
		// Simulate the ON clause being written as "right.user_id = left.id"
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "user_id", RightColumn: "id", JoinType: "LEFT"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("LEFT JOIN (reversed ON) failed: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found["carol"] != "" {
			t.Errorf("expected carol='' (unmatched), got %q", found["carol"])
		}
	})
}

func TestEngineRightJoin(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,alice\n2,bob\n3,carol\n"
	csvRight := "1,100\n2,200\n4,400\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "user_id", Type: "INT"}, {Name: "amount", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("right join normal ON", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "id", RightColumn: "user_id", JoinType: "RIGHT"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("RIGHT JOIN failed: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows (2 matched + 1 unmatched right), got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found[""] != "400" {
			t.Errorf("expected unmatched right row with amount=400, got %q", found[""])
		}
	})

	t.Run("right join reversed ON", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "user_id", RightColumn: "id", JoinType: "RIGHT"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("RIGHT JOIN (reversed ON) failed: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found[""] != "400" {
			t.Errorf("expected unmatched right row with amount=400, got %q", found[""])
		}
	})
}

func TestEngineFullJoin(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,alice\n2,bob\n3,carol\n"
	csvRight := "1,100\n2,200\n4,400\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "user_id", Type: "INT"}, {Name: "amount", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("full join normal ON", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "id", RightColumn: "user_id", JoinType: "FULL"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("FULL JOIN failed: %v", err)
		}
		if len(rows) != 4 {
			t.Fatalf("expected 4 rows, got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found["carol"] != "" {
			t.Errorf("expected carol='' (unmatched left), got %q", found["carol"])
		}
		if found[""] != "400" {
			t.Errorf("expected unmatched right row with amount=400, got %q", found[""])
		}
	})

	t.Run("full join reversed ON", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "user_id", RightColumn: "id", JoinType: "FULL"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("FULL JOIN (reversed ON) failed: %v", err)
		}
		if len(rows) != 4 {
			t.Fatalf("expected 4 rows, got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found["carol"] != "" {
			t.Errorf("expected carol='' (unmatched left), got %q", found["carol"])
		}
		if found[""] != "400" {
			t.Errorf("expected unmatched right row with amount=400, got %q", found[""])
		}
	})
}

func TestEngineSemiJoin(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,alice\n2,bob\n3,carol\n"
	csvRight := "1,100\n2,200\n4,400\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "user_id", Type: "INT"}, {Name: "amount", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	query := &parser.SelectQuery{
		Columns: []string{"name"},
		Table:   "left_t",
		Joins: []parser.JoinClause{
			{RightTable: "right_t", LeftColumn: "id", RightColumn: "user_id", JoinType: "SEMI"},
		},
	}
	rows, err := engine.executeSelect(query)
	if err != nil {
		t.Fatalf("SEMI JOIN failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (alice, bob), got %d", len(rows))
	}
	if rows[0]["name"] != "alice" && rows[1]["name"] != "alice" {
		t.Error("expected alice in SEMI JOIN result")
	}
	if rows[0]["name"] != "bob" && rows[1]["name"] != "bob" {
		t.Error("expected bob in SEMI JOIN result")
	}
	// Carol should NOT be in result
	for _, r := range rows {
		if r["name"] == "carol" {
			t.Error("carol should not be in SEMI JOIN result")
		}
	}
}

func TestEngineCrossJoin(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,alice\n2,bob\n"
	csvRight := "x\ny\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "val", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	query := &parser.SelectQuery{
		Columns: []string{"name", "val"},
		Table:   "left_t",
		Joins: []parser.JoinClause{
			{RightTable: "right_t", JoinType: "CROSS"},
		},
	}
	rows, err := engine.executeSelect(query)
	if err != nil {
		t.Fatalf("CROSS JOIN failed: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows (2*2), got %d", len(rows))
	}
}

func TestEngineJoinStringKey(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "alice,25\nbob,30\ncarol,35\n"
	csvRight := "alice,100\nbob,200\ndave,400\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "name", Type: "STRING"}, {Name: "age", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "user_name", Type: "STRING"}, {Name: "amount", Type: "INT"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("left join on STRING key", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "name", RightColumn: "user_name", JoinType: "LEFT"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("LEFT JOIN on STRING failed: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["name"]] = r["amount"]
		}
		if found["alice"] != "100" {
			t.Errorf("expected alice=100, got %q", found["alice"])
		}
		if found["bob"] != "200" {
			t.Errorf("expected bob=200, got %q", found["bob"])
		}
		if found["carol"] != "" {
			t.Errorf("expected carol='' (unmatched), got %q", found["carol"])
		}
	})

	t.Run("inner join on STRING key with reversed ON", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "amount"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "user_name", RightColumn: "name", JoinType: "INNER"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("INNER JOIN on STRING (reversed ON) failed: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
	})
}

func TestEngineJoinDecimalKey(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,10.50\n2,20.00\n3,30.75\n"
	csvRight := "10.50,prod_a\n20.00,prod_b\n40.00,prod_c\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "price", Type: "DECIMAL(10,2)"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "item_price", Type: "DECIMAL(10,2)"}, {Name: "product", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("left join on DECIMAL key", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"id", "product"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "price", RightColumn: "item_price", JoinType: "LEFT"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("LEFT JOIN on DECIMAL failed: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		found := map[string]string{}
		for _, r := range rows {
			found[r["id"]] = r["product"]
		}
		if found["1"] != "prod_a" {
			t.Errorf("expected id=1 -> prod_a, got %q", found["1"])
		}
		if found["2"] != "prod_b" {
			t.Errorf("expected id=2 -> prod_b, got %q", found["2"])
		}
		if found["3"] != "" {
			t.Errorf("expected id=3 -> '' (unmatched), got %q", found["3"])
		}
	})

	t.Run("inner join on DECIMAL with reversed ON", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"id", "product"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "item_price", RightColumn: "price", JoinType: "INNER"},
			},
		}
		rows, err := engine.executeSelect(query)
		if err != nil {
			t.Fatalf("INNER JOIN on DECIMAL (reversed ON) failed: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
	})
}

func TestEngineJoinArithmeticExpr(t *testing.T) {
	dir := t.TempDir()
	csvLeft := "1,10,alice\n2,20,bob\n3,30,carol\n"
	csvRight := "1,0.9\n2,0.8\n4,0.7\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "amount", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "ref_id", Type: "INT"}, {Name: "discount", Type: "DECIMAL(10,2)"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("arithmetic with table-prefixed columns", func(t *testing.T) {
		// Simulates: SELECT l.amount * r.discount AS final FROM left_t l JOIN right_t r ON l.id = r.ref_id
		// Using table-prefixed columns in the arithmetic expression to verify stripColAlias works
		query := &parser.SelectQuery{
			Table:   "left_t",
			TableAlias: "l",
			Columns: []string{"l.amount * r.discount"},
			ColumnExprs: []parser.Expression{
				&parser.BinaryExpr{
					Left:  &parser.ColumnRef{Name: "l.amount"},
					Operator: "*",
					Right: &parser.ColumnRef{Name: "r.discount"},
				},
			},
			Joins: []parser.JoinClause{
				{RightTable: "right_t", RightAlias: "r", LeftColumn: "id", RightColumn: "ref_id", JoinType: "INNER"},
			},
		}
		rows, err := eng.executeSelect(query)
		if err != nil {
			t.Fatalf("arithmetic expression query failed: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
		// l.amount=10 * r.discount=0.9 = 9, l.amount=20 * r.discount=0.8 = 16
		// The project node evaluates the expression, result key is stripped to "amount * r.discount"
		for _, r := range rows {
			t.Logf("row: %v", r)
		}
	})
}

func TestEngineJoinKeyNormalization(t *testing.T) {
	// Test that type-aware normalization allows differently formatted
	// representations of the same logical value to match in JOIN.
	dir := t.TempDir()
	// Left: INT leading zero, DECIMAL trailing zero, STRING with spaces
	csvLeft := "001,10.50, alice \n002,20.00, bob \n003,30.75, carol \n"
	// Right: INT no leading zero, DECIMAL no trailing zero, STRING trimmed (or vice versa)
	csvRight := "1,10.5,alice\n2,20,bob\n4,40.00,dave\n"
	if err := os.WriteFile(filepath.Join(dir, "left.csv"), []byte(csvLeft), 0o644); err != nil {
		t.Fatalf("write left csv failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "right.csv"), []byte(csvRight), 0o644); err != nil {
		t.Fatalf("write right csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := NewEngine(cat)

	leftTable := &catalog.Table{
		Name:    "left_t",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "price", Type: "DECIMAL(10,2)"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "left.csv",
		},
	}
	if err := cat.CreateTable(leftTable); err != nil {
		t.Fatalf("create left_t failed: %v", err)
	}
	rightTable := &catalog.Table{
		Name:    "right_t",
		Columns: []catalog.ColumnDef{{Name: "ref_id", Type: "INT"}, {Name: "ref_price", Type: "DECIMAL(10,2)"}, {Name: "ref_name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage": "local", "format": "csv", "path": dir, "file_pattern": "right.csv",
		},
	}
	if err := cat.CreateTable(rightTable); err != nil {
		t.Fatalf("create right_t failed: %v", err)
	}

	t.Run("INT normalization: 001 matches 1", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"price", "ref_name"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "id", RightColumn: "ref_id", JoinType: "INNER"},
			},
		}
		rows, err := eng.executeSelect(query)
		if err != nil {
			t.Fatalf("INNER JOIN on INT with normalization failed: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 matched rows, got %d", len(rows))
		}
	})

	t.Run("DECIMAL normalization: 10.50 matches 10.5, 20.00 matches 20", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "ref_name"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "price", RightColumn: "ref_price", JoinType: "INNER"},
			},
		}
		rows, err := eng.executeSelect(query)
		if err != nil {
			t.Fatalf("INNER JOIN on DECIMAL with normalization failed: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 matched rows, got %d", len(rows))
		}
	})

	t.Run("STRING normalization: spaces trimmed", func(t *testing.T) {
		query := &parser.SelectQuery{
			Columns: []string{"name", "ref_name"},
			Table:   "left_t",
			Joins: []parser.JoinClause{
				{RightTable: "right_t", LeftColumn: "name", RightColumn: "ref_name", JoinType: "INNER"},
			},
		}
		rows, err := eng.executeSelect(query)
		if err != nil {
			t.Fatalf("INNER JOIN on STRING with normalization failed: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 matched rows, got %d", len(rows))
		}
	})
}

func TestEngineJoinWithWhere(t *testing.T) {
	dir := t.TempDir()
	usersCSV := "1,alice,25\n2,bob,30\n"
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte(usersCSV), 0o644); err != nil {
		t.Fatalf("write users csv failed: %v", err)
	}
	ordersCSV := "1,1,100\n2,2,200\n3,1,300\n"
	if err := os.WriteFile(filepath.Join(dir, "orders.csv"), []byte(ordersCSV), 0o644); err != nil {
		t.Fatalf("write orders csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	usersTable := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(usersTable); err != nil {
		t.Fatalf("create users table failed: %v", err)
	}
	ordersTable := &catalog.Table{
		Name: "orders",
		Columns: []catalog.ColumnDef{
			{Name: "order_id", Type: "INT"},
			{Name: "user_id", Type: "INT"},
			{Name: "amount", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "orders.csv",
		},
	}
	if err := cat.CreateTable(ordersTable); err != nil {
		t.Fatalf("create orders table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"name", "amount"},
		Table:   "users",
		Joins: []parser.JoinClause{
			{RightTable: "orders", LeftColumn: "id", RightColumn: "user_id"},
		},
		Where: &parser.ComparisonExpr{Column: "amount", Operator: ">", Value: "200"},
	})
	if err != nil {
		t.Fatalf("execute join with where failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row from join with where, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" || rows[0]["amount"] != "300" {
		t.Errorf("expected alice-300, got %s-%s", rows[0]["name"], rows[0]["amount"])
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
			"path":     dir,
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
			"path":  targetDir,
			"file_name": "result.csv",
		},
	}
	if err := cat.CreateTable(targetTable); err != nil {
		t.Fatalf("create target table failed: %v", err)
	}

	stmt := &parser.InsertOverwriteStmt{
		TableName: "result_users",
		Query: &parser.SelectQuery{
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

func TestEngineEmptyCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(csvPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	table := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "empty.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name"},
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows from empty CSV, got %d", len(rows))
	}
}

func TestEngineCSVWithEmptyFields(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,alice@example.com\n2,,bob@example.com\n3,charlie,\n"
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
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
			"path":     dir,
			"file_pattern": "data.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name", "email"},
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[1]["name"] != "" {
		t.Errorf("expected empty name for row 1, got %q", rows[1]["name"])
	}
	if rows[2]["email"] != "" {
		t.Errorf("expected empty email for row 2, got %q", rows[2]["email"])
	}
}

func TestEngineCSVWithQuotedFields(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,\"Alice, Smith\",alice@example.com\n2,\"Bob \"\"The Man\"\"\",bob@example.com\n"
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
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
			"path":     dir,
			"file_pattern": "data.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name", "email"},
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice, Smith" {
		t.Errorf("expected quoted name 'Alice, Smith', got %q", rows[0]["name"])
	}
	if rows[1]["name"] != "Bob \"The Man\"" {
		t.Errorf("expected escaped name 'Bob \"The Man\"', got %q", rows[1]["name"])
	}
}

func TestEngineCSVMissingColumns(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice\n2,bob\n"
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
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
			"path":     dir,
			"file_pattern": "data.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns: []string{"id", "name", "email"},
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("execute select failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["email"] != "" {
		t.Errorf("expected empty email for missing column, got %q", rows[0]["email"])
	}
}

func TestEngineTableAliasedJoin(t *testing.T) {
	dir := t.TempDir()
	usersCSV := "1,alice,25\n2,bob,30\n"
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte(usersCSV), 0o644); err != nil {
		t.Fatalf("write users csv failed: %v", err)
	}
	ordersCSV := "1,1,100\n2,2,200\n3,1,300\n"
	if err := os.WriteFile(filepath.Join(dir, "orders.csv"), []byte(ordersCSV), 0o644); err != nil {
		t.Fatalf("write orders csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)

	usersTable := &catalog.Table{
		Name: "users",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(usersTable); err != nil {
		t.Fatalf("create users table failed: %v", err)
	}
	ordersTable := &catalog.Table{
		Name: "orders",
		Columns: []catalog.ColumnDef{
			{Name: "order_id", Type: "INT"},
			{Name: "user_id", Type: "INT"},
			{Name: "amount", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "orders.csv",
		},
	}
	if err := cat.CreateTable(ordersTable); err != nil {
		t.Fatalf("create orders table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:    []string{"u.name", "o.amount"},
		Table:      "users",
		TableAlias: "u",
		Joins: []parser.JoinClause{
			{RightTable: "orders", RightAlias: "o", LeftColumn: "u.id", RightColumn: "o.user_id"},
		},
	})
	if err != nil {
		t.Fatalf("execute aliased join failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows from join, got %d", len(rows))
	}
	hasAlice100 := false
	for _, row := range rows {
		if row["name"] == "alice" && row["amount"] == "100" {
			hasAlice100 = true
		}
	}
	if !hasAlice100 {
		t.Error("expected alice-100 row")
	}
}

func TestEngineColumnAliasOrderBy(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,30\n3,charlie,35\n"
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
			{Name: "age", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(table); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:       []string{"name", "SUM(age)"},
		Table:         "users",
		GroupBy:       []string{"name"},
		Aggregates:    []parser.AggregateExpr{{FuncName: "SUM", Column: "age"}},
		OrderBy:       []parser.SortOrder{{Column: "total"}},
		ColumnAliases: map[string]string{"total": "SUM(age)"},
	})
	if err != nil {
		t.Fatalf("execute select with column alias order by failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" || rows[0]["total"] != "25" {
		t.Errorf("expected alice-25 first by total, got %s-%s", rows[0]["name"], rows[0]["total"])
	}
}

func TestEngineWindowRowNumber(t *testing.T) {
	dir := t.TempDir()
	data := "1,alice,25\n2,bob,30\n3,alice,35\n"
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte(data), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	if err := cat.CreateTable(&catalog.Table{
		Name:    "users",
		Columns: []catalog.ColumnDef{{Name: "id"}, {Name: "name"}, {Name: "age"}},
		WithOptions: map[string]string{
			"storage":       "local",
			"format":        "csv",
			"path":      dir,
			"file_pattern":  "users.csv",
		},
	}); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	engine := NewEngine(cat)
	rows, err := engine.executeSelect(&parser.SelectQuery{
		Columns:     []string{"name", "ROW_NUMBER()"},
		Table:       "users",
		WindowExprs: []parser.WindowExpr{
			{FuncName: "ROW_NUMBER", Args: nil, PartitionBy: nil, OrderBy: nil},
		},
	})
	if err != nil {
		t.Fatalf("execute window function failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for i, row := range rows {
		if row["ROW_NUMBER()"] != strconv.Itoa(i+1) {
			t.Errorf("expected row %d to have ROW_NUMBER()=%d, got %s", i, i+1, row["ROW_NUMBER()"])
		}
	}
}

func TestEngineInsertOverwritePartitionedTable(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,2024-01-01\n2,bob,2024-01-01\n3,carol,2024-01-02\n"
	sourcePath := filepath.Join(dir, "events.csv")
	if err := os.WriteFile(sourcePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	sourceTable := &catalog.Table{
		Name: "events",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "events.csv",
		},
	}
	if err := cat.CreateTable(sourceTable); err != nil {
		t.Fatalf("create source table failed: %v", err)
	}

	targetDir := filepath.Join(dir, "out")
	targetTable := &catalog.Table{
		Name: "events_partitioned",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		PartitionBy: []string{"dt"},
		WithOptions: map[string]string{
			"storage":   "local",
			"format":    "csv",
			"path":  targetDir,
			"file_name": "data.csv",
		},
	}
	if err := cat.CreateTable(targetTable); err != nil {
		t.Fatalf("create target table failed: %v", err)
	}

	stmt := &parser.InsertOverwriteStmt{
		TableName: "events_partitioned",
		Query: &parser.SelectQuery{
			Columns: []string{"id", "name", "dt"},
			Table:   "events",
		},
	}
	if err := engine.Execute(stmt); err != nil {
		t.Fatalf("execute insert overwrite failed: %v", err)
	}

	part1Dir := filepath.Join(targetDir, "dt=2024-01-01")
	part2Dir := filepath.Join(targetDir, "dt=2024-01-02")

	check := func(path, expect string) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s failed: %v", path, err)
		}
		if !strings.Contains(string(data), expect) {
			t.Errorf("expected %s to contain %q, got %s", path, expect, string(data))
		}
	}

	check(filepath.Join(part1Dir, "data.csv"), "alice")
	check(filepath.Join(part1Dir, "data.csv"), "bob")
	check(filepath.Join(part2Dir, "data.csv"), "carol")
}

func TestEngineInsertInto(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice\n2,bob\n"
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
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(sourceTable); err != nil {
		t.Fatalf("create source table failed: %v", err)
	}

	targetDir := filepath.Join(dir, "out")
	targetTable := &catalog.Table{
		Name: "result",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":   "local",
			"format":    "csv",
			"path":  targetDir,
			"file_name": "result.csv",
		},
	}
	if err := cat.CreateTable(targetTable); err != nil {
		t.Fatalf("create target table failed: %v", err)
	}

	stmt1 := &parser.InsertOverwriteStmt{
		TableName: "result",
		Append:    true,
		Query: &parser.SelectQuery{
			Columns: []string{"id", "name"},
			Table:   "users",
		},
	}
	if err := engine.Execute(stmt1); err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	if err := engine.Execute(stmt1); err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatalf("read target dir failed: %v", err)
	}
	// Should have 1 merged file
	if len(entries) != 1 || entries[0].Name() != "result.csv" {
		t.Errorf("expected 1 result.csv file, got %d files", len(entries))
	}
	// Verify content has both rows
	data, err := os.ReadFile(filepath.Join(targetDir, "result.csv"))
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

func TestEnginePartitionPruning(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,2024-01-01\n2,bob,2024-01-02\n3,carol,2024-01-01\n"
	sourcePath := filepath.Join(dir, "events.csv")
	if err := os.WriteFile(sourcePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	sourceTable := &catalog.Table{
		Name: "events",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "events.csv",
		},
	}
	if err := cat.CreateTable(sourceTable); err != nil {
		t.Fatalf("create source table failed: %v", err)
	}

	targetDir := filepath.Join(dir, "out")
	targetTable := &catalog.Table{
		Name: "events_partitioned",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
			{Name: "dt", Type: "STRING"},
		},
		PartitionBy: []string{"dt"},
		WithOptions: map[string]string{
			"storage":   "local",
			"format":    "csv",
			"path":  targetDir,
			"file_name": "data.csv",
		},
	}
	if err := cat.CreateTable(targetTable); err != nil {
		t.Fatalf("create target table failed: %v", err)
	}

	writeStmt := &parser.InsertOverwriteStmt{
		TableName: "events_partitioned",
		Query: &parser.SelectQuery{
			Columns: []string{"id", "name", "dt"},
			Table:   "events",
		},
	}
	if err := engine.Execute(writeStmt); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	query := &parser.SelectQuery{
		Columns: []string{"id", "name"},
		Table:   "events_partitioned",
		Where: &parser.ComparisonExpr{
			Column:   "dt",
			Operator: "=",
			Value:    "2024-01-01",
		},
	}
	rows, err := engine.executeSelect(query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows from pruned query, got %d", len(rows))
	}
}

func TestEngineUnion(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice\n2,bob\n"
	sourcePath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(sourcePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}
	adminData := "1,admin\n3,charlie\n"
	adminPath := filepath.Join(dir, "admins.csv")
	if err := os.WriteFile(adminPath, []byte(adminData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	createTable := func(name, path string, cols []catalog.ColumnDef) {
		tbl := &catalog.Table{
			Name:    name,
			Columns: cols,
			WithOptions: map[string]string{
				"storage":      "local",
				"format":       "csv",
				"path":     dir,
				"file_pattern": path,
			},
		}
		if err := cat.CreateTable(tbl); err != nil {
			t.Fatalf("create %s failed: %v", name, err)
		}
	}
	createTable("users", "users.csv", []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}})
	createTable("admins", "admins.csv", []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}})

	query := &parser.SelectQuery{
		Columns: []string{"name"},
		Table:   "users",
		UnionQuery: &parser.SelectQuery{
			Columns: []string{"name"},
			Table:   "admins",
		},
		UnionAll: true,
	}
	rows, err := engine.executeSelect(query)
	if err != nil {
		t.Fatalf("execute union failed: %v", err)
	}
	if len(rows) != 4 {
		t.Errorf("expected 4 rows from UNION ALL, got %d", len(rows))
	}
}

func TestEngineUnionDedup(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice\n2,bob\n"
	sourcePath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(sourcePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	tbl := &catalog.Table{
		Name:    "data",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "data.csv",
		},
	}
	if err := cat.CreateTable(tbl); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	query := &parser.SelectQuery{
		Columns: []string{"name"},
		Table:   "data",
		UnionQuery: &parser.SelectQuery{
			Columns: []string{"name"},
			Table:   "data",
		},
	}
	rows, err := engine.executeSelect(query)
	if err != nil {
		t.Fatalf("execute union failed: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 deduped rows from UNION, got %d", len(rows))
	}
}

func TestEngineCaseWhen(t *testing.T) {
	dir := t.TempDir()
	csvData := "1,alice,25\n2,bob,15\n3,carol,30\n"
	sourcePath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(sourcePath, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write csv failed: %v", err)
	}

	cat := catalog.NewCatalog()
	engine := NewEngine(cat)
	tbl := &catalog.Table{
		Name:    "users",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}, {Name: "name", Type: "STRING"}, {Name: "age", Type: "INT"}},
		WithOptions: map[string]string{
			"storage":      "local",
			"format":       "csv",
			"path":     dir,
			"file_pattern": "users.csv",
		},
	}
	if err := cat.CreateTable(tbl); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	query := &parser.SelectQuery{
		Columns: []string{"name", "category"},
		ColumnExprs: []parser.Expression{
			nil,
			&parser.CaseExpr{
				Branches: []parser.CaseBranch{
					{
						Condition: &parser.ComparisonExpr{Column: "age", Operator: ">=", Value: "18"},
						Result:    &parser.ComparisonExpr{Column: "", Operator: "=", Value: "adult"},
					},
				},
				Else: &parser.ComparisonExpr{Column: "", Operator: "=", Value: "child"},
			},
		},
		Table: "users",
	}
	rows, err := engine.executeSelect(query)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	expected := map[string]string{"alice": "adult", "bob": "child", "carol": "adult"}
	for _, row := range rows {
		name := row["name"]
		cat := row["category"]
		if expected[name] != cat {
			t.Errorf("for %s, expected category %s, got %s", name, expected[name], cat)
		}
	}
}
