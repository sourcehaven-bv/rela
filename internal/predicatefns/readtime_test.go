package predicatefns_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
)

func readTimeMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"grant": {Properties: map[string]metamodel.PropertyDef{
				"due":         {Type: metamodel.PropertyTypeDate},
				"valid_until": {Type: metamodel.PropertyTypeDatetime},
			}},
		},
	}
}

// TestReadTimeConditions pins today(), now() and the date vs datetime rule
// at a read time carried on ctx: 2026-11-16T10:00:00Z.
func TestReadTimeConditions(t *testing.T) {
	ctx := readtime.With(context.Background(), time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC))
	// The evaluator's own clock is a different day; ctx must win.
	ev := predicatefns.NewEvaluatorWithClock(readTimeMeta(), fixedClock())

	tests := []struct {
		name string
		expr string
		rec  map[string]any
		want bool
	}{
		{"date before today", "entity.due < today()", map[string]any{"due": "2026-11-15"}, true},
		{"date equal today is not before", "entity.due < today()", map[string]any{"due": "2026-11-16"}, false},
		{"date equal today", "entity.due == today()", map[string]any{"due": "2026-11-16"}, true},
		{"datetime earlier today is before now", "entity.valid_until < now()",
			map[string]any{"valid_until": "2026-11-16T09:59:59Z"}, true},
		{"datetime later today is not before now", "entity.valid_until < now()",
			map[string]any{"valid_until": "2026-11-16T10:00:01Z"}, false},
		{"datetime later today is not before today", "entity.valid_until >= today()",
			map[string]any{"valid_until": "2026-11-16T00:00:00Z"}, true},
		{"date compares as UTC midnight against now", "entity.due < now()",
			map[string]any{"due": "2026-11-16"}, true},
		{"same day via days_between", "days_between(entity.valid_until, today()) == 0",
			map[string]any{"valid_until": "2026-11-16T23:00:00Z"}, true},
		{"offset with date_add", "entity.due <= date_add(today(), 7, 'day')",
			map[string]any{"due": "2026-11-23"}, true},
		{"nil guard", "entity.due ~= nil and entity.due < today()", map[string]any{}, false},
		{"today is midnight of now", "today() < now()", map[string]any{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := ev.Compile("grant", tc.expr)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := ev.Matches(ctx, prog, "grant", "G-1", tc.rec)
			if err != nil {
				t.Fatalf("matches: %v", err)
			}
			if got != tc.want {
				t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

// TestFromFilterDateVariables pins that a --where date variable compiles to
// a function of the read time, not to the day it was compiled on: one
// cached program gives different answers at different read times.
func TestFromFilterDateVariables(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(readTimeMeta(), fixedClock())
	at := func(day int) time.Time { return time.Date(2026, 11, day, 10, 0, 0, 0, time.UTC) }
	tests := []struct {
		where string
		rec   map[string]any
		at    time.Time
		want  bool
	}{
		{"due<$today", map[string]any{"due": "2026-11-15"}, at(16), true},
		{"due<$today", map[string]any{"due": "2026-11-15"}, at(15), false},
		{"due<=$today+7d", map[string]any{"due": "2026-11-23"}, at(16), true},
		{"due<=$today+1w", map[string]any{"due": "2026-11-24"}, at(16), false},
		{"due>=$yesterday", map[string]any{"due": "2026-11-15"}, at(16), true},
		{"due=$tomorrow", map[string]any{"due": "2026-11-17"}, at(16), true},
		{"valid_until<$now", map[string]any{"valid_until": "2026-11-16T09:00:00Z"}, at(16), true},
		{"valid_until>=$today-1d", map[string]any{"valid_until": "2026-11-15T00:00:00Z"}, at(16), true},
	}
	for _, tc := range tests {
		t.Run(tc.where, func(t *testing.T) {
			fs, err := filter.ParseAll([]string{tc.where})
			if err != nil {
				t.Fatal(err)
			}
			prog, err := ev.CompileFilter("grant", fs)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := ev.Matches(readtime.With(context.Background(), tc.at), prog, "grant", "G-1", tc.rec)
			if err != nil {
				t.Fatalf("matches: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
