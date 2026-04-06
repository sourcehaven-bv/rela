package lua

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"
	lua "github.com/yuin/gopher-lua"
)

const daysPerWeek = 7

// weekdayNames maps lowercase weekday names to time.Weekday values.
var weekdayNames = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// registerDateHelpers adds date and RRULE utility functions to the rela table.
func registerDateHelpers(ls *lua.LState, rela *lua.LTable) {
	ls.SetField(rela, "date_add", ls.NewFunction(luaDateAdd))
	ls.SetField(rela, "date_weekday", ls.NewFunction(luaDateWeekday))
	ls.SetField(rela, "date_next_weekday", ls.NewFunction(luaDateNextWeekday))
	ls.SetField(rela, "rrule_next", ls.NewFunction(luaRruleNext))
}

// parseDate parses a date string in RFC3339 or date-only format.
// Date-only strings are parsed in local time to avoid off-by-one errors
// when comparing with time.Now() near midnight.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.ParseInLocation("2006-01-02", s, time.Local)
	}
	return t, err
}

// parseDateUTC parses a date string in RFC3339 or date-only format, always in UTC.
// Used by rrule_next where all date comparisons happen in UTC.
func parseDateUTC(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
	}
	return t, err
}

// luaDateAdd implements rela.date_add(date, offset) -> string
// Adds an offset like "7d", "2w", "1m", "1y" to a date. Negative offsets
// are supported with a leading minus: "-3d", "-1m".
func luaDateAdd(ls *lua.LState) int {
	dateStr := ls.CheckString(1)
	offsetStr := ls.CheckString(2)

	t, err := parseDate(dateStr)
	if err != nil {
		ls.RaiseError("date_add: invalid date %q", dateStr)
		return 0
	}

	years, months, days, err := parseOffset(offsetStr)
	if err != nil {
		ls.RaiseError("date_add: invalid offset %q: %s", offsetStr, err)
		return 0
	}

	result := t.AddDate(years, months, days)
	ls.Push(lua.LString(result.Format("2006-01-02")))
	return 1
}

// luaDateWeekday implements rela.date_weekday(date) -> string
// Returns the lowercase weekday name for a date.
func luaDateWeekday(ls *lua.LState) int {
	dateStr := ls.CheckString(1)

	t, err := parseDate(dateStr)
	if err != nil {
		ls.RaiseError("date_weekday: invalid date %q", dateStr)
		return 0
	}

	ls.Push(lua.LString(strings.ToLower(t.Weekday().String())))
	return 1
}

// luaDateNextWeekday implements rela.date_next_weekday(date, weekday) -> string
// Returns the next occurrence of the given weekday strictly after the given date.
func luaDateNextWeekday(ls *lua.LState) int {
	dateStr := ls.CheckString(1)
	dayName := ls.CheckString(2)

	t, err := parseDate(dateStr)
	if err != nil {
		ls.RaiseError("date_next_weekday: invalid date %q", dateStr)
		return 0
	}

	target, ok := weekdayNames[strings.ToLower(dayName)]
	if !ok {
		ls.RaiseError("date_next_weekday: invalid weekday %q", dayName)
		return 0
	}

	daysAhead := int(target-t.Weekday()+daysPerWeek) % daysPerWeek
	if daysAhead == 0 {
		daysAhead = daysPerWeek // always advance, never same day
	}

	result := t.AddDate(0, 0, daysAhead)
	ls.Push(lua.LString(result.Format("2006-01-02")))
	return 1
}

// luaRruleNext implements rela.rrule_next(rrule_string, after?) -> string|nil
// Computes the next occurrence of an RRULE after the given date (or today).
// Accepts both "RRULE:FREQ=..." and bare "FREQ=..." formats.
//
// Rules with INTERVAL > 1 require an explicit DTSTART in the RRULE string
// to anchor the interval counting. Without it, the interval cadence would
// drift on every invocation.
func luaRruleNext(ls *lua.LState) int {
	rruleStr := ls.CheckString(1)
	afterStr := ls.OptString(2, "")

	var after time.Time
	if afterStr == "" {
		// Use today at midnight UTC for consistency with RRULE dates.
		now := time.Now()
		after = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		var err error
		after, err = parseDateUTC(afterStr)
		if err != nil {
			ls.RaiseError("rrule_next: invalid after date %q", afterStr)
			return 0
		}
	}

	// Strip RRULE: prefix if present
	rruleStr = strings.TrimPrefix(rruleStr, "RRULE:")

	// Parse in UTC — RRULE deals with dates, not times.
	opt, err := rrule.StrToROption(rruleStr)
	if err != nil {
		ls.RaiseError("rrule_next: invalid RRULE %q: %s", rruleStr, err)
		return 0
	}

	// Reject INTERVAL > 1 without DTSTART — the interval cadence needs a
	// stable anchor point, otherwise it drifts on every invocation.
	if opt.Interval > 1 && opt.Dtstart.IsZero() {
		ls.RaiseError("rrule_next: INTERVAL > 1 requires DTSTART in the RRULE string")
		return 0
	}

	rule, err := rrule.NewRRule(*opt)
	if err != nil {
		ls.RaiseError("rrule_next: failed to create rule: %s", err)
		return 0
	}

	// Get the next occurrence strictly after the after date.
	next := rule.After(after, false)
	if next.IsZero() {
		ls.Push(lua.LNil)
		return 1
	}

	ls.Push(lua.LString(next.Format("2006-01-02")))
	return 1
}

// parseOffset parses an offset string like "7d", "2w", "1m", "1y", "-3d"
// and returns (years, months, days).
func parseOffset(s string) (years, months, days int, err error) {
	if len(s) < 2 {
		return 0, 0, 0, fmt.Errorf("too short")
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid number %q", numStr)
	}

	switch unit {
	case 'd':
		return 0, 0, n, nil
	case 'w':
		return 0, 0, n * daysPerWeek, nil
	case 'm':
		return 0, n, 0, nil
	case 'y':
		return n, 0, 0, nil
	default:
		return 0, 0, 0, fmt.Errorf("unknown unit %q (use d, w, m, y)", string(unit))
	}
}
