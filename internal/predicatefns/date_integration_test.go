package predicatefns_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// datePropMeta is a metamodel with a date-typed property, the shape a
// real recurring-task condition runs against.
func datePropMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Properties: map[string]metamodel.PropertyDef{
					"due":       {Type: metamodel.PropertyTypeDate},
					"herhaling": {Type: metamodel.PropertyTypeRrule},
					"status":    {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
}

// TestDateFuncs_ThroughEvaluator is the integration proof for
// TKT-HQONQE: the date functions work against a REAL entity whose date
// property was bound by EntityRecord from the metamodel — the same path
// automation and validation conditions take.
//
// This is the load-bearing check. A date property that bound as a
// String instead of a Date would compile fine and fail only at eval, in
// production; here it fails in CI.
func TestDateFuncs_ThroughEvaluator(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	ev := predicatefns.NewEvaluatorWithClock(
		datePropMeta(), func() time.Time { return now })

	tests := []struct {
		name  string
		src   string
		props map[string]any
		want  bool
	}{
		{
			"due within a week",
			"days_between(entity.due, today()) <= 7",
			map[string]any{"due": "2026-08-21"},
			true,
		},
		{
			"due beyond a week",
			"days_between(entity.due, today()) <= 7",
			map[string]any{"due": "2026-09-30"},
			false,
		},
		{
			"overdue is also within a week",
			"days_between(entity.due, today()) <= 7",
			map[string]any{"due": "2026-08-01"},
			true,
		},
		{
			"strictly overdue",
			"days_between(entity.due, today()) < 0",
			map[string]any{"due": "2026-08-01"},
			true,
		},
		{
			"grace period lapsed via date_add",
			"date_add(entity.due, 3, 'day') < today()",
			map[string]any{"due": "2026-08-01"},
			true,
		},
		{
			"grace period still open",
			"date_add(entity.due, 3, 'day') < today()",
			map[string]any{"due": "2026-08-17"},
			false,
		},
		{
			"next occurrence is in the future",
			"rrule_next(entity.herhaling, entity.due) > today()",
			map[string]any{"due": "2026-08-18", "herhaling": "FREQ=WEEKLY"},
			true,
		},
		{
			"combined with a plain property comparison",
			"entity.status == 'todo' and days_between(entity.due, today()) <= 7",
			map[string]any{"due": "2026-08-20", "status": "todo"},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := ev.Compile("taak", tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got, err := ev.Matches(context.Background(), prog, "taak", "TASK-1", tc.props)
			if err != nil {
				t.Fatalf("eval %q: %v", tc.src, err)
			}
			if got != tc.want {
				t.Errorf("%q with %v = %v, want %v", tc.src, tc.props, got, tc.want)
			}
		})
	}
}

// TestDateFuncs_MissingDateProperty pins the absent-value case: a date
// function applied to a property the entity does not carry is an eval
// error (the value binds as nil), NOT a silent false. Callers translate
// that to no-match, but it must not look like a real comparison.
func TestDateFuncs_MissingDateProperty(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	ev := predicatefns.NewEvaluatorWithClock(
		datePropMeta(), func() time.Time { return now })

	prog, err := ev.Compile("taak", "days_between(entity.due, today()) <= 7")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, err = ev.Matches(context.Background(), prog, "taak", "TASK-1",
		map[string]any{"status": "todo"}) // no `due`
	if err == nil {
		t.Error("want an eval error for a missing date property, got none")
	}
}
