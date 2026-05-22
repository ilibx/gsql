package plan

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/sqlparse"
	"github.com/ilibx/gsql/pkg/storage"
)

type AggregateDef struct {
	FuncName string
	Column   string
}

type PlanNode interface {
	Type() string
	Execute() ([]storage.Row, error)
	Explain(indent int) string
}

type TableScanNode struct {
	Table            *catalog.Table
	CTE              []storage.Row
	CTEName          string
	PartitionFilters []storage.PartitionFilter
}

func NewTableScanNode(table *catalog.Table) *TableScanNode {
	return &TableScanNode{Table: table}
}

func NewTableScanNodeWithFilters(table *catalog.Table, filters []storage.PartitionFilter) *TableScanNode {
	return &TableScanNode{Table: table, PartitionFilters: filters}
}

func NewCTETableScanNode(name string, rows []storage.Row) *TableScanNode {
	return &TableScanNode{CTE: rows, CTEName: strings.ToLower(name)}
}

func (n *TableScanNode) Type() string {
	return "TableScan"
}

func (n *TableScanNode) Execute() ([]storage.Row, error) {
	if len(n.CTE) > 0 {
		return cloneRows(n.CTE), nil
	}
	if n.Table == nil {
		return nil, fmt.Errorf("table scan node has no table")
	}
	return storage.ReadTableRows(n.Table, n.PartitionFilters...)
}

func (n *TableScanNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	if len(n.CTE) > 0 {
		return fmt.Sprintf("%sTableScan(CTE=%s)", prefix, n.CTEName)
	}
	return fmt.Sprintf("%sTableScan(table=%s)", prefix, n.Table.Name)
}

type FilterNode struct {
	Child     PlanNode
	Predicate sqlparse.Expression
}

func NewFilterNode(child PlanNode, predicate sqlparse.Expression) *FilterNode {
	return &FilterNode{Child: child, Predicate: predicate}
}

func (n *FilterNode) Type() string {
	return "Filter"
}

func (n *FilterNode) Execute() ([]storage.Row, error) {
	rows, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	return filterRowsParallel(rows, n.Predicate), nil
}

func (n *FilterNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	return fmt.Sprintf("%sFilter(%s)\n%s", prefix, expressionToString(n.Predicate), n.Child.Explain(indent+1))
}

type ProjectNode struct {
	Child     PlanNode
	Columns   []string
	Exprs     []sqlparse.Expression // parallel to Columns, nil means plain column ref
}

func NewProjectNode(child PlanNode, columns []string) *ProjectNode {
	return &ProjectNode{Child: child, Columns: columns}
}

func NewProjectNodeWithExprs(child PlanNode, columns []string, exprs []sqlparse.Expression) *ProjectNode {
	return &ProjectNode{Child: child, Columns: columns, Exprs: exprs}
}

func (n *ProjectNode) Type() string {
	return "Project"
}

func (n *ProjectNode) Execute() ([]storage.Row, error) {
	rows, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	if len(n.Columns) == 1 && n.Columns[0] == "*" {
		return rows, nil
	}
	if len(n.Exprs) > 0 {
		return projectRowsWithExprs(rows, n.Columns, n.Exprs), nil
	}
	return projectRowsParallel(rows, n.Columns), nil
}

func projectRowsWithExprs(rows []storage.Row, columns []string, exprs []sqlparse.Expression) []storage.Row {
	result := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		projectedRow := make(storage.Row)
		for i, col := range columns {
			if i < len(exprs) && exprs[i] != nil {
				projectedRow[col] = evaluateExpressionValue(row, exprs[i])
			} else {
				projectedRow[col] = row[col]
			}
		}
		result = append(result, projectedRow)
	}
	return result
}

func (n *ProjectNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	return fmt.Sprintf("%sProject(%s)\n%s", prefix, strings.Join(n.Columns, ", "), n.Child.Explain(indent+1))
}

type WindowNode struct {
	Child       PlanNode
	WindowExprs []sqlparse.WindowExpr
}

func NewWindowNode(child PlanNode, windowExprs []sqlparse.WindowExpr) *WindowNode {
	return &WindowNode{Child: child, WindowExprs: windowExprs}
}

func (n *WindowNode) Type() string { return "Window" }

