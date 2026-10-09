package samples

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

func TestLoadSampleData(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	p := parser.NewParser()

	data, err := os.ReadFile(filepath.Join(rootDir, "tests/setup.sql"))
	if err != nil {
		t.Fatalf("read setup.sql failed: %v", err)
	}
	stmts, err := p.Parse(string(data))
	if err != nil {
		t.Fatalf("parse setup.sql failed: %v", err)
	}
	for _, stmt := range stmts {
		if err := eng.Execute(stmt); err != nil {
			t.Fatalf("execute stmt failed: %v", err)
		}
	}
}

func TestCaseDiffLeftJoin(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	p := parser.NewParser()

	data, err := os.ReadFile(filepath.Join(rootDir, "samples", "test_case_diff.sql"))
	if err != nil {
		t.Fatalf("read test_case_diff.sql failed: %v", err)
	}
	stmts, err := p.Parse(string(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var sel *parser.SelectStmt
	for _, stmt := range stmts {
		if s, ok := stmt.(*parser.SelectStmt); ok {
			sel = s
		} else if err := eng.Execute(stmt); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	}
	if sel == nil {
		t.Fatal("no SELECT statement found in sample")
	}
	rows, err := eng.ExecuteSelect(sel.Query)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}

	// 两张表 5 vs 4 行；r2/r3 数值有差异，r5 只在左侧。
	// diff 应为：r1=0, r2=1, r3=1, r4=0, r5=0
	expected := map[string]string{
		"r1": "0",
		"r2": "1",
		"r3": "1",
		"r4": "0",
		"r5": "0",
	}
	got := make(map[string]string, len(rows))
	for _, row := range rows {
		got[row["request_id"]] = row["diff"]
	}
	if len(got) != len(rows) {
		t.Fatalf("duplicate request_id in output: %v", got)
	}
	for id, want := range expected {
		if got[id] != want {
			t.Errorf("diff for %s = %q, want %q (all: %v)", id, got[id], want, got)
		}
	}
}
