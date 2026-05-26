package plan

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

type FuncType int

const (
	FuncScalar    FuncType = iota
	FuncAggregate
	FuncWindow
)

type FuncDef struct {
	Name    string
	Type    FuncType
	MinArgs int
	MaxArgs int
	ScalarFn  func(args []string) string
	AggFn     func(rows []storage.Row, column string, distinct bool, args []string) string
	WindowFn  func(rows []storage.Row, args []string, orderBy []parser.SortOrder, colKey string) []storage.Row
}

var (
	builtinsMu sync.RWMutex
	builtins   = make(map[string]*FuncDef)
)

func RegisterFunc(def *FuncDef) {
	builtinsMu.Lock()
	defer builtinsMu.Unlock()
	name := strings.ToUpper(def.Name)
	def.Name = name
	builtins[name] = def
}

func LookupFunc(name string) (*FuncDef, bool) {
	builtinsMu.RLock()
	defer builtinsMu.RUnlock()
	f, ok := builtins[strings.ToUpper(name)]
	return f, ok
}

func ListFuncs() []*FuncDef {
	builtinsMu.RLock()
	defer builtinsMu.RUnlock()
	out := make([]*FuncDef, 0, len(builtins))
	for _, f := range builtins {
		out = append(out, f)
	}
	return out
}

func computeAggregate(rows []storage.Row, funcName, column string, distinct bool, args []string) string {
	fn, ok := LookupFunc(funcName)
	if !ok || fn.AggFn == nil {
		return ""
	}
	return fn.AggFn(rows, column, distinct, args)
}

func init() {
	registerAggregateBuiltins()
	registerWindowBuiltins()
	registerMathBuiltins()
	registerStringBuiltins()
	registerConditionalBuiltins()
	registerDateTimeBuiltins()
}

func toFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func compareValues(a, b, op string) bool {
	fa, aOK := toFloat(a)
	fb, bOK := toFloat(b)
	if aOK && bOK {
		switch op {
		case "<":
			return fa < fb
		case ">":
			return fa > fb
		case "<=":
			return fa <= fb
		case ">=":
			return fa >= fb
		}
	}
	switch op {
	case "<":
		return a < b
	case ">":
		return a > b
	case "<=":
		return a <= b
	case ">=":
		return a >= b
	}
	return false
}

func roundFloat(val float64, precision int) float64 {
	pow := math.Pow(10, float64(precision))
	return math.Round(val*pow) / pow
}
