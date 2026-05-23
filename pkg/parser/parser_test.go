package parser

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
	if createStmt.External {
		t.Error("expected external=false for CREATE TABLE")
	}
}

func TestParseCreateExternalTable(t *testing.T) {
	sql := `CREATE EXTERNAL TABLE users (
  id INT,
  name STRING
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
	if !createStmt.External {
		t.Error("expected external=true for CREATE EXTERNAL TABLE")
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

func TestParseSelectWhereAndOr(t *testing.T) {
	sql := `SELECT id, name FROM users WHERE age > 25 AND name = 'alice' OR email LIKE '%@example.com';`

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
	logical, ok := selectStmt.Query.Where.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected LogicalExpr, got %T", selectStmt.Query.Where)
	}
	if logical.Operator != "OR" {
		t.Errorf("expected OR, got %s", logical.Operator)
	}
	leftOr, ok := logical.Left.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected LogicalExpr on left, got %T", logical.Left)
	}
	if leftOr.Operator != "AND" {
		t.Errorf("expected AND, got %s", leftOr.Operator)
	}
}

func TestParseGroupBy(t *testing.T) {
	sql := `SELECT name, COUNT(*) FROM users GROUP BY name;`

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
	if len(selectStmt.Query.GroupBy) != 1 || selectStmt.Query.GroupBy[0] != "name" {
		t.Errorf("expected GROUP BY name, got %v", selectStmt.Query.GroupBy)
	}
	if len(selectStmt.Query.Aggregates) != 1 {
		t.Fatalf("expected 1 aggregate, got %d", len(selectStmt.Query.Aggregates))
	}
	if selectStmt.Query.Aggregates[0].FuncName != "COUNT" || selectStmt.Query.Aggregates[0].Column != "*" {
		t.Errorf("unexpected aggregate: %#v", selectStmt.Query.Aggregates[0])
	}
}

func TestParseAggregateFunctions(t *testing.T) {
	sql := `SELECT COUNT(*), SUM(amount), AVG(price), MIN(age), MAX(score) FROM users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if len(selectStmt.Query.Aggregates) != 5 {
		t.Fatalf("expected 5 aggregates, got %d", len(selectStmt.Query.Aggregates))
	}
	expected := []struct {
		Func string
		Col  string
	}{
		{"COUNT", "*"},
		{"SUM", "amount"},
		{"AVG", "price"},
		{"MIN", "age"},
		{"MAX", "score"},
	}
	for i, exp := range expected {
		if selectStmt.Query.Aggregates[i].FuncName != exp.Func || selectStmt.Query.Aggregates[i].Column != exp.Col {
			t.Errorf("aggregate %d: expected %s(%s), got %s(%s)", i, exp.Func, exp.Col, selectStmt.Query.Aggregates[i].FuncName, selectStmt.Query.Aggregates[i].Column)
		}
	}
	if len(selectStmt.Query.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(selectStmt.Query.Columns))
	}
}

func TestParseGroupByOrderBy(t *testing.T) {
	sql := `SELECT name, COUNT(*) FROM users WHERE age > 20 GROUP BY name ORDER BY name LIMIT 10;`

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
	if len(selectStmt.Query.GroupBy) != 1 || selectStmt.Query.GroupBy[0] != "name" {
		t.Errorf("expected GROUP BY name, got %v", selectStmt.Query.GroupBy)
	}
	if len(selectStmt.Query.OrderBy) != 1 || selectStmt.Query.OrderBy[0].Column != "name" || selectStmt.Query.OrderBy[0].Desc {
		t.Errorf("expected ORDER BY name ASC, got %v", selectStmt.Query.OrderBy)
	}
	if selectStmt.Query.Limit != 10 {
		t.Errorf("expected LIMIT 10, got %d", selectStmt.Query.Limit)
	}
	if selectStmt.Query.Where == nil {
		t.Fatal("expected WHERE expression")
	}
}

func TestParseSelectStar(t *testing.T) {
	sql := `SELECT * FROM users;`

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
	if len(selectStmt.Query.Columns) != 1 || selectStmt.Query.Columns[0] != "*" {
		t.Errorf("expected [*], got %v", selectStmt.Query.Columns)
	}
	if selectStmt.Query.Table != "users" {
		t.Errorf("expected table users, got %s", selectStmt.Query.Table)
	}
}

