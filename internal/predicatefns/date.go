package predicatefns

import (
	"context"
	"fmt"
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
	unitDay  = "day"
	unitWeek = "week"
)

// daysPerWeek converts a week count to days.
const daysPerWeek = 7

// ErrRruleExhausted re-exports [metamodel.ErrRruleExhausted] so a
// caller of this package can tell a finished schedule from a malformed
// rule with errors.Is, without importing metamodel for the sentinel
// alone.
var ErrRruleExhausted = metamodel.ErrRruleExhausted

// hoursPerDay is the multiplier for converting a whole-day count to a
// time.Duration. Safe here because every value is UTC-midnight
// truncated, so no DST transition can shorten or lengthen the day.
const hoursPerDay = 24

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
	delta := utcDay(a).Sub(utcDay(b))
	// Int, not Number: a day count is a whole number, and the engine
	// requires both sides of an ordered comparison to share a type. As
	// Number this could not be compared against an integer-typed
	// property — `days_between(...) <= entity.doorlooptijd` would fail
	// to compile, which is precisely the recurring-task shape this
	// function exists for.
	return predicate.NewInt(int64(delta.Hours() / hoursPerDay)), nil
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

	days := int(n.Float())
	switch strings.ToLower(strings.TrimSpace(unit.String())) {
	case unitDay:
	case unitWeek:
		days *= daysPerWeek
	default:
		return nil, fmt.Errorf(
			"predicatefns: date_add: unsupported unit %q (use %q or %q)",
			unit.String(), unitDay, unitWeek)
	}

	return predicate.NewDate(utcDay(d.Time()).AddDate(0, 0, days)), nil
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

// RruleNext returns the first occurrence of an RRULE strictly after
// `after`, or an error wrapping [ErrRruleExhausted] when the rule has
// none left. Exported because [predicate.Program.Eval] flattens host
// errors into message strings, so a Go caller that needs to tell
// "finished schedule" from "malformed rule" must call this directly.
func RruleNext(rule string, after time.Time) (time.Time, error) {
	v, err := rruleNext(context.Background(), []predicate.Value{
		predicate.NewString(rule),
		predicate.NewDate(after),
	})
	if err != nil {
		return time.Time{}, err
	}
	d, ok := v.(predicate.Date)
	if !ok {
		return time.Time{}, errArg
	}
	return d.Time(), nil
}
