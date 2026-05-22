package plan

import (
	"math"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/sqlparse"
)

func TestExplainLogical(t *testing.T) {
	scan := NewLogicalScan("users", "u")
	filter := NewLogicalFilter(scan, nil)
	proj := NewLogicalProject(filter, []string{"name", "age"}, nil)

	explain := ExplainLogical(proj, 0)
	if !strings.Contains(explain, "LogicalProject") {
		t.Errorf("expected LogicalProject in explain, got: %s", explain)
	}
	if !strings.Contains(explain, "LogicalFilter") {
		t.Errorf("expected LogicalFilter in explain, got: %s", explain)
	}
	if !strings.Contains(explain, "LogicalScan") {
		t.Errorf("expected LogicalScan in explain, got: %s", explain)
	}
}

func TestOptimizerPassThrough(t *testing.T) {
	scan := NewLogicalScan("users", "")
	proj := NewLogicalProject(scan, []string{"name"}, nil)

	opt := DefaultOptimizer()
	optimized := opt.Optimize(proj)

	if optimized.Type() != "LogicalProject" {
		t.Errorf("expected LogicalProject, got %s", optimized.Type())
	}
	children := optimized.Children()
	if len(children) != 1 || children[0].Type() != "LogicalScan" {
		t.Errorf("expected LogicalScan child, got %v", children)
	}
}

func TestLogicalJoinPlan(t *testing.T) {
	leftScan := NewLogicalScan("users", "u")
	rightScan := NewLogicalScan("orders", "o")
	join := NewLogicalJoin(leftScan, rightScan, "id", "user_id")
	proj := NewLogicalProject(join, []string{"name", "amount"}, nil)

	explain := ExplainLogical(proj, 0)
	if !strings.Contains(explain, "LogicalJoin") {
		t.Errorf("expected LogicalJoin in explain, got: %s", explain)
	}
	if !strings.Contains(explain, "id = user_id") {
		t.Errorf("expected join condition in explain, got: %s", explain)
	}
}

func TestColumnPruning(t *testing.T) {
	// Build: Filter(age > 0) -> Scan(users)
	filter := NewLogicalFilter(
		NewLogicalScan("users", ""),
		&sqlparse.ComparisonExpr{Column: "age", Operator: ">", Value: "0"},
	)

	opt := DefaultOptimizer()
	optimized := opt.OptimizeWithPruning(filter)

	// After pruning: Filter(age > 0) -> Project([age]) -> Scan(users)
	if optimized.Type() != "LogicalFilter" {
		t.Fatalf("expected LogicalFilter, got %s", optimized.Type())
	}
	proj := optimized.Children()[0]
	if proj.Type() != "LogicalProject" {
		t.Fatalf("expected LogicalProject on top of Scan, got %s", proj.Type())
	}
	projNode := proj.(*LogicalProject)
	if len(projNode.Columns) != 1 || projNode.Columns[0] != "age" {
		t.Errorf("expected Project([age]), got %v", projNode.Columns)
	}
	if projNode.Children()[0].Type() != "LogicalScan" {
		t.Errorf("expected LogicalScan under Project, got %s", projNode.Children()[0].Type())
	}
}

