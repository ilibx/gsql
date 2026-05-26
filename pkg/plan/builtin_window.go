package plan

import (
	"strconv"

	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

func registerWindowBuiltins() {
	RegisterFunc(&FuncDef{
		Name: "LEAD", Type: FuncWindow, MinArgs: 1, MaxArgs: 3,
		WindowFn: windowLead,
	})
	RegisterFunc(&FuncDef{
		Name: "LAG", Type: FuncWindow, MinArgs: 1, MaxArgs: 3,
		WindowFn: windowLag,
	})
	RegisterFunc(&FuncDef{
		Name: "NTILE", Type: FuncWindow, MinArgs: 1, MaxArgs: 1,
		WindowFn: windowNtile,
	})
	RegisterFunc(&FuncDef{
		Name: "FIRST_VALUE", Type: FuncWindow, MinArgs: 1, MaxArgs: 1,
		WindowFn: windowFirstValue,
	})
	RegisterFunc(&FuncDef{
		Name: "LAST_VALUE", Type: FuncWindow, MinArgs: 1, MaxArgs: 1,
		WindowFn: windowLastValue,
	})
	RegisterFunc(&FuncDef{
		Name: "CUME_DIST", Type: FuncWindow, MinArgs: 0, MaxArgs: 0,
		WindowFn: windowCumeDist,
	})
	RegisterFunc(&FuncDef{
		Name: "PERCENT_RANK", Type: FuncWindow, MinArgs: 0, MaxArgs: 0,
		WindowFn: windowPercentRank,
	})
	RegisterFunc(&FuncDef{
		Name: "NTH_VALUE", Type: FuncWindow, MinArgs: 2, MaxArgs: 2,
		WindowFn: windowNthValue,
	})
}

func windowLead(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	if len(rows) == 0 || len(args) < 1 {
		return rows
	}
	col := args[0]
	offset := 1
	defaultVal := ""
	if len(args) >= 2 {
		if o, err := strconv.Atoi(args[1]); err == nil {
			offset = o
		}
	}
	if len(args) >= 3 {
		defaultVal = args[2]
	}
	for i := range rows {
		idx := i + offset
		if idx < len(rows) {
			rows[i][colKey] = rows[idx][col]
		} else {
			rows[i][colKey] = defaultVal
		}
	}
	return rows
}

func windowLag(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	if len(rows) == 0 || len(args) < 1 {
		return rows
	}
	col := args[0]
	offset := 1
	defaultVal := ""
	if len(args) >= 2 {
		if o, err := strconv.Atoi(args[1]); err == nil {
			offset = o
		}
	}
	if len(args) >= 3 {
		defaultVal = args[2]
	}
	for i := range rows {
		idx := i - offset
		if idx >= 0 {
			rows[i][colKey] = rows[idx][col]
		} else {
			rows[i][colKey] = defaultVal
		}
	}
	return rows
}

func windowNtile(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	if len(rows) == 0 || len(args) < 1 {
		return rows
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		for i := range rows {
			rows[i][colKey] = "1"
		}
		return rows
	}
	if n > len(rows) {
		n = len(rows)
	}
	bucketSize := len(rows) / n
	remainder := len(rows) % n
	bucket := 1
	count := 0
	for i := range rows {
		rows[i][colKey] = strconv.Itoa(bucket)
		count++
		size := bucketSize
		if bucket <= remainder {
			size++
		}
		if count >= size {
			bucket++
			count = 0
		}
	}
	return rows
}

func windowFirstValue(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	if len(rows) == 0 || len(args) < 1 {
		return rows
	}
	col := args[0]
	fv := rows[0][col]
	for i := range rows {
		rows[i][colKey] = fv
	}
	return rows
}

func windowLastValue(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	if len(rows) == 0 || len(args) < 1 {
		return rows
	}
	col := args[0]
	lv := rows[len(rows)-1][col]
	for i := range rows {
		rows[i][colKey] = lv
	}
	return rows
}

func windowCumeDist(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	n := len(rows)
	if n == 0 {
		return rows
	}
	for i := range rows {
		// position / total — tied with next rank logic simplified
		cume := float64(i+1) / float64(n)
		rows[i][colKey] = strconv.FormatFloat(cume, 'f', 6, 64)
	}
	return rows
}

func windowPercentRank(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	n := len(rows)
	if n <= 1 {
		for i := range rows {
			rows[i][colKey] = "0.000000"
		}
		return rows
	}
	denom := float64(n - 1)
	for i := range rows {
		pr := float64(i) / denom
		rows[i][colKey] = strconv.FormatFloat(pr, 'f', 6, 64)
	}
	return rows
}

func windowNthValue(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row {
	if len(rows) == 0 || len(args) < 2 {
		return rows
	}
	col := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 1 {
		return rows
	}
	idx := n - 1
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	nv := rows[idx][col]
	for i := range rows {
		rows[i][colKey] = nv
	}
	return rows
}
