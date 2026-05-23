package plan

import (
	"math"
	"sort"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/parser"
)

type OptimizeRule func(LogicalNode) LogicalNode

type Optimizer struct {
	rules []OptimizeRule
}

func NewOptimizer(rules ...OptimizeRule) *Optimizer {
	return &Optimizer{rules: rules}
}

func DefaultOptimizer() *Optimizer {
	return NewOptimizer(MergeFilters)
}

// CatalogOptimizer returns an optimizer that includes cost-based rules
// using table statistics from the given catalog.
func CatalogOptimizer(cat *catalog.Catalog) *Optimizer {
	return NewOptimizer(MergeFilters, CostBasedJoinReorder(cat))
}

// MergeFilters merges adjacent LogicalFilter nodes into one conjunction.
// Filter(A) -> Filter(B) -> child  becomes  Filter(A AND B) -> child.
func MergeFilters(node LogicalNode) LogicalNode {
	filter, ok := node.(*LogicalFilter)
	if !ok {
		return node
	}
	innerFilter, ok := filter.Child.(*LogicalFilter)
	if !ok {
		return node
	}
	merged := &parser.LogicalExpr{
		Left:     innerFilter.Predicate,
		Operator: "AND",
		Right:    filter.Predicate,
	}
	return NewLogicalFilter(innerFilter.Child, merged)
}

func collectTableNames(node LogicalNode, names map[string]bool) {
	switch n := node.(type) {
	case *LogicalScan:
		names[n.TableName] = true
	}
	for _, child := range node.Children() {
		collectTableNames(child, names)
	}
}

// EstimateRows returns a best-guess row count for the subtree rooted at node.
// It uses catalog table statistics for scans and applies heuristic selectivity
// for operators (filter, join, etc.).  Returns math.Inf(1) when unknown.
func EstimateRows(node LogicalNode, tables map[string]*catalog.Table) float64 {
	switch n := node.(type) {
	case *LogicalScan:
		if t, ok := tables[n.TableName]; ok && t.EstimatedRows > 0 {
			return float64(t.EstimatedRows)
		}
		return math.Inf(1) // unknown
	case *LogicalCTEScan:
		return 100 // arbitrary default for CTEs
	case *LogicalFilter:
		childRows := EstimateRows(n.Child, tables)
		if math.IsInf(childRows, 1) {
			return childRows
		}
		return math.Max(1, childRows*0.2) // assume 20% selectivity
	case *LogicalProject:
		return EstimateRows(n.Child, tables)
	case *LogicalJoin:
		leftRows := EstimateRows(n.Left, tables)
		rightRows := EstimateRows(n.Right, tables)
		if math.IsInf(leftRows, 1) || math.IsInf(rightRows, 1) {
			return math.Inf(1)
		}
		return math.Max(leftRows, rightRows) // assume many-to-many
	case *LogicalAggregate:
		childRows := EstimateRows(n.Child, tables)
		if math.IsInf(childRows, 1) {
			return childRows
		}
		if len(n.GroupBy) > 0 {
			return math.Max(1, float64(len(n.GroupBy))*10)
		}
		return 1
	case *LogicalWindow:
		return EstimateRows(n.Child, tables)
	case *LogicalSort:
		return EstimateRows(n.Child, tables)
	case *LogicalLimit:
		return float64(n.N)
	default:
		return math.Inf(1)
	}
}

// CostBasedJoinReorder is an optimizer rule that reorders join children
// so the smaller (estimated) input is placed on the right side (build side
// for hash join). It traverses the tree bottom-up.
func CostBasedJoinReorder(cat *catalog.Catalog) OptimizeRule {
	tables := make(map[string]*catalog.Table)
	if cat != nil {
		for _, name := range cat.TableNames() {
			if t, ok := cat.GetTable(name); ok {
				tables[name] = t
			}
		}
	}
	return func(node LogicalNode) LogicalNode {
		return reorderJoins(node, tables)
	}
}

func reorderJoins(node LogicalNode, tables map[string]*catalog.Table) LogicalNode {
	// Recurse into children first (bottom-up)
	children := node.Children()
	if len(children) > 0 {
		changed := make([]LogicalNode, len(children))
		for i, c := range children {
			changed[i] = reorderJoins(c, tables)
		}
		node = rebuildNode(node, changed)
	}

	join, ok := node.(*LogicalJoin)
	if !ok {
		return node
	}

	leftRows := EstimateRows(join.Left, tables)
	rightRows := EstimateRows(join.Right, tables)

	// If both sides have known costs and right is larger than left, swap.
	// Hash join benefits from the smaller side being the build (right) side.
	if !math.IsInf(leftRows, 1) && !math.IsInf(rightRows, 1) && rightRows > leftRows {
		return NewLogicalJoin(join.Right, join.Left, join.RightColumn, join.LeftColumn)
	}
	return node
}

func (opt *Optimizer) Optimize(node LogicalNode) LogicalNode {
	for _, rule := range opt.rules {
		node = rule(node)
	}
	return walkAndOptimize(node, opt)
}

func (opt *Optimizer) OptimizeWithPruning(node LogicalNode) LogicalNode {
	node = opt.Optimize(node)
	return ColumnPruning(node)
}

