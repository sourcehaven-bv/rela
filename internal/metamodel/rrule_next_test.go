package metamodel_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestNextRrule(t *testing.T) {
	base := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		rule string
		want string
	}{
		{"daily", "FREQ=DAILY", "2026-08-19"},
		{"weekly", "FREQ=WEEKLY", "2026-08-25"},
		{"monthly", "FREQ=MONTHLY", "2026-09-18"},
		{"yearly", "FREQ=YEARLY", "2027-08-18"},
		{"RRULE: prefix accepted", "RRULE:FREQ=DAILY", "2026-08-19"},
		{
			// INTERVAL > 1 needs DTSTART or ValidateRrule rejects it:
			// without an anchor the cadence drifts.
			"interval with DTSTART",
			"DTSTART:20260818T000000Z\nFREQ=DAILY;INTERVAL=3",
			"2026-08-21",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := metamodel.NextRrule(tc.rule, base)
			if err != nil {
				t.Fatalf("NextRrule(%q): %v", tc.rule, err)
			}
			if got.Format(time.DateOnly) != tc.want {
				t.Errorf("NextRrule(%q) = %s, want %s",
					tc.rule, got.Format(time.DateOnly), tc.want)
			}
		})
	}
}

// TestNextRrule_TruncatesToUTCMidnight pins that a caller passing a
// mid-day instant still gets a clean date back, so results compare
// equal to date literals and to today().
func TestNextRrule_TruncatesToUTCMidnight(t *testing.T) {
	got, err := metamodel.NextRrule("FREQ=DAILY",
		time.Date(2026, 8, 18, 17, 45, 30, 0, time.UTC))
	if err != nil {
		t.Fatalf("NextRrule: %v", err)
	}
	want := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

// TestNextRrule_Exhausted pins the sentinel: a COUNT-limited rule with
// nothing left reports ErrRruleExhausted, distinct from a parse failure.
func TestNextRrule_Exhausted(t *testing.T) {
	_, err := metamodel.NextRrule(
		"DTSTART:20260818T000000Z\nFREQ=DAILY;COUNT=1",
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("want an error for an exhausted rule, got none")
	}
	if !errors.Is(err, metamodel.ErrRruleExhausted) {
		t.Errorf("want ErrRruleExhausted, got %v", err)
	}
}

// TestNextRrule_Malformed pins that a broken rule is an error and is
// NOT reported as exhaustion — an operator typo must not read as a
// finished schedule.
func TestNextRrule_Malformed(t *testing.T) {
	for _, rule := range []string{"NOT-A-RULE", "FREQ=NEVER", "", "FREQ=DAILY;INTERVAL=3"} {
		t.Run("rule="+rule, func(t *testing.T) {
			_, err := metamodel.NextRrule(rule,
				time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))
			if err == nil {
				t.Fatalf("rule %q: want an error, got none", rule)
			}
			if errors.Is(err, metamodel.ErrRruleExhausted) {
				t.Errorf("rule %q: malformed rule reported as exhausted", rule)
			}
		})
	}
}
