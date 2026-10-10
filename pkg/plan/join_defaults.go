package plan

import (
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/storage"
)

// DefaultValueForType returns the value used to fill a column of the side
// that has no matching row in an outer/semi join. String-like and unknown
// types fill with "" which the engine treats as NULL; numeric types fill
// with "0"; booleans with "false".
func DefaultValueForType(colType string) string {
	base := strings.ToUpper(strings.TrimSpace(colType))
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	switch base {
	case "INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "LONG",
		"DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL":
		return "0"
	case "BOOLEAN", "BOOL":
		return "false"
	default:
		return ""
	}
}

// sideDefaults collects a default value for every column a join side can
// produce, keyed exactly as rows from that side carry them. For a nested
// join the keys mirror joinBatch's matched-merge layout (colliding right
// columns stored qualified with the join's RightPrefix).
func sideDefaults(node LogicalNode, ctx *PhysicalPlanContext) storage.Row {
	if ctx == nil {
		return nil
	}
	switch n := node.(type) {
	case *LogicalScan:
		table, ok := ctx.Catalog.GetTable(n.TableName)
		if !ok {
			return nil
		}
		defaults := make(storage.Row, len(table.Columns))
		for _, c := range table.Columns {
			// a declared column DEFAULT wins over the type-based default
			if c.HasDefault {
				defaults[c.Name] = c.Default
			} else {
				defaults[c.Name] = DefaultValueForType(c.Type)
			}
		}
		return defaults
	case *LogicalCTEScan:
		return cteDefaults(ctx.CTEData[strings.ToLower(n.CTEName)])
	case *LogicalJoin:
		left := sideDefaults(n.Left, ctx)
		right := sideDefaults(n.Right, ctx)
		if left == nil {
			return right
		}
		if right == nil {
			return left
		}
		merged := copyRow(left)
		for k, v := range right {
			if _, conflict := merged[k]; conflict && n.RightPrefix != "" {
				merged[n.RightPrefix+"."+k] = v
			} else {
				merged[k] = v
			}
		}
		return merged
	case *LogicalFilter:
		return sideDefaults(n.Child, ctx)
	case *LogicalProject:
		return sideDefaults(n.Child, ctx)
	case *LogicalAggregate:
		return sideDefaults(n.Child, ctx)
	case *LogicalWindow:
		return sideDefaults(n.Child, ctx)
	case *LogicalSort:
		return sideDefaults(n.Child, ctx)
	case *LogicalLimit:
		return sideDefaults(n.Child, ctx)
	default:
		return nil
	}
}

// cteDefaults infers default values for CTE/subquery rows, which carry no
// declared column types. The first non-empty value seen for each column
// decides the fill: numeric-looking columns fill with "0", everything else
// with "".
func cteDefaults(rows []storage.Row) storage.Row {
	if len(rows) == 0 {
		return nil
	}
	const maxScan = 100
	numeric := make(map[string]bool)
	kind := make(map[string]bool) // key seen with a non-empty sample
	for i, r := range rows {
		if i >= maxScan {
			break
		}
		for k, v := range r {
			if kind[k] || v == "" {
				continue
			}
			kind[k] = true
			if _, err := strconv.ParseFloat(v, 64); err == nil {
				numeric[k] = true
			}
		}
	}
	defaults := make(storage.Row, len(rows[0]))
	for k := range rows[0] {
		if numeric[k] {
			defaults[k] = "0"
		} else {
			defaults[k] = ""
		}
	}
	return defaults
}