// ColumnPruning adds Project nodes on top of LogicalScan to prune
// columns that are not referenced anywhere in the plan tree.
// Must be called on the ROOT node (not recursively on children).
func ColumnPruning(node LogicalNode) LogicalNode {
	needed := collectAllColumnRefs(node)
	return injectPruneProjects(node, needed)
}

// collectAllColumnRefs walks the tree and returns all column names
// referenced in any expression, projection, sort, or aggregate.
func collectAllColumnRefs(node LogicalNode) map[string]bool {
	cols := make(map[string]bool)
	collectColumnRefs(node, cols)
	return cols
}

func collectColumnRefs(node LogicalNode, cols map[string]bool) {
	switch n := node.(type) {
	case *LogicalFilter:
		collectExprCols(n.Predicate, cols)
	case *LogicalProject:
		for _, c := range n.Columns {
			cols[c] = true
		}
		for _, e := range n.Exprs {
			collectExprCols(e, cols)
		}
	case *LogicalSort:
		for _, o := range n.OrderBy {
			cols[o.Column] = true
		}
	case *LogicalWindow:
		for _, w := range n.WindowExprs {
			for _, a := range w.Args {
				if a != "*" {
					cols[a] = true
				}
			}
			for _, p := range w.PartitionBy {
				cols[p] = true
			}
			for _, o := range w.OrderBy {
				cols[o.Column] = true
			}
		}
	case *LogicalAggregate:
		for _, g := range n.GroupBy {
			cols[g] = true
		}
		for _, a := range n.Aggregates {
			if a.Column != "*" {
				cols[a.Column] = true
			}
		}
	case *LogicalJoin:
		cols[n.LeftColumn] = true
		cols[n.RightColumn] = true
	}
	for _, child := range node.Children() {
		collectColumnRefs(child, cols)
	}
}

func collectExprCols(expr parser.Expression, cols map[string]bool) {
	if expr == nil {
		return
	}
	switch v := expr.(type) {
	case *parser.ComparisonExpr:
		cols[v.Column] = true
	case *parser.LogicalExpr:
		collectExprCols(v.Left, cols)
		collectExprCols(v.Right, cols)
	case *parser.NullTestExpr:
		cols[v.Column] = true
	case *parser.InExpr:
		cols[v.Column] = true
	case *parser.BinaryExpr:
		collectExprCols(v.Left, cols)
		collectExprCols(v.Right, cols)
	case *parser.ColumnRef:
		cols[v.Name] = true
	case *parser.CaseExpr:
		for _, b := range v.Branches {
			collectExprCols(b.Condition, cols)
			collectExprCols(b.Result, cols)
		}
		collectExprCols(v.Else, cols)
	}
}

func injectPruneProjects(node LogicalNode, needed map[string]bool) LogicalNode {
	switch node.(type) {
	case *LogicalScan:
		if len(needed) == 0 {
			return node
		}
		cols := sortedUniqueKeys(needed)
		return NewLogicalProject(node, cols, nil)
	case *LogicalJoin:
		// Skip pruning under joins — without schema info we can't
		// determine which columns belong to which side.
		return node
	case *LogicalProject:
		children := node.Children()
		changed := false
		newChildren := make([]LogicalNode, len(children))
		for i, c := range children {
			newChildren[i] = injectPruneProjects(c, needed)
			if newChildren[i] != c {
				changed = true
			}
		}
		if !changed {
			return node
		}
		return rebuildNode(node, newChildren)
	default:
		children := node.Children()
		if len(children) == 0 {
			return node
		}
		changed := false
		newChildren := make([]LogicalNode, len(children))
		for i, c := range children {
			newChildren[i] = injectPruneProjects(c, needed)
			if newChildren[i] != c {
				changed = true
			}
		}
		if !changed {
			return node
		}
		return rebuildNode(node, newChildren)
	}
}

func sortedUniqueKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func walkAndOptimize(node LogicalNode, opt *Optimizer) LogicalNode {
	children := node.Children()
	if len(children) == 0 {
		return node
	}

	changed := make([]LogicalNode, len(children))
	for i, child := range children {
		changed[i] = opt.Optimize(child)
	}

	return rebuildNode(node, changed)
}

func rebuildNode(node LogicalNode, children []LogicalNode) LogicalNode {
	switch n := node.(type) {
	case *LogicalFilter:
		if len(children) != 1 {
			return node
		}
		return NewLogicalFilter(children[0], n.Predicate)
	case *LogicalProject:
		if len(children) != 1 {
			return node
		}
		return NewLogicalProject(children[0], n.Columns, n.Exprs)
	case *LogicalJoin:
		if len(children) != 2 {
			return node
		}
		return NewLogicalJoin(children[0], children[1], n.LeftColumn, n.RightColumn)
	case *LogicalAggregate:
		if len(children) != 1 {
			return node
		}
		return NewLogicalAggregate(children[0], n.GroupBy, n.Aggregates)
	case *LogicalWindow:
		if len(children) != 1 {
			return node
		}
		return NewLogicalWindow(children[0], n.WindowExprs)
	case *LogicalSort:
		if len(children) != 1 {
			return node
		}
		return NewLogicalSort(children[0], n.OrderBy)
	case *LogicalLimit:
		if len(children) != 1 {
			return node
		}
		return NewLogicalLimit(children[0], n.N)
	default:
		return node
	}
}
