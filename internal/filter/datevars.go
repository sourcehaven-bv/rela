package filter

import (
	"regexp"
	"strconv"
	"time"
)

// Date variables a filter value may name instead of a literal date. They
// resolve against the operation's read time (see internal/readtime), in UTC:
//
//	$today        the read day, YYYY-MM-DD
//	$now          the read time, RFC 3339 (whole seconds, Z)
//	$tomorrow     $today+1d
//	$yesterday    $today-1d
//
// $today takes an optional offset of whole days or weeks: `$today+7d`,
// `$today-2w`. $now takes none: a predicate condition has no way to shift
// an instant by less than a day (date_add works on days), so `$now+1d`
// could not mean the same in a `--where` transpiled to predicate as it does
// here. Write `$today+1d` for a day boundary instead.
//
// A date variable is not a pattern: `$today*` or `$todayx` is a literal.
var dateVarRe = regexp.MustCompile(`^\$today(?:([+-])(\d{1,6})([dw]))?$`)

// DateLayout is the layout $today resolves to and a `date` property stores.
const DateLayout = "2006-01-02"

// daysPerWeek converts a week offset to days.
const daysPerWeek = 7

// maxOffsetDays bounds an offset, as date_add bounds its count: `--where`
// transpiles `$today+Nd` to date_add, so a larger offset would resolve here
// and fail there.
const maxOffsetDays = 1_000_000

// ResolveDateVariable returns the value a date variable denotes at the read
// time now, and true; any other value is returned unchanged with false.
func ResolveDateVariable(value string, now time.Time) (string, bool) {
	v, ok := DateVariable(value)
	if !ok {
		return value, false
	}
	t := now.UTC().Truncate(time.Second).AddDate(0, 0, v.Days)
	if v.Now {
		return t.Format(time.RFC3339), true
	}
	return t.Format(DateLayout), true
}

// DateVar is a parsed date variable: the read time ($now) or the read day
// ($today) moved by Days days. Days is always 0 when Now is set.
type DateVar struct {
	Now  bool
	Days int
}

// DateVariable parses value as a date variable. ok is false for any other
// value. A caller that turns the variable into an expression evaluated
// later (predicatefns.FromFilter) uses it rather than [ResolveDateVariable].
func DateVariable(value string) (DateVar, bool) {
	switch value {
	case "$tomorrow":
		return DateVar{Days: 1}, true
	case "$yesterday":
		return DateVar{Days: -1}, true
	case "$now":
		return DateVar{Now: true}, true
	}
	m := dateVarRe.FindStringSubmatch(value)
	if m == nil {
		return DateVar{}, false
	}
	var days int
	if m[1] != "" {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			// coverage-ignore: the pattern admits at most six digits, which always parse
			return DateVar{}, false
		}
		if m[3] == "w" {
			n *= daysPerWeek
		}
		if n > maxOffsetDays {
			return DateVar{}, false
		}
		if m[1] == "-" {
			n = -n
		}
		days = n
	}
	return DateVar{Days: days}, true
}

// BindDateVariables returns filters with every date variable resolved at
// the read time now. The input is not modified: a parsed filter may be
// cached and shared across requests, each with its own read time.
//
// Nil: a nil filter in the slice is kept as nil.
func BindDateVariables(filters []*Filter, now time.Time) []*Filter {
	out := make([]*Filter, len(filters))
	for i, f := range filters {
		out[i] = f
		if f == nil {
			continue
		}
		if v, ok := ResolveDateVariable(f.Value, now); ok {
			c := *f
			c.Value = v
			out[i] = &c
		}
	}
	return out
}
