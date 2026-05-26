package plan

import (
	"testing"
)

func TestLookupFuncExists(t *testing.T) {
	tests := []struct {
		name     string
		funcType FuncType
	}{
		{"COUNT", FuncAggregate},
		{"SUM", FuncAggregate},
		{"AVG", FuncAggregate},
		{"MIN", FuncAggregate},
		{"MAX", FuncAggregate},
		{"STDDEV", FuncAggregate},
		{"VARIANCE", FuncAggregate},
		{"COLLECT_LIST", FuncAggregate},
		{"COLLECT_SET", FuncAggregate},
		{"ROW_NUMBER", FuncWindow},
		{"RANK", FuncWindow},
		{"DENSE_RANK", FuncWindow},
		{"LEAD", FuncWindow},
		{"LAG", FuncWindow},
		{"NTILE", FuncWindow},
		{"FIRST_VALUE", FuncWindow},
		{"LAST_VALUE", FuncWindow},
		{"CUME_DIST", FuncWindow},
		{"PERCENT_RANK", FuncWindow},
		{"NTH_VALUE", FuncWindow},
		{"ROUND", FuncScalar},
		{"FLOOR", FuncScalar},
		{"CEIL", FuncScalar},
		{"ABS", FuncScalar},
		{"SQRT", FuncScalar},
		{"POWER", FuncScalar},
		{"MOD", FuncScalar},
		{"GREATEST", FuncScalar},
		{"LEAST", FuncScalar},
		{"CONCAT", FuncScalar},
		{"SUBSTRING", FuncScalar},
		{"UPPER", FuncScalar},
		{"LOWER", FuncScalar},
		{"TRIM", FuncScalar},
		{"LENGTH", FuncScalar},
		{"REPLACE", FuncScalar},
		{"IF", FuncScalar},
		{"COALESCE", FuncScalar},
		{"NVL", FuncScalar},
		{"NULLIF", FuncScalar},
		{"CURRENT_DATE", FuncScalar},
		{"YEAR", FuncScalar},
		{"MONTH", FuncScalar},
		{"DAY", FuncScalar},
		{"DATEDIFF", FuncScalar},
		{"DATE_ADD", FuncScalar},
		{"DATE_SUB", FuncScalar},
		{"CORR", FuncAggregate},
		{"COVAR_POP", FuncAggregate},
		{"COVAR_SAMP", FuncAggregate},
		{"PERCENTILE", FuncAggregate},
		{"PERCENTILE_APPROX", FuncAggregate},
		{"HISTOGRAM_NUMERIC", FuncAggregate},
		{"WIDTH_BUCKET", FuncScalar},
		{"FROM_UTC_TIMESTAMP", FuncScalar},
		{"TO_UTC_TIMESTAMP", FuncScalar},
	}
	for _, tt := range tests {
		fn, ok := LookupFunc(tt.name)
		if !ok {
			t.Errorf("function %s not registered", tt.name)
			continue
		}
		if fn.Type != tt.funcType {
			t.Errorf("%s: expected type %v, got %v", tt.name, tt.funcType, fn.Type)
		}
	}
}

func TestAggCount(t *testing.T) {
	fn, ok := LookupFunc("COUNT")
	if !ok {
		t.Fatal("COUNT not registered")
	}
	rows := makeRows("name", "a", "b", "c", "a")
	result := fn.AggFn(rows, "name", false, nil)
	if result != "4" {
		t.Errorf("COUNT(*): expected 4, got %s", result)
	}
	result = fn.AggFn(rows, "name", true, nil)
	if result != "3" {
		t.Errorf("COUNT(DISTINCT name): expected 3, got %s", result)
	}
}

func TestAggSum(t *testing.T) {
	fn, ok := LookupFunc("SUM")
	if !ok {
		t.Fatal("SUM not registered")
	}
	rows := makeRows("val", "1", "2", "3", "4")
	result := fn.AggFn(rows, "val", false, nil)
	if result != "10" {
		t.Errorf("SUM: expected 10, got %s", result)
	}
}

