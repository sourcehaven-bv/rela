package visibility_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The header path must gate EXACTLY like the entity path. Anything else is a
// read-side ACL divergence, which is the failure mode this whole package
// exists to prevent — so this is asserted as equivalence against
// ListEntities rather than against a hand-written expectation that could
// drift from the real gate.
func TestScriptReader_ListEntityHeadersGatesLikeListEntities(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)
	ctx := context.Background()

	for _, q := range []struct {
		name  string
		query store.EntityQuery
	}{
		{"all types", store.EntityQuery{}},
		{"readable type", store.EntityQuery{Type: "ticket"}},
		{"hidden type", store.EntityQuery{Type: "secret"}},
		{"by id, mixed visibility", store.EntityQuery{IDs: []string{"TKT-1", "SEC-1"}}},
	} {
		t.Run(q.name, func(t *testing.T) {
			var wantIDs []string
			for e, err := range sr.ListEntities(ctx, q.query) {
				if err != nil {
					t.Fatalf("ListEntities: %v", err)
				}
				wantIDs = append(wantIDs, e.ID)
			}

			var gotIDs []string
			for h, err := range sr.ListEntityHeaders(ctx, q.query) {
				if err != nil {
					t.Fatalf("ListEntityHeaders: %v", err)
				}
				gotIDs = append(gotIDs, h.ID)
			}

			if fmt.Sprint(wantIDs) != fmt.Sprint(gotIDs) {
				t.Errorf("header gating diverged from entity gating:\n entities=%v\n headers =%v",
					wantIDs, gotIDs)
			}
			// Belt and braces: the hidden type must never appear, whatever
			// the equivalence above says.
			for _, id := range gotIDs {
				if id == "SEC-1" {
					t.Error("hidden entity SEC-1 leaked through the header path")
				}
			}
		})
	}
}

// Field redaction must apply to headers too: "may read every row of this
// type" is not "may see every property" (RR-OXE47R). A header that skipped
// redaction would be MORE revealing than the entity read it replaces.
func TestScriptReader_ListEntityHeadersRedactsFields(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "One", "secret": "hunter2"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	reader, err := visibility.NewPolicyReader(
		typeGate{allow: "ticket"}, hidePropRedactor{name: "secret"}, st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	sr, err := visibility.NewScriptReader(reader, st, nil)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}

	n := 0
	for h, err := range sr.ListEntityHeaders(ctx, store.EntityQuery{}) {
		if err != nil {
			t.Fatalf("ListEntityHeaders: %v", err)
		}
		n++
		if _, present := h.Properties["secret"]; present {
			t.Error("hidden property value survived into a header")
		}
		if h.Properties["title"] != "One" {
			t.Errorf("visible property lost: %v", h.Properties["title"])
		}
		if len(h.Redacted) != 1 || h.Redacted[0] != "secret" {
			t.Errorf("header must report what was withheld, got %v", h.Redacted)
		}
	}
	if n != 1 {
		t.Fatalf("expected 1 header, got %d", n)
	}
}

// Redaction must not write through to stored state: the header's Properties
// map may alias the store's (memstore clones, but a backend need not), so
// filtering has to allocate rather than delete in place.
func TestScriptReader_ListEntityHeadersDoesNotMutateStore(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "One", "secret": "hunter2"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	reader, err := visibility.NewPolicyReader(
		typeGate{allow: "ticket"}, hidePropRedactor{name: "secret"}, st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	sr, err := visibility.NewScriptReader(reader, st, nil)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}

	for range sr.ListEntityHeaders(ctx, store.EntityQuery{}) { //nolint:revive // draining
	}

	got, err := st.GetEntity(ctx, "TKT-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Properties["secret"] != "hunter2" {
		t.Error("redaction wrote through to the store")
	}
}

// Chunking must not change verdicts. With more rows than one gate chunk, a
// row's visibility still depends only on the row — never on which other
// rows happened to share its chunk.
func TestScriptReader_ListEntityHeadersChunkBoundary(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	// Straddle the 512-row chunk so the last chunk is partial, and
	// interleave types so every chunk contains both.
	const n = 1300
	wantVisible := 0
	for i := range n {
		typ := "ticket"
		if i%3 == 0 {
			typ = "secret"
		} else {
			wantVisible++
		}
		if err := st.CreateEntity(ctx, &entity.Entity{
			ID: fmt.Sprintf("E-%04d", i), Type: typ,
			Properties: map[string]any{"title": "t"},
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	sr := newTicketOnlyScriptReader(t, st)

	got := 0
	for h, err := range sr.ListEntityHeaders(ctx, store.EntityQuery{}) {
		if err != nil {
			t.Fatalf("ListEntityHeaders: %v", err)
		}
		if h.Type != "ticket" {
			t.Fatalf("hidden type %q leaked at %s", h.Type, h.ID)
		}
		got++
	}
	if got != wantVisible {
		t.Errorf("expected %d visible headers across chunk boundaries, got %d", wantVisible, got)
	}
}

// Abandoning the scan early (analyze caps a section at N issues) must stop
// cleanly, without gating or yielding the remainder.
func TestScriptReader_ListEntityHeadersStopsOnEarlyReturn(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)

	seen := 0
	for range sr.ListEntityHeaders(context.Background(), store.EntityQuery{}) {
		seen++
		break
	}
	if seen != 1 {
		t.Errorf("expected iteration to stop after 1, got %d", seen)
	}
}

// hidePropRedactor hides one named property, so redaction can be asserted
// without standing up a full affordances policy.
type hidePropRedactor struct{ name string }

func (r hidePropRedactor) HiddenProperties(
	_ context.Context, e *entity.Entity,
) map[string]struct{} {
	if _, ok := e.Properties[r.name]; !ok {
		return nil
	}
	return map[string]struct{}{r.name: {}}
}