func (n *WindowNode) Execute() ([]storage.Row, error) {
	rows, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || len(n.WindowExprs) == 0 {
		return rows, nil
	}
	result := computeWindowFunctions(rows, n.WindowExprs)
	return result, nil
}

func (n *WindowNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	parts := make([]string, len(n.WindowExprs))
	for i, w := range n.WindowExprs {
		argStr := ""
		if len(w.Args) > 0 {
			argStr = w.Args[0]
		}
		s := w.FuncName + "(" + argStr + ") OVER("
		if len(w.PartitionBy) > 0 {
			s += "PARTITION BY " + strings.Join(w.PartitionBy, ", ") + " "
		}
		if len(w.OrderBy) > 0 {
			obParts := make([]string, len(w.OrderBy))
			for j, o := range w.OrderBy {
				obParts[j] = o.Column
				if o.Desc {
					obParts[j] += " DESC"
				}
			}
			s += "ORDER BY " + strings.Join(obParts, ", ")
		}
		s += ")"
		parts[i] = s
	}
	return fmt.Sprintf("%sWindow(%s)\n%s", prefix, strings.Join(parts, ", "), n.Child.Explain(indent+1))
}

type SortNode struct {
	Child   PlanNode
	OrderBy []sqlparse.SortOrder
}

func NewSortNode(child PlanNode, orderBy []sqlparse.SortOrder) *SortNode {
	return &SortNode{Child: child, OrderBy: orderBy}
}

func (n *SortNode) Type() string {
	return "Sort"
}

func (n *SortNode) Execute() ([]storage.Row, error) {
	rows, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	sortRows(rows, n.OrderBy)
	return rows, nil
}

func (n *SortNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	parts := make([]string, len(n.OrderBy))
	for i, o := range n.OrderBy {
		s := o.Column
		if o.Desc {
			s += " DESC"
		}
		parts[i] = s
	}
	return fmt.Sprintf("%sSort(%s)\n%s", prefix, strings.Join(parts, ", "), n.Child.Explain(indent+1))
}

type LimitNode struct {
	Child PlanNode
	N     int
}

func NewLimitNode(child PlanNode, n int) *LimitNode {
	return &LimitNode{Child: child, N: n}
}

func (n *LimitNode) Type() string {
	return "Limit"
}

func (n *LimitNode) Execute() ([]storage.Row, error) {
	rows, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	if n.N >= len(rows) || n.N <= 0 {
		return rows, nil
	}
	return rows[:n.N], nil
}

func (n *LimitNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	return fmt.Sprintf("%sLimit(%d)\n%s", prefix, n.N, n.Child.Explain(indent+1))
}

type AggregateNode struct {
	Child      PlanNode
	GroupBy    []string
	Aggregates []AggregateDef
}

func NewAggregateNode(child PlanNode, groupBy []string, aggregates []AggregateDef) *AggregateNode {
	return &AggregateNode{Child: child, GroupBy: groupBy, Aggregates: aggregates}
}

func (n *AggregateNode) Type() string {
	return "Aggregate"
}

func (n *AggregateNode) Execute() ([]storage.Row, error) {
	rows, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}

	type groupEntry struct {
		key   string
		group []storage.Row
	}
	var groups []groupEntry

	if len(n.GroupBy) > 0 {
		groupMap := make(map[string][]storage.Row)
		for _, row := range rows {
			key := makeGroupKey(row, n.GroupBy)
			groupMap[key] = append(groupMap[key], row)
		}
		for key, group := range groupMap {
			groups = append(groups, groupEntry{key: key, group: group})
		}
		sort.Slice(groups, func(i, j int) bool {
			return groups[i].key < groups[j].key
		})
	} else {
		groups = []groupEntry{{key: "", group: rows}}
	}

	var result []storage.Row
	for _, ge := range groups {
		row := make(storage.Row)
		if len(n.GroupBy) > 0 && len(ge.group) > 0 {
			for _, col := range n.GroupBy {
				row[col] = ge.group[0][col]
			}
		}
		for _, agg := range n.Aggregates {
			colKey := agg.FuncName + "(" + agg.Column + ")"
			row[colKey] = computeAggregate(ge.group, agg.FuncName, agg.Column)
		}
		result = append(result, row)
	}
	return result, nil
}

