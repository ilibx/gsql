package samples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/sqlparse"
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
	parser := sqlparse.NewParser()
	stmts, err := parser.Parse(string(data))
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
	runSQLOnEngine(t, engine.NewEngine(catalog.NewCatalog()), filepath.Join("samples", "setup.sql"))
}

func TestQueryBasic(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("samples", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("samples", "query_basic.sql"))
}

func TestQueryJoin(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("samples", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("samples", "query_join.sql"))
}

func TestQueryCTEAndSubquery(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("samples", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("samples", "query_cte_subquery.sql"))
}

func TestQueryInsertOverwrite(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("samples", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("samples", "query_insert.sql"))
	if _, err := os.Stat(filepath.Join(rootDir, "samples", "result", "top_users.csv")); os.IsNotExist(err) {
		t.Fatal("expected output file samples/result/top_users.csv")
	}
}

func TestQueryHiveCoverage(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("samples", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("samples", "query_hive_coverage.sql"))
}

func TestQueryHiveSyntax(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	runSQLOnEngine(t, eng, filepath.Join("samples", "setup.sql"))
	runSQLOnEngine(t, eng, filepath.Join("samples", "query_hive_syntax.sql"))
}