func TestAggAvg(t *testing.T) {
	fn, ok := LookupFunc("AVG")
	if !ok {
		t.Fatal("AVG not registered")
	}
	rows := makeRows("val", "2", "4", "6")
	result := fn.AggFn(rows, "val", false, nil)
	if result != "4" {
		t.Errorf("AVG: expected 4, got %s", result)
	}
}

func TestAggStddev(t *testing.T) {
	fn, ok := LookupFunc("STDDEV")
	if !ok {
		t.Fatal("STDDEV not registered")
	}
	rows := makeRows("val", "1", "2", "3")
	result := fn.AggFn(rows, "val", false, nil)
	if result == "" {
		t.Error("STDDEV: got empty result")
	}
}

func TestScalarRound(t *testing.T) {
	fn, ok := LookupFunc("ROUND")
	if !ok {
		t.Fatal("ROUND not registered")
	}
	r := fn.ScalarFn([]string{"3.14159", "2"})
	if r != "3.14" {
		t.Errorf("ROUND(3.14159, 2): expected 3.14, got %s", r)
	}
	r = fn.ScalarFn([]string{"3.5"})
	if r != "4" {
		t.Errorf("ROUND(3.5): expected 4, got %s", r)
	}
}

func TestScalarAbs(t *testing.T) {
	fn, ok := LookupFunc("ABS")
	if !ok {
		t.Fatal("ABS not registered")
	}
	r := fn.ScalarFn([]string{"-5"})
	if r != "5" {
		t.Errorf("ABS(-5): expected 5, got %s", r)
	}
}

func TestScalarConcat(t *testing.T) {
	fn, ok := LookupFunc("CONCAT")
	if !ok {
		t.Fatal("CONCAT not registered")
	}
	r := fn.ScalarFn([]string{"hello", " ", "world"})
	if r != "hello world" {
		t.Errorf("CONCAT: expected 'hello world', got '%s'", r)
	}
}

func TestScalarUpper(t *testing.T) {
	fn, ok := LookupFunc("UPPER")
	if !ok {
		t.Fatal("UPPER not registered")
	}
	r := fn.ScalarFn([]string{"hello"})
	if r != "HELLO" {
		t.Errorf("UPPER: expected 'HELLO', got '%s'", r)
	}
}

func TestScalarLength(t *testing.T) {
	fn, ok := LookupFunc("LENGTH")
	if !ok {
		t.Fatal("LENGTH not registered")
	}
	r := fn.ScalarFn([]string{"hello"})
	if r != "5" {
		t.Errorf("LENGTH: expected '5', got '%s'", r)
	}
}

func TestScalarIf(t *testing.T) {
	fn, ok := LookupFunc("IF")
	if !ok {
		t.Fatal("IF not registered")
	}
	r := fn.ScalarFn([]string{"true", "yes", "no"})
	if r != "yes" {
		t.Errorf("IF(true, yes, no): expected 'yes', got '%s'", r)
	}
	r = fn.ScalarFn([]string{"false", "yes", "no"})
	if r != "no" {
		t.Errorf("IF(false, yes, no): expected 'no', got '%s'", r)
	}
}

func TestScalarCoalesce(t *testing.T) {
	fn, ok := LookupFunc("COALESCE")
	if !ok {
		t.Fatal("COALESCE not registered")
	}
	r := fn.ScalarFn([]string{"", "", "hello"})
	if r != "hello" {
		t.Errorf("COALESCE: expected 'hello', got '%s'", r)
	}
}

func TestListFuncs(t *testing.T) {
	funcs := ListFuncs()
	if len(funcs) == 0 {
		t.Error("ListFuncs returned empty")
	}
	names := make(map[string]bool)
	for _, f := range funcs {
		if names[f.Name] {
			t.Errorf("duplicate function: %s", f.Name)
		}
		names[f.Name] = true
	}
}

func makeRows(col string, vals ...string) []map[string]string {
	rows := make([]map[string]string, len(vals))
	for i, v := range vals {
		rows[i] = map[string]string{col: v}
	}
	return rows
}