func (n *AggregateNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	aggParts := make([]string, len(n.Aggregates))
	for i, agg := range n.Aggregates {
		aggParts[i] = agg.FuncName + "(" + agg.Column + ")"
	}
	gb := strings.Join(n.GroupBy, ", ")
	return fmt.Sprintf("%sAggregate(GroupBy=[%s], Aggregates=[%s])\n%s", prefix, gb, strings.Join(aggParts, ", "), n.Child.Explain(indent+1))
}

func makeGroupKey(row storage.Row, cols []string) string {
	var parts []string
	for _, col := range cols {
		parts = append(parts, row[col])
	}
	return strings.Join(parts, "\x00")
}

func computeAggregate(rows []storage.Row, funcName, column string) string {
	switch funcName {
	case "COUNT":
		return strconv.Itoa(len(rows))
	case "SUM":
		var total float64
		for _, row := range rows {
			v, err := strconv.ParseFloat(row[column], 64)
			if err == nil {
				total += v
			}
		}
		return strconv.FormatFloat(total, 'f', -1, 64)
	case "AVG":
		var total float64
		count := 0
		for _, row := range rows {
			v, err := strconv.ParseFloat(row[column], 64)
			if err == nil {
				total += v
				count++
			}
		}
		if count == 0 {
			return "0"
		}
		return strconv.FormatFloat(total/float64(count), 'f', -1, 64)
	case "MIN":
		var min string
		for i, row := range rows {
			if i == 0 || row[column] < min {
				min = row[column]
			}
		}
		return min
	case "MAX":
		var max string
		for i, row := range rows {
			if i == 0 || row[column] > max {
				max = row[column]
			}
		}
		return max
	default:
		return ""
	}
}

type JoinNode struct {
	Left        PlanNode
	Right       PlanNode
	LeftColumn  string
	RightColumn string
}

func NewJoinNode(left, right PlanNode, leftCol, rightCol string) *JoinNode {
	return &JoinNode{Left: left, Right: right, LeftColumn: leftCol, RightColumn: rightCol}
}

func (n *JoinNode) Type() string {
	return "Join"
}

func (n *JoinNode) Execute() ([]storage.Row, error) {
	leftRows, err := n.Left.Execute()
	if err != nil {
		return nil, err
	}
	rightRows, err := n.Right.Execute()
	if err != nil {
		return nil, err
	}

	// Use the shorter side for building the hash map
	if len(leftRows) > len(rightRows) {
		leftRows, rightRows = rightRows, leftRows
		leftCol, rightCol := n.LeftColumn, n.RightColumn
		n.LeftColumn, n.RightColumn = rightCol, leftCol
		defer func() {
			n.LeftColumn, n.RightColumn = leftCol, rightCol
		}()
	}

	hash := make(map[string][]storage.Row, len(rightRows))
	for _, row := range rightRows {
		key := row[n.RightColumn]
		hash[key] = append(hash[key], row)
	}

	var result []storage.Row
	for _, lr := range leftRows {
		key := lr[n.LeftColumn]
		if matched, ok := hash[key]; ok {
			for _, rr := range matched {
				merged := make(storage.Row, len(lr)+len(rr))
				for k, v := range lr {
					merged[k] = v
				}
				for k, v := range rr {
					merged[k] = v
				}
				result = append(result, merged)
			}
		}
	}
	return result, nil
}

