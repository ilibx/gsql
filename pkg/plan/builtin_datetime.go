package plan

import (
	"strconv"
	"strings"
	"time"
)

func registerDateTimeBuiltins() {
	RegisterFunc(&FuncDef{
		Name: "CURRENT_DATE", Type: FuncScalar, MinArgs: 0, MaxArgs: 0,
		ScalarFn: fnCurrentDate,
	})
	RegisterFunc(&FuncDef{
		Name: "CURRENT_TIMESTAMP", Type: FuncScalar, MinArgs: 0, MaxArgs: 0,
		ScalarFn: fnCurrentTimestamp,
	})
	RegisterFunc(&FuncDef{
		Name: "UNIX_TIMESTAMP", Type: FuncScalar, MinArgs: 0, MaxArgs: 1,
		ScalarFn: fnUnixTimestamp,
	})
	RegisterFunc(&FuncDef{
		Name: "FROM_UNIXTIME", Type: FuncScalar, MinArgs: 1, MaxArgs: 2,
		ScalarFn: fnFromUnixTime,
	})
	RegisterFunc(&FuncDef{
		Name: "TO_DATE", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnToDate,
	})
	RegisterFunc(&FuncDef{
		Name: "YEAR", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnYear,
	})
	RegisterFunc(&FuncDef{
		Name: "MONTH", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnMonth,
	})
	RegisterFunc(&FuncDef{
		Name: "DAY", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnDay,
	})
	RegisterFunc(&FuncDef{
		Name: "DAYOFMONTH", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnDay,
	})
	RegisterFunc(&FuncDef{
		Name: "HOUR", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnHour,
	})
	RegisterFunc(&FuncDef{
		Name: "MINUTE", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnMinute,
	})
	RegisterFunc(&FuncDef{
		Name: "SECOND", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnSecond,
	})
	RegisterFunc(&FuncDef{
		Name: "WEEKOFYEAR", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnWeekOfYear,
	})
	RegisterFunc(&FuncDef{
		Name: "DATEDIFF", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnDateDiff,
	})
	RegisterFunc(&FuncDef{
		Name: "DATE_ADD", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnDateAdd,
	})
	RegisterFunc(&FuncDef{
		Name: "DATE_SUB", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnDateSub,
	})
	RegisterFunc(&FuncDef{
		Name: "DATE_FORMAT", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnDateFormat,
	})
	RegisterFunc(&FuncDef{
		Name: "ADD_MONTHS", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnAddMonths,
	})
	RegisterFunc(&FuncDef{
		Name: "LAST_DAY", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnLastDay,
	})
	RegisterFunc(&FuncDef{
		Name: "NEXT_DAY", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnNextDay,
	})
	RegisterFunc(&FuncDef{
		Name: "MONTHS_BETWEEN", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnMonthsBetween,
	})
	RegisterFunc(&FuncDef{
		Name: "QUARTER", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnQuarter,
	})
	RegisterFunc(&FuncDef{
		Name: "TRUNC", Type: FuncScalar, MinArgs: 1, MaxArgs: 2,
		ScalarFn: fnTrunc,
	})
	RegisterFunc(&FuncDef{
		Name: "FROM_UTC_TIMESTAMP", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnFromUtcTimestamp,
	})
	RegisterFunc(&FuncDef{
		Name: "TO_UTC_TIMESTAMP", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnToUtcTimestamp,
	})
}

var dateParseFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02",
	"2006/01/02",
	"01/02/2006",
	"2006-01-02 15:04:05.999999999Z07:00",
	time.RFC3339,
}

func parseDate(s string) (time.Time, bool) {
	for _, fmt := range dateParseFormats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func now() time.Time {
	return time.Now()
}

func fnCurrentDate([]string) string {
	return now().Format("2006-01-02")
}

func fnCurrentTimestamp([]string) string {
	return now().Format("2006-01-02 15:04:05")
}

func fnUnixTimestamp(args []string) string {
	if len(args) == 0 {
		return strconv.FormatInt(now().Unix(), 10)
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.FormatInt(t.Unix(), 10)
}

func fnFromUnixTime(args []string) string {
	if len(args) < 1 {
		return ""
	}
	ts, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return args[0]
	}
	t := time.Unix(ts, 0)
	fmt := "2006-01-02 15:04:05"
	if len(args) >= 2 {
		fmt = hiveFmtToGo(args[1])
	}
	return t.Format(fmt)
}

func fnToDate(args []string) string {
	if len(args) < 1 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	return t.Format("2006-01-02")
}

func fnYear(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.Itoa(t.Year())
}

func fnMonth(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.Itoa(int(t.Month()))
}

func fnDay(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.Itoa(t.Day())
}

func fnHour(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.Itoa(t.Hour())
}

func fnMinute(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.Itoa(t.Minute())
}

func fnSecond(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	return strconv.Itoa(t.Second())
}

func fnWeekOfYear(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	_, week := t.ISOWeek()
	return strconv.Itoa(week)
}

func fnDateDiff(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	end, eok := parseDate(args[0])
	start, sok := parseDate(args[1])
	if !eok || !sok {
		return "0"
	}
	days := end.Sub(start) / (24 * time.Hour)
	return strconv.FormatInt(int64(days), 10)
}

func fnDateAdd(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return args[0]
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

func fnDateSub(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return args[0]
	}
	return t.AddDate(0, 0, -n).Format("2006-01-02")
}

func fnDateFormat(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	return t.Format(hiveFmtToGo(args[1]))
}

func fnAddMonths(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return args[0]
	}
	result := t.AddDate(0, n, 0)
	// If the original day exceeds the last day of the target month,
	// truncate to the last day of the target month (standard SQL behavior).
	// Go's AddDate wraps to next month in this case.
	if t.Day() != result.Day() {
		result = result.AddDate(0, 0, -result.Day())
	}
	return result.Format("2006-01-02")
}

