package predicatefns_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// day builds a UTC-midnight date value, the shape every date host
// function produces and consumes.
func day(t *testing.T, s string) predicate.Date {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return predicate.NewDate(parsed)
}

// dateEnv builds an env+bindings pair with the stdlib declared and
// bound, plus `a`/`b` date vars and a `rule` string var, so a test can
// compile a real expression rather than poke the Go functions directly.
func dateEnv(t *testing.T, now time.Time) (*predicate.Env, *predicate.Bindings) {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("a", predicate.DateType); err != nil {
		t.Fatalf("declare a: %v", err)
	}
	if err := env.DeclareVar("b", predicate.DateType); err != nil {
		t.Fatalf("declare b: %v", err)
	}
	if err := env.DeclareVar("rule", predicate.StringType); err != nil {
		t.Fatalf("declare rule: %v", err)
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare stdlib: %v", err)
	}
	b := predicate.NewBindings()
	if err := predicatefns.Bind(b, now); err != nil {
		t.Fatalf("bind stdlib: %v", err)
	}
	return env, b
}

func TestDaysBetween(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want float64
	}{
		{"same day", "2026-08-18", "2026-08-18", 0},
		{"a one day after b", "2026-08-19", "2026-08-18", 1},
		{"a one day before b", "2026-08-17", "2026-08-18", -1},
		{"a week out", "2026-08-25", "2026-08-18", 7},
		{"across a month boundary", "2026-09-01", "2026-08-30", 2},
		{"across a year boundary", "2027-01-01", "2026-12-31", 1},
		{"leap day", "2028-03-01", "2028-02-28", 2},
		{"large span", "2027-08-18", "2026-08-18", 365},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, b := dateEnv(t, time.Now())
			prog, err := predicate.Compile(env, "days_between(a, b) == "+trimFloat(tc.want))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if err := b.SetVar("a", day(t, tc.a)); err != nil {
				t.Fatalf("set a: %v", err)
			}
			if err := b.SetVar("b", day(t, tc.b)); err != nil {
				t.Fatalf("set b: %v", err)
			}
			v, err := prog.Eval(context.Background(), b)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			got, ok := v.(predicate.Bool)
			if !ok {
				t.Fatalf("want Bool, got %T", v)
			}
			if !got.Bool() {
				t.Errorf("days_between(%s, %s) != %v", tc.a, tc.b, tc.want)
			}
		})
	}
}

// TestDaysBetween_TimeComponentTruncated pins RR-YPYTP: a date carrying
// a late-in-the-day time component must still yield a whole-day count,
// not a fraction that rounds the wrong way.
func TestDaysBetween_TimeComponentTruncated(t *testing.T) {
	env, b := dateEnv(t, time.Now())
	prog, err := predicate.Compile(env, "days_between(a, b) == 1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// a is 23:59 on the 19th; b is 00:00 on the 18th. Raw subtraction
	// gives 1.999… days, which truncates to 1 only because both sides
	// are floored to UTC midnight first.
	if err := b.SetVar("a", predicate.NewDate(
		time.Date(2026, 8, 19, 23, 59, 0, 0, time.UTC))); err != nil {
		t.Fatalf("set a: %v", err)
	}
	if err := b.SetVar("b", predicate.NewDate(
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("set b: %v", err)
	}
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := v.(predicate.Bool); !got.Bool() {
		t.Error("time components not truncated to whole days")
	}
}

// TestDaysBetween_LocalMidnightBoundary pins the UTC convention against
// a non-UTC input: a date built in a zone east of UTC late in the day is
// still counted by its UTC calendar day, matching today().
func TestDaysBetween_LocalMidnightBoundary(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	env, b := dateEnv(t, time.Now())
	// 2026-08-19 08:00 +09:00 == 2026-08-18 23:00 UTC, so against a
	// UTC 2026-08-18 baseline the difference is 0 days, not 1.
	prog, cErr := predicate.Compile(env, "days_between(a, b) == 0")
	if cErr != nil {
		t.Fatalf("compile: %v", cErr)
	}
	if err := b.SetVar("a", predicate.NewDate(
		time.Date(2026, 8, 19, 8, 0, 0, 0, tokyo))); err != nil {
		t.Fatalf("set a: %v", err)
	}
	if err := b.SetVar("b", predicate.NewDate(
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("set b: %v", err)
	}
	v, evErr := prog.Eval(context.Background(), b)
	if evErr != nil {
		t.Fatalf("eval: %v", evErr)
	}
	if got := v.(predicate.Bool); !got.Bool() {
		t.Error("date not normalized to its UTC calendar day")
	}
}

// TestDaysBetween_DSTSpan pins that a span crossing a DST transition
// still counts whole calendar days. Both ends are normalized to UTC
// midnight before subtracting, so the 25-hour local day cannot round the
// result down.
func TestDaysBetween_DSTSpan(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	env, b := dateEnv(t, time.Now())
	prog, cErr := predicate.Compile(env, "days_between(a, b) == 6")
	if cErr != nil {
		t.Fatalf("compile: %v", cErr)
	}
	// US DST ends 2026-11-01, so this span contains a 25-hour local day.
	if err := b.SetVar("a", predicate.NewDate(
		time.Date(2026, 11, 5, 0, 0, 0, 0, ny))); err != nil {
		t.Fatalf("set a: %v", err)
	}
	if err := b.SetVar("b", predicate.NewDate(
		time.Date(2026, 10, 30, 0, 0, 0, 0, ny))); err != nil {
		t.Fatalf("set b: %v", err)
	}
	v, evErr := prog.Eval(context.Background(), b)
	if evErr != nil {
		t.Fatalf("eval: %v", evErr)
	}
	if got := v.(predicate.Bool); !got.Bool() {
		t.Error("DST transition skewed the day count")
	}
}

func TestDateAdd(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"add days", "date_add(a, 3, 'day')", "2026-08-21"},
		{"subtract days", "date_add(a, -3, 'day')", "2026-08-15"},
		{"zero days", "date_add(a, 0, 'day')", "2026-08-18"},
		{"add weeks", "date_add(a, 2, 'week')", "2026-09-01"},
		{"subtract weeks", "date_add(a, -1, 'week')", "2026-08-11"},
		{"crosses month end", "date_add(a, 14, 'day')", "2026-09-01"},
		{"unit is case-insensitive", "date_add(a, 1, 'DAY')", "2026-08-19"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, b := dateEnv(t, time.Now())
			prog, err := predicate.Compile(env, tc.expr+" == b")
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if err := b.SetVar("a", day(t, "2026-08-18")); err != nil {
				t.Fatalf("set a: %v", err)
			}
			if err := b.SetVar("b", day(t, tc.want)); err != nil {
				t.Fatalf("set b: %v", err)
			}
			v, err := prog.Eval(context.Background(), b)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			got, ok := v.(predicate.Bool)
			if !ok {
				t.Fatalf("want Bool, got %T", v)
			}
			if !got.Bool() {
				t.Errorf("%s != %s", tc.expr, tc.want)
			}
		})
	}
}

