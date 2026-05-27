package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/parser"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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

func runTestSQL(t *testing.T, name, dir, file string) {
	t.Run(name, func(t *testing.T) {
		if err := os.Chdir(rootDir); err != nil {
			t.Fatalf("chdir to %s failed: %v", rootDir, err)
		}
		cat := catalog.NewCatalog()
		eng := engine.NewEngine(cat)
		runSQLOnEngine(t, eng, filepath.Join("tests", "setup.sql"))
		runSQLOnEngine(t, eng, filepath.Join("tests", dir, file))
	})
}

// basic
func TestSelect(t *testing.T)        { runTestSQL(t, "select", "basic", "test_select.sql") }
func TestWhere(t *testing.T)         { runTestSQL(t, "where", "basic", "test_where.sql") }
func TestLike(t *testing.T)          { runTestSQL(t, "like", "basic", "test_like.sql") }
func TestIsNull(t *testing.T)        { runTestSQL(t, "is_null", "basic", "test_is_null.sql") }
func TestIn(t *testing.T)            { runTestSQL(t, "in", "basic", "test_in.sql") }
func TestDistinct(t *testing.T)      { runTestSQL(t, "distinct", "basic", "test_distinct.sql") }
func TestOrderBy(t *testing.T)       { runTestSQL(t, "order_by", "basic", "test_order_by.sql") }
func TestLimit(t *testing.T)         { runTestSQL(t, "limit", "basic", "test_limit.sql") }
func TestArithmetic(t *testing.T)    { runTestSQL(t, "arithmetic", "basic", "test_arithmetic.sql") }
func TestColumnAlias(t *testing.T)   { runTestSQL(t, "column_alias", "basic", "test_column_alias.sql") }
func TestTableAlias(t *testing.T)    { runTestSQL(t, "table_alias", "basic", "test_table_alias.sql") }
func TestCount(t *testing.T)         { runTestSQL(t, "count", "basic", "test_count.sql") }
func TestAggregate(t *testing.T)     { runTestSQL(t, "aggregate", "basic", "test_aggregate.sql") }
func TestGroupBy(t *testing.T)       { runTestSQL(t, "group_by", "basic", "test_group_by.sql") }
func TestHaving(t *testing.T)        { runTestSQL(t, "having", "basic", "test_having.sql") }
func TestJoin(t *testing.T)          { runTestSQL(t, "join", "basic", "test_join.sql") }
func TestMultiJoin(t *testing.T)     { runTestSQL(t, "multi_join", "basic", "test_multi_join.sql") }
func TestSubquery(t *testing.T)      { runTestSQL(t, "subquery", "basic", "test_subquery.sql") }
func TestCTE(t *testing.T)           { runTestSQL(t, "cte", "basic", "test_cte.sql") }
func TestMultiCTE(t *testing.T)      { runTestSQL(t, "multi_cte", "basic", "test_multi_cte.sql") }
func TestUnion(t *testing.T)         { runTestSQL(t, "union", "basic", "test_union.sql") }
func TestWindow(t *testing.T)        { runTestSQL(t, "window", "basic", "test_window.sql") }
func TestCaseWhen(t *testing.T)      { runTestSQL(t, "case_when", "basic", "test_case_when.sql") }

// storage
func TestInsertOverwrite(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("tests", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("tests", "storage", "test_insert_overwrite.sql"))
	if _, err := os.Stat(filepath.Join(rootDir, "samples", "result", "top_users.csv")); os.IsNotExist(err) {
		t.Fatal("expected output file samples/result/top_users.csv")
	}
}
func TestInsertPartition(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("tests", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("tests", "storage", "test_insert_partition.sql"))
	summaryFile := filepath.Join(rootDir, "samples", "result", "monthly", "year=2026", "summary.csv")
	if !fileExists(summaryFile) {
		t.Fatalf("expected partition output %s", summaryFile)
	}
}
func TestPartition(t *testing.T)          { runTestSQL(t, "partition", "storage", "test_partition.sql") }
func TestPartitionFormat(t *testing.T)    { runTestSQL(t, "partition_format", "storage", "test_partition_format.sql") }
func TestExternalTable(t *testing.T)      { runTestSQL(t, "external_table", "storage", "test_external_table.sql") }

// format
func TestCSVDelimiter(t *testing.T)       { runTestSQL(t, "csv_delimiter", "format", "test_csv_delimiter.sql") }
func TestCSVSkipHeader(t *testing.T)      { runTestSQL(t, "csv_skip_header", "format", "test_csv_skip_header.sql") }

// functions
func TestFuncAggregate(t *testing.T)   { runTestSQL(t, "func_aggregate", "functions", "test_func_aggregate.sql") }
func TestFuncMath(t *testing.T)        { runTestSQL(t, "func_math", "functions", "test_func_math.sql") }
func TestFuncString(t *testing.T)      { runTestSQL(t, "func_string", "functions", "test_func_string.sql") }
func TestFuncConditional(t *testing.T) { runTestSQL(t, "func_conditional", "functions", "test_func_conditional.sql") }
func TestFuncDatetime(t *testing.T)    { runTestSQL(t, "func_datetime", "functions", "test_func_datetime.sql") }
func TestFuncWindow(t *testing.T)      { runTestSQL(t, "func_window", "functions", "test_func_window.sql") }
func TestFuncMisc(t *testing.T)        { runTestSQL(t, "func_misc", "functions", "test_func_misc.sql") }
