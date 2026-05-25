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
	catalog      *catalog.Catalog
	VerboseLevel int
}

func NewEngine(catalog *catalog.Catalog) *Engine {
	return &Engine{catalog: catalog}
}

func (e *Engine) Execute(stmt parser.Statement) error {
	switch node := stmt.(type) {
	case *parser.CreateTableStmt:
		return e.executeCreateTable(node)
	case *parser.InsertOverwriteStmt:
		if node.Values != nil {
			return e.executeInsertValues(node)
		}
		return e.executeInsertOverwrite(node)
	case *parser.SelectStmt:
		rows, err := e.executeSelect(node.Query)
		if err != nil {
			return err
		}
		printRows(rows)
		return nil
	case *parser.ExplainStmt:
		return e.executeExplain(node.Query)
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
	if e.VerboseLevel > 0 {
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

	// position-based remap: SELECT col i -> target (data+partition) col i
	// skip for SELECT * (wildcard) since column names are used directly
	if !(len(stmt.Query.Columns) == 1 && stmt.Query.Columns[0] == "*") {
		// target columns in order: data columns first, then partition columns
		targetCols := make([]string, 0, len(target.Columns)+len(target.PartitionBy))
		for _, c := range target.Columns {
			targetCols = append(targetCols, c.Name)
		}
		targetCols = append(targetCols, target.PartitionBy...)

		aliases := make(map[string]string)
		for alias, orig := range stmt.Query.ColumnAliases {
			aliases[stripTableAlias(orig)] = alias
		}

		for ri, row := range rows {
			nr := make(storage.Row)
			for i, origCol := range stmt.Query.Columns {
				stripped := stripTableAlias(origCol)
				if resolved, ok := aliases[stripped]; ok {
					stripped = resolved
				}
				if i < len(targetCols) {
					nr[targetCols[i]] = row[stripped]
				}
			}
			rows[ri] = nr
		}
	}

	if err := storage.WriteRows(target, rows, stmt.Append); err != nil {
		return fmt.Errorf("write target table %s failed: %w", stmt.TableName, err)
	}
	if e.VerboseLevel > 0 {
		action := "overwrote"
		if stmt.Append {
			action = "appended"
		}
		fmt.Printf("-- %s %d rows to table %s\n", action, len(rows), stmt.TableName)
	}
	return nil
}

func (e *Engine) executeExplain(query *parser.SelectQuery) error {
	cteTables := make(map[string][]storage.Row)

	for _, cte := range query.CTEs {
		rows, err := e.executeSelectWithCTEs(cte.Query, cteTables)
		if err != nil {
			return err
		}
		cteTables[strings.ToLower(cte.Name)] = cloneRows(rows)
	}

	if query.FromSubquery != nil {
		rows, err := e.executeSelectWithCTEs(query.FromSubquery, cteTables)
		if err != nil {
			return err
		}
		if query.FromAlias != "" {
			cteTables[strings.ToLower(query.FromAlias)] = cloneRows(rows)
		}
	}

	logical, err := e.buildLogicalPlan(query, cteTables)
	if err != nil {
		return err
	}

	optimized := plan.CatalogOptimizer(e.catalog).OptimizeWithPruning(logical)

	physCtx := &plan.PhysicalPlanContext{
		Catalog: e.catalog,
		CTEData: cteTables,
	}
	root, err := plan.LogicalToPhysical(optimized, physCtx)
	if err != nil {
		return err
	}

	fmt.Println("=== Logical Plan ===")
	fmt.Println(plan.ExplainLogical(logical, 0))
	fmt.Println()
	fmt.Println("=== Optimized Logical Plan ===")
	fmt.Println(plan.ExplainLogical(optimized, 0))
	fmt.Println()
	fmt.Println("=== Physical Plan ===")
	fmt.Println(root.Explain(0))
	return nil
}

func (e *Engine) executeSelect(query *parser.SelectQuery) ([]storage.Row, error) {
	return e.executeSelectWithCTEs(query, make(map[string][]storage.Row))
}

func (e *Engine) ExecuteSelect(query *parser.SelectQuery) ([]storage.Row, error) {
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

	if query.FromValues != nil {
		var rows []storage.Row
		for _, vals := range query.FromValues {
			row := make(storage.Row)
			for i, v := range vals {
				col := fmt.Sprintf("COL_%d", i)
				if i < len(query.FromValuesCols) && query.FromValuesCols[i] != "" {
					col = query.FromValuesCols[i]
				}
				row[col] = v
			}
			rows = append(rows, row)
		}
		if query.FromAlias != "" {
			cteTables[strings.ToLower(query.FromAlias)] = rows
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

	// apply column aliases to row keys (after UNION so both sides get remapped)
	if len(query.ColumnAliases) > 0 {
		origToAlias := make(map[string]string)
		for alias, orig := range query.ColumnAliases {
			origToAlias[stripTableAlias(orig)] = alias
		}
		for _, row := range rows {
			for orig, alias := range origToAlias {
				if val, exists := row[orig]; exists {
					row[alias] = val
					delete(row, orig)
				}
			}
		}
	}

	return rows, nil
}

func (e *Engine) buildLogicalPlan(sel *parser.SelectQuery, cteTables map[string][]storage.Row) (plan.LogicalNode, error) {
	// Resolve subquery expressions (IN/EXISTS) before building the plan
	if err := e.resolveSubqueryExprs(sel, cteTables); err != nil {
		return nil, err
	}
	root := e.buildBaseRelation(sel, cteTables)
	root = e.addJoins(root, sel, cteTables)
	root = e.addFiltersAndGrouping(root, sel)
	if err := e.validateGroupByColumns(sel); err != nil {
		return nil, err
	}
	root = e.addWindowFunctions(root, sel)
	root = e.addProjection(root, sel)
	root = e.addSortAndLimit(root, sel)
	return root, nil
}

func (e *Engine) buildBaseRelation(sel *parser.SelectQuery, cteTables map[string][]storage.Row) plan.LogicalNode {
	tableName := sel.Table
	if sel.FromSubquery != nil && sel.FromAlias != "" {
		tableName = sel.FromAlias
	}
	if sel.FromValues != nil && sel.FromAlias != "" {
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

	if len(groupBy) > 0 || hasRealAggregates(sel.Aggregates) {
		aggDefs := make([]plan.AggregateDef, 0, len(sel.Aggregates))
		for _, agg := range sel.Aggregates {
			if agg.FuncName != "" {
				aggDefs = append(aggDefs, plan.AggregateDef{FuncName: agg.FuncName, Column: stripTableAlias(agg.Column), Distinct: agg.Distinct})
			}
		}
		if len(groupBy) > 0 || len(aggDefs) > 0 {
			root = plan.NewLogicalAggregate(root, groupBy, aggDefs)
		}
	}

	having := sel.Having
	if having != nil {
		having = stripExprAliasesWithMap(having, sel.ColumnAliases)
		root = plan.NewLogicalFilter(root, having)
	}
	return root
}

func hasRealAggregates(aggs []parser.AggregateExpr) bool {
	for _, a := range aggs {
		if a.FuncName != "" {
			return true
		}
	}
	return false
}

func (e *Engine) validateGroupByColumns(sel *parser.SelectQuery) error {
	if len(sel.GroupBy) == 0 {
		return nil
	}
	if len(sel.Columns) == 1 && sel.Columns[0] == "*" {
		return nil
	}
	// build set of valid column names: group-by columns + aggregate expressions
	valid := make(map[string]bool)
	for _, gb := range sel.GroupBy {
		valid[stripTableAlias(gb)] = true
	}
	for _, agg := range sel.Aggregates {
		valid[agg.FuncName+"("+stripTableAlias(agg.Column)+")"] = true
	}
	for _, col := range sel.Columns {
		stripped := stripTableAlias(col)
		if !valid[stripped] {
			return fmt.Errorf("column %q must appear in GROUP BY clause or be used in an aggregate function", stripped)
		}
	}
	return nil
}

func (e *Engine) addWindowFunctions(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	windowExprs := make([]parser.WindowExpr, 0, len(sel.WindowExprs))
	for _, w := range sel.WindowExprs {
		if w.FuncName == "" && len(w.Args) == 0 {
			continue
		}
		args := make([]string, len(w.Args))
		for j, a := range w.Args {
			args[j] = stripTableAlias(a)
		}
		partBy := make([]string, len(w.PartitionBy))
		for j, p := range w.PartitionBy {
			partBy[j] = stripTableAlias(p)
		}
		ob := make([]parser.SortOrder, len(w.OrderBy))
		for j, o := range w.OrderBy {
			ob[j] = parser.SortOrder{Column: stripTableAlias(o.Column), Desc: o.Desc}
		}
		windowExprs = append(windowExprs, parser.WindowExpr{
			FuncName:    w.FuncName,
			Args:        args,
			PartitionBy: partBy,
			OrderBy:     ob,
		})
	}
	if len(windowExprs) > 0 {
		return plan.NewLogicalWindow(root, windowExprs)
	}
	return root
}

func (e *Engine) addSortAndLimit(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	orderBy := make([]parser.SortOrder, len(sel.OrderBy))
	for i, o := range sel.OrderBy {
		s := stripTableAlias(o.Column)
		if alias, ok := sel.ColumnAliases[s]; ok {
			s = stripTableAlias(alias)
		}
		orderBy[i] = parser.SortOrder{Column: s, Desc: o.Desc}
	}
	if len(orderBy) > 0 {
		root = plan.NewLogicalSort(root, orderBy)
	}

	if sel.HasLimit {
		root = plan.NewLogicalLimit(root, sel.Limit)
	}
	return root
}

func (e *Engine) addProjection(root plan.LogicalNode, sel *parser.SelectQuery) plan.LogicalNode {
	var columns []string
	var columnExprs []parser.Expression
	for i, col := range sel.Columns {
		if stripTableAlias(col) == "*" {
			expanded := e.expandStarColumns(sel)
			for _, ec := range expanded {
				colName := stripTableAlias(ec)
				columns = append(columns, colName)
				columnExprs = append(columnExprs, nil)
			}
			continue
		}
		stripped := stripTableAlias(col)
		columns = append(columns, stripped)
		if i < len(sel.ColumnExprs) {
			columnExprs = append(columnExprs, sel.ColumnExprs[i])
		} else {
			columnExprs = append(columnExprs, nil)
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

func (e *Engine) expandStarColumns(sel *parser.SelectQuery) []string {
	var expanded []string

	if sel.Table != "" {
		if t, ok := e.catalog.GetTable(sel.Table); ok {
			prefix := sel.TableAlias
			for _, colDef := range t.Columns {
				if prefix != "" {
					expanded = append(expanded, prefix+"."+colDef.Name)
				} else {
					expanded = append(expanded, colDef.Name)
				}
			}
		}
	}

	if sel.FromValues != nil && len(sel.FromValuesCols) > 0 {
		expanded = append(expanded, sel.FromValuesCols...)
	}

	for _, join := range sel.Joins {
		if t, ok := e.catalog.GetTable(join.RightTable); ok {
			prefix := join.RightAlias
			if prefix == "" {
				prefix = join.RightTable
			}
			for _, colDef := range t.Columns {
				expanded = append(expanded, prefix+"."+colDef.Name)
			}
		}
	}

	if len(expanded) == 0 {
		expanded = append(expanded, "*")
	}
	return expanded
}

func stripTableAlias(col string) string {
	// strip table alias from qualified name: "u.name" -> "name"
	// only if the dot is not inside a function call
	if idx := strings.IndexByte(col, '.'); idx >= 0 {
		parenIdx := strings.IndexByte(col, '(')
		if parenIdx == -1 || idx < parenIdx {
			return col[idx+1:]
		}
	}
	// strip table alias from function argument: "SUM(o.amount)" -> "SUM(amount)"
	if idx := strings.IndexByte(col, '('); idx >= 0 && col[len(col)-1] == ')' {
		funcName := col[:idx]
		arg := col[idx+1 : len(col)-1]
		if argIdx := strings.IndexByte(arg, '.'); argIdx >= 0 {
			return funcName + "(" + arg[argIdx+1:] + ")"
		}
	}
	return col
}

func (e *Engine) resolveSubqueryExprs(sel *parser.SelectQuery, cteTables map[string][]storage.Row) error {
	var err error
	if sel.Where != nil {
		sel.Where, err = e.resolveExprSubqueries(sel.Where, cteTables)
		if err != nil {
			return err
		}
	}
	if sel.Having != nil {
		sel.Having, err = e.resolveExprSubqueries(sel.Having, cteTables)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) resolveExprSubqueries(expr parser.Expression, cteTables map[string][]storage.Row) (parser.Expression, error) {
	if expr == nil {
		return nil, nil
	}
	switch v := expr.(type) {
	case *parser.InExpr:
		if v.Subquery != nil {
			rows, err := e.executeSelectWithCTEs(v.Subquery, cteTables)
			if err != nil {
				return nil, fmt.Errorf("IN subquery: %w", err)
			}
			values := make([]string, 0, len(rows))
			for _, row := range rows {
				for _, val := range row {
					values = append(values, val)
					break // only first column
				}
			}
			return &parser.InExpr{Column: v.Column, Not: v.Not, Values: values}, nil
		}
		return expr, nil
	case *parser.ExistsExpr:
		rows, err := e.executeSelectWithCTEs(v.Subquery, cteTables)
		if err != nil {
			return nil, fmt.Errorf("EXISTS subquery: %w", err)
		}
		exists := len(rows) > 0
		if v.Not {
			exists = !exists
		}
		if exists {
			return &parser.ComparisonExpr{Column: "", Operator: "=", Value: "1"}, nil
		}
		return &parser.ComparisonExpr{Column: "", Operator: "=", Value: "0"}, nil
	case *parser.LogicalExpr:
		left, err := e.resolveExprSubqueries(v.Left, cteTables)
		if err != nil {
			return nil, err
		}
		right, err := e.resolveExprSubqueries(v.Right, cteTables)
		if err != nil {
			return nil, err
		}
		return &parser.LogicalExpr{Left: left, Operator: v.Operator, Right: right}, nil
	case *parser.BinaryExpr:
		left, err := e.resolveExprSubqueries(v.Left, cteTables)
		if err != nil {
			return nil, err
		}
		right, err := e.resolveExprSubqueries(v.Right, cteTables)
		if err != nil {
			return nil, err
		}
		return &parser.BinaryExpr{Left: left, Operator: v.Operator, Right: right}, nil
	case *parser.CaseExpr:
		for i, branch := range v.Branches {
			cond, cerr := e.resolveExprSubqueries(branch.Condition, cteTables)
			if cerr != nil {
				return nil, cerr
			}
			res, rerr := e.resolveExprSubqueries(branch.Result, cteTables)
			if rerr != nil {
				return nil, rerr
			}
			v.Branches[i] = parser.CaseBranch{Condition: cond, Result: res}
		}
		if v.Else != nil {
			elseExpr, eerr := e.resolveExprSubqueries(v.Else, cteTables)
			if eerr != nil {
				return nil, eerr
			}
			v.Else = elseExpr
		}
		return v, nil
	default:
		return expr, nil
	}
}

func stripExprAliases(expr parser.Expression) parser.Expression {
	return stripExprAliasesWithMap(expr, nil)
}

func stripExprAliasesWithMap(expr parser.Expression, aliases map[string]string) parser.Expression {
	if expr == nil {
		return nil
	}
	switch v := expr.(type) {
	case *parser.ComparisonExpr:
		col := stripTableAlias(v.Column)
		if aliases != nil {
			if resolved, ok := aliases[col]; ok {
				col = resolved
			}
		}
		rightCol := stripTableAlias(v.RightColumn)
		if aliases != nil {
			if resolved, ok := aliases[rightCol]; ok {
				rightCol = resolved
			}
		}
		return &parser.ComparisonExpr{
			Column:      col,
			Operator:    v.Operator,
			Value:       v.Value,
			RightColumn: rightCol,
			Expr:        v.Expr,
		}
	case *parser.LogicalExpr:
		return &parser.LogicalExpr{
			Left:     stripExprAliasesWithMap(v.Left, aliases),
			Operator: v.Operator,
			Right:    stripExprAliasesWithMap(v.Right, aliases),
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

func (e *Engine) executeInsertValues(stmt *parser.InsertOverwriteStmt) error {
	target, ok := e.catalog.GetTable(stmt.TableName)
	if !ok {
		return fmt.Errorf("target table %s does not exist", stmt.TableName)
	}

	// 检查 VALUES 列数是否与目标表列数匹配
	if len(stmt.Values) == 0 {
		return fmt.Errorf("no values provided for INSERT")
	}
	if len(stmt.Values[0]) != len(target.Columns) {
		return fmt.Errorf("column count mismatch: VALUES has %d columns, table %s has %d columns", len(stmt.Values[0]), stmt.TableName, len(target.Columns))
	}

	// 将 VALUES 转换为 Row 格式
	rows := make([]storage.Row, 0, len(stmt.Values))
	for _, rowValues := range stmt.Values {
		row := make(storage.Row)
		for i, colDef := range target.Columns {
			row[colDef.Name] = rowValues[i]
		}
		rows = append(rows, row)
	}

	if e.VerboseLevel > 0 {
		action := "overwrite"
		if stmt.Append {
			action = "append"
		}
		fmt.Printf("-- executing INSERT %s with %d rows to table %s\n", action, len(rows), stmt.TableName)
	}

	if err := storage.WriteRows(target, rows, stmt.Append); err != nil {
		return fmt.Errorf("write target table %s failed: %w", stmt.TableName, err)
	}
	if e.VerboseLevel > 0 {
		action := "overwrote"
		if stmt.Append {
			action = "appended"
		}
		fmt.Printf("-- %s %d rows to table %s\n", action, len(rows), stmt.TableName)
	}
	return nil
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
