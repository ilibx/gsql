package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/parser"
)

var rootDir string

func init() {
	wd, _ := os.Getwd()
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			rootDir = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			rootDir = wd
			break
		}
		dir = parent
	}
}

func runSQLOnEngine(t *testing.T, eng *engine.Engine, relPath string) {
	fullPath := filepath.Join(rootDir, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read %s failed: %v", fullPath, err)
	}
	p := parser.NewParser()
	stmts, err := p.Parse(string(data))
	if err != nil {
		t.Fatalf("parse %s failed: %v", fullPath, err)
	}
	for _, stmt := range stmts {
		if err := eng.Execute(stmt); err != nil {
			t.Fatalf("execute stmt in %s failed: %v", fullPath, err)
		}
	}
}

func TestSetup(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	runSQLOnEngine(t, engine.NewEngine(catalog.NewCatalog()), filepath.Join("tests", "setup.sql"))
}

func runTestSQL(t *testing.T, name, file string) {
	t.Run(name, func(t *testing.T) {
		if err := os.Chdir(rootDir); err != nil {
			t.Fatalf("chdir to %s failed: %v", rootDir, err)
		}
		cat := catalog.NewCatalog()
		eng := engine.NewEngine(cat)
		runSQLOnEngine(t, eng, filepath.Join("tests", "setup.sql"))
		runSQLOnEngine(t, eng, filepath.Join("tests", file))
	})
}

func TestSelect(t *testing.T)            { runTestSQL(t, "select", "test_select.sql") }
func TestWhere(t *testing.T)             { runTestSQL(t, "where", "test_where.sql") }
func TestLike(t *testing.T)              { runTestSQL(t, "like", "test_like.sql") }
func TestIsNull(t *testing.T)            { runTestSQL(t, "is_null", "test_is_null.sql") }
func TestIn(t *testing.T)                { runTestSQL(t, "in", "test_in.sql") }
func TestDistinct(t *testing.T)          { runTestSQL(t, "distinct", "test_distinct.sql") }
func TestOrderBy(t *testing.T)           { runTestSQL(t, "order_by", "test_order_by.sql") }
func TestLimit(t *testing.T)             { runTestSQL(t, "limit", "test_limit.sql") }
func TestAggregate(t *testing.T)         { runTestSQL(t, "aggregate", "test_aggregate.sql") }
func TestGroupBy(t *testing.T)           { runTestSQL(t, "group_by", "test_group_by.sql") }
func TestHaving(t *testing.T)            { runTestSQL(t, "having", "test_having.sql") }
func TestArithmetic(t *testing.T)        { runTestSQL(t, "arithmetic", "test_arithmetic.sql") }
func TestCaseWhen(t *testing.T)          { runTestSQL(t, "case_when", "test_case_when.sql") }
func TestColumnAlias(t *testing.T)       { runTestSQL(t, "column_alias", "test_column_alias.sql") }
func TestTableAlias(t *testing.T)        { runTestSQL(t, "table_alias", "test_table_alias.sql") }
func TestJoin(t *testing.T)              { runTestSQL(t, "join", "test_join.sql") }
func TestMultiJoin(t *testing.T)         { runTestSQL(t, "multi_join", "test_multi_join.sql") }
func TestSubquery(t *testing.T)          { runTestSQL(t, "subquery", "test_subquery.sql") }
func TestCTE(t *testing.T)               { runTestSQL(t, "cte", "test_cte.sql") }
func TestMultiCTE(t *testing.T)          { runTestSQL(t, "multi_cte", "test_multi_cte.sql") }
func TestUnion(t *testing.T)             { runTestSQL(t, "union", "test_union.sql") }
func TestWindow(t *testing.T)            { runTestSQL(t, "window", "test_window.sql") }
func TestExternalTable(t *testing.T)     { runTestSQL(t, "external_table", "test_external_table.sql") }
func TestPartition(t *testing.T)         { runTestSQL(t, "partition", "test_partition.sql") }

func TestInsertOverwrite(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("tests", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("tests", "test_insert_overwrite.sql"))
	if _, err := os.Stat(filepath.Join(rootDir, "samples", "result", "top_users.csv")); os.IsNotExist(err) {
		t.Fatal("expected output file samples/result/top_users.csv")
	}
}