func (n *JoinNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	return fmt.Sprintf("%sJoin(%s = %s)\n%s\n%s", prefix, n.LeftColumn, n.RightColumn, n.Left.Explain(indent+1), n.Right.Explain(indent+1))
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

func filterRowsParallel(rows []storage.Row, expr sqlparse.Expression) []storage.Row {
	if len(rows) == 0 || expr == nil {
		return rows
	}

	workers := runtime.GOMAXPROCS(0)
	chunkSize := (len(rows) + workers - 1) / workers
	type segmentResult struct {
		index int
		rows  []storage.Row
	}
	resultCh := make(chan segmentResult, workers)
	for i := 0; i < workers; i++ {
		start := i * chunkSize
		if start >= len(rows) {
			resultCh <- segmentResult{index: i, rows: nil}
			continue
		}
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		go func(idx int, slice []storage.Row) {
			filtered := make([]storage.Row, 0, len(slice))
			for _, row := range slice {
				if evaluateExpression(row, expr) {
					filtered = append(filtered, row)
				}
			}
			resultCh <- segmentResult{index: idx, rows: filtered}
		}(i, rows[start:end])
	}

	results := make([][]storage.Row, workers)
	for i := 0; i < workers; i++ {
		res := <-resultCh
		results[res.index] = res.rows
	}
	var result []storage.Row
	for _, segment := range results {
		if len(segment) > 0 {
			result = append(result, segment...)
		}
	}
	return result
}

func projectRowsParallel(rows []storage.Row, columns []string) []storage.Row {
	if len(rows) == 0 {
		return rows
	}
	workers := runtime.GOMAXPROCS(0)
	chunkSize := (len(rows) + workers - 1) / workers
	type segmentResult struct {
		index int
		rows  []storage.Row
	}
	resultCh := make(chan segmentResult, workers)
	for i := 0; i < workers; i++ {
		start := i * chunkSize
		if start >= len(rows) {
			resultCh <- segmentResult{index: i, rows: nil}
			continue
		}
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		go func(idx int, slice []storage.Row) {
			projected := make([]storage.Row, 0, len(slice))
			for _, row := range slice {
				projectedRow := make(storage.Row)
				for _, col := range columns {
					projectedRow[col] = row[col]
				}
				projected = append(projected, projectedRow)
			}
			resultCh <- segmentResult{index: idx, rows: projected}
		}(i, rows[start:end])
	}

	results := make([][]storage.Row, workers)
	for i := 0; i < workers; i++ {
		res := <-resultCh
		results[res.index] = res.rows
	}
	var result []storage.Row
	for _, segment := range results {
		if len(segment) > 0 {
			result = append(result, segment...)
		}
	}
	return result
}

func sortRows(rows []storage.Row, orderBy []sqlparse.SortOrder) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, o := range orderBy {
			vi, vj := rows[i][o.Column], rows[j][o.Column]
			if vi != vj {
				if o.Desc {
					return vi > vj
				}
				return vi < vj
			}
		}
		return false
	})
}

func evaluateExpression(row storage.Row, expr sqlparse.Expression) bool {
	switch v := expr.(type) {
	case *sqlparse.ComparisonExpr:
		left := row[v.Column]
		right := v.Value
		switch v.Operator {
		case "=":
			return left == right
		case "!=":
			return left != right
		case "LIKE":
			return matchLike(left, right)
		case "<", ">", "<=", ">=":
			return compareValues(left, right, v.Operator)
		default:
			return false
		}
	case *sqlparse.LogicalExpr:
		left := evaluateExpression(row, v.Left)
		right := evaluateExpression(row, v.Right)
		switch strings.ToUpper(v.Operator) {
		case "AND":
			return left && right
		case "OR":
			return left || right
		default:
			return false
		}
	case *sqlparse.NullTestExpr:
		val := row[v.Column]
		if v.IsNull {
			return val == ""
		}
		return val != ""
	case *sqlparse.InExpr:
		val := row[v.Column]
		if v.Not {
			return !stringInSlice(val, v.Values)
		}
		return stringInSlice(val, v.Values)
	case *sqlparse.BinaryExpr:
		left := evaluateExpressionValue(row, v.Left)
		right := evaluateExpressionValue(row, v.Right)
		leftNum, lErr := strconv.ParseFloat(left, 64)
		rightNum, rErr := strconv.ParseFloat(right, 64)
		if lErr == nil && rErr == nil {
			switch v.Operator {
			case "+":
				return leftNum+rightNum != 0
			case "-":
				return leftNum-rightNum != 0
			case "*":
				return leftNum*rightNum != 0
			case "/":
				if rightNum == 0 {
					return false
				}
				return leftNum/rightNum != 0
			}
		}
		return false
	default:
		return false
	}
}

