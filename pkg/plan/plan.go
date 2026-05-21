package plan

import (
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
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
	Predicate string
}

func NewFilterNode(child PlanNode, predicate string) *FilterNode {
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
	return filterRowsParallel(rows, n.Predicate)
}

func (n *FilterNode) Explain(indent int) string {
	prefix := strings.Repeat("  ", indent)
	return fmt.Sprintf("%sFilter(%s)\n%s", prefix, n.Predicate, n.Child.Explain(indent+1))
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

func filterRowsParallel(rows []storage.Row, where string) ([]storage.Row, error) {
	if len(rows) == 0 || strings.TrimSpace(where) == "" {
		return rows, nil
	}
	predicate := parseSimplePredicate(where)
	if predicate == nil {
		return filterRows(rows, where), nil
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
				if predicate(row) {
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
	return result, nil
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

func parseSimplePredicate(where string) func(storage.Row) bool {
	re := regexp.MustCompile(`(?i)^(\w+)\s*=\s*'([^']*)'$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(where))
	if len(matches) != 3 {
		return nil
	}
	key := matches[1]
	value := matches[2]
	return func(row storage.Row) bool {
		return row[key] == value
	}
}

func filterRows(rows []storage.Row, where string) []storage.Row {
	predicate := parseSimplePredicate(where)
	if predicate == nil {
		return rows
	}
	filtered := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		if predicate(row) {
			filtered = append(filtered, row)
		}
	}
	return filtered
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
