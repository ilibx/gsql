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
