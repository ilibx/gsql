package plan

import (
	"strconv"
	"strings"
)

func registerStringBuiltins() {
	RegisterFunc(&FuncDef{
		Name: "CONCAT", Type: FuncScalar, MinArgs: 1, MaxArgs: -1,
		ScalarFn: fnConcat,
	})
	RegisterFunc(&FuncDef{
		Name: "CONCAT_WS", Type: FuncScalar, MinArgs: 2, MaxArgs: -1,
		ScalarFn: fnConcatWS,
	})
	RegisterFunc(&FuncDef{
		Name: "SUBSTRING", Type: FuncScalar, MinArgs: 2, MaxArgs: 3,
		ScalarFn: fnSubstring,
	})
	RegisterFunc(&FuncDef{
		Name: "SUBSTR", Type: FuncScalar, MinArgs: 2, MaxArgs: 3,
		ScalarFn: fnSubstring,
	})
	RegisterFunc(&FuncDef{
		Name: "UPPER", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnUpper,
	})
	RegisterFunc(&FuncDef{
		Name: "UCASE", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnUpper,
	})
	RegisterFunc(&FuncDef{
		Name: "LOWER", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLower,
	})
	RegisterFunc(&FuncDef{
		Name: "LCASE", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLower,
	})
	RegisterFunc(&FuncDef{
		Name: "TRIM", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnTrim,
	})
	RegisterFunc(&FuncDef{
		Name: "LTRIM", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLtrim,
	})
	RegisterFunc(&FuncDef{
		Name: "RTRIM", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnRtrim,
	})
	RegisterFunc(&FuncDef{
		Name: "LENGTH", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLength,
	})
	RegisterFunc(&FuncDef{
		Name: "REPLACE", Type: FuncScalar, MinArgs: 3, MaxArgs: 3,
		ScalarFn: fnReplace,
	})
	RegisterFunc(&FuncDef{
		Name: "REVERSE", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnReverse,
	})
	RegisterFunc(&FuncDef{
		Name: "LOCATE", Type: FuncScalar, MinArgs: 2, MaxArgs: 3,
		ScalarFn: fnLocate,
	})
	RegisterFunc(&FuncDef{
		Name: "INSTR", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnInstr,
	})
	RegisterFunc(&FuncDef{
		Name: "LPAD", Type: FuncScalar, MinArgs: 3, MaxArgs: 3,
		ScalarFn: fnLpad,
	})
	RegisterFunc(&FuncDef{
		Name: "RPAD", Type: FuncScalar, MinArgs: 3, MaxArgs: 3,
		ScalarFn: fnRpad,
	})
	RegisterFunc(&FuncDef{
		Name: "INITCAP", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnInitcap,
	})
	RegisterFunc(&FuncDef{
		Name: "ASCII", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnAscii,
	})
	RegisterFunc(&FuncDef{
		Name: "SPLIT", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnSplit,
	})
}

func fnConcat(args []string) string {
	return strings.Join(args, "")
}

func fnConcatWS(args []string) string {
	if len(args) < 2 {
		return ""
	}
	sep := args[0]
	return strings.Join(args[1:], sep)
}

func fnSubstring(args []string) string {
	if len(args) < 2 {
		return ""
	}
	s := args[0]
	pos, err := strconv.Atoi(args[1])
	if err != nil {
		return s
	}
	if pos < 1 {
		pos = 1
	}
	start := pos - 1
	if start >= len(s) {
		return ""
	}
	if len(args) >= 3 {
		length, err := strconv.Atoi(args[2])
		if err != nil || length < 0 {
			return s[start:]
		}
		end := start + length
		if end > len(s) {
			end = len(s)
		}
		return s[start:end]
	}
	return s[start:]
}

func fnUpper(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return strings.ToUpper(args[0])
}

func fnLower(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return strings.ToLower(args[0])
}

func fnTrim(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return strings.TrimSpace(args[0])
}

func fnLtrim(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return strings.TrimLeft(args[0], " ")
}

func fnRtrim(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return strings.TrimRight(args[0], " ")
}

func fnLength(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	return strconv.Itoa(len(args[0]))
}

func fnReplace(args []string) string {
	if len(args) < 3 {
		return ""
	}
	return strings.ReplaceAll(args[0], args[1], args[2])
}

func fnReverse(args []string) string {
	if len(args) < 1 {
		return ""
	}
	runes := []rune(args[0])
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func fnLocate(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	sub := args[0]
	s := args[1]
	start := 0
	if len(args) >= 3 {
		if p, err := strconv.Atoi(args[2]); err == nil && p > 1 {
			start = p - 1
		}
	}
	if start >= len(s) {
		return "0"
	}
	idx := strings.Index(s[start:], sub)
	if idx < 0 {
		return "0"
	}
	return strconv.Itoa(start + idx + 1)
}

func fnInstr(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	return fnLocate(args)
}

func fnLpad(args []string) string {
	if len(args) < 3 {
		return ""
	}
	s := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return s
	}
	pad := args[2]
	if n <= len(s) {
		return s[:n]
	}
	return strings.Repeat(pad, n-len(s)) + s
}

func fnRpad(args []string) string {
	if len(args) < 3 {
		return ""
	}
	s := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return s
	}
	pad := args[2]
	if n <= len(s) {
		return s[:n]
	}
	return s + strings.Repeat(pad, n-len(s))
}

func fnInitcap(args []string) string {
	if len(args) < 1 {
		return ""
	}
	s := args[0]
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func fnAscii(args []string) string {
	if len(args) < 1 || len(args[0]) == 0 {
		return "0"
	}
	r := []rune(args[0])
	return strconv.Itoa(int(r[0]))
}

func fnSplit(args []string) string {
	if len(args) < 2 {
		return ""
	}
	// SPLIT returns a string representation of array
	parts := strings.Split(args[0], args[1])
	return "[" + strings.Join(parts, ",") + "]"
}
