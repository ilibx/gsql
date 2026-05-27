package plan

import (
	"math"
	"math/rand"
	"strconv"
)

func registerMathBuiltins() {
	RegisterFunc(&FuncDef{
		Name: "ROUND", Type: FuncScalar, MinArgs: 1, MaxArgs: 2,
		ScalarFn: fnRound,
	})
	RegisterFunc(&FuncDef{
		Name: "FLOOR", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnFloor,
	})
	RegisterFunc(&FuncDef{
		Name: "CEIL", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnCeil,
	})
	RegisterFunc(&FuncDef{
		Name: "CEILING", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnCeil,
	})
	RegisterFunc(&FuncDef{
		Name: "ABS", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnAbs,
	})
	RegisterFunc(&FuncDef{
		Name: "SQRT", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnSqrt,
	})
	RegisterFunc(&FuncDef{
		Name: "EXP", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnExp,
	})
	RegisterFunc(&FuncDef{
		Name: "LN", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLn,
	})
	RegisterFunc(&FuncDef{
		Name: "LOG10", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLog10,
	})
	RegisterFunc(&FuncDef{
		Name: "LOG2", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLog2,
	})
	RegisterFunc(&FuncDef{
		Name: "POWER", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnPower,
	})
	RegisterFunc(&FuncDef{
		Name: "POW", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnPower,
	})
	RegisterFunc(&FuncDef{
		Name: "MOD", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnMod,
	})
	RegisterFunc(&FuncDef{
		Name: "SIGN", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnSign,
	})
	RegisterFunc(&FuncDef{
		Name: "RAND", Type: FuncScalar, MinArgs: 0, MaxArgs: 1,
		ScalarFn: fnRand,
	})
	RegisterFunc(&FuncDef{
		Name: "GREATEST", Type: FuncScalar, MinArgs: 2, MaxArgs: -1,
		ScalarFn: fnGreatest,
	})
	RegisterFunc(&FuncDef{
		Name: "LEAST", Type: FuncScalar, MinArgs: 2, MaxArgs: -1,
		ScalarFn: fnLeast,
	})
	RegisterFunc(&FuncDef{
		Name: "WIDTH_BUCKET", Type: FuncScalar, MinArgs: 4, MaxArgs: 4,
		ScalarFn: fnWidthBucket,
	})
}

func fnRound(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	if len(args) >= 2 {
		d, err := strconv.Atoi(args[1])
		if err != nil {
			d = 0
		}
		return strconv.FormatFloat(roundFloat(v, d), 'f', d, 64)
	}
	return strconv.FormatFloat(math.Round(v), 'f', -1, 64)
}

func fnFloor(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Floor(v), 'f', -1, 64)
}

func fnCeil(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Ceil(v), 'f', -1, 64)
}

func fnAbs(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Abs(v), 'f', -1, 64)
}

func fnSqrt(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Sqrt(v), 'f', -1, 64)
}

func fnExp(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Exp(v), 'f', -1, 64)
}

func fnLn(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Log(v), 'f', -1, 64)
}

func fnLog10(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Log10(v), 'f', -1, 64)
}

func fnLog2(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	return strconv.FormatFloat(math.Log2(v), 'f', -1, 64)
}

func fnPower(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	x, xok := toFloat(args[0])
	y, yok := toFloat(args[1])
	if !xok || !yok {
		return args[0]
	}
	return strconv.FormatFloat(math.Pow(x, y), 'f', -1, 64)
}

func fnMod(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	x, xok := toFloat(args[0])
	y, yok := toFloat(args[1])
	if !xok || !yok || y == 0 {
		return args[0]
	}
	return strconv.FormatFloat(math.Mod(x, y), 'f', -1, 64)
}

func fnSign(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	v, ok := toFloat(args[0])
	if !ok {
		return args[0]
	}
	if v > 0 {
		return "1.0"
	} else if v < 0 {
		return "-1.0"
	}
	return "0.0"
}

func fnRand(args []string) string {
	if len(args) >= 1 {
		if seed, err := strconv.ParseInt(args[0], 10, 64); err == nil {
			return strconv.FormatFloat(randWrapper(seed), 'f', -1, 64)
		}
	}
	return strconv.FormatFloat(randWrapper(0), 'f', -1, 64)
}

var randWrapper = func(seed int64) float64 {
	if seed != 0 {
		return rand.New(rand.NewSource(seed)).Float64()
	}
	return rand.Float64()
}

func fnGreatest(args []string) string {
	if len(args) == 0 {
		return ""
	}
	best := args[0]
	for i := 1; i < len(args); i++ {
		if compareValues(args[i], best, ">") {
			best = args[i]
		}
	}
	return best
}

func fnLeast(args []string) string {
	if len(args) == 0 {
		return ""
	}
	best := args[0]
	for i := 1; i < len(args); i++ {
		if compareValues(args[i], best, "<") {
			best = args[i]
		}
	}
	return best
}

func fnWidthBucket(args []string) string {
	if len(args) < 4 {
		return "0"
	}
	x, xok := toFloat(args[0])
	min, minok := toFloat(args[1])
	max, maxok := toFloat(args[2])
	num, numerr := strconv.Atoi(args[3])
	if !xok || !minok || !maxok || numerr != nil || num < 1 || min >= max {
		return "0"
	}
	if x < min {
		return "0"
	}
	if x >= max {
		return strconv.Itoa(num + 1)
	}
	bucket := int(math.Floor((x-min)/(max-min)*float64(num))) + 1
	return strconv.Itoa(bucket)
}
