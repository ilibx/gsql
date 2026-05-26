package plan

import "strconv"

func registerConditionalBuiltins() {
	RegisterFunc(&FuncDef{
		Name: "IF", Type: FuncScalar, MinArgs: 3, MaxArgs: 3,
		ScalarFn: fnIf,
	})
	RegisterFunc(&FuncDef{
		Name: "COALESCE", Type: FuncScalar, MinArgs: 1, MaxArgs: -1,
		ScalarFn: fnCoalesce,
	})
	RegisterFunc(&FuncDef{
		Name: "NVL", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnNvl,
	})
	RegisterFunc(&FuncDef{
		Name: "NULLIF", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnNullif,
	})
	RegisterFunc(&FuncDef{
		Name: "CAST", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnCast,
	})
}

func fnCast(args []string) string {
	if len(args) < 2 {
		return args[0]
	}
	val := args[0]
	typ := args[1]
	switch typ {
	case "INT", "INTEGER":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return strconv.Itoa(int(f))
		}
		return "0"
	case "BIGINT":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return strconv.FormatInt(int64(f), 10)
		}
		return "0"
	case "FLOAT", "DOUBLE":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
		return "0"
	case "STRING", "VARCHAR", "CHAR":
		return val
	case "BOOLEAN":
		if val == "true" || val == "TRUE" || val == "1" || val == "t" {
			return "true"
		}
		return "false"
	default:
		return val
	}
}

func fnIf(args []string) string {
	if len(args) < 3 {
		return ""
	}
	cond := args[0]
	if cond == "true" || cond == "TRUE" || cond == "1" {
		return args[1]
	}
	v, err := toFloat(cond)
	if err == nil && v != 0 {
		return args[1]
	}
	return args[2]
}

func fnCoalesce(args []string) string {
	for _, a := range args {
		if a != "" {
			return a
		}
	}
	return ""
}

func fnNvl(args []string) string {
	if len(args) < 2 {
		return ""
	}
	if args[0] != "" {
		return args[0]
	}
	return args[1]
}

func fnNullif(args []string) string {
	if len(args) < 2 {
		return ""
	}
	if args[0] == args[1] {
		return ""
	}
	return args[0]
}
