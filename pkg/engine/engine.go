package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/plan"
	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

type Engine struct {
	catalog *catalog.Catalog
	Verbose bool
}

func NewEngine(catalog *catalog.Catalog) *Engine {
	return &Engine{catalog: catalog}
}

func (e *Engine) Execute(stmt parser.Statement) error {
	switch node := stmt.(type) {
	case *parser.CreateTableStmt:
		return e.executeCreateTable(node)
	case *parser.InsertOverwriteStmt:
		return e.executeInsertOverwrite(node)
	case *parser.SelectStmt:
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

func (e *Engine) executeCreateTable(stmt *parser.CreateTableStmt) error {
	columns := make([]catalog.ColumnDef, 0, len(stmt.Columns))
	for _, col := range stmt.Columns {
		columns = append(columns, catalog.ColumnDef{Name: col.Name, Type: col.Type})
	}
	table := &catalog.Table{
		Name:        stmt.Name,
		Columns:     columns,
		WithOptions: stmt.WithOptions,
		External:    stmt.External,
		PartitionBy: stmt.PartitionBy,
	}
	if err := e.catalog.CreateTable(table); err != nil {
		return err
	}
	if e.Verbose {
		fmt.Printf("-- created table %s with %d columns\n", stmt.Name, len(stmt.Columns))
	}
	return nil
}

func (e *Engine) executeInsertOverwrite(stmt *parser.InsertOverwriteStmt) error {
	target, ok := e.catalog.GetTable(stmt.TableName)
	if !ok {
		return fmt.Errorf("target table %s does not exist", stmt.TableName)
	}

	rows, err := e.executeSelect(stmt.Query)
	if err != nil {
		return err
	}

	if err := storage.WriteRows(target, rows, stmt.Append); err != nil {

		return fmt.Errorf("write target table %s failed: %w", stmt.TableName, err)
	}
	if e.Verbose {
		action := "overwrote"
		if stmt.Append {
			action = "appended"
		}
		fmt.Printf("-- %s %d rows to table %s\n", action, len(rows), stmt.TableName)
	}
	return nil
}

func (e *Engine) executeSelect(query *parser.SelectQuery) ([]storage.Row, error) {
	return e.executeSelectWithCTEs(query, make(map[string][]storage.Row))
}

func (e *Engine) executeSelectWithCTEs(query *parser.SelectQuery, cteTables map[string][]storage.Row) ([]storage.Row, error) {
	for _, cte := range query.CTEs {
		rows, err := e.executeSelectWithCTEs(cte.Query, cteTables)
		if err != nil {
			return nil, err
		}
		cteTables[strings.ToLower(cte.Name)] = cloneRows(rows)
	}

	if query.FromSubquery != nil {
		rows, err := e.executeSelectWithCTEs(query.FromSubquery, cteTables)
		if err != nil {
			return nil, err
		}
		if query.FromAlias != "" {
			cteTables[strings.ToLower(query.FromAlias)] = cloneRows(rows)
		}
	}

	logical, err := e.buildLogicalPlan(query, cteTables)
	if err != nil {
		return nil, err
	}

	optimized := plan.CatalogOptimizer(e.catalog).OptimizeWithPruning(logical)

	physCtx := &plan.PhysicalPlanContext{
		Catalog: e.catalog,
		CTEData: cteTables,
	}
	root, err := plan.LogicalToPhysical(optimized, physCtx)
	if err != nil {
		return nil, err
	}
	rows, err := root.Execute()
	if err != nil {
		return nil, err
	}

	// UNION [ALL]
	if query.UnionQuery != nil {
		rightRows, err := e.executeSelectWithCTEs(query.UnionQuery, cteTables)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rightRows...)
		if !query.UnionAll {
			rows = dedupRows(rows)
		}
	}

	return rows, nil
}

func (e *Engine) buildLogicalPlan(sel *parser.SelectQuery, cteTables map[string][]storage.Row) (plan.LogicalNode, error) {
	root := e.buildBaseRelation(sel, cteTables)
	root = e.addJoins(root, sel, cteTables)
	root = e.addFiltersAndGrouping(root, sel)
	root = e.addWindowFunctions(root, sel)
	root = e.addSortAndLimit(root, sel)
	root = e.addProjection(root, sel)
	return root, nil
}

func (e *Engine) buildBaseRelation(sel *parser.SelectQuery, cteTables map[string][]storage.Row) plan.LogicalNode {
	tableName := sel.Table
	if sel.FromSubquery != nil && sel.FromAlias != "" {
		tableName = sel.FromAlias
	}

	if _, ok := cteTables[strings.ToLower(tableName)]; ok {
		return plan.NewLogicalCTEScan(tableName)
	}
	return plan.NewLogicalScan(tableName, sel.TableAlias)
}

func (e *Engine) addJoins(root plan.LogicalNode, sel *parser.SelectQuery, cteTables map[string][]storage.Row) plan.LogicalNode {
	for _, join := range sel.Joins {
		var rightRoot plan.LogicalNode
		if _, ok := cteTables[strings.ToLower(join.RightTable)]; ok {
			rightRoot = plan.NewLogicalCTEScan(join.RightTable)
		} else {
			rightRoot = plan.NewLogicalScan(join.RightTable, join.RightAlias)
		}
		leftCol := stripTableAlias(join.LeftColumn)
		rightCol := stripTableAlias(join.RightColumn)
		root = plan.NewLogicalJoin(root, rightRoot, leftCol, rightCol)
	}
	return root
}

func (e *Engine) addFiltersAndGrouping(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	where := sel.Where
	if where != nil {
		where = stripExprAliases(where)
		root = plan.NewLogicalFilter(root, where)
	}

	groupBy := make([]string, len(sel.GroupBy))
	for i, col := range sel.GroupBy {
		groupBy[i] = stripTableAlias(col)
	}

	if len(groupBy) > 0 || len(sel.Aggregates) > 0 {
		aggDefs := make([]plan.AggregateDef, len(sel.Aggregates))
		for i, agg := range sel.Aggregates {
			aggDefs[i] = plan.AggregateDef{FuncName: agg.FuncName, Column: stripTableAlias(agg.Column)}
		}
		root = plan.NewLogicalAggregate(root, groupBy, aggDefs)
	}

	having := sel.Having
	if having != nil {
		having = stripExprAliases(having)
		root = plan.NewLogicalFilter(root, having)
	}
	return root
}

func (e *Engine) addWindowFunctions(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	hasWindow := false
	for _, w := range sel.WindowExprs {
		if w.FuncName != "" {
			hasWindow = true
			break
		}
	}
	if !hasWindow {
		return root
	}

	windowExprs := make([]parser.WindowExpr, len(sel.WindowExprs))
	for i, w := range sel.WindowExprs {
		if w.FuncName == "" {
			continue
		}
		partBy := make([]string, len(w.PartitionBy))
		for j, p := range w.PartitionBy {
			partBy[j] = stripTableAlias(p)
		}
		ob := make([]parser.SortOrder, len(w.OrderBy))
		for j, o := range w.OrderBy {
			ob[j] = parser.SortOrder{Column: stripTableAlias(o.Column), Desc: o.Desc}
		}
		windowExprs[i] = parser.WindowExpr{
			FuncName:    w.FuncName,
			Args:        w.Args,
			PartitionBy: partBy,
			OrderBy:     ob,
		}
	}
	return plan.NewLogicalWindow(root, windowExprs)
}

func (e *Engine) addSortAndLimit(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	orderBy := make([]parser.SortOrder, len(sel.OrderBy))
	for i, o := range sel.OrderBy {
		s := stripTableAlias(o.Column)
		if alias, ok := sel.ColumnAliases[s]; ok {
			s = alias
		}
		orderBy[i] = parser.SortOrder{Column: s, Desc: o.Desc}
	}
	if len(orderBy) > 0 {
		root = plan.NewLogicalSort(root, orderBy)
	}

	if sel.Limit > 0 {
		root = plan.NewLogicalLimit(root, sel.Limit)
	}
	return root
}

func (e *Engine) addProjection(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	columns := make([]string, len(sel.Columns))
	columnExprs := make([]parser.Expression, len(sel.Columns))
	for i, col := range sel.Columns {
		columns[i] = stripTableAlias(col)
	}
	for i, expr := range sel.ColumnExprs {
		if i < len(columnExprs) {
			columnExprs[i] = expr
		}
	}

	if sel.Distinct && len(columns) > 0 {
		root = plan.NewLogicalAggregate(root, columns, nil)
	}

	if len(columns) > 0 {
		root = plan.NewLogicalProject(root, columns, columnExprs)
	}
	return root
}

func stripTableAlias(col string) string {
	if idx := strings.IndexByte(col, '.'); idx >= 0 {
		return col[idx+1:]
	}
	return col
}

func stripExprAliases(expr parser.Expression) parser.Expression {
	if expr == nil {
		return nil
	}
	switch v := expr.(type) {
	case *parser.ComparisonExpr:
		return &parser.ComparisonExpr{
			Column:   stripTableAlias(v.Column),
			Operator: v.Operator,
			Value:    v.Value,
		}
	case *parser.LogicalExpr:
		return &parser.LogicalExpr{
			Left:     stripExprAliases(v.Left),
			Operator: v.Operator,
			Right:    stripExprAliases(v.Right),
		}
	}
	return expr
}

func dedupRows(rows []storage.Row) []storage.Row {
	seen := make(map[string]bool)
	result := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		var parts []string
		for k, v := range row {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)
		key := strings.Join(parts, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, row)
	}
	return result
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