func TestColumnPruningMultipleRefs(t *testing.T) {
	// Build: Sort(id, name DESC) -> Aggregate(SUM(amount), GROUP BY name) -> Filter(age > 0) -> Scan
	agg := NewLogicalAggregate(
		NewLogicalFilter(
			NewLogicalScan("orders", ""),
			&sqlparse.ComparisonExpr{Column: "age", Operator: ">", Value: "0"},
		),
		[]string{"name"},
		[]AggregateDef{{FuncName: "SUM", Column: "amount"}},
	)
	sort := NewLogicalSort(agg, []sqlparse.SortOrder{{Column: "id"}, {Column: "name", Desc: true}})

	opt := DefaultOptimizer()
	optimized := opt.OptimizeWithPruning(sort)

	// Walk the tree to find the pruned Project above Scan
	var hasPrunedProject bool
	var walk func(LogicalNode) bool
	walk = func(n LogicalNode) bool {
		if proj, ok := n.(*LogicalProject); ok {
			if len(proj.Children()) > 0 {
				if _, ok := proj.Children()[0].(*LogicalScan); ok {
					hasPrunedProject = true
					colSet := make(map[string]bool)
					for _, c := range proj.Columns {
						colSet[c] = true
					}
					for _, needed := range []string{"age", "name", "amount", "id"} {
						if !colSet[needed] {
							t.Errorf("pruned Project missing column %s, got %v", needed, proj.Columns)
						}
					}
					return true
				}
			}
		}
		for _, child := range n.Children() {
			if walk(child) {
				return true
			}
		}
		return false
	}
	walk(optimized)
	if !hasPrunedProject {
		t.Error("expected a pruned Project above Scan")
	}
}

func TestMergeFilters(t *testing.T) {
	// Build: Filter(A) -> Filter(B) -> Scan
	condA := &sqlparse.ComparisonExpr{Column: "a", Operator: "=", Value: "1"}
	condB := &sqlparse.ComparisonExpr{Column: "b", Operator: ">", Value: "0"}
	innerFilter := NewLogicalFilter(NewLogicalScan("t", ""), condB)
	outerFilter := NewLogicalFilter(innerFilter, condA)

	opt := DefaultOptimizer()
	optimized := opt.OptimizeWithPruning(outerFilter)

	// After merge: Filter(A AND B) -> Project(cols) -> Scan
	filter, ok := optimized.(*LogicalFilter)
	if !ok {
		t.Fatalf("expected LogicalFilter, got %T", optimized)
	}
	logical, ok := filter.Predicate.(*sqlparse.LogicalExpr)
	if !ok {
		t.Fatalf("expected LogicalExpr (merged), got %T", filter.Predicate)
	}
	if logical.Operator != "AND" {
		t.Errorf("expected AND, got %s", logical.Operator)
	}
	if filter.Child.Type() != "LogicalProject" {
		t.Errorf("expected child to be LogicalProject (pruned), got %s", filter.Child.Type())
	}
	// Verify the Project wraps the Scan
	if len(filter.Child.Children()) > 0 && filter.Child.Children()[0].Type() != "LogicalScan" {
		t.Errorf("expected Project -> Scan, got Project -> %s", filter.Child.Children()[0].Type())
	}
}

func TestLogicalToPhysicalConversion(t *testing.T) {
	scan := NewLogicalScan("users", "u")
	filter := NewLogicalFilter(scan, nil)
	proj := NewLogicalProject(filter, []string{"name"}, nil)

	cat := catalog.NewCatalog()
	ctx := &PhysicalPlanContext{
		Catalog: cat,
		CTEData: nil,
	}

	_, err := LogicalToPhysical(proj, ctx)
	if err == nil {
		t.Error("expected error because table 'users' does not exist, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error mentioning 'not found', got: %v", err)
	}
}

