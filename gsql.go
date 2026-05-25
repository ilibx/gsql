package gsql

import (
	"fmt"

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
