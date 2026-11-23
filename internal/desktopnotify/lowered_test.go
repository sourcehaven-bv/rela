package desktopnotify

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// loweredConditions are conditions queryplan.LowerCondition answers, so the
// store decides them (TKT-I3MGCRT): or, not, ~= and ordered comparisons on
// a string and an integer property.
var loweredConditions = []string{
	"entity.status == 'open' or entity.status == 'new'",
	"entity.status ~= 'done'",
	"not (entity.status == 'done')",
	"entity.status < 'open'",
	"entity.status >= 'done' and entity.status <= 'open'",
	"entity.status > ''",
	"entity.punten > 3",
	"entity.punten <= 3 or entity.status == 'open'",
	"not (entity.punten < 3)",
	"entity.punten == 5",
	"entity.punten ~= 5",
	"-1 < entity.punten and not (entity.status ~= 'open' or entity.punten >= 5)",
}

// mixedValues are the stored values the differential seeds status and
// punten with: unset, null, the empty string, strings that sort around the
// literals, integers, a non-integral number, an integer-shaped string and a
// list.
var mixedValues = []any{
	"absent", nil, "", "open", "new", "done", "Zed", "a", "7", 0, 3, 5, -4, 2.5, 1e19, true,
	[]any{"open"}, map[string]any{"id": "open"},
}

// narrowedConditions compare a date with today(), so they are judged in Go;
// the conjuncts that lower narrow the store read.
var narrowedConditions = []string{
	"entity.status ~= 'done' and entity.vervaldatum <= today()",
	"(entity.status == 'open' or entity.punten > 1) and entity.vervaldatum <= today()",
	"not (entity.status < 'n') and entity.punten ~= 5 and entity.vervaldatum > today()",
}

// mixedDates are the stored vervaldatum values, around fixedNow.
var mixedDates = []any{"absent", "2026-10-01", "2026-10-04", "2026-12-01", "", "TBD"}

func seedMixed(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	for i := range len(mixedValues) * 3 {
		e := &entity.Entity{ID: fmt.Sprintf("TAAK-%03d", i), Type: "taak", Properties: map[string]any{"title": "t"}}
		if v := mixedValues[i%len(mixedValues)]; v != "absent" {
			e.Properties["status"] = v
		}
		if v := mixedValues[(i*7+3)%len(mixedValues)]; v != "absent" {
			e.Properties["punten"] = v
		}
		if v := mixedDates[i%len(mixedDates)]; v != "absent" {
			e.Properties["vervaldatum"] = v
		}
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("CreateEntity %s: %v", e.ID, err)
		}
	}
}

// assertLoweredMatchesGo evaluates every lowered and narrowed condition as
// a rule and as the badge on st, and compares the result with the Go
// evaluator run on every row read back from st. A lowered condition must
// never fail in Go; a narrowed one may, and the row then does not match.
func assertLoweredMatchesGo(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	var rows []*entity.Entity
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: "taak", Faces: store.InWorld(store.TrivialScope())}) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		rows = append(rows, e)
	}
	if len(rows) == 0 {
		t.Fatal("no rows seeded")
	}
	for i, src := range slices.Concat(loweredConditions, narrowedConditions) {
		lowered := i < len(loweredConditions)
		cfg := load(t, fmt.Sprintf(
			"notifications:\n  - id: r\n    type: taak\n    condition: %q\nbadge:\n  type: taak\n  condition: %q\n", src, src))
		if cfg.rules[0].exact != lowered || cfg.badge.exact != lowered {
			t.Fatalf("%q: exact = %v, want %v", src, cfg.rules[0].exact, lowered)
		}
		if !lowered && len(cfg.rules[0].pushed) == 0 && cfg.rules[0].where == nil {
			t.Fatalf("%q pushes nothing; the test needs a narrowed read", src)
		}
		var want []string
		for _, e := range rows {
			ok, err := cfg.eval.Matches(ctx, cfg.rules[0].prog, e.Type, e.ID, e.Properties)
			if err != nil && lowered {
				t.Fatalf("%q on %s (%v): Go evaluation failed: %v", src, e.ID, e.Properties, err)
			}
			if ok {
				want = append(want, e.ID)
			}
		}
		slices.Sort(want)
		res, err := cfg.Evaluate(ctx, st, store.TrivialScope())
		if err != nil {
			t.Fatalf("%q: Evaluate: %v", src, err)
		}
		got := make([]string, 0, len(res.Matches))
		for _, m := range res.Matches {
			got = append(got, m.EntityID)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%q:\n  go    %s\n  store %s", src, strings.Join(want, " "), strings.Join(got, " "))
		}
		if res.Badge != len(want) {
			t.Errorf("%q: badge %d, want %d", src, res.Badge, len(want))
		}
	}
}

