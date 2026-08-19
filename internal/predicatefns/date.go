package predicatefns

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// Date arithmetic host functions.
//
// # Why functions and not operators
//
// `entity.due + 30` reads naturally but hides a policy decision: 30 of
// what? Adding a month to Jan 31 has no single right answer, and Go's
// AddDate silently normalizes it to Mar 2/3. A function names the unit
// at the call site, so the operator's ambiguity never arises.
//
// # UTC (RR-YPYTP)
//
// Every function here truncates to UTC midnight, matching today() and
// the date-literal parser (parseDateLiteral yields UTC when the layout
// carries no zone). Truncating in now.Location() instead would make
// `days_between(entity.due, today())` skew by up to a day either side of
// local midnight.
//
// # Parsing at eval (RR-A3EZR)
//
// Date VALUES arrive already parsed, so days_between/date_add do no
// parsing. rrule_next is the deliberate exception: its rule is a string
// that routinely comes from an entity property (atlas's `terugkerend`
// stores one in `herhaling`), so it can never be compile-validated.
// It parses at eval and returns a SINGLE occurrence — never an
// unbounded expansion.

const (
	FuncDaysBetween = "days_between" // days_between(a, b) number
	FuncDateAdd     = "date_add"     // date_add(d, n, unit) date
	FuncRruleNext   = "rrule_next"   // rrule_next(rule, after) date
)

// Units accepted by date_add. Deliberately day/week only for v1 —
// see the unitDays doc.
const (
	unitDay   = "day"
	unitWeek  = "week"
	unitMonth = "month"
	unitYear  = "year"
)

// daysPerWeek converts a week count to days.
const daysPerWeek = 7

// monthsPerYear converts a year count to months for addMonths.
const monthsPerYear = 12

// maxDateAddDays bounds date_add's count. ~2700 years of days: far
// beyond any real schedule, and small enough that the result stays
// inside time.Time's comfortable range after the week multiplier.
const maxDateAddDays = 1_000_000

// secondsPerDay converts a Unix-second difference to whole days. Safe
// because both operands are UTC-midnight truncated, so the division is
// exact and no DST transition can shorten or lengthen the day.
//
// Deliberately NOT computed via time.Duration: that is an int64
// nanosecond count capping at ~292 years, and Sub SATURATES rather than
// wrapping. A birthdate, or a zero-valued date (year 1), would silently
// yield a plausible-looking wrong number — 9999-01-01 minus 1000-01-01
// gave 106751 days instead of 3286817. Unix seconds cannot saturate in
// any realistic range.
const secondsPerDay = 24 * 60 * 60

// utcDay truncates t to UTC midnight. This is the single place the
// convention is applied, so today(), days_between and date_add cannot
// drift apart.
func utcDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// twoDates extracts two Date arguments.
func twoDates(args []predicate.Value) (first, second time.Time, err error) {
	if len(args) != 2 {
		return time.Time{}, time.Time{}, errArg
	}
	a, ok := args[0].(predicate.Date)
	if !ok {
		return time.Time{}, time.Time{}, errArg
	}
	b, ok := args[1].(predicate.Date)
	if !ok {
		return time.Time{}, time.Time{}, errArg
	}
	return a.Time(), b.Time(), nil
}

// daysBetween implements days_between(a, b) -> number: whole days from
// b to a, signed. `days_between(due, today())` is therefore POSITIVE
// while due is in the future and NEGATIVE once it has passed, which is
// what makes `days_between(entity.due, today()) <= 7` read as "due
// within a week".
//
// Both sides are truncated to UTC midnight first, so the result is
// always a whole number of days and never a fraction from stray time
// components.
func daysBetween(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	a, b, err := twoDates(args)
	if err != nil {
		return nil, err
	}
	// Int, not Number: a day count is a whole number, and the engine
	// requires both sides of an ordered comparison to share a type. As
	// Number this could not be compared against an integer-typed
	// property — `days_between(...) <= entity.doorlooptijd` would fail
	// to compile, which is precisely the recurring-task shape this
	// function exists for.
	days := (utcDay(a).Unix() - utcDay(b).Unix()) / secondsPerDay
	return predicate.NewInt(days), nil
}