// TestDateAdd_LeapDay pins that day arithmetic crosses Feb 29 in a leap
// year rather than skipping it.
func TestDateAdd_LeapDay(t *testing.T) {
	env, b := dateEnv(t, time.Now())
	prog, err := predicate.Compile(env, "date_add(a, 1, 'day') == b")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := b.SetVar("a", day(t, "2028-02-28")); err != nil {
		t.Fatalf("set a: %v", err)
	}
	if err := b.SetVar("b", day(t, "2028-02-29")); err != nil {
		t.Fatalf("set b: %v", err)
	}
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := v.(predicate.Bool); !got.Bool() {
		t.Error("leap day skipped")
	}
}

// TestDateAdd_RejectsUnsupportedUnit pins the v1 restriction. month/year
// are refused rather than delegated to AddDate's silent end-of-month
// normalization.
func TestDateAdd_RejectsUnsupportedUnit(t *testing.T) {
	for _, unit := range []string{"month", "year", "fortnight", ""} {
		t.Run("unit="+unit, func(t *testing.T) {
			env, b := dateEnv(t, time.Now())
			prog, err := predicate.Compile(env,
				"date_add(a, 1, '"+unit+"') == a")
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if err := b.SetVar("a", day(t, "2026-01-31")); err != nil {
				t.Fatalf("set a: %v", err)
			}
			_, err = prog.Eval(context.Background(), b)
			if err == nil {
				t.Fatalf("unit %q: want eval error, got none", unit)
			}
			if !strings.Contains(err.Error(), "unsupported unit") {
				t.Errorf("unit %q: error %q does not name the problem", unit, err)
			}
		})
	}
}

