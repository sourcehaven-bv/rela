package remotesync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestRun_RelationMetaIsValidated pins that source relation meta is checked
// against the local relation type before it is written: the manager's
// relation writes check the endpoints only.
func TestRun_RelationMetaIsValidated(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()
	src.relate("R1", "satisfies", "C1", map[string]any{"weight": 99})

	rep := pull(t, tg, m, src, Options{})
	if rep.Relations.Rejected != 1 || rep.Relations.Created != 1 {
		t.Fatalf("report: %+v", rep)
	}
	r1, c1 := tg.byRef(t, "R1"), tg.byRef(t, "C1")
	_, err := tg.st.GetRelation(context.Background(), entity.RelationKey{From: r1.ID, Type: "satisfies", To: c1.ID})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an out-of-range weight was written: %v", err)
	}
}

func TestRejectedWriteAndUniqueOn(t *testing.T) {
	unique := func(prop string) error {
		return &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{
			{Type: metamodel.ValidationErrorUnique, Property: prop},
		}}
	}
	tests := []struct {
		name     string
		err      error
		rejected bool
		onRef    bool
	}{
		{"validation", &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{
			{Type: metamodel.ValidationErrorRequired, Property: "title"},
		}}, true, false},
		{"invalid relation", fmt.Errorf("invalid relation: %w", &metamodel.InvalidRelationError{}), true, false},
		{"unknown relation", fmt.Errorf("invalid relation: %w", &metamodel.RelationNotFoundError{}), true, false},
		{"cardinality", fmt.Errorf("w: %w", entitymanager.ErrCardinalityExceeded), true, false},
		{"unique on the ref", unique("cat"), true, true},
		{"unique on a field", unique("title"), true, false},
		{"store unique naming no property", store.UniquePropertyError{}, true, true},
		{"store unique on a field", store.UniquePropertyError{Property: "title"}, true, false},
		{"other", errors.New("disk full"), false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rejectedWrite(tc.err); got != tc.rejected {
				t.Errorf("rejectedWrite = %v", got)
			}
			if got := uniqueOn(tc.err, "cat"); got != tc.onRef {
				t.Errorf("uniqueOn = %v", got)
			}
		})
	}
}

// TestRejectedWrite_ManagerErrors pins that the manager's own refusals,
// matched without importing it, count as rejected.
func TestRejectedWrite_ManagerErrors(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	var ids []string
	for _, typ := range []string{"component", "component"} {
		res, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: typ, Properties: map[string]any{"name": "n"}},
			entity.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, res.Entity.ID)
	}
	key := entity.RelationKey{From: ids[0], Type: "satisfies", To: ids[1]}
	_, err := tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{})
	if err == nil || !rejectedWrite(err) {
		t.Errorf("relation between types satisfies does not allow: rejectedWrite(%v) = false", err)
	}
	key.Type = "depends"
	if _, err = tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{}); err != nil {
		t.Fatal(err)
	}
	_, err = tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{})
	if !errors.Is(err, entity.ErrRelationAlreadyExists) {
		t.Errorf("second create: %v, want entity.ErrRelationAlreadyExists", err)
	}
	_, err = tg.mgr.DeleteEntity(ctx, ids[0], false)
	if !errors.Is(err, entity.ErrHasRelations) {
		t.Errorf("delete: %v, want entity.ErrHasRelations", err)
	}
}

// TestRun_HeldRowsGoLast pins that a row which stopped on a failure does
// not take the fetch budget ahead of the other changed rows.
func TestRun_HeldRowsGoLast(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, func(p *Remote) { p.MaxFetch = 1 })
	src := seedSource()
	// The budget is not part of the mapping, so a run without it shares the
	// ledger.
	pull(t, tg, testMapping(t, tg.meta, nil), src, Options{})
	src.ents["C1"].Props = map[string]any{"name": "Theirs"}
	src.ents["R1"].Props = map[string]any{"title": "Login 2", "state": "open"}
	broken := true
	src.fail = func(path string) error {
		if broken && strings.HasPrefix(path, "components/") {
			return errors.New("connection reset")
		}
		return nil
	}

	rep, err := Run(ctx, m, tg.deps(src), Options{})
	if err == nil || rep.Entities.Failed != 1 || rep.Waiting != 1 {
		t.Fatalf("first run: %+v, %v", rep, err)
	}
	broken = false
	rep = pull(t, tg, m, src, Options{})
	if rep.Entities.Updated != 1 || rep.Waiting != 1 {
		t.Fatalf("second run: %+v", rep)
	}
	if got := tg.byRef(t, "R1").Properties["title"]; got != "Login 2" {
		t.Fatalf("R1 title = %v", got)
	}
}

