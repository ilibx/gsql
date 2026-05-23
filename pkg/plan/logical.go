package plan

import (
	"fmt"
	"strings"

	"github.com/ilibx/gsql/pkg/parser"
)

type LogicalNode interface {
	Type() string
	Children() []LogicalNode
}

type LogicalScan struct {
	TableName string
	Alias     string
}

func NewLogicalScan(tableName, alias string) *LogicalScan {
	return &LogicalScan{TableName: tableName, Alias: alias}
}

func (n *LogicalScan) Type() string { return "LogicalScan" }

func (n *LogicalScan) Children() []LogicalNode { return nil }

type LogicalCTEScan struct {
	CTEName string
}

func NewLogicalCTEScan(name string) *LogicalCTEScan {
	return &LogicalCTEScan{CTEName: name}
}

func (n *LogicalCTEScan) Type() string { return "LogicalCTEScan" }

func (n *LogicalCTEScan) Children() []LogicalNode { return nil }

type LogicalFilter struct {
	Child     LogicalNode
	Predicate parser.Expression
}

func NewLogicalFilter(child LogicalNode, predicate parser.Expression) *LogicalFilter {
	return &LogicalFilter{Child: child, Predicate: predicate}
}

func (n *LogicalFilter) Type() string { return "LogicalFilter" }

func (n *LogicalFilter) Children() []LogicalNode { return []LogicalNode{n.Child} }

type LogicalProject struct {
	Child   LogicalNode
	Columns []string
	Exprs   []parser.Expression
}

func NewLogicalProject(child LogicalNode, columns []string, exprs []parser.Expression) *LogicalProject {
	return &LogicalProject{Child: child, Columns: columns, Exprs: exprs}
}

func (n *LogicalProject) Type() string { return "LogicalProject" }

func (n *LogicalProject) Children() []LogicalNode { return []LogicalNode{n.Child} }

type LogicalJoin struct {
	Left        LogicalNode
	Right       LogicalNode
	LeftColumn  string
	RightColumn string
}

func NewLogicalJoin(left, right LogicalNode, leftCol, rightCol string) *LogicalJoin {
	return &LogicalJoin{Left: left, Right: right, LeftColumn: leftCol, RightColumn: rightCol}
}

func (n *LogicalJoin) Type() string { return "LogicalJoin" }

func (n *LogicalJoin) Children() []LogicalNode { return []LogicalNode{n.Left, n.Right} }

type LogicalAggregate struct {
	Child      LogicalNode
	GroupBy    []string
	Aggregates []AggregateDef
}

func NewLogicalAggregate(child LogicalNode, groupBy []string, aggregates []AggregateDef) *LogicalAggregate {
	return &LogicalAggregate{Child: child, GroupBy: groupBy, Aggregates: aggregates}
}

func (n *LogicalAggregate) Type() string { return "LogicalAggregate" }

func (n *LogicalAggregate) Children() []LogicalNode { return []LogicalNode{n.Child} }

type LogicalWindow struct {
	Child       LogicalNode
	WindowExprs []parser.WindowExpr
}

func NewLogicalWindow(child LogicalNode, windowExprs []parser.WindowExpr) *LogicalWindow {
	return &LogicalWindow{Child: child, WindowExprs: windowExprs}
}

func (n *LogicalWindow) Type() string { return "LogicalWindow" }

func (n *LogicalWindow) Children() []LogicalNode { return []LogicalNode{n.Child} }

type LogicalSort struct {
	Child   LogicalNode
	OrderBy []parser.SortOrder
}

func NewLogicalSort(child LogicalNode, orderBy []parser.SortOrder) *LogicalSort {
	return &LogicalSort{Child: child, OrderBy: orderBy}
}

func (n *LogicalSort) Type() string { return "LogicalSort" }

func (n *LogicalSort) Children() []LogicalNode { return []LogicalNode{n.Child} }

type LogicalLimit struct {
	Child LogicalNode
	N     int
}

func NewLogicalLimit(child LogicalNode, n int) *LogicalLimit {
	return &LogicalLimit{Child: child, N: n}
}

func (n *LogicalLimit) Type() string { return "LogicalLimit" }

func (n *LogicalLimit) Children() []LogicalNode { return []LogicalNode{n.Child} }

func ExplainLogical(node LogicalNode, indent int) string {
	prefix := strings.Repeat("  ", indent)
	var desc string
	switch n := node.(type) {
	case *LogicalScan:
		if n.Alias != "" {
			desc = fmt.Sprintf("%s AS %s", n.TableName, n.Alias)
		} else {
			desc = n.TableName
		}
	case *LogicalCTEScan:
		desc = n.CTEName
	case *LogicalFilter:
		desc = fmt.Sprintf("WHERE %s", expressionToString(n.Predicate))
	case *LogicalProject:
		desc = strings.Join(n.Columns, ", ")
	case *LogicalJoin:
		desc = fmt.Sprintf("ON %s = %s", n.LeftColumn, n.RightColumn)
	case *LogicalAggregate:
		aggParts := make([]string, len(n.Aggregates))
		for i, a := range n.Aggregates {
			aggParts[i] = a.FuncName + "(" + a.Column + ")"
		}
		gb := strings.Join(n.GroupBy, ", ")
		desc = fmt.Sprintf("GROUP BY [%s] AGG [%s]", gb, strings.Join(aggParts, ", "))
	case *LogicalSort:
		parts := make([]string, len(n.OrderBy))
		for i, o := range n.OrderBy {
			s := o.Column
			if o.Desc {
				s += " DESC"
			}
			parts[i] = s
		}
		desc = strings.Join(parts, ", ")
	case *LogicalWindow:
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
		desc = strings.Join(parts, ", ")
	case *LogicalLimit:
		desc = fmt.Sprintf("%d", n.N)
	default:
		desc = "unknown"
	}
	result := prefix + node.Type() + "(" + desc + ")"
	for _, child := range node.Children() {
		result += "\n" + ExplainLogical(child, indent+1)
	}
	return result
}
