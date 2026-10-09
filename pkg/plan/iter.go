package plan

import (
	"io"

	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

// RowIter is the streaming interface every PlanNode produces. Next returns
// io.EOF when exhausted. Pull-based so callers control memory: operators
// upstream never materialize their input unless the operation requires it
// (sort/aggregate/window/join-build).
type RowIter = storage.RowIter

// Collect drains an iterator into a slice. Used by Execute as a materializing
// wrapper so existing callers keep working.
func Collect(it RowIter) ([]storage.Row, error) {
	defer it.Close()
	var rows []storage.Row
	for {
		row, err := it.Next()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
}

// materialize is the default PlanNode.Execute implementation.
func materialize(n PlanNode) ([]storage.Row, error) {
	it, err := n.Iterate()
	if err != nil {
		return nil, err
	}
	rows, err := Collect(it)
	if err != nil {
		return nil, err
	}
	debugPrintRows(n.Type(), rows)
	return rows, nil
}

// sliceIter replays an in-memory row set.
type sliceIter struct {
	rows []storage.Row
	pos  int
}

func newSliceIter(rows []storage.Row) *sliceIter { return &sliceIter{rows: rows} }

func (it *sliceIter) Next() (storage.Row, error) {
	if it.pos >= len(it.rows) {
		return nil, io.EOF
	}
	row := it.rows[it.pos]
	it.pos++
	return row, nil
}

func (it *sliceIter) Close() error { return nil }

// filterIter streams child rows through a predicate.
type filterIter struct {
	child     RowIter
	predicate parser.Expression
}

func (it *filterIter) Next() (storage.Row, error) {
	for {
		row, err := it.child.Next()
		if err != nil {
			return nil, err
		}
		if evaluateExpression(row, it.predicate) {
			return row, nil
		}
	}
}

func (it *filterIter) Close() error { return it.child.Close() }

// projectIter streams one projection per input row.
type projectIter struct {
	child   RowIter
	columns []string
	exprs   []parser.Expression
}

func (it *projectIter) Next() (storage.Row, error) {
	row, err := it.child.Next()
	if err != nil {
		return nil, err
	}
	projected := make(storage.Row, len(it.columns))
	for i, col := range it.columns {
		if i < len(it.exprs) && it.exprs[i] != nil {
			projected[col] = evaluateExpressionValue(row, it.exprs[i])
		} else {
			projected[col] = projectRowValue(row, col)
		}
	}
	return projected, nil
}

func (it *projectIter) Close() error { return it.child.Close() }

// limitIter stops the child stream after n rows.
type limitIter struct {
	child RowIter
	rest  int
}

func (it *limitIter) Next() (storage.Row, error) {
	if it.rest <= 0 {
		return nil, io.EOF
	}
	row, err := it.child.Next()
	if err != nil {
		return nil, err
	}
	it.rest--
	return row, nil
}

func (it *limitIter) Close() error { return it.child.Close() }
