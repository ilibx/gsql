package plan

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/storage"
)

func registerAggregateBuiltins() {
	RegisterFunc(&FuncDef{
		Name: "COUNT", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggCount,
	})
	RegisterFunc(&FuncDef{
		Name: "SUM", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggSum,
	})
	RegisterFunc(&FuncDef{
		Name: "AVG", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggAvg,
	})
	RegisterFunc(&FuncDef{
		Name: "MIN", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggMin,
	})
	RegisterFunc(&FuncDef{
		Name: "MAX", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggMax,
	})
	RegisterFunc(&FuncDef{
		Name: "STDDEV", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggStddev,
	})
	RegisterFunc(&FuncDef{
		Name: "STDDEV_POP", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggStddev,
	})
	RegisterFunc(&FuncDef{
		Name: "STDDEV_SAMP", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggStddevSamp,
	})
	RegisterFunc(&FuncDef{
		Name: "VARIANCE", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggVariance,
	})
	RegisterFunc(&FuncDef{
		Name: "VAR_POP", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggVariance,
	})
	RegisterFunc(&FuncDef{
		Name: "VAR_SAMP", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggVarianceSamp,
	})
	RegisterFunc(&FuncDef{
		Name: "COLLECT_LIST", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggCollectList,
	})
	RegisterFunc(&FuncDef{
		Name: "COLLECT_SET", Type: FuncAggregate, MinArgs: 1, MaxArgs: 1,
		AggFn: aggCollectSet,
	})
	RegisterFunc(&FuncDef{
		Name: "CORR", Type: FuncAggregate, MinArgs: 2, MaxArgs: 2,
		AggFn: aggCorr,
	})
	RegisterFunc(&FuncDef{
		Name: "COVAR_POP", Type: FuncAggregate, MinArgs: 2, MaxArgs: 2,
		AggFn: aggCovarPop,
	})
	RegisterFunc(&FuncDef{
		Name: "COVAR_SAMP", Type: FuncAggregate, MinArgs: 2, MaxArgs: 2,
		AggFn: aggCovarSamp,
	})
	RegisterFunc(&FuncDef{
		Name: "PERCENTILE", Type: FuncAggregate, MinArgs: 2, MaxArgs: 2,
		AggFn: aggPercentile,
	})
	RegisterFunc(&FuncDef{
		Name: "PERCENTILE_APPROX", Type: FuncAggregate, MinArgs: 2, MaxArgs: 3,
		AggFn: aggPercentileApprox,
	})
	RegisterFunc(&FuncDef{
		Name: "HISTOGRAM_NUMERIC", Type: FuncAggregate, MinArgs: 2, MaxArgs: 2,
		AggFn: aggHistogramNumeric,
	})
}

func aggCount(rows []storage.Row, column string, distinct bool, _ []string) string {
	if distinct {
		seen := make(map[string]bool)
		for _, row := range rows {
			if column == "*" {
				seen["*"] = true
			} else if v := row[column]; v != "" {
				seen[v] = true
			}
		}
		return strconv.Itoa(len(seen))
	}
	return strconv.Itoa(len(rows))
}

func aggSum(rows []storage.Row, column string, _ bool, _ []string) string {
	var total float64
	for _, row := range rows {
		v, err := strconv.ParseFloat(row[column], 64)
		if err == nil {
			total += v
		}
	}
	return strconv.FormatFloat(total, 'f', -1, 64)
}

func aggAvg(rows []storage.Row, column string, _ bool, _ []string) string {
	var total float64
	count := 0
	for _, row := range rows {
		v, err := strconv.ParseFloat(row[column], 64)
		if err == nil {
			total += v
			count++
		}
	}
	if count == 0 {
		return "0"
	}
	return strconv.FormatFloat(total/float64(count), 'f', -1, 64)
}

func aggMin(rows []storage.Row, column string, _ bool, _ []string) string {
	var min string
	set := false
	for _, row := range rows {
		if row[column] == "" {
			continue
		}
		if !set || compareValues(row[column], min, "<") {
			min = row[column]
			set = true
		}
	}
	if !set && len(rows) > 0 {
		min = rows[0][column]
	}
	return min
}

func aggMax(rows []storage.Row, column string, _ bool, _ []string) string {
	var max string
	set := false
	for _, row := range rows {
		if row[column] == "" {
			continue
		}
		if !set || compareValues(row[column], max, ">") {
			max = row[column]
			set = true
		}
	}
	if !set && len(rows) > 0 {
		max = rows[0][column]
	}
	return max
}

func aggStddev(rows []storage.Row, column string, _ bool, _ []string) string {
	v := computeVariance(rows, column, false)
	val, _ := strconv.ParseFloat(v, 64)
	return strconv.FormatFloat(math.Sqrt(val), 'f', -1, 64)
}

func aggStddevSamp(rows []storage.Row, column string, _ bool, _ []string) string {
	v := computeVariance(rows, column, true)
	val, _ := strconv.ParseFloat(v, 64)
	return strconv.FormatFloat(math.Sqrt(val), 'f', -1, 64)
}

func aggVariance(rows []storage.Row, column string, _ bool, _ []string) string {
	return computeVariance(rows, column, false)
}

func aggVarianceSamp(rows []storage.Row, column string, _ bool, _ []string) string {
	return computeVariance(rows, column, true)
}

