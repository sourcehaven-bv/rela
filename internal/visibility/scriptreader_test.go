package visibility_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// seedScriptWorld builds a small graph: two tickets and one secret, with a
// relation from a ticket to the secret.
func seedScriptWorld(t *testing.T) store.Store {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "One"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Two"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "Hush"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "TKT-1", "relates", "SEC-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	if _, err := st.CreateRelation(ctx, "TKT-1", "relates", "TKT-2", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	return st
}

// typeGate permits only the named entity type, so tests can express
// "readable" vs "hidden" without a full policy.
type typeGate struct{ allow string }

func (g typeGate) PermitsRead(_ context.Context, entityType, _ string) (bool, error) {
	return entityType == g.allow, nil
}

func (g typeGate) PermitsReadMany(
	_ context.Context, entityType string, ids []string,
) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = entityType == g.allow
	}
	return out, nil
}

func newTicketOnlyScriptReader(t *testing.T, st store.Store) *visibility.ScriptReader {
	t.Helper()
	reader, err := visibility.NewPolicyReader(typeGate{allow: "ticket"}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	sr, err := visibility.NewScriptReader(reader, st, nil)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}
	return sr
}

func TestScriptReader_GetEntityGatesOnStoredType(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)
	ctx := context.Background()

	if _, err := sr.GetEntity(ctx, "TKT-1"); err != nil {
		t.Errorf("readable entity: %v", err)
	}
	// A denied entity is reported as not-found, indistinguishable from a
	// genuine miss — the oracle-free contract.
	_, denied := sr.GetEntity(ctx, "SEC-1")
	_, missing := sr.GetEntity(ctx, "NOPE")
	if denied == nil {
		t.Error("hidden entity was returned")
	}
	if !errors.Is(denied, store.ErrNotFound) || !errors.Is(missing, store.ErrNotFound) {
		t.Errorf("denied=%v missing=%v — both must be ErrNotFound", denied, missing)
	}
}

func TestScriptReader_ListEntitiesFilters(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)

	var tickets, secrets int
	for e, err := range sr.ListEntities(context.Background(), store.EntityQuery{Type: "ticket"}) {
		if err != nil {
			t.Fatalf("list tickets: %v", err)
		}
		_ = e
		tickets++
	}
	for range sr.ListEntities(context.Background(), store.EntityQuery{Type: "secret"}) {
		secrets++
	}
	if tickets != 2 {
		t.Errorf("visible tickets = %d, want 2", tickets)
	}
	if secrets != 0 {
		t.Errorf("hidden secrets yielded %d rows, want 0", secrets)
	}
}

// TestScriptReader_ListEntitiesEarlyReturn pins the iterator contract: a
// consumer that stops early must not wedge the producer.
func TestScriptReader_ListEntitiesEarlyReturn(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)

	seen := 0
	for range sr.ListEntities(context.Background(), store.EntityQuery{Type: "ticket"}) {
		seen++
		break
	}
	if seen != 1 {
		t.Errorf("early-return consumer saw %d rows, want 1", seen)
	}
}

func TestScriptReader_ListRelationsRequiresBothEndpoints(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)

	var kept []string
	for rel, err := range sr.ListRelations(context.Background(), store.RelationQuery{From: "TKT-1"}) {
		if err != nil {
			t.Fatalf("list relations: %v", err)
		}
		kept = append(kept, rel.To)
	}
	if len(kept) != 1 || kept[0] != "TKT-2" {
		t.Errorf("relations = %v, want only the both-endpoints-visible TKT-2", kept)
	}
}

// TestScriptReader_RejectsNilCollaborators pins constructors-reject-nil.
func TestScriptReader_RejectsNilCollaborators(t *testing.T) {
	st := memstore.New()
	reader, err := visibility.NewPolicyReader(visibility.NopGate{}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	if _, err := visibility.NewScriptReader(nil, st, nil); err == nil {
		t.Error("nil reader accepted")
	}
	if _, err := visibility.NewScriptReader(reader, nil, nil); err == nil {
		t.Error("nil store accepted")
	}
}

// TestDenyReader_RefusesEverything pins the fail-closed substitute used
// when a policy is configured but the gate cannot be built: every read
// refuses with ErrReaderUnavailable, which is deliberately NOT a
// not-found — the failure must be diagnosable, not look like empty data.
func TestDenyReader_RefusesEverything(t *testing.T) {
	var dr visibility.DenyReader
	ctx := context.Background()

	if _, err := dr.GetEntity(ctx, "TKT-1"); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("GetEntity err = %v, want ErrReaderUnavailable", err)
	}
	if errors.Is(visibility.ErrReaderUnavailable, store.ErrNotFound) {
		t.Error("ErrReaderUnavailable must not be a not-found — a gate fault is not 'no such entity'")
	}

	for _, err := range dr.ListEntities(ctx, store.EntityQuery{Type: "ticket"}) {
		if !errors.Is(err, visibility.ErrReaderUnavailable) {
			t.Errorf("ListEntities err = %v, want ErrReaderUnavailable", err)
		}
	}
	for _, err := range dr.ListRelations(ctx, store.RelationQuery{}) {
		if !errors.Is(err, visibility.ErrReaderUnavailable) {
			t.Errorf("ListRelations err = %v, want ErrReaderUnavailable", err)
		}
	}
}

// TestDenyTracer_RefusesEverything pins the traversal counterpart.
func TestDenyTracer_RefusesEverything(t *testing.T) {
	var dt visibility.DenyTracer
	ctx := context.Background()

	if got := dt.TraceFrom(ctx, "TKT-1", 2); got != nil {
		t.Errorf("TraceFrom = %+v, want nil", got)
	}
	if got := dt.TraceTo(ctx, "TKT-1", 2); got != nil {
		t.Errorf("TraceTo = %+v, want nil", got)
	}
	if got := dt.FindPath(ctx, "TKT-1", "TKT-2"); got != nil {
		t.Errorf("FindPath = %v, want nil", got)
	}
	if _, err := dt.FindOrphans(ctx); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("FindOrphans err = %v, want ErrReaderUnavailable", err)
	}
	if dt.HasCycle(ctx, "TKT-1") {
		t.Error("HasCycle = true, want false")
	}
	// It must satisfy the interface it substitutes for.
	var _ tracer.Tracer = dt
}
