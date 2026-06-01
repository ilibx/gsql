package plan

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

type AggregateDef struct {
	FuncName string
	Column   string
	Args     []string
	Distinct bool
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
	Predicate parser.Expression
}

func NewFilterNode(child PlanNode, predicate parser.Expression) *FilterNode {
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
	Exprs     []parser.Expression // parallel to Columns, nil means plain column ref
}

func NewProjectNode(child PlanNode, columns []string) *ProjectNode {
	return &ProjectNode{Child: child, Columns: columns}
}

func NewProjectNodeWithExprs(child PlanNode, columns []string, exprs []parser.Expression) *ProjectNode {
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

func projectRowsWithExprs(rows []storage.Row, columns []string, exprs []parser.Expression) []storage.Row {
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
	parts := make([]string, 0, len(n.Columns))
	for i, c := range n.Columns {
		if c == "" && i < len(n.Exprs) && n.Exprs[i] != nil {
			parts = append(parts, expressionToString(n.Exprs[i]))
		} else if c != "" {
			parts = append(parts, c)
		}
	}
	return fmt.Sprintf("%sProject(%s)\n%s", prefix, strings.Join(parts, ", "), n.Child.Explain(indent+1))
}

type WindowNode struct {
	Child       PlanNode
	WindowExprs []parser.WindowExpr
}

func NewWindowNode(child PlanNode, windowExprs []parser.WindowExpr) *WindowNode {
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
	OrderBy []parser.SortOrder
}

func NewSortNode(child PlanNode, orderBy []parser.SortOrder) *SortNode {
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
	if n.N >= len(rows) || n.N < 0 {
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
			distinctPrefix := ""
			if agg.Distinct {
				distinctPrefix = "DISTINCT "
			}
			colKey := agg.FuncName + "(" + distinctPrefix + agg.Column + ")"
			args := agg.Args
			if len(args) == 0 && agg.Column != "" {
				args = []string{agg.Column}
			}
			row[colKey] = computeAggregate(ge.group, agg.FuncName, agg.Column, agg.Distinct, args)
		}
		result = append(result, row)
	}
	return result, nil
}