func computeVariance(rows []storage.Row, column string, sample bool) string {
	var vals []float64
	for _, row := range rows {
		v, err := strconv.ParseFloat(row[column], 64)
		if err == nil {
			vals = append(vals, v)
		}
	}
	n := len(vals)
	if n == 0 {
		return "0"
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(n)
	var sqDiff float64
	for _, v := range vals {
		d := v - mean
		sqDiff += d * d
	}
	div := n
	if sample && n > 1 {
		div = n - 1
	}
	return strconv.FormatFloat(sqDiff/float64(div), 'f', -1, 64)
}

func aggCollectList(rows []storage.Row, column string, _ bool, _ []string) string {
	var parts []string
	for _, row := range rows {
		if v := row[column]; v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return "[]"
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ",") + "]"
}

func aggCollectSet(rows []storage.Row, column string, _ bool, _ []string) string {
	seen := make(map[string]bool)
	for _, row := range rows {
		if v := row[column]; v != "" {
			seen[v] = true
		}
	}
	if len(seen) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(seen))
	for k := range seen {
		parts = append(parts, k)
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ",") + "]"
}

func aggCorr(rows []storage.Row, column string, _ bool, args []string) string {
	col1 := column
	col2 := ""
	if len(args) > 1 {
		col2 = args[1]
	} else {
		return "0"
	}
	n := 0
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, row := range rows {
		x, errX := strconv.ParseFloat(row[col1], 64)
		y, errY := strconv.ParseFloat(row[col2], 64)
		if errX != nil || errY != nil {
			continue
		}
		n++
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}
	if n == 0 {
		return "0"
	}
	num := float64(n)*sumXY - sumX*sumY
	den := math.Sqrt((float64(n)*sumX2 - sumX*sumX) * (float64(n)*sumY2 - sumY*sumY))
	if den == 0 {
		return "0"
	}
	return strconv.FormatFloat(num/den, 'f', -1, 64)
}

func aggCovarPop(rows []storage.Row, column string, _ bool, args []string) string {
	return computeCovariance(rows, column, args, false)
}

func aggCovarSamp(rows []storage.Row, column string, _ bool, args []string) string {
	return computeCovariance(rows, column, args, true)
}

func computeCovariance(rows []storage.Row, column string, args []string, sample bool) string {
	col1 := column
	col2 := ""
	if len(args) > 1 {
		col2 = args[1]
	} else {
		return "0"
	}
	n := 0
	var sumX, sumY, sumXY float64
	for _, row := range rows {
		x, errX := strconv.ParseFloat(row[col1], 64)
		y, errY := strconv.ParseFloat(row[col2], 64)
		if errX != nil || errY != nil {
			continue
		}
		n++
		sumX += x
		sumY += y
		sumXY += x * y
	}
	if n <= 1 {
		return "0"
	}
	div := n
	if sample {
		div = n - 1
	}
	cov := (sumXY - sumX*sumY/float64(n)) / float64(div)
	return strconv.FormatFloat(cov, 'f', -1, 64)
}

func aggPercentile(rows []storage.Row, column string, _ bool, args []string) string {
	var p float64
	if len(args) > 1 {
		var err error
		p, err = strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "0"
		}
	} else {
		return "0"
	}
	var vals []float64
	for _, row := range rows {
		v, err := strconv.ParseFloat(row[column], 64)
		if err == nil {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return "0"
	}
	sort.Float64s(vals)
	idx := p * float64(len(vals)-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= len(vals) {
		return strconv.FormatFloat(vals[len(vals)-1], 'f', -1, 64)
	}
	frac := idx - float64(lo)
	result := vals[lo]*(1-frac) + vals[hi]*frac
	return strconv.FormatFloat(result, 'f', -1, 64)
}

func aggPercentileApprox(rows []storage.Row, column string, _ bool, args []string) string {
	return aggPercentile(rows, column, false, args)
}

func aggHistogramNumeric(rows []storage.Row, column string, _ bool, args []string) string {
	var numBuckets int
	if len(args) > 1 {
		var err error
		numBuckets, err = strconv.Atoi(args[1])
		if err != nil || numBuckets < 1 {
			numBuckets = 10
		}
	} else {
		return "[]"
	}
	var vals []float64
	for _, row := range rows {
		v, err := strconv.ParseFloat(row[column], 64)
		if err == nil {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return "[]"
	}
	sort.Float64s(vals)
	min := vals[0]
	max := vals[len(vals)-1]
	if min == max {
		return "[[" + strconv.FormatFloat(min, 'f', -1, 64) + "," + strconv.Itoa(len(vals)) + "]]"
	}
	bucketWidth := (max - min) / float64(numBuckets)
	buckets := make([]int, numBuckets)
	for _, v := range vals {
		idx := int((v - min) / bucketWidth)
		if idx >= numBuckets {
			idx = numBuckets - 1
		}
		buckets[idx]++
	}
	parts := make([]string, numBuckets)
	for i, count := range buckets {
		lo := min + float64(i)*bucketWidth
		hi := lo + bucketWidth
		parts[i] = "[" + strconv.FormatFloat(lo, 'f', -1, 64) + "-" + strconv.FormatFloat(hi, 'f', -1, 64) + ":" + strconv.Itoa(count) + "]"
	}
	return "[" + strings.Join(parts, ",") + "]"
}