func TestExtractPartitionFilters(t *testing.T) {
	partitionCols := []string{"dt", "year"}

	t.Run("simple equality", func(t *testing.T) {
		expr := &sqlparse.ComparisonExpr{Column: "dt", Operator: "=", Value: "2024-01-01"}
		remaining, filters := extractPartitionFilters(expr, partitionCols)
		if len(filters) != 1 || filters[0].Column != "dt" || filters[0].Value != "2024-01-01" {
			t.Errorf("unexpected filters: %v", filters)
		}
		if remaining != nil {
			t.Errorf("expected nil remaining, got %v", remaining)
		}
	})

	t.Run("non-partition equality", func(t *testing.T) {
		expr := &sqlparse.ComparisonExpr{Column: "name", Operator: "=", Value: "alice"}
		remaining, filters := extractPartitionFilters(expr, partitionCols)
		if len(filters) != 0 {
			t.Errorf("expected 0 filters, got %d", len(filters))
		}
		if remaining != expr {
			t.Errorf("expected expr to be preserved unchanged")
		}
	})

	t.Run("AND with partition equality", func(t *testing.T) {
		nameEq := &sqlparse.ComparisonExpr{Column: "name", Operator: "=", Value: "alice"}
		dtEq := &sqlparse.ComparisonExpr{Column: "dt", Operator: "=", Value: "2024-01-01"}
		expr := &sqlparse.LogicalExpr{Left: nameEq, Operator: "AND", Right: dtEq}
		remaining, filters := extractPartitionFilters(expr, partitionCols)
		if len(filters) != 1 || filters[0].Column != "dt" {
			t.Errorf("expected 1 dt filter, got %v", filters)
		}
		if remaining == nil {
			t.Fatal("expected remaining predicate (name equality)")
		}
		comp, ok := remaining.(*sqlparse.ComparisonExpr)
		if !ok || comp.Column != "name" {
			t.Errorf("expected remaining name comparison, got %T %v", remaining, remaining)
		}
	})

	t.Run("non-equality operator", func(t *testing.T) {
		expr := &sqlparse.ComparisonExpr{Column: "dt", Operator: ">", Value: "2024-01-01"}
		remaining, filters := extractPartitionFilters(expr, partitionCols)
		if len(filters) != 0 {
			t.Errorf("expected 0 filters for non-equality, got %d", len(filters))
		}
		if remaining != expr {
			t.Errorf("expected expr to be preserved")
		}
	})

	t.Run("nil expr", func(t *testing.T) {
		remaining, filters := extractPartitionFilters(nil, partitionCols)
		if len(filters) != 0 || remaining != nil {
			t.Errorf("expected nil remaining and 0 filters")
		}
	})
}

func TestCostBasedJoinReorder(t *testing.T) {
	cat := catalog.NewCatalog()
	cat.CreateTable(&catalog.Table{
		Name:          "big_table",
		Columns:       []catalog.ColumnDef{{Name: "id", Type: "INT"}},
		EstimatedRows: 10000,
	})
	cat.CreateTable(&catalog.Table{
		Name:          "small_table",
		Columns:       []catalog.ColumnDef{{Name: "ref", Type: "INT"}},
		EstimatedRows: 100,
	})

	// Build: Join(small LEFT, big RIGHT, ON small.ref = big.id)
	leftSmall := NewLogicalScan("small_table", "s")
	rightBig := NewLogicalScan("big_table", "b")
	join := NewLogicalJoin(leftSmall, rightBig, "ref", "id")

	opt := CatalogOptimizer(cat)
	optimized := opt.Optimize(join)

	joinNode, ok := optimized.(*LogicalJoin)
	if !ok {
		t.Fatalf("expected LogicalJoin, got %T", optimized)
	}

	// Verify smaller table (100 rows) moved to right side (build side for hash join)
	leftScan := joinNode.Left.(*LogicalScan)
	rightScan := joinNode.Right.(*LogicalScan)
	if rightScan.TableName != "small_table" {
		t.Errorf("expected small_table on right (build) side, got %s on right", rightScan.TableName)
	}
	if leftScan.TableName != "big_table" {
		t.Errorf("expected big_table on left (probe) side, got %s on left", leftScan.TableName)
	}
	// Verify join condition columns swapped too
	if joinNode.LeftColumn != "id" || joinNode.RightColumn != "ref" {
		t.Errorf("expected LeftColumn=id, RightColumn=ref after swap, got LeftColumn=%s, RightColumn=%s",
			joinNode.LeftColumn, joinNode.RightColumn)
	}
}