func fnLastDay(args []string) string {
	if len(args) < 1 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	firstOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	lastDay := firstOfMonth.AddDate(0, 1, -1)
	return lastDay.Format("2006-01-02")
}

func fnNextDay(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	dayName := strings.ToLower(args[1])
	weekdayMap := map[string]time.Weekday{
		"sunday": time.Sunday, "sun": time.Sunday,
		"monday": time.Monday, "mon": time.Monday,
		"tuesday": time.Tuesday, "tue": time.Tuesday,
		"wednesday": time.Wednesday, "wed": time.Wednesday,
		"thursday": time.Thursday, "thu": time.Thursday,
		"friday": time.Friday, "fri": time.Friday,
		"saturday": time.Saturday, "sat": time.Saturday,
	}
	target, ok := weekdayMap[dayName]
	if !ok {
		return args[0]
	}
	daysUntil := (target - t.Weekday() + 7) % 7
	if daysUntil == 0 {
		daysUntil = 7
	}
	return t.AddDate(0, 0, int(daysUntil)).Format("2006-01-02")
}

func fnMonthsBetween(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	end, eok := parseDate(args[0])
	start, sok := parseDate(args[1])
	if !eok || !sok {
		return "0"
	}
	months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
	// Add fractional month based on day difference
	dayDiff := float64(end.Day() - start.Day())
	daysInMonth := time.Date(start.Year(), start.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	result := float64(months) + dayDiff/float64(daysInMonth)
	return strconv.FormatFloat(result, 'f', 6, 64)
}

func fnQuarter(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	t, ok := parseDate(args[0])
	if !ok {
		return "0"
	}
	q := (int(t.Month()) - 1) / 3
	return strconv.Itoa(q + 1)
}

func fnTrunc(args []string) string {
	if len(args) < 1 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	unit := "DD"
	if len(args) >= 2 {
		unit = strings.ToUpper(args[1])
	}
	switch unit {
	case "YY", "YYYY", "YEAR":
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
	case "MM", "MONTH", "MON":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
	case "DD", "DAY":
		return t.Format("2006-01-02")
	case "HH", "HOUR":
		return t.Format("2006-01-02 15:00:00")
	case "MI", "MINUTE":
		return t.Format("2006-01-02 15:04:00")
	default:
		return t.Format("2006-01-02")
	}
}

func fnFromUtcTimestamp(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	if !ok {
		return args[0]
	}
	loc, err := time.LoadLocation(args[1])
	if err != nil {
		return args[0]
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

func fnToUtcTimestamp(args []string) string {
	if len(args) < 2 {
		return ""
	}
	t, ok := parseDate(args[0])
	// If the input doesn't specify a timezone, parse it as being in the target tz
	if !ok {
		return args[0]
	}
	loc, err := time.LoadLocation(args[1])
	if err != nil {
		return args[0]
	}
	// Treat the parsed time as being in the source timezone
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
	return t.UTC().Format("2006-01-02 15:04:05")
}

func hiveFmtToGo(fmt string) string {
	// Ordered replacements: longer patterns first to avoid partial replacement
	type repl struct{ from, to string }
	repls := []repl{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"MM", "01"},
		{"M", "1"},
		{"dd", "02"},
		{"d", "2"},
		{"HH", "15"},
		{"H", "15"},
		{"mm", "04"},
		{"m", "4"},
		{"ss", "05"},
		{"s", "5"},
		{"SSS", "000"},
	}
	result := fmt
	for _, r := range repls {
		result = strings.ReplaceAll(result, r.from, r.to)
	}
	return result
}