func TestParseHaving(t *testing.T) {
	sql := `SELECT name, COUNT(*) FROM users GROUP BY name HAVING COUNT(*) > 1;`

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
	if selectStmt.Query.Having == nil {
		t.Fatal("expected Having expression")
	}
	cmp, ok := selectStmt.Query.Having.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected ComparisonExpr for HAVING, got %T", selectStmt.Query.Having)
	}
	if cmp.Column != "COUNT(*)" || cmp.Operator != ">" || cmp.Value != "1" {
		t.Errorf("unexpected HAVING: %#v", cmp)
	}
}

func TestParseHavingAndOr(t *testing.T) {
	sql := `SELECT name, COUNT(*) FROM users GROUP BY name HAVING COUNT(*) > 1 AND AVG(age) < 50;`

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
	if selectStmt.Query.Having == nil {
		t.Fatal("expected Having expression")
	}
	logical, ok := selectStmt.Query.Having.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected LogicalExpr for HAVING, got %T", selectStmt.Query.Having)
	}
	if logical.Operator != "AND" {
		t.Errorf("expected AND, got %s", logical.Operator)
	}
}

func TestParseSubqueryFrom(t *testing.T) {
	sql := `SELECT * FROM (SELECT id, name FROM users) AS t WHERE id > 10;`

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
	if selectStmt.Query.FromSubquery == nil {
		t.Fatal("expected FromSubquery")
	}
	if selectStmt.Query.FromAlias != "t" {
		t.Errorf("expected alias t, got %s", selectStmt.Query.FromAlias)
	}
	if len(selectStmt.Query.FromSubquery.Columns) != 2 {
		t.Errorf("expected 2 columns in subquery, got %d", len(selectStmt.Query.FromSubquery.Columns))
	}
	if selectStmt.Query.FromSubquery.Table != "users" {
		t.Errorf("expected subquery table users, got %s", selectStmt.Query.FromSubquery.Table)
	}
}

func TestParseSubqueryFromWithCTE(t *testing.T) {
	sql := `WITH cte AS (SELECT id, name FROM users) SELECT * FROM (SELECT id FROM cte) AS sub;`

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
	if len(selectStmt.Query.CTEs) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(selectStmt.Query.CTEs))
	}
	if selectStmt.Query.FromSubquery == nil {
		t.Fatal("expected FromSubquery")
	}
	if selectStmt.Query.FromAlias != "sub" {
		t.Errorf("expected alias sub, got %s", selectStmt.Query.FromAlias)
	}
}

func TestParseJoin(t *testing.T) {
	sql := `SELECT * FROM t1 JOIN t2 ON t1.id = t2.id WHERE t1.name = 'alice';`

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
	if len(selectStmt.Query.Joins) != 1 {
		t.Fatalf("expected 1 JOIN, got %d", len(selectStmt.Query.Joins))
	}
	join := selectStmt.Query.Joins[0]
	if join.RightTable != "t2" {
		t.Errorf("expected right table t2, got %s", join.RightTable)
	}
	if join.LeftColumn != "t1.id" {
		t.Errorf("expected left column t1.id, got %s", join.LeftColumn)
	}
	if join.RightColumn != "t2.id" {
		t.Errorf("expected right column t2.id, got %s", join.RightColumn)
	}
	if selectStmt.Query.Table != "t1" {
		t.Errorf("expected left table t1, got %s", selectStmt.Query.Table)
	}
	if selectStmt.Query.Where == nil {
		t.Fatal("expected WHERE clause")
	}
}

