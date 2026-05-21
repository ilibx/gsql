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

type PlanNode interface {
	Type() string
	Execute() ([]storage.Row, error)
	Explain(indent int) string
}

type TableScanNode struct {
	Table   *catalog.Table
	CTE     []storage.Row
	CTEName string
}

func NewTableScanNode(table *catalog.Table) *TableScanNode {
	return &TableScanNode{Table: table}
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
	return storage.ReadTableRows(n.Table)
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
	Child   PlanNode
	Columns []string
}

func NewProjectNode(child PlanNode, columns []string) *ProjectNode {
	return &ProjectNode{Child: child, Columns: columns}
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
	return projectRowsParallel(rows, n.Columns), nil
}

func (n *ProjectNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	return fmt.Sprintf("%sProject(%s)\n%s", prefix, strings.Join(n.Columns, ", "), n.Child.Explain(indent+1))
}

type SortNode struct {
	Child   PlanNode
	OrderBy []string
}

func NewSortNode(child PlanNode, orderBy []string) *SortNode {
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
	return fmt.Sprintf("%sSort(%s)\n%s", prefix, strings.Join(n.OrderBy, ", "), n.Child.Explain(indent+1))
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

func sortRows(rows []storage.Row, orderBy []string) {
	if len(orderBy) == 0 {
		return
	}
	key := orderBy[0]
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i][key] < rows[j][key]
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
	default:
		return false
	}
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

func expressionToString(expr sqlparse.Expression) string {
	switch v := expr.(type) {
	case *sqlparse.ComparisonExpr:
		return fmt.Sprintf("%s %s '%s'", v.Column, v.Operator, v.Value)
	default:
		return "unknown"
	}
}