func evaluateExpressionValue(row storage.Row, expr sqlparse.Expression) string {
	switch v := expr.(type) {
	case *sqlparse.ColumnRef:
		return row[v.Name]
	case *sqlparse.ComparisonExpr:
		if v.Column == "" {
			return v.Value
		}
		return row[v.Column]
	case *sqlparse.NullTestExpr:
		val := row[v.Column]
		if v.IsNull {
			if val == "" {
				return "true"
			}
			return "false"
		}
		if val != "" {
			return "true"
		}
		return "false"
	case *sqlparse.InExpr:
		val := row[v.Column]
		in := stringInSlice(val, v.Values)
		if v.Not {
			in = !in
		}
		if in {
			return "true"
		}
		return "false"
	case *sqlparse.BinaryExpr:
		left := evaluateExpressionValue(row, v.Left)
		right := evaluateExpressionValue(row, v.Right)
		leftNum, lErr := strconv.ParseFloat(left, 64)
		rightNum, rErr := strconv.ParseFloat(right, 64)
		if lErr == nil && rErr == nil {
			switch v.Operator {
			case "+":
				return formatFloat(leftNum + rightNum)
			case "-":
				return formatFloat(leftNum - rightNum)
			case "*":
				return formatFloat(leftNum * rightNum)
			case "/":
				if rightNum == 0 {
					return "0"
				}
				return formatFloat(leftNum / rightNum)
			}
		}
		return ""
	case *sqlparse.LogicalExpr:
		left := evaluateExpression(row, v.Left)
		right := evaluateExpression(row, v.Right)
		switch strings.ToUpper(v.Operator) {
		case "AND":
			if left && right {
				return "true"
			}
			return "false"
		case "OR":
			if left || right {
				return "true"
			}
			return "false"
		default:
			return "false"
		}
	case *sqlparse.CaseExpr:
		for _, branch := range v.Branches {
			if evaluateExpression(row, branch.Condition) {
				return evaluateExpressionValue(row, branch.Result)
			}
		}
		if v.Else != nil {
			return evaluateExpressionValue(row, v.Else)
		}
		return ""
	default:
		return ""
	}
}

func stringInSlice(s string, list []string) bool {
	for _, item := range list {
		if s == item {
			return true
		}
	}
	return false
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func matchLike(value, pattern string) bool {
	if pattern == "%" {
		return true
	}
	if strings.HasPrefix(pattern, "%") && strings.HasSuffix(pattern, "%") {
		return strings.Contains(value, strings.Trim(pattern, "%"))
	}
	if strings.HasPrefix(pattern, "%") {
		return strings.HasSuffix(value, strings.TrimPrefix(pattern, "%"))
	}
	if strings.HasSuffix(pattern, "%") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "%"))
	}
	return value == pattern
}

func compareValues(left, right, op string) bool {
	leftNum, lErr := strconv.ParseFloat(left, 64)
	rightNum, rErr := strconv.ParseFloat(right, 64)
	if lErr == nil && rErr == nil {
		switch op {
		case "<":
			return leftNum < rightNum
		case ">":
			return leftNum > rightNum
		case "<=":
			return leftNum <= rightNum
		case ">=":
			return leftNum >= rightNum
		}
		return false
	}
	switch op {
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		return false
	}
}

type PhysicalPlanContext struct {
	Catalog *catalog.Catalog
	CTEData map[string][]storage.Row
}

// extractPartitionFilters walks an expression tree and extracts equality
// comparisons on partition columns. Returns the remaining predicate (with
// those comparisons removed) and the extracted partition filters.
// Handles simple equality and AND-conjoined equalities.
func extractPartitionFilters(expr sqlparse.Expression, partitionCols []string) (sqlparse.Expression, []storage.PartitionFilter) {
	if expr == nil || len(partitionCols) == 0 {
		return expr, nil
	}

	isPartition := func(col string) bool {
		for _, pc := range partitionCols {
			if pc == col {
				return true
			}
		}
		return false
	}

	var filters []storage.PartitionFilter

	// rebuildExpr removes partition equalities from the expression tree.
	var rebuildExpr func(sqlparse.Expression) sqlparse.Expression
	rebuildExpr = func(e sqlparse.Expression) sqlparse.Expression {
		if e == nil {
			return nil
		}
		switch v := e.(type) {
		case *sqlparse.ComparisonExpr:
			if v.Operator == "=" && isPartition(v.Column) {
				filters = append(filters, storage.PartitionFilter{Column: v.Column, Value: v.Value})
				return nil // remove this node
			}
			return v
		case *sqlparse.LogicalExpr:
			if v.Operator != "AND" {
				return v // can't prune through OR
			}
			left := rebuildExpr(v.Left)
			right := rebuildExpr(v.Right)
			if left == nil && right == nil {
				return nil
			}
			if left == nil {
				return right
			}
			if right == nil {
				return left
			}
			return &sqlparse.LogicalExpr{Left: left, Operator: "AND", Right: right}
		default:
			return v
		}
	}

	remaining := rebuildExpr(expr)
	return remaining, filters
}