func TestParseMultipleJoins(t *testing.T) {
	sql := `SELECT * FROM t1 JOIN t2 ON t1.id = t2.id JOIN t3 ON t2.id = t3.id;`

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
	if len(selectStmt.Query.Joins) != 2 {
		t.Fatalf("expected 2 JOINs, got %d", len(selectStmt.Query.Joins))
	}
	if selectStmt.Query.Joins[0].RightTable != "t2" {
		t.Errorf("expected first join right table t2, got %s", selectStmt.Query.Joins[0].RightTable)
	}
	if selectStmt.Query.Joins[1].RightTable != "t3" {
		t.Errorf("expected second join right table t3, got %s", selectStmt.Query.Joins[1].RightTable)
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

func TestParseColumnAlias(t *testing.T) {
	sql := `SELECT name AS n, COUNT(*) AS cnt FROM users GROUP BY name;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.ColumnAliases["n"] != "name" {
		t.Errorf("expected alias n->name, got %v", selectStmt.Query.ColumnAliases)
	}
	if selectStmt.Query.ColumnAliases["cnt"] != "COUNT(*)" {
		t.Errorf("expected alias cnt->COUNT(*), got %v", selectStmt.Query.ColumnAliases)
	}
	if len(selectStmt.Query.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %v", selectStmt.Query.Columns)
	}
	if selectStmt.Query.Columns[0] != "name" {
		t.Errorf("expected column 0 'name', got %s", selectStmt.Query.Columns[0])
	}
	if selectStmt.Query.Columns[1] != "COUNT(*)" {
		t.Errorf("expected column 1 'COUNT(*)', got %s", selectStmt.Query.Columns[1])
	}
}

func TestParseTableAlias(t *testing.T) {
	sql := `SELECT u.name, o.amount FROM users u JOIN orders o ON u.id = o.user_id;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.TableAlias != "u" {
		t.Errorf("expected table alias u, got %s", selectStmt.Query.TableAlias)
	}
	if len(selectStmt.Query.Joins) != 1 {
		t.Fatalf("expected 1 JOIN, got %d", len(selectStmt.Query.Joins))
	}
	if selectStmt.Query.Joins[0].RightAlias != "o" {
		t.Errorf("expected join alias o, got %s", selectStmt.Query.Joins[0].RightAlias)
	}
	if selectStmt.Query.Joins[0].LeftColumn != "u.id" {
		t.Errorf("expected left column u.id, got %s", selectStmt.Query.Joins[0].LeftColumn)
	}
	if selectStmt.Query.Joins[0].RightColumn != "o.user_id" {
		t.Errorf("expected right column o.user_id, got %s", selectStmt.Query.Joins[0].RightColumn)
	}
	if len(selectStmt.Query.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %v", selectStmt.Query.Columns)
	}
	if selectStmt.Query.Columns[0] != "u.name" {
		t.Errorf("expected column 0 'u.name', got %s", selectStmt.Query.Columns[0])
	}
	if selectStmt.Query.Columns[1] != "o.amount" {
		t.Errorf("expected column 1 'o.amount', got %s", selectStmt.Query.Columns[1])
	}
}

func TestParseDottedColumnInWhere(t *testing.T) {
	sql := `SELECT name FROM users WHERE users.age > 30;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	cmp, ok := selectStmt.Query.Where.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected ComparisonExpr, got %T", selectStmt.Query.Where)
	}
	if cmp.Column != "users.age" {
		t.Errorf("expected column users.age, got %s", cmp.Column)
	}
	if cmp.Operator != ">" {
		t.Errorf("expected operator >, got %s", cmp.Operator)
	}
	if cmp.Value != "30" {
		t.Errorf("expected value 30, got %s", cmp.Value)
	}
}

func TestParseOrderByColumnAlias(t *testing.T) {
	sql := `SELECT COUNT(*) AS cnt FROM users ORDER BY cnt;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.ColumnAliases["cnt"] != "COUNT(*)" {
		t.Errorf("expected alias cnt->COUNT(*), got %v", selectStmt.Query.ColumnAliases)
	}
	if len(selectStmt.Query.OrderBy) != 1 || selectStmt.Query.OrderBy[0].Column != "cnt" || selectStmt.Query.OrderBy[0].Desc {
		t.Errorf("expected ORDER BY cnt ASC, got %v", selectStmt.Query.OrderBy)
	}
}

func TestParseStarWithAlias(t *testing.T) {
	sql := `SELECT * FROM users AS u;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.TableAlias != "u" {
		t.Errorf("expected table alias u, got %s", selectStmt.Query.TableAlias)
	}
	if len(selectStmt.Query.Columns) != 1 || selectStmt.Query.Columns[0] != "*" {
		t.Errorf("expected columns [*], got %v", selectStmt.Query.Columns)
	}
}

func TestParseMultipleAliasedJoins(t *testing.T) {
	sql := `SELECT u.name, p.name, o.amount FROM users u JOIN orders o ON u.id = o.user_id JOIN products p ON o.product_id = p.id;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if selectStmt.Query.TableAlias != "u" {
		t.Errorf("expected table alias u, got %s", selectStmt.Query.TableAlias)
	}
	if len(selectStmt.Query.Joins) != 2 {
		t.Fatalf("expected 2 JOINs, got %d", len(selectStmt.Query.Joins))
	}
	if selectStmt.Query.Joins[0].RightAlias != "o" {
		t.Errorf("expected first join alias o, got %s", selectStmt.Query.Joins[0].RightAlias)
	}
	if selectStmt.Query.Joins[1].RightAlias != "p" {
		t.Errorf("expected second join alias p, got %s", selectStmt.Query.Joins[1].RightAlias)
	}
}

func TestParseWindowFunction(t *testing.T) {
	sql := `SELECT name, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS rn FROM users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if len(selectStmt.Query.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %v", selectStmt.Query.Columns)
	}
	if selectStmt.Query.Columns[0] != "name" {
		t.Errorf("expected column 0 'name', got %s", selectStmt.Query.Columns[0])
	}
	if selectStmt.Query.Columns[1] != "ROW_NUMBER()" {
		t.Errorf("expected column 1 'ROW_NUMBER()', got %s", selectStmt.Query.Columns[1])
	}
	if len(selectStmt.Query.WindowExprs) != 1 {
		t.Fatalf("expected 1 window expr, got %d", len(selectStmt.Query.WindowExprs))
	}
	w := selectStmt.Query.WindowExprs[0]
	if w.FuncName != "ROW_NUMBER" {
		t.Errorf("expected ROW_NUMBER, got %s", w.FuncName)
	}
	if len(w.PartitionBy) != 1 || w.PartitionBy[0] != "dept" {
		t.Errorf("expected PARTITION BY dept, got %v", w.PartitionBy)
	}
	if len(w.OrderBy) != 1 || w.OrderBy[0].Column != "salary" || !w.OrderBy[0].Desc {
		t.Errorf("expected ORDER BY salary DESC, got %v", w.OrderBy)
	}
	if selectStmt.Query.ColumnAliases["rn"] != "ROW_NUMBER()" {
		t.Errorf("expected alias rn, got %v", selectStmt.Query.ColumnAliases)
	}
}

func TestParseWindowFunctionAggregate(t *testing.T) {
	sql := `SELECT dept, name, SUM(amount) OVER (PARTITION BY dept) AS total FROM orders;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if len(selectStmt.Query.WindowExprs) != 1 {
		t.Fatalf("expected 1 window expr, got %d", len(selectStmt.Query.WindowExprs))
	}
	w := selectStmt.Query.WindowExprs[0]
	if w.FuncName != "SUM" {
		t.Errorf("expected SUM, got %s", w.FuncName)
	}
	if len(w.Args) != 1 || w.Args[0] != "amount" {
		t.Errorf("expected SUM(amount), got args %v", w.Args)
	}
	if len(w.PartitionBy) != 1 || w.PartitionBy[0] != "dept" {
		t.Errorf("expected PARTITION BY dept, got %v", w.PartitionBy)
	}
	if selectStmt.Query.ColumnAliases["total"] != "SUM(amount)" {
		t.Errorf("expected alias total, got %v", selectStmt.Query.ColumnAliases)
	}
}

func TestParseCreatePartitionedTable(t *testing.T) {
	sql := `CREATE TABLE events (
  id INT,
  name STRING,
  dt STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/data/events'
)
PARTITIONED BY (dt);`

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
	if len(createStmt.PartitionBy) != 1 || createStmt.PartitionBy[0] != "dt" {
		t.Errorf("expected PARTITIONED BY [dt], got %v", createStmt.PartitionBy)
	}
	if createStmt.External {
		t.Error("expected external=false")
	}
}

func TestParseInsertInto(t *testing.T) {
	sql := `INSERT INTO TABLE result SELECT id, name FROM users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	insertStmt, ok := stmts[0].(*InsertOverwriteStmt)
	if !ok {
		t.Fatalf("expected InsertOverwriteStmt, got %T", stmts[0])
	}
	if !insertStmt.Append {
		t.Error("expected Append=true for INSERT INTO")
	}
	if insertStmt.TableName != "result" {
		t.Errorf("expected table name result, got %s", insertStmt.TableName)
	}
}

func TestParseInsertOverwrite(t *testing.T) {
	sql := `INSERT OVERWRITE TABLE result SELECT id, name FROM users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	insertStmt, ok := stmts[0].(*InsertOverwriteStmt)
	if !ok {
		t.Fatalf("expected InsertOverwriteStmt, got %T", stmts[0])
	}
	if insertStmt.Append {
		t.Error("expected Append=false for INSERT OVERWRITE")
	}
}

func TestParseCreatePartitionedTableMultiColumn(t *testing.T) {
	sql := `CREATE TABLE events (
  id INT,
  name STRING,
  year INT,
  month INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/data/events'
)
PARTITIONED BY (year, month);`

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
	if len(createStmt.PartitionBy) != 2 {
		t.Fatalf("expected 2 partition columns, got %d", len(createStmt.PartitionBy))
	}
	if createStmt.PartitionBy[0] != "year" || createStmt.PartitionBy[1] != "month" {
		t.Errorf("expected [year, month], got %v", createStmt.PartitionBy)
	}
}

func TestParseUnion(t *testing.T) {
	sql := `SELECT id, name FROM users UNION SELECT id, name FROM admins;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	sel, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if sel.Query.UnionQuery == nil {
		t.Fatal("expected UnionQuery to be set")
	}
	if sel.Query.UnionAll {
		t.Error("expected UnionAll=false for UNION")
	}
	if sel.Query.Columns[0] != "id" {
		t.Errorf("expected first query column id, got %s", sel.Query.Columns[0])
	}
	if sel.Query.UnionQuery.Table != "admins" {
		t.Errorf("expected second query table admins, got %s", sel.Query.UnionQuery.Table)
	}
}

func TestParseUnionAll(t *testing.T) {
	sql := `SELECT id FROM users UNION ALL SELECT id FROM admins;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	sel, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if sel.Query.UnionQuery == nil {
		t.Fatal("expected UnionQuery to be set")
	}
	if !sel.Query.UnionAll {
		t.Error("expected UnionAll=true for UNION ALL")
	}
}

func TestParseCaseWhen(t *testing.T) {
	sql := `SELECT name, CASE WHEN age > 18 THEN 'adult' ELSE 'child' END AS category FROM users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	sel, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if len(sel.Query.ColumnExprs) != 2 {
		t.Fatalf("expected 2 column expressions, got %d", len(sel.Query.ColumnExprs))
	}
	if sel.Query.ColumnExprs[0] != nil {
		t.Errorf("expected first column expr to be nil (plain column ref)")
	}
	caseExpr, ok := sel.Query.ColumnExprs[1].(*CaseExpr)
	if !ok {
		t.Fatalf("expected CaseExpr, got %T", sel.Query.ColumnExprs[1])
	}
	if len(caseExpr.Branches) != 1 {
		t.Errorf("expected 1 WHEN branch, got %d", len(caseExpr.Branches))
	}
	if caseExpr.Else == nil {
		t.Errorf("expected ELSE branch to be set")
	}
}

func TestParseCaseWhenNoElse(t *testing.T) {
	sql := `SELECT CASE WHEN status = 'active' THEN 1 END AS flag FROM users;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	sel, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	caseExpr, ok := sel.Query.ColumnExprs[0].(*CaseExpr)
	if !ok {
		t.Fatalf("expected CaseExpr, got %T", sel.Query.ColumnExprs[0])
	}
	if len(caseExpr.Branches) != 1 {
		t.Errorf("expected 1 WHEN branch, got %d", len(caseExpr.Branches))
	}
	if caseExpr.Else != nil {
		t.Errorf("expected ELSE to be nil, got %v", caseExpr.Else)
	}
}

func TestParseMultiUnion(t *testing.T) {
	sql := `SELECT id FROM a UNION ALL SELECT id FROM b UNION SELECT id FROM c;`

	stmts, err := NewParser().Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	sel, ok := stmts[0].(*SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts[0])
	}
	if sel.Query.UnionQuery == nil || !sel.Query.UnionAll {
		t.Error("expected first UNION ALL")
	}
	second := sel.Query.UnionQuery
	if second == nil {
		t.Fatal("expected second union query")
	}
	if second.UnionQuery == nil {
		t.Fatal("expected third union query")
	}
	if second.UnionAll {
		t.Error("expected UNION (not ALL) for second union")
	}
}
