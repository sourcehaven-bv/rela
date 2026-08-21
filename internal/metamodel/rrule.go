package metamodel

import (
	"errors"
	"fmt"
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"
)

// ValidateRrule validates an RRULE string. It strips the "RRULE:" prefix if
// present, parses the rule, and rejects INTERVAL > 1 without DTSTART (which
// would cause interval cadence drift).
//
// This function is the single source of truth for RRULE validation, used by
// both the metamodel property validator and the Lua rrule_next helper.
func ValidateRrule(s string) error {
	cleaned := strings.TrimPrefix(s, "RRULE:")

	opt, err := rrule.StrToROption(cleaned)
	if err != nil {
		return fmt.Errorf("invalid RRULE: %w", err)
	}

	if opt.Interval > 1 && opt.Dtstart.IsZero() {
		return errors.New("INTERVAL > 1 requires DTSTART in the RRULE string")
	}

	return nil
}

// ErrRruleExhausted marks "this rule has no further occurrence" — a
// COUNT that has been reached or an UNTIL that has passed. It is
// distinct from a malformed rule so a caller can tell a finished
// schedule from an operator typo.
var ErrRruleExhausted = errors.New("rrule exhausted")

// NextRrule returns the first occurrence of s strictly after `after`,
// truncated to UTC midnight.
//
// It lives here, beside [ValidateRrule], so RRULE mechanics stay in one
// package: callers get validation and occurrence-stepping with identical
// prefix handling and identical error text, and no other package needs
// to depend on the rrule library.
//
// A rule without DTSTART is anchored at `after`, so a bare rule stored
// in an `rrule` property is interpreted relative to the date the caller
// asked about. (ValidateRrule already rejects INTERVAL > 1 without
// DTSTART, since that combination drifts.)
//
// Returns [ErrRruleExhausted] when the rule has no occurrence left.
func NextRrule(s string, after time.Time) (time.Time, error) {
	if err := ValidateRrule(s); err != nil {
		return time.Time{}, err
	}
	opt, err := rrule.StrToROption(strings.TrimPrefix(s, "RRULE:"))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RRULE: %w", err)
	}

	utc := after.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	if opt.Dtstart.IsZero() {
		opt.Dtstart = day
	}
	rule, err := rrule.NewRRule(*opt)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RRULE: %w", err)
	}

	next := rule.After(day, false)
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("%w: %q has no occurrence after %s",
			ErrRruleExhausted, s, day.Format(time.DateOnly))
	}
	n := next.UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC), nil
}
