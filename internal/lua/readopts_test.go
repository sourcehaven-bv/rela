package lua

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// evalOpts pushes the Lua expression expr as the single argument to a Go
// function and returns what parseReadOpts made of it.
func evalOpts(t *testing.T, expr string, known ...string) (readOpts, error) {
	t.Helper()
	ls := lua.NewState()
	defer ls.Close()

	var got readOpts
	var gotErr error
	ls.SetGlobal("probe", ls.NewFunction(func(s *lua.LState) int {
		got, gotErr = parseReadOpts(s, 1, known...)
		return 0
	}))
	if err := ls.DoString("probe(" + expr + ")"); err != nil {
		t.Fatalf("running probe(%s): %v", expr, err)
	}
	return got, gotErr
}

func TestParseReadOpts_DefaultsToCeiling(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"", "nil", "{}"} {
		got, err := evalOpts(t, expr)
		if err != nil {
			t.Fatalf("probe(%s) errored: %v", expr, err)
		}
		if got.limit != maxReadLimit {
			t.Errorf("probe(%s) limit = %d, want the ceiling %d", expr, got.limit, maxReadLimit)
		}
	}
}

func TestParseReadOpts_LimitLowersButNeverRaises(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		expr string
		want int
	}{
		{"{limit = 1}", 1},
		{"{limit = 10}", 10},
		// The ceiling is a HARD bound: asking for more than maxReadLimit
		// clamps rather than errors, so a script that wants "as much as
		// possible" can say so without knowing the number.
		{"{limit = 999999}", maxReadLimit},
	} {
		got, err := evalOpts(t, tc.expr)
		if err != nil {
			t.Fatalf("probe(%s) errored: %v", tc.expr, err)
		}
		if got.limit != tc.want {
			t.Errorf("probe(%s) limit = %d, want %d", tc.expr, got.limit, tc.want)
		}
	}
}

// TestParseReadOpts_RejectsBadLimit pins that a malformed limit fails loudly
// rather than being ignored. Silently dropping it is the TKT-9FKX8X defect:
// the script asks a bounded question and unknowingly gets an unbounded answer.
func TestParseReadOpts_RejectsBadLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ expr, wantMsg string }{
		{`{limit = "10"}`, "must be a number"},
		{"{limit = true}", "must be a number"},
		{"{limit = {}}", "must be a number"},
		{"{limit = 1.5}", "whole number"},
		{"{limit = -1}", "must be positive"},
		// 0 means "unbounded" on store.ListEntitiesPage. Inheriting the
		// OPPOSITE meaning here silently would be a near-miss of the worst
		// kind, so it is an error rather than either interpretation.
		{"{limit = 0}", "must be positive"},
	} {
		_, err := evalOpts(t, tc.expr)
		if err == nil {
			t.Fatalf("probe(%s) was accepted; a malformed limit must raise", tc.expr)
		}
		if !strings.Contains(err.Error(), tc.wantMsg) {
			t.Errorf("probe(%s) error = %q, want it to mention %q", tc.expr, err, tc.wantMsg)
		}
	}
}

// TestParseReadOpts_RejectsUnknownOption pins that a typo'd or
// not-yet-implemented key raises. This is what makes `cursor` ABSENT rather
// than inert (DEC-IYHLNF): a script written against the future paging API
// fails loudly here instead of silently re-reading page one forever.
func TestParseReadOpts_RejectsUnknownOption(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{
		`{cursor = "abc"}`,
		"{limt = 10}",
		`{limit = 10, offset = 5}`,
	} {
		_, err := evalOpts(t, expr)
		if err == nil {
			t.Fatalf("probe(%s) was accepted; an unknown option must raise", expr)
		}
		if !strings.Contains(err.Error(), "unknown option") {
			t.Errorf("probe(%s) error = %q, want it to name the unknown option", expr, err)
		}
	}
}

// TestParseReadOpts_KnownKeysAccepted pins that binding-specific keys pass
// through the unknown-key check when the binding declares them.
func TestParseReadOpts_KnownKeysAccepted(t *testing.T) {
	t.Parallel()
	got, err := evalOpts(t, `{limit = 5, filter = "status=open"}`, "filter")
	if err != nil {
		t.Fatalf("a declared option was rejected: %v", err)
	}
	if got.limit != 5 {
		t.Errorf("limit = %d, want 5", got.limit)
	}
	// And still rejected when NOT declared.
	if _, err := evalOpts(t, `{filter = "x"}`); err == nil {
		t.Error("an undeclared option was accepted")
	}
}

func TestParseReadOpts_RejectsNonTable(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{`"ticket"`, "42", "true"} {
		_, err := evalOpts(t, expr)
		if err == nil {
			t.Fatalf("probe(%s) was accepted; options must be a table", expr)
		}
		if !strings.Contains(err.Error(), "must be a table") {
			t.Errorf("probe(%s) error = %q, want it to say options must be a table", expr, err)
		}
	}
}