func LogicalToPhysical(node LogicalNode, ctx *PhysicalPlanContext) (PlanNode, error) {
	switch n := node.(type) {
	case *LogicalScan:
		table, ok := ctx.Catalog.GetTable(n.TableName)
		if !ok {
			return nil, fmt.Errorf("table %s not found", n.TableName)
		}
		return NewTableScanNode(table), nil
	case *LogicalCTEScan:
		rows, ok := ctx.CTEData[strings.ToLower(n.CTEName)]
		if !ok {
			return nil, fmt.Errorf("CTE %s not found", n.CTEName)
		}
		return NewCTETableScanNode(n.CTEName, rows), nil
	case *LogicalFilter:
		child, err := LogicalToPhysical(n.Child, ctx)
		if err != nil {
			return nil, err
		}
		// Partition pruning: extract equalities on partition columns
		if scan, ok := child.(*TableScanNode); ok && scan.Table != nil && len(scan.Table.PartitionBy) > 0 {
			remaining, filters := extractPartitionFilters(n.Predicate, scan.Table.PartitionBy)
			if len(filters) > 0 {
				scan.PartitionFilters = append(scan.PartitionFilters, filters...)
				if remaining == nil {
					return child, nil // skip filter entirely
				}
				return NewFilterNode(child, remaining), nil
			}
		}
		return NewFilterNode(child, n.Predicate), nil
	case *LogicalProject:
		child, err := LogicalToPhysical(n.Child, ctx)
		if err != nil {
			return nil, err
		}
		if len(n.Exprs) > 0 {
			return NewProjectNodeWithExprs(child, n.Columns, n.Exprs), nil
		}
		return NewProjectNode(child, n.Columns), nil
	case *LogicalJoin:
		left, err := LogicalToPhysical(n.Left, ctx)
		if err != nil {
			return nil, err
		}
		right, err := LogicalToPhysical(n.Right, ctx)
		if err != nil {
			return nil, err
		}
		return NewJoinNode(left, right, n.LeftColumn, n.RightColumn), nil
	case *LogicalAggregate:
		child, err := LogicalToPhysical(n.Child, ctx)
		if err != nil {
			return nil, err
		}
		return NewAggregateNode(child, n.GroupBy, n.Aggregates), nil
	case *LogicalWindow:
		child, err := LogicalToPhysical(n.Child, ctx)
		if err != nil {
			return nil, err
		}
		return NewWindowNode(child, n.WindowExprs), nil
	case *LogicalSort:
		child, err := LogicalToPhysical(n.Child, ctx)
		if err != nil {
			return nil, err
		}
		return NewSortNode(child, n.OrderBy), nil
	case *LogicalLimit:
		child, err := LogicalToPhysical(n.Child, ctx)
		if err != nil {
			return nil, err
		}
		return NewLimitNode(child, n.N), nil
	default:
		return nil, fmt.Errorf("unknown logical node type: %T", node)
	}
}

func expressionToString(expr sqlparse.Expression) string {
	switch v := expr.(type) {
	case *sqlparse.ComparisonExpr:
		return fmt.Sprintf("%s %s '%s'", v.Column, v.Operator, v.Value)
	case *sqlparse.LogicalExpr:
		return fmt.Sprintf("(%s %s %s)", expressionToString(v.Left), v.Operator, expressionToString(v.Right))
	case *sqlparse.NullTestExpr:
		if v.IsNull {
			return fmt.Sprintf("%s IS NULL", v.Column)
		}
		return fmt.Sprintf("%s IS NOT NULL", v.Column)
	case *sqlparse.InExpr:
		values := strings.Join(v.Values, ", ")
		if v.Not {
			return fmt.Sprintf("%s NOT IN (%s)", v.Column, values)
		}
		return fmt.Sprintf("%s IN (%s)", v.Column, values)
	case *sqlparse.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", expressionToString(v.Left), v.Operator, expressionToString(v.Right))
	case *sqlparse.ColumnRef:
		return v.Name
	default:
		return "unknown"
	}
}