func (n *AggregateNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	aggParts := make([]string, len(n.Aggregates))
	for i, agg := range n.Aggregates {
		if agg.Distinct {
			aggParts[i] = agg.FuncName + "(DISTINCT " + agg.Column + ")"
		} else {
			aggParts[i] = agg.FuncName + "(" + agg.Column + ")"
		}
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

type LateralViewExplodeNode struct {
	Child     PlanNode
	Col       string // column to explode
	Alias     string // output column alias (value for POSEXPLODE/EXPLODE(map))
	PosAlias  string // position column alias (for POSEXPLODE) or key alias (for EXPLODE(map))
	IsOuter   bool   // OUTER: keep row even if explode column is empty
	WithPos   bool   // true for POSEXPLODE (integer index)
	WithMap   bool   // true for EXPLODE(map) (key-value pairs)
}

func NewLateralViewExplodeNode(child PlanNode, col, alias string, isOuter ...bool) *LateralViewExplodeNode {
	outer := false
	if len(isOuter) > 0 {
		outer = isOuter[0]
	}
	return &LateralViewExplodeNode{Child: child, Col: col, Alias: alias, IsOuter: outer}
}

func (n *LateralViewExplodeNode) Type() string {
	if n.WithPos {
		return "LateralViewPosExplode"
	}
	return "LateralViewExplode"
}

func (n *LateralViewExplodeNode) Execute() ([]storage.Row, error) {
	input, err := n.Child.Execute()
	if err != nil {
		return nil, err
	}
	var result []storage.Row
	for _, row := range input {
		val := row[n.Col]
		var elements []string
		if val == "" {
			elements = nil
		} else {
			elements = strings.Split(val, ",")
		}
		if len(elements) == 0 {
			if n.IsOuter {
				newRow := make(storage.Row)
				for k, v := range row {
					newRow[k] = v
				}
				newRow[n.Alias] = ""
				if n.WithPos || n.WithMap {
					newRow[n.PosAlias] = ""
				}
				result = append(result, newRow)
			}
			continue
		}
		if n.WithMap {
			// EXPLODE(map): elements come as k1,v1,k2,v2,...
			for i := 0; i+1 < len(elements); i += 2 {
				newRow := make(storage.Row)
				for k, v := range row {
					newRow[k] = v
				}
				newRow[n.PosAlias] = strings.TrimSpace(elements[i])
				newRow[n.Alias] = strings.TrimSpace(elements[i+1])
				result = append(result, newRow)
			}
		} else if n.WithPos {
			// POSEXPLODE: index + value
			for idx, elem := range elements {
				newRow := make(storage.Row)
				for k, v := range row {
					newRow[k] = v
				}
				newRow[n.Alias] = strings.TrimSpace(elem)
				newRow[n.PosAlias] = strconv.Itoa(idx)
				result = append(result, newRow)
			}
		} else {
			// EXPLODE(arr): just values
			for _, elem := range elements {
				newRow := make(storage.Row)
				for k, v := range row {
					newRow[k] = v
				}
				newRow[n.Alias] = strings.TrimSpace(elem)
				result = append(result, newRow)
			}
		}
	}
	return result, nil
}

func (n *LateralViewExplodeNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	outer := ""
	if n.IsOuter {
		outer = " OUTER"
	}
	name := "LateralViewExplode"
	pos := ""
	if n.WithPos {
		name = "LateralViewPosExplode"
		pos = fmt.Sprintf(", pos=%s", n.PosAlias)
	} else if n.WithMap {
		name = "LateralViewExplodeMap"
		pos = fmt.Sprintf(", key=%s", n.PosAlias)
	}
	return fmt.Sprintf("%s%s%s(col=%s, alias=%s%s)\n%s", prefix, name, outer, n.Col, n.Alias, pos, n.Child.Explain(indent+1))
}

type JoinNode struct {
	Left         PlanNode
	Right        PlanNode
	LeftColumn   string
	RightColumn  string
	JoinType     string // INNER, LEFT, RIGHT, FULL, SEMI, CROSS
	NormalizeKey func(string) string // optional: normalizes join key values for type-aware comparison
}

func NewJoinNode(left, right PlanNode, leftCol, rightCol string) *JoinNode {
	return &JoinNode{Left: left, Right: right, LeftColumn: leftCol, RightColumn: rightCol, JoinType: "INNER"}
}

func NewJoinNodeWithType(left, right PlanNode, leftCol, rightCol, joinType string) *JoinNode {
	return &JoinNode{Left: left, Right: right, LeftColumn: leftCol, RightColumn: rightCol, JoinType: joinType}
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

	leftCol := n.LeftColumn
	rightCol := n.RightColumn

	normalize := n.NormalizeKey
	if normalize == nil {
		normalize = func(s string) string { return s }
	}

	if n.JoinType == "CROSS" {
		var result []storage.Row
		for _, lr := range leftRows {
			for _, rr := range rightRows {
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
		return result, nil
	}

	// Build hash on the right side
	hash := make(map[string][]storage.Row, len(rightRows))
	for _, row := range rightRows {
		key := normalize(row[rightCol])
		hash[key] = append(hash[key], row)
	}

	// Track which left and right keys were matched
	matchedKeys := make(map[string]bool)

	var result []storage.Row
	for _, lr := range leftRows {
		key := normalize(lr[leftCol])
		if matched, ok := hash[key]; ok {
			matchedKeys[key] = true
			if n.JoinType == "SEMI" {
				result = append(result, copyRow(lr))
				continue
			}
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
		} else if n.JoinType == "LEFT" || n.JoinType == "FULL" {
			result = append(result, copyRow(lr))
		}
	}

	// RIGHT JOIN / FULL JOIN: add unmatched right rows with NULL left
	if n.JoinType == "RIGHT" || n.JoinType == "FULL" {
		for key, rows := range hash {
			if matchedKeys[key] {
				continue
			}
			for _, rr := range rows {
				result = append(result, copyRow(rr))
			}
		}
	}

	return result, nil
}

func copyRow(row storage.Row) storage.Row {
	r := make(storage.Row, len(row))
	for k, v := range row {
		r[k] = v
	}
	return r
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

func filterRowsParallel(rows []storage.Row, expr parser.Expression) []storage.Row {
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
			defer func() {
				if r := recover(); r != nil {
					resultCh <- segmentResult{index: idx, rows: nil}
				}
			}()
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

func sortRows(rows []storage.Row, orderBy []parser.SortOrder) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, o := range orderBy {
			vi, vj := rows[i][o.Column], rows[j][o.Column]
			cmp := compareNumeric(vi, vj)
			if cmp != 0 {
				if o.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
}

// compareNumeric compares two string values. If both can be parsed as numbers,
// it compares numerically. Otherwise it compares lexicographically.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareNumeric(a, b string) int {
	if a == b {
		return 0
	}
	fa, errA := strconv.ParseFloat(a, 64)
	fb, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		if fa < fb {
			return -1
		}
		if fa > fb {
			return 1
		}
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func evaluateExpression(row storage.Row, expr parser.Expression) bool {
	switch v := expr.(type) {
	case *parser.ComparisonExpr:
		var left string
		if v.Expr != nil {
			left = evaluateExpressionValue(row, v.Expr)
		} else if v.Column == "" {
			left = v.Value
		} else if strings.Contains(v.Column, "(") {
			// function call result stored in Expr, or compute from column name
			if v.Expr != nil {
				left = evaluateExpressionValue(row, v.Expr)
			} else {
				left = row[stripColAlias(v.Column)]
			}
		} else {
			left = row[stripColAlias(v.Column)]
		}
		var right string
		if v.RightColumn != "" {
			right = row[stripColAlias(v.RightColumn)]
		} else {
			right = v.Value
		}
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
	case *parser.FuncCallExpr:
		val := evaluateExpressionValue(row, v)
		return val == "true" || val == "TRUE" || val == "1"
	case *parser.LogicalExpr:
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
	case *parser.NullTestExpr:
		val := row[stripColAlias(v.Column)]
		if v.IsNull {
			return val == ""
		}
		return val != ""
	case *parser.InExpr:
		val := row[stripColAlias(v.Column)]
		if v.Not {
			return !stringInSlice(val, v.Values)
		}
		return stringInSlice(val, v.Values)
	case *parser.BinaryExpr:
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

func stripColAlias(col string) string {
	if idx := strings.IndexByte(col, '.'); idx >= 0 {
		// ensure it's not a decimal number
		if idx > 0 && col[idx-1] >= '0' && col[idx-1] <= '9' && idx+1 < len(col) && col[idx+1] >= '0' && col[idx+1] <= '9' {
			return col
		}
		return col[idx+1:]
	}
	return col
}

func evaluateExpressionValue(row storage.Row, expr parser.Expression) string {
	switch v := expr.(type) {
	case *parser.ColumnRef:
		return row[stripColAlias(v.Name)]
	case *parser.ComparisonExpr:
		if v.Expr != nil {
			return evaluateExpressionValue(row, v.Expr)
		}
		if v.RightColumn != "" {
			return row[stripColAlias(v.RightColumn)]
		}
		if v.Column == "" {
			return v.Value
		}
		return row[stripColAlias(v.Column)]
	case *parser.NullTestExpr:
		val := row[stripColAlias(v.Column)]
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
	case *parser.FuncCallExpr:
		if fn, ok := LookupFunc(v.FuncName); ok && fn.ScalarFn != nil {
			args := make([]string, len(v.Args))
			for i, a := range v.Args {
				// Resolve column references; pass through literals and type names
				if rowVal, exists := row[a]; exists {
					args[i] = rowVal
				} else {
					args[i] = a
				}
			}
			return fn.ScalarFn(args)
		}
		return ""
	case *parser.LogicalExpr:
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
	case *parser.CaseExpr:
		for _, branch := range v.Branches {
			if evaluateExpression(row, branch.Condition) {
				return evaluateExpressionValue(row, branch.Result)
			}
		}
		if v.Else != nil {
			return evaluateExpressionValue(row, v.Else)
		}
		return ""
	case *parser.LiteralExpr:
		return v.Value
	case *parser.BinaryExpr:
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
					return ""
				}
				return formatFloat(leftNum / rightNum)
			}
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

type PhysicalPlanContext struct {
	Catalog *catalog.Catalog
	CTEData map[string][]storage.Row
}

// extractPartitionFilters walks an expression tree and extracts equality
// and range comparisons on partition columns. Returns the remaining predicate
// (with those comparisons removed) and the extracted partition filters.
// Handles "=", ">", "<", ">=", "<=" and AND-conjoined conditions.
func extractPartitionFilters(expr parser.Expression, partitionCols []string) (parser.Expression, []storage.PartitionFilter) {
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

	var rebuildExpr func(parser.Expression) parser.Expression
	rebuildExpr = func(e parser.Expression) parser.Expression {
		if e == nil {
			return nil
		}
		switch v := e.(type) {
		case *parser.ComparisonExpr:
			if v.RightColumn == "" && isPartitionOp(v.Operator) && isPartition(v.Column) {
				filters = append(filters, storage.PartitionFilter{Column: v.Column, Operator: v.Operator, Value: v.Value})
				return nil
			}
			return v
		case *parser.LogicalExpr:
			if v.Operator != "AND" {
				return v
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
			return &parser.LogicalExpr{Left: left, Operator: "AND", Right: right}
		default:
			return v
		}
	}

	remaining := rebuildExpr(expr)
	return remaining, filters
}

func isPartitionOp(op string) bool {
	switch op {
	case "=", ">", "<", ">=", "<=":
		return true
	default:
		return false
	}
}

func LogicalToPhysical(node LogicalNode, ctx *PhysicalPlanContext) (PlanNode, error) {
	switch n := node.(type) {
	case *LogicalScan:
		if n.TableName == "" {
			// No FROM clause — single empty row (e.g., SELECT 1 AS x)
			return NewCTETableScanNode("", []storage.Row{{}}), nil
		}
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
		jn := NewJoinNodeWithType(left, right, n.LeftColumn, n.RightColumn, n.JoinType)
		jn.NormalizeKey = n.NormalizeKey
		return jn, nil
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

func expressionToString(expr parser.Expression) string {
	switch v := expr.(type) {
	case *parser.ComparisonExpr:
		if v.Expr != nil {
			return fmt.Sprintf("%s %s %s", expressionToString(v.Expr), v.Operator, v.Value)
		}
		if v.RightColumn != "" {
			return fmt.Sprintf("%s %s %s", v.Column, v.Operator, v.RightColumn)
		}
		return fmt.Sprintf("%s %s '%s'", v.Column, v.Operator, v.Value)
	case *parser.LogicalExpr:
		return fmt.Sprintf("(%s %s %s)", expressionToString(v.Left), v.Operator, expressionToString(v.Right))
	case *parser.NullTestExpr:
		if v.IsNull {
			return fmt.Sprintf("%s IS NULL", v.Column)
		}
		return fmt.Sprintf("%s IS NOT NULL", v.Column)
	case *parser.InExpr:
		values := strings.Join(v.Values, ", ")
		if v.Not {
			return fmt.Sprintf("%s NOT IN (%s)", v.Column, values)
		}
		return fmt.Sprintf("%s IN (%s)", v.Column, values)
	case *parser.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", expressionToString(v.Left), v.Operator, expressionToString(v.Right))
	case *parser.ColumnRef:
		return v.Name
	case *parser.LiteralExpr:
		return v.Value
	default:
		return "unknown"
	}
}

func computeWindowFunctions(rows []storage.Row, windowExprs []parser.WindowExpr) []storage.Row {
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

func partitionRows(rows []storage.Row, windowExprs []parser.WindowExpr) [][]storage.Row {
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

func computePartitionWindow(rows []storage.Row, windowExprs []parser.WindowExpr) []storage.Row {
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
		argStr := ""
		if len(w.Args) > 0 {
			argStr = w.Args[0]
		}
		colKey := w.FuncName + "(" + argStr + ")"
		switch w.FuncName {
		case "COUNT":
			computeWindowAggFrame(rows, colKey, w.Args, "COUNT", w.Frame, w.OrderBy)
		case "SUM":
			computeWindowAggFrame(rows, colKey, w.Args, "SUM", w.Frame, w.OrderBy)
		case "AVG":
			computeWindowAggFrame(rows, colKey, w.Args, "AVG", w.Frame, w.OrderBy)
		case "MIN":
			computeWindowAggFrame(rows, colKey, w.Args, "MIN", w.Frame, w.OrderBy)
		case "MAX":
			computeWindowAggFrame(rows, colKey, w.Args, "MAX", w.Frame, w.OrderBy)
		default:
			// Lookup registered window functions (LEAD, LAG, NTILE, etc.)
			if fn, ok := LookupFunc(w.FuncName); ok && fn.WindowFn != nil {
				rows = fn.WindowFn(rows, w.Args, w.OrderBy, colKey)
			}
		}
	}
	return rows
}

func computeRank(rows []storage.Row, colKey string, orderBy []parser.SortOrder, dense bool) {
	if len(rows) == 0 {
		return
	}
	rank := 1
	rows[0][colKey] = "1"
	for i := 1; i < len(rows); i++ {
		tied := rowsEqualByOrder(rows[i], rows[i-1], orderBy)
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

func rowsEqualByOrder(a, b storage.Row, orderBy []parser.SortOrder) bool {
	if len(orderBy) == 0 {
		return false
	}
	for _, o := range orderBy {
		if a[o.Column] != b[o.Column] {
			return false
		}
	}
	return true
}

func computeWindowAggFrame(rows []storage.Row, colKey string, args []string, aggFunc string, frame *parser.WindowFrame, orderBy []parser.SortOrder) {
	if len(args) == 0 || args[0] == "" {
		return
	}
	col := args[0]
	// Set ORDER BY column on frame for RANGE frame calculations
	if frame != nil && len(orderBy) > 0 {
		frame.OrderBy = orderBy[0].Column
	}
	for i := range rows {
		start, end := getFrameRange(rows, i, frame)
		var total float64
		count := 0
		minVal := ""
		maxVal := ""
		for j := start; j <= end && j < len(rows); j++ {
			val := rows[j][col]
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
			case "SUM", "AVG":
				total += num
				count++
			case "MIN":
				if minVal == "" || compareValues(val, minVal, "<") {
					minVal = val
				}
			case "MAX":
				if maxVal == "" || compareValues(val, maxVal, ">") {
					maxVal = val
				}
			}
		}
		switch aggFunc {
		case "COUNT":
			rows[i][colKey] = strconv.Itoa(count)
		case "SUM":
			rows[i][colKey] = strconv.FormatFloat(total, 'f', -1, 64)
		case "AVG":
			if count == 0 {
				rows[i][colKey] = "0"
			} else {
				rows[i][colKey] = strconv.FormatFloat(total/float64(count), 'f', -1, 64)
			}
		case "MIN":
			rows[i][colKey] = minVal
		case "MAX":
			rows[i][colKey] = maxVal
		}
	}
}

func getFrameRange(rows []storage.Row, currentIdx int, frame *parser.WindowFrame) (int, int) {
	n := len(rows)
	if frame == nil {
		return 0, n - 1
	}
	if frame.FrameType == parser.FrameRows || frame.OrderBy == "" {
		// ROWS: physical offsets (default if no ORDER BY)
		start := resolveFrameBoundRow(n, currentIdx, frame.Start)
		end := resolveFrameBoundRow(n, currentIdx, frame.End)
		if start < 0 {
			start = 0
		}
		if end >= n {
			end = n - 1
		}
		return start, end
	}
	// RANGE: logical offsets based on ORDER BY value
	return resolveFrameRange(rows, currentIdx, frame)
}

func resolveFrameBoundRow(n, currentIdx int, bound parser.WindowFrameBoundary) int {
	switch bound.BoundType {
	case parser.FrameUnboundedPreceding:
		return 0
	case parser.FrameNPreceding:
		return currentIdx - bound.N
	case parser.FrameCurrentRow:
		return currentIdx
	case parser.FrameNFollowing:
		return currentIdx + bound.N
	case parser.FrameUnboundedFollowing:
		return n - 1
	default:
		return 0
	}
}

func resolveFrameRange(rows []storage.Row, currentIdx int, frame *parser.WindowFrame) (int, int) {
	n := len(rows)
	if n == 0 {
		return 0, 0
	}

	currentVal, err := strconv.ParseFloat(rows[currentIdx][frame.OrderBy], 64)
	if err != nil {
		return 0, n - 1 // fallback to full range if value is not numeric
	}

	start := 0
	end := n - 1

	// Resolve start boundary
	switch frame.Start.BoundType {
	case parser.FrameUnboundedPreceding:
		start = 0
	case parser.FrameNPreceding:
		threshold := currentVal - float64(frame.Start.N)
		start = findRangeStart(rows, frame.OrderBy, threshold)
	case parser.FrameCurrentRow:
		start = findRangeStart(rows, frame.OrderBy, currentVal)
	case parser.FrameNFollowing:
		threshold := currentVal + float64(frame.Start.N)
		start = findRangeStart(rows, frame.OrderBy, threshold)
	case parser.FrameUnboundedFollowing:
		start = 0 // start from beginning, but also check end below
	default:
		start = 0
	}

	// Resolve end boundary
	switch frame.End.BoundType {
	case parser.FrameUnboundedPreceding:
		end = findRangeEnd(rows, frame.OrderBy, currentVal-float64(frame.End.N)) // shouldn't happen
	case parser.FrameNPreceding:
		threshold := currentVal - float64(frame.End.N)
		end = findRangeEnd(rows, frame.OrderBy, threshold)
	case parser.FrameCurrentRow:
		end = findRangeEnd(rows, frame.OrderBy, currentVal)
	case parser.FrameNFollowing:
		threshold := currentVal + float64(frame.End.N)
		end = findRangeEnd(rows, frame.OrderBy, threshold)
	case parser.FrameUnboundedFollowing:
		end = n - 1
	default:
		end = n - 1
	}

	if start < 0 {
		start = 0
	}
	if end >= n {
		end = n - 1
	}
	if start > end {
		start = end
	}
	return start, end
}

func findRangeStart(rows []storage.Row, orderCol string, threshold float64) int {
	for i, row := range rows {
		val, err := strconv.ParseFloat(row[orderCol], 64)
		if err != nil {
			continue
		}
		if val >= threshold {
			return i
		}
	}
	return 0
}

func findRangeEnd(rows []storage.Row, orderCol string, threshold float64) int {
	for i := len(rows) - 1; i >= 0; i-- {
		val, err := strconv.ParseFloat(rows[i][orderCol], 64)
		if err != nil {
			continue
		}
		if val <= threshold {
			return i
		}
	}
	return len(rows) - 1
}
