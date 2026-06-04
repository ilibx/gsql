package gsql

import (
	"fmt"

	"github.com/flosch/pongo2/v5"
	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/parser"
)

type DB struct {
	catalog *catalog.Catalog
	engine  *engine.Engine
	parser  *parser.Parser
}

func Open() *DB {
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	p := parser.NewParser()
	return &DB{catalog: cat, engine: eng, parser: p}
}

func (db *DB) Catalog() *catalog.Catalog {
	return db.catalog
}

func (db *DB) SetVerbose(level int) {
	db.engine.VerboseLevel = level
	catalog.DebugLevel = level
}

func (db *DB) Exec(sql string) ([]map[string]string, error) {
	stmts, err := db.parser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse sql failed: %w", err)
	}
	var lastResult []map[string]string
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *parser.SelectStmt:
			rows, err := db.engine.ExecuteSelect(s.Query)
			if err != nil {
				return nil, fmt.Errorf("execute failed: %w", err)
			}
			result := make([]map[string]string, len(rows))
			for i, row := range rows {
				result[i] = map[string]string(row)
			}
			lastResult = result
		default:
			if err := db.engine.Execute(stmt); err != nil {
				return nil, fmt.Errorf("execute failed: %w", err)
			}
		}
	}
	return lastResult, nil
}

// ExecTemplate renders a Jinja2 template with the given parameters,
// then parses and executes the resulting SQL.
// params values can be strings, or parsed JSON objects/arrays for iteration.
// Parameters support Jinja2 syntax: {{ key }}, {% for %}, etc.
func (db *DB) ExecTemplate(tpl string, params map[string]interface{}) ([]map[string]string, error) {
	t, err := pongo2.FromString(tpl)
	if err != nil {
		return nil, fmt.Errorf("parse template failed: %w", err)
	}

	ctx := pongo2.Context{}
	for k, v := range params {
		ctx[k] = v
	}

	rendered, err := t.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("render template failed: %w", err)
	}

	return db.Exec(rendered)
}