func TestEvaluate_LoweredMatchesGo_Memstore(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedMixed(t, st)
	assertLoweredMatchesGo(t, st)
}

// The file store is reopened, so every value is the one parsed back from
// its markdown file.
func TestEvaluate_LoweredMatchesGo_FSStore(t *testing.T) {
	t.Parallel()
	fsys := storage.NewMemFS()
	open := func() *fsstore.FSStore {
		rooted, err := storage.NewRootedFS(fsys, "/")
		if err != nil {
			t.Fatalf("NewRootedFS: %v", err)
		}
		st, err := fsstore.New(fsstore.Config{
			FS: fsys, Rooted: rooted,
			EntitiesKey: "entities", RelationsKey: "relations", AttachmentsKey: "attachments", CacheKey: ".rela",
			Schemas: map[string]store.EntityTypeSchema{"taak": {Plural: "taken"}, "note": {Plural: "notes"}},
		})
		if err != nil {
			t.Fatalf("fsstore.New: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })
		return st
	}
	seedMixed(t, open())
	assertLoweredMatchesGo(t, open())
}

// Conditions evaluate with LoweringSemantics, also on the Go path: an
// ordered comparison of an unset string or integer is false, not an error,
// so the rest of the condition still decides (TKT-I3MGCRT).
func TestEvaluate_UnsetOrderedComparisonIsFalse(t *testing.T) {
	t.Parallel()
	cfg := load(t, `
notifications:
  - id: either
    type: taak
    condition: "entity.punten < 3 or entity.vervaldatum <= today()"
  - id: negated
    type: taak
    condition: "not (entity.punten < 3) and entity.vervaldatum <= today()"
`)
	for _, r := range cfg.rules {
		if r.exact {
			t.Fatalf("rule %s lowered exactly; this test needs the Go path", r.id)
		}
	}
	// No punten: the comparison reads as false.
	st := newStore(t, taak{"TAAK-1", "t", "open", "2026-10-01"})
	res, err := cfg.Evaluate(context.Background(), st, store.TrivialScope())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	var got []string
	for _, m := range res.Matches {
		got = append(got, m.RuleID+":"+m.EntityID)
	}
	if want := []string{"either:TAAK-1", "negated:TAAK-1"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want %v", got, want)
	}
	if len(res.EvalErrors) != 0 {
		t.Errorf("EvalErrors = %v, want none", res.EvalErrors)
	}
}

// Two date rules on one type whose filters differ only inside an `or` read
// separately: the scan cache keys on the where tree, so neither rule is
// judged over the other's rows.
func TestEvaluate_DistinctWhereTreesDoNotShareAScan(t *testing.T) {
	t.Parallel()
	cfg := load(t, `
notifications:
  - id: a
    type: taak
    condition: "(entity.status == 'open' or entity.status == 'new') and entity.vervaldatum <= today()"
  - id: b
    type: taak
    condition: "(entity.status == 'open' or entity.status == 'late') and entity.vervaldatum <= today()"
`)
	st := newStore(t,
		taak{"TAAK-1", "t", "new", "2026-10-01"},
		taak{"TAAK-2", "t", "late", "2026-10-01"},
		taak{"TAAK-3", "t", "open", "2026-10-01"},
	)
	counting := storetest.NewCounting(st)
	res, err := cfg.Evaluate(context.Background(), counting, store.TrivialScope())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	var got []string
	for _, m := range res.Matches {
		got = append(got, m.RuleID+":"+m.EntityID)
	}
	if want := []string{"a:TAAK-1", "a:TAAK-3", "b:TAAK-2", "b:TAAK-3"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want %v", got, want)
	}
	if calls := counting.Cost().Calls; calls != 2 {
		t.Errorf("calls = %d, want 2 (one read per filter)", calls)
	}
}