func TestCostBasedJoinReorderNoSwap(t *testing.T) {
	cat := catalog.NewCatalog()
	cat.CreateTable(&catalog.Table{
		Name:          "small",
		Columns:       []catalog.ColumnDef{{Name: "id", Type: "INT"}},
		EstimatedRows: 100,
	})
	cat.CreateTable(&catalog.Table{
		Name:          "big",
		Columns:       []catalog.ColumnDef{{Name: "ref", Type: "INT"}},
		EstimatedRows: 10000,
	})

	// Build: Join(big LEFT, small RIGHT) — should NOT swap since right is already smaller
	leftBig := NewLogicalScan("big", "b")
	rightSmall := NewLogicalScan("small", "s")
	join := NewLogicalJoin(leftBig, rightSmall, "id", "ref")

	opt := CatalogOptimizer(cat)
	optimized := opt.Optimize(join)

	joinNode, ok := optimized.(*LogicalJoin)
	if !ok {
		t.Fatalf("expected LogicalJoin, got %T", optimized)
	}

	leftScan := joinNode.Left.(*LogicalScan)
	rightScan := joinNode.Right.(*LogicalScan)
	if rightScan.TableName != "small" {
		t.Errorf("expected small to stay on right, got %s", rightScan.TableName)
	}
	if leftScan.TableName != "big" {
		t.Errorf("expected big to stay on left, got %s", leftScan.TableName)
	}
}

func TestEstimateRows(t *testing.T) {
	tables := map[string]*catalog.Table{
		"users": {
			Name:          "users",
			EstimatedRows: 1000,
		},
	}

	scan := NewLogicalScan("users", "")
	rows := EstimateRows(scan, tables)
	if rows != 1000 {
		t.Errorf("expected 1000, got %f", rows)
	}

	filter := NewLogicalFilter(scan, &sqlparse.ComparisonExpr{Column: "age", Operator: ">", Value: "0"})
	rows = EstimateRows(filter, tables)
	if rows != 200 {
		t.Errorf("expected 200 (20%% of 1000), got %f", rows)
	}

	proj := NewLogicalProject(filter, []string{"name"}, nil)
	rows = EstimateRows(proj, tables)
	if rows != 200 {
		t.Errorf("expected 200 (pass-through), got %f", rows)
	}

	limit := NewLogicalLimit(scan, 10)
	rows = EstimateRows(limit, tables)
	if rows != 10 {
		t.Errorf("expected 10, got %f", rows)
	}
}

func TestEstimateRowsUnknown(t *testing.T) {
	tables := map[string]*catalog.Table{
		"users": {Name: "users", EstimatedRows: 0},
	}
	scan := NewLogicalScan("users", "")
	rows := EstimateRows(scan, tables)
	if !math.IsInf(rows, 1) {
		t.Errorf("expected +Inf for unknown table, got %f", rows)
	}
}

func TestCostBasedJoinReorderNoStats(t *testing.T) {
	cat := catalog.NewCatalog()
	cat.CreateTable(&catalog.Table{
		Name:    "a",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}},
		// EstimatedRows: 0 — unknown
	})
	cat.CreateTable(&catalog.Table{
		Name:    "b",
		Columns: []catalog.ColumnDef{{Name: "id", Type: "INT"}},
	})

	join := NewLogicalJoin(NewLogicalScan("a", ""), NewLogicalScan("b", ""), "id", "id")
	opt := CatalogOptimizer(cat)
	optimized := opt.Optimize(join)

	// Should remain unchanged when both sides have unknown cardinality
	joinNode, ok := optimized.(*LogicalJoin)
	if !ok {
		t.Fatalf("expected LogicalJoin, got %T", optimized)
	}
	leftScan := joinNode.Left.(*LogicalScan)
	rightScan := joinNode.Right.(*LogicalScan)
	if leftScan.TableName != "a" || rightScan.TableName != "b" {
		t.Errorf("expected no swap for unknown stats, got left=%s right=%s", leftScan.TableName, rightScan.TableName)
	}
}
