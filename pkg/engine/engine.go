package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/plan"
	"github.com/ilibx/gsql/pkg/sqlparse"
	"github.com/ilibx/gsql/pkg/storage"
)

type Engine struct {
	catalog *catalog.Catalog
}

func NewEngine(catalog *catalog.Catalog) *Engine {
	return &Engine{catalog: catalog}
}

func (e *Engine) Execute(stmt sqlparse.Statement) error {
	switch node := stmt.(type) {
	case *sqlparse.CreateTableStmt:
		return e.executeCreateTable(node)
	case *sqlparse.InsertOverwriteStmt:
		return e.executeInsertOverwrite(node)
	case *sqlparse.SelectStmt:
		rows, err := e.executeSelect(node.Query)
		if err != nil {
			return err
		}
		printRows(rows)
		return nil
	default:
		return fmt.Errorf("unsupported statement type: %T", node)
	}
}

func (e *Engine) executeCreateTable(stmt *sqlparse.CreateTableStmt) error {
	columns := make([]catalog.ColumnDef, 0, len(stmt.Columns))
	for _, col := range stmt.Columns {
		columns = append(columns, catalog.ColumnDef{Name: col.Name, Type: col.Type})
	}
	table := &catalog.Table{
		Name:        stmt.Name,
		Columns:     columns,
		WithOptions: stmt.WithOptions,
	}
	if err := e.catalog.CreateTable(table); err != nil {
		return err
	}
	fmt.Printf("created table %s with %d columns\n", stmt.Name, len(stmt.Columns))
	return nil
}

func (e *Engine) executeInsertOverwrite(stmt *sqlparse.InsertOverwriteStmt) error {
	target, ok := e.catalog.GetTable(stmt.TableName)
	if !ok {
		return fmt.Errorf("target table %s does not exist", stmt.TableName)
	}

	rows, err := e.executeSelect(stmt.Query)
	if err != nil {
		return err
	}

	if err := storage.WriteRows(target, rows); err != nil {
		return fmt.Errorf("write target table %s failed: %w", stmt.TableName, err)
	}
	fmt.Printf("wrote %d rows to table %s\n", len(rows), stmt.TableName)
	return nil
}

func (e *Engine) executeSelect(query *sqlparse.SelectQuery) ([]storage.Row, error) {
	return e.executeSelectWithCTEs(query, make(map[string][]storage.Row))
}

func (e *Engine) executeSelectWithCTEs(query *sqlparse.SelectQuery, cteTables map[string][]storage.Row) ([]storage.Row, error) {
	for _, cte := range query.CTEs {
		rows, err := e.executeSelectWithCTEs(cte.Query, cteTables)
		if err != nil {
			return nil, err
		}
		cteTables[strings.ToLower(cte.Name)] = cloneRows(rows)
	}

	root, err := e.buildPlan(query, cteTables)
	if err != nil {
		return nil, err
	}
	return root.Execute()
}

func (e *Engine) buildPlan(sel *sqlparse.SelectQuery, cteTables map[string][]storage.Row) (plan.PlanNode, error) {
	var root plan.PlanNode
	if rows, ok := cteTables[strings.ToLower(sel.Table)]; ok {
		root = plan.NewCTETableScanNode(sel.Table, rows)
	} else {
		table, ok := e.catalog.GetTable(sel.Table)
		if !ok {
			return nil, fmt.Errorf("table %s not found", sel.Table)
		}
		root = plan.NewTableScanNode(table)
	}

	if sel.Where != nil {
		root = plan.NewFilterNode(root, sel.Where)
	}
	if len(sel.OrderBy) > 0 {
		root = plan.NewSortNode(root, sel.OrderBy)
	}
	if sel.Limit > 0 {
		root = plan.NewLimitNode(root, sel.Limit)
	}
	if len(sel.Columns) > 0 {
		root = plan.NewProjectNode(root, sel.Columns)
	}
	return root, nil
}

func cloneRows(rows []storage.Row) []storage.Row {
	cloned := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		newRow := make(storage.Row)
		for k, v := range row {
			newRow[k] = v
		}
		cloned = append(cloned, newRow)
	}
	return cloned
}

func printRows(rows []storage.Row) {
	if len(rows) == 0 {
		fmt.Println("No rows returned")
		return
	}
	keys := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println(strings.Join(keys, ", "))
	for _, row := range rows {
		values := make([]string, len(keys))
		for i, key := range keys {
			values[i] = row[key]
		}
		fmt.Println(strings.Join(values, ", "))
	}
}