// TestRun_AdoptedRowsAreNotDeleted pins that on_gone: delete only deletes
// rows the pull created: a local row it adopted is detached.
func TestRun_AdoptedRowsAreNotDeleted(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, func(p *Remote) { p.OnGone = OnGoneDelete })
	src := seedSource()
	if _, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement", Content: "Body", Properties: map[string]any{
		"title": "Login", "status": "open", "cat": metamodel.ExternalRefValue{ID: "R1"}.Map(),
	}}, entity.CreateOptions{WriteExternalRefs: true}); err != nil {
		t.Fatal(err)
	}
	if rep := pull(t, tg, m, src, Options{}); rep.Entities.Adopted != 1 {
		t.Fatalf("first run: %+v", rep)
	}
	delete(src.ents, "R1")
	delete(src.rels, "R1--satisfies--C1")
	rep := pull(t, tg, m, src, Options{AllowGone: 2})
	if rep.Entities.Detached != 1 || rep.Entities.Deleted != 0 || rep.Relations.Deleted != 1 {
		t.Fatalf("report: %+v", rep)
	}
	if tg.byRef(t, "R1") == nil {
		t.Fatal("an adopted row was deleted")
	}
}

// TestRun_DeleteWindow pins that max_gone also bounds the deletes of the
// last day, so a source cannot delete in bulk a few rows per run.
func TestRun_DeleteWindow(t *testing.T) {
	tg := newTarget(t)
	two := 2
	m := testMapping(t, tg.meta, func(p *Remote) { p.OnGone = OnGoneDelete; p.MaxGone = &two })
	src := newFakeSource()
	for i := range 4 {
		src.put(&fakeEnt{Type: "req", ID: fmt.Sprintf("R%d", i), Props: map[string]any{"title": "t"}})
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	deps := tg.deps(src)
	deps.Now = func() time.Time { return now }
	run := func(o Options) (Report, error) { return Run(context.Background(), m, deps, o) }
	if _, err := run(Options{}); err != nil {
		t.Fatal(err)
	}

	delete(src.ents, "R0")
	delete(src.ents, "R1")
	if rep, err := run(Options{}); err != nil || rep.Entities.Deleted != 2 {
		t.Fatalf("first deletes: %v %+v", err, rep)
	}
	delete(src.ents, "R2")
	if _, err := run(Options{}); !errors.Is(err, ErrGoneGuard) {
		t.Fatalf("a third delete within a day: %v", err)
	}
	now = now.Add(deleteWindow + time.Minute)
	if rep, err := run(Options{}); err != nil || rep.Entities.Deleted != 1 {
		t.Fatalf("after the window: %v %+v", err, rep)
	}
}

func TestRun_ByteBudget(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, func(p *Remote) { p.MaxBytes = 1 })
	rep := pull(t, tg, m, seedSource(), Options{})
	if rep.Entities.Created != 1 || rep.Waiting != 3 {
		t.Fatalf("report: %+v", rep)
	}
}

func TestRun_DryRunCountsRelationsOfNewRows(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	rep := pull(t, tg, m, seedSource(), Options{DryRun: true})
	if rep.Entities.Created != 4 || rep.Relations.Created != 2 || rep.Relations.Deferred != 0 {
		t.Fatalf("report: %+v", rep)
	}
	if tg.count(t, "requirement") != 0 {
		t.Fatal("a dry run wrote")
	}
}