func computeWindowFunctions(rows []storage.Row, windowExprs []sqlparse.WindowExpr) []storage.Row {
	// Sort rows into partitions
	partitions := partitionRows(rows, windowExprs)
	// Compute window functions per partition
	result := make([]storage.Row, 0, len(rows))
	for _, part := range partitions {
		computed := computePartitionWindow(part, windowExprs)
		result = append(result, computed...)
	}
	return result
}

func partitionRows(rows []storage.Row, windowExprs []sqlparse.WindowExpr) [][]storage.Row {
	// Collect all PARTITION BY columns from all window expressions
	partCols := make(map[string]bool)
	for _, w := range windowExprs {
		for _, p := range w.PartitionBy {
			partCols[p] = true
		}
	}
	if len(partCols) == 0 {
		// No PARTITION BY — whole result is one partition
		return [][]storage.Row{rows}
	}
	partColList := sortedMapKeys(partCols)
	// Group rows by partition key values
	groups := make(map[string][]storage.Row)
	for _, row := range rows {
		key := makePartitionKey(row, partColList)
		groups[key] = append(groups[key], row)
	}
	result := make([][]storage.Row, 0, len(groups))
	for _, g := range groups {
		result = append(result, g)
	}
	return result
}

func makePartitionKey(row storage.Row, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = row[c]
	}
	return strings.Join(parts, "\x00")
}

func sortedMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func computePartitionWindow(rows []storage.Row, windowExprs []sqlparse.WindowExpr) []storage.Row {
	if len(rows) == 0 {
		return rows
	}
	// Sort within partition by each window expression's ORDER BY
	for _, w := range windowExprs {
		if len(w.OrderBy) > 0 {
			sortRows(rows, w.OrderBy)
			break // use first window's ORDER BY for interleaving
		}
	}

	// Compute each window function and add result columns to rows
	for _, w := range windowExprs {
		if w.FuncName == "" {
			continue
		}
		colKey := w.FuncName + "(" + strings.Join(w.Args, ",") + ")"
		switch w.FuncName {
		case "ROW_NUMBER":
			for i := range rows {
				rows[i][colKey] = strconv.Itoa(i + 1)
			}
		case "RANK":
			computeRank(rows, colKey, false)
		case "DENSE_RANK":
			computeRank(rows, colKey, true)
		case "COUNT":
			computeWindowAgg(rows, colKey, w.Args, "COUNT")
		case "SUM":
			computeWindowAgg(rows, colKey, w.Args, "SUM")
		case "AVG":
			computeWindowAgg(rows, colKey, w.Args, "AVG")
		case "MIN":
			computeWindowAgg(rows, colKey, w.Args, "MIN")
		case "MAX":
			computeWindowAgg(rows, colKey, w.Args, "MAX")
		}
	}
	return rows
}

func computeRank(rows []storage.Row, colKey string, dense bool) {
	if len(rows) == 0 {
		return
	}
	rank := 1
	rows[0][colKey] = "1"
	for i := 1; i < len(rows); i++ {
		tied := rowsEqual(rows[i], rows[i-1])
		if !tied {
			if dense {
				rank++
			} else {
				rank = i + 1
			}
		}
		rows[i][colKey] = strconv.Itoa(rank)
	}
}

func rowsEqual(a, b storage.Row) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func computeWindowAgg(rows []storage.Row, colKey string, args []string, aggFunc string) {
	if len(args) == 0 || args[0] == "" {
		return
	}
	col := args[0]
	var total float64
	count := 0
	for _, row := range rows {
		val := row[col]
		if val == "" {
			continue
		}
		num, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		switch aggFunc {
		case "COUNT":
			count++
		case "SUM", "AVG", "MIN", "MAX":
			total += num
		}
		count++
	}
	countVal := count
	if aggFunc == "COUNT" {
		for i := range rows {
			rows[i][colKey] = strconv.Itoa(countVal)
		}
	} else if aggFunc == "SUM" {
		str := strconv.FormatFloat(total, 'f', -1, 64)
		for i := range rows {
			rows[i][colKey] = str
		}
	} else if aggFunc == "AVG" {
		avg := total / float64(count)
		str := strconv.FormatFloat(avg, 'f', -1, 64)
		for i := range rows {
			rows[i][colKey] = str
		}
	}
}