func TestRruleNext(t *testing.T) {
	tests := []struct {
		name  string
		rule  string
		after string
		want  string
	}{
		{"daily", "FREQ=DAILY", "2026-08-18", "2026-08-19"},
		{"weekly", "FREQ=WEEKLY", "2026-08-18", "2026-08-25"},
		{"monthly", "FREQ=MONTHLY", "2026-08-18", "2026-09-18"},
		{"yearly", "FREQ=YEARLY", "2026-08-18", "2027-08-18"},
		{
			// INTERVAL > 1 requires DTSTART or ValidateRrule rejects it:
			// without an anchor the cadence drifts. Pinned here so the
			// stdlib and the `rrule` property type stay in agreement.
			"every 3 days from a DTSTART",
			"DTSTART:20260818T000000Z\nFREQ=DAILY;INTERVAL=3",
			"2026-08-18", "2026-08-21",
		},
		{"RRULE: prefix accepted", "RRULE:FREQ=DAILY", "2026-08-18", "2026-08-19"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, b := dateEnv(t, time.Now())
			prog, err := predicate.Compile(env, "rrule_next(rule, a) == b")
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if err := b.SetVar("rule", predicate.NewString(tc.rule)); err != nil {
				t.Fatalf("set rule: %v", err)
			}
			if err := b.SetVar("a", day(t, tc.after)); err != nil {
				t.Fatalf("set a: %v", err)
			}
			if err := b.SetVar("b", day(t, tc.want)); err != nil {
				t.Fatalf("set b: %v", err)
			}
			v, err := prog.Eval(context.Background(), b)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			got, ok := v.(predicate.Bool)
			if !ok {
				t.Fatalf("want Bool, got %T", v)
			}
			if !got.Bool() {
				t.Errorf("rrule_next(%q, %s) != %s", tc.rule, tc.after, tc.want)
			}
		})
	}
}

// TestRruleNext_Malformed pins that a broken rule is an ERROR, not a
// silent "no occurrence" — an operator typo must not read as a finished
// schedule.
func TestRruleNext_Malformed(t *testing.T) {
	for _, rule := range []string{"NOT-A-RULE", "FREQ=NEVER", ""} {
		t.Run("rule="+rule, func(t *testing.T) {
			env, b := dateEnv(t, time.Now())
			prog, err := predicate.Compile(env, "rrule_next(rule, a) == a")
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if err := b.SetVar("rule", predicate.NewString(rule)); err != nil {
				t.Fatalf("set rule: %v", err)
			}
			if err := b.SetVar("a", day(t, "2026-08-18")); err != nil {
				t.Fatalf("set a: %v", err)
			}
			if _, err := prog.Eval(context.Background(), b); err == nil {
				t.Errorf("rule %q: want eval error, got none", rule)
			}
		})
	}
}

// TestRruleNext_Exhausted pins that a rule with no further occurrence
// reports ErrRruleExhausted. It cannot return nil: the engine enforces
// declared return types (eval.go:156), and a zero Date would compare as
// a real year-1 value no caller could tell from a genuine date.
//
// Asserted through the exported [predicatefns.RruleNext] rather than
// through Eval, because predicate.EvalError flattens a host error into
// a message string (errors.go:41) — nothing wrapped survives that
// boundary. The sentinel is the contract for a Go caller.
func TestRruleNext_Exhausted(t *testing.T) {
	// COUNT=1 anchored at the same day: its single occurrence is not
	// strictly after `after`, so nothing remains.
	_, err := predicatefns.RruleNext(
		"DTSTART:20260818T000000Z\nFREQ=DAILY;COUNT=1",
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("want an error for an exhausted rule, got none")
	}
	if !errors.Is(err, predicatefns.ErrRruleExhausted) {
		t.Errorf("want ErrRruleExhausted, got %v", err)
	}
}

// TestRruleNext_ExhaustedThroughEval pins the operator-visible half:
// through Eval the sentinel is flattened, but the message still says
// "exhausted" so a finished schedule is distinguishable from a typo in
// a log line.
func TestRruleNext_ExhaustedThroughEval(t *testing.T) {
	env, b := dateEnv(t, time.Now())
	prog, err := predicate.Compile(env, "rrule_next(rule, a) == a")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := b.SetVar("rule", predicate.NewString(
		"DTSTART:20260818T000000Z\nFREQ=DAILY;COUNT=1")); err != nil {
		t.Fatalf("set rule: %v", err)
	}
	if err := b.SetVar("a", day(t, "2026-08-18")); err != nil {
		t.Fatalf("set a: %v", err)
	}
	_, err = prog.Eval(context.Background(), b)
	if err == nil {
		t.Fatal("want an error for an exhausted rule, got none")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("error %q does not identify exhaustion", err)
	}
}

// TestDateFuncs_WrongArgTypes pins that the compiler rejects mistyped
// arguments, so an operator sees the mistake at load rather than at
// write time.
func TestDateFuncs_WrongArgTypes(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"days_between on a string", "days_between(rule, a) == 0"},
		{"date_add unit as a number", "date_add(a, 1, 2) == a"},
		{"date_add count as a string", "date_add(a, 'x', 'day') == a"},
		{"rrule_next rule as a date", "rrule_next(a, a) == a"},
		{"days_between arity", "days_between(a) == 0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, _ := dateEnv(t, time.Now())
			if _, err := predicate.Compile(env, tc.expr); err == nil {
				t.Errorf("want compile error for %q, got none", tc.expr)
			}
		})
	}
}

// trimFloat renders a whole-number float as predicate source ("7", "-1").
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
