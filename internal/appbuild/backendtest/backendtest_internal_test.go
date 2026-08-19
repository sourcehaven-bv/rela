package backendtest

import (
	"fmt"
	"strings"
	"testing"
)

// The skip-vs-fail decision is the whole safety property of this package: get it
// wrong in the permissive direction and CI's Postgres job silently stops testing
// anything (RR-0EWZQW). It is asserted directly, with getenv injected, because
// an end-to-end assertion cannot distinguish "gate worked" from "no database
// configured" — the two look identical from outside.
func TestDSNRequired(t *testing.T) {
	env := func(v string) func(string) string {
		return func(key string) string {
			if key != requireDBEnv {
				return ""
			}
			return v
		}
	}

	for _, tc := range []struct {
		name string
		val  string
		want bool
	}{
		{"unset does not require", "", false},
		// A shell that exports the variable empty or explicitly off must behave
		// like an unset one, or every developer inheriting it hard-fails.
		{"zero does not require", "0", false},
		{"false does not require", "false", false},
		{"FALSE does not require", "FALSE", false},
		{"one requires", "1", true},
		{"true requires", "true", true},
		// Anything else is opt-in: the variable exists to be strict, so an
		// unrecognized value must not silently fall back to permissive.
		{"arbitrary value requires", "yes", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := dsnRequired(env(tc.val)); got != tc.want {
				t.Errorf("dsnRequired(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// recorder captures which of Skipf/Fatalf missingDSN chose, so the branch is
// observable without actually skipping or failing the running test.
type recorder struct {
	skipped bool
	failed  bool
	msg     string
}

func (r *recorder) Helper()        {}
func (r *recorder) Cleanup(func()) {}
func (r *recorder) Skipf(f string, a ...any) {
	r.skipped = true
	r.msg = fmt.Sprintf(f, a...)
}
func (r *recorder) Fatalf(f string, a ...any) {
	r.failed = true
	r.msg = fmt.Sprintf(f, a...)
}

func TestMissingDSN_SkipsOrFails(t *testing.T) {
	t.Run("skips when not required", func(t *testing.T) {
		r := &recorder{}
		missingDSN(r, func(string) string { return "" })
		if !r.skipped || r.failed {
			t.Errorf("want skip, got skipped=%v failed=%v", r.skipped, r.failed)
		}
		if !strings.Contains(r.msg, testDBEnv) {
			t.Errorf("skip message must name %s so the reason is actionable; got %q", testDBEnv, r.msg)
		}
	})

	t.Run("fails when required", func(t *testing.T) {
		r := &recorder{}
		missingDSN(r, func(key string) string {
			if key == requireDBEnv {
				return "1"
			}
			return ""
		})
		if !r.failed || r.skipped {
			t.Errorf("want fail, got skipped=%v failed=%v", r.skipped, r.failed)
		}
		// The operator has to know which variable to set and which to unset.
		for _, want := range []string{testDBEnv, requireDBEnv} {
			if !strings.Contains(r.msg, want) {
				t.Errorf("failure message must name %s; got %q", want, r.msg)
			}
		}
	})
}