// dateAdd implements date_add(d, n, unit) -> date. n may be negative to
// subtract. unit is "day" or "week" only.
//
// month/year are rejected rather than delegated to AddDate: AddDate
// normalizes Jan 31 + 1 month to Mar 2/3 (or Mar 3 in a leap year),
// which is a silent policy decision of exactly the kind this package's
// function-over-operator choice exists to make explicit. They can be
// added later with a stated clamp-to-end-of-month rule.
func dateAdd(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	if len(args) != 3 {
		return nil, errArg
	}
	d, ok := args[0].(predicate.Date)
	if !ok {
		return nil, errArg
	}
	// Number, not Int: literal coercion happens only in comparisons, not
	// in argument position, so a plain `date_add(d, 3, 'day')` arrives as
	// a Number. days_between differs — its RESULT is compared against
	// properties, so it returns Int.
	n, ok := args[1].(predicate.Number)
	if !ok {
		return nil, errArg
	}
	unit, ok := args[2].(predicate.String)
	if !ok {
		return nil, errArg
	}

	// Reject a fractional or out-of-range count rather than truncating
	// it: this package makes the caller state what they mean instead of
	// silently normalizing. int(1e20) saturates to maxint and AddDate
	// then wraps the date BACKWARDS, which is the sort of thing that
	// looks like a working config until it doesn't.
	f := n.Float()
	if f != math.Trunc(f) || math.Abs(f) > maxDateAddDays {
		return nil, fmt.Errorf(
			"predicatefns: date_add: count %v must be a whole number within ±%d",
			f, int64(maxDateAddDays))
	}

	n32 := int(f)
	base := utcDay(d.Time())
	switch strings.ToLower(strings.TrimSpace(unit.String())) {
	case unitDay:
		return predicate.NewDate(base.AddDate(0, 0, n32)), nil
	case unitWeek:
		return predicate.NewDate(base.AddDate(0, 0, n32*daysPerWeek)), nil
	case unitMonth:
		return predicate.NewDate(addMonths(base, n32)), nil
	case unitYear:
		// A year is 12 months, so leap-day clamping comes free:
		// 2028-02-29 + 1 year clamps to 2029-02-28 rather than rolling
		// into March.
		return predicate.NewDate(addMonths(base, n32*monthsPerYear)), nil
	default:
		return nil, fmt.Errorf(
			"predicatefns: date_add: unsupported unit %q (use %q, %q, %q or %q)",
			unit.String(), unitDay, unitWeek, unitMonth, unitYear)
	}
}

// addMonths shifts t by n months, CLAMPING to the last valid day of the
// target month rather than spilling into the next one.
//
//	2026-01-31 + 1 month -> 2026-02-28   (not 2026-03-03)
//	2028-01-31 + 1 month -> 2028-02-29   (leap year)
//	2026-03-31 - 1 month -> 2026-02-28
//
// time.AddDate normalizes instead: it builds February 31st and lets the
// calendar carry it into March. That is defensible for a general date
// library and wrong for a schedule — "the last of every month" is the
// canonical recurring shape, and normalization silently turns it into
// "the 3rd of every other month".
//
// Clamping is the rule stated in the docs; it is not a guess the caller
// has to reverse-engineer, which is why month/year can be offered at all
// (they were withheld in v1 precisely because AddDate's behavior was
// unstated).
func addMonths(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	// Normalize to the first of the month, shift, then re-apply the day
	// clamped to that month's length. Going via day 1 avoids AddDate's
	// overflow entirely.
	shifted := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, n, 0)
	if last := daysInMonth(shifted.Year(), shifted.Month()); d > last {
		d = last
	}
	return time.Date(shifted.Year(), shifted.Month(), d, 0, 0, 0, 0, time.UTC)
}

// daysInMonth returns the number of days in the given month, leap years
// included. Day 0 of the following month IS the last day of this one.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// rruleNext implements rrule_next(rule, after) -> date: the first
// occurrence of `rule` strictly after `after`.
//
// The RRULE mechanics live in [metamodel.NextRrule] so validation and
// occurrence-stepping share one implementation, and this package needs
// no dependency on the rrule library.
//
// # Exhausted rules
//
// A rule that is exhausted (COUNT reached, UNTIL passed) has no next
// occurrence, and the engine enforces declared return types
// (eval.go:156) — a Date-returning host function may not return Nil. So
// exhaustion is an error carrying [ErrRruleExhausted] rather than a
// nil-ish Date. A zero date would be worse: it is a real, comparable
// value, so `rrule_next(r, d) > today()` would silently be false for
// year 1 and no caller could tell "finished" from "far future".
//
// An automation treats an eval error as no-match plus a warning, which
// is right for both exhaustion and a malformed rule — but the messages
// differ so an operator can tell a finished schedule from a typo.
func rruleNext(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	if len(args) != 2 {
		return nil, errArg
	}
	ruleStr, ok := args[0].(predicate.String)
	if !ok {
		return nil, errArg
	}
	after, ok := args[1].(predicate.Date)
	if !ok {
		return nil, errArg
	}

	next, err := metamodel.NextRrule(ruleStr.String(), after.Time())
	if err != nil {
		return nil, fmt.Errorf("predicatefns: rrule_next: %w", err)
	}
	return predicate.NewDate(next), nil
}
