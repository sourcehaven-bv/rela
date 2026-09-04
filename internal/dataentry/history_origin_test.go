package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// originGate is a readGate that permits reading exactly the ids in allow, and
// can be made to fail, so the source-gating and its fail-closed branch are both
// reachable.
type originGate struct {
	allow map[string]bool
	err   error
	// calls counts PermitsReadMany invocations, so the batching claim is
	// testable rather than merely asserted in a comment.
	calls *int
}

func (g originGate) PermitsRead(_ context.Context, _, id string) (bool, error) {
	return g.allow[id], g.err
}

func (g originGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	if g.calls != nil {
		*g.calls++
	}
	if g.err != nil {
		return nil, g.err
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = g.allow[id]
	}
	return m, nil
}

func (originGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: true}
}

func (originGate) SearchScope(context.Context, []string) map[string]search.TypeScope {
	return nil
}
func (originGate) HoldsPermission(context.Context, string) bool       { return true }
func (originGate) PermitsWorld(context.Context, string) (bool, error) { return true, nil }

// TestGateOriginSources_HidesUnreadableSources is the security assertion for
// this feature.
//
// origin_source is an ENTITY ID and entity existence is a genuine secret (the
// row-level read rule: a denied GET is indistinguishable from a 404). A
// cross-entity copy names a row the reader may have no grant for, so echoing
// it back on the history endpoint would turn that endpoint into an existence
// oracle for any id that gets copied into something the caller CAN read.
//
// The definition name is deliberately NOT gated in the same way — it is
// operator-authored configuration, which the project rules say is not
// confidential.
func TestGateOriginSources_HidesUnreadableSources(t *testing.T) {
	visible := store.Origin{
		Kind: store.OriginCopy, Source: "POL-1", SourceFace: "draft",
		SourceType: "policy", Definition: "publish",
	}
	hidden := store.Origin{
		Kind: store.OriginCopy, Source: "SECRET-9", SourceFace: "draft",
		SourceType: "policy", Definition: "publish",
	}

	metas := []store.VersionMeta{{Version: 1, Origin: visible}, {Version: 2, Origin: hidden}}
	gate := originGate{allow: map[string]bool{"POL-1": true}}

	got := gateOriginSources(context.Background(), gate, metas)
	if got[0] != "POL-1@draft" {
		t.Errorf("readable source = %q, want %q", got[0], "POL-1@draft")
	}
	if _, present := got[1]; present {
		t.Errorf("an unreadable source leaked as %q; naming it confirms the row exists", got[1])
	}

	// And the rendered wire must carry the same distinction: the kind and the
	// DEFINITION still appear for the hidden case (configuration is not
	// secret) while the source id is simply absent.
	hiddenWire := originWire(hidden, got[1])
	if _, present := hiddenWire["source"]; present {
		t.Error("wire must omit an ungated source entirely, not null it")
	}
	if hiddenWire["definition"] != "publish" {
		t.Error("the copy definition name is configuration and stays visible")
	}
	if hiddenWire["kind"] != string(store.OriginCopy) {
		t.Error("the mechanism stays visible — only the ROW it names is gated")
	}
}

// TestGateOriginSources_FailsClosedOnGateError: a probe failure must withhold
// the label, not emit it. Failing closed loses a decoration; failing open
// leaks the existence of a row.
func TestGateOriginSources_FailsClosedOnGateError(t *testing.T) {
	metas := []store.VersionMeta{{Version: 1, Origin: store.Origin{
		Kind: store.OriginCopy, Source: "POL-1", SourceType: "policy",
	}}}
	gate := originGate{allow: map[string]bool{"POL-1": true}, err: errors.New("gate down")}

	if got := gateOriginSources(context.Background(), gate, metas); len(got) != 0 {
		t.Errorf("gate error must withhold every source label; got %v", got)
	}
}

// TestGateOriginSources_BatchesPerType pins that a long timeline costs one
// probe per distinct source TYPE, not one per version — the difference between
// a constant and an N+1 on an endpoint that returns a whole history.
func TestGateOriginSources_BatchesPerType(t *testing.T) {
	var metas []store.VersionMeta
	for i := range 20 {
		metas = append(metas, store.VersionMeta{Version: i + 1, Origin: store.Origin{
			Kind: store.OriginCopy, Source: "POL-1", SourceType: "policy",
		}})
	}
	calls := 0
	gate := originGate{allow: map[string]bool{"POL-1": true}, calls: &calls}

	gateOriginSources(context.Background(), gate, metas)
	if calls != 1 {
		t.Errorf("PermitsReadMany called %d times for 1 distinct source type, want 1", calls)
	}
}

// TestOriginWire_DirectEditRendersNothing is the "manual edits are marked
// similarly" half of the user's request, at the wire.
//
// The marking is the ABSENCE of an origin block. There is deliberately no
// "kind": "manual" — a default label would make the unmarked case ambiguous
// between "typed by hand" and "written before this field existed", and the
// hand edit is already fully described by the `principal` the version already
// carries.
func TestOriginWire_DirectEditRendersNothing(t *testing.T) {
	if got := originWire(store.Origin{}, ""); got != nil {
		t.Errorf("a direct edit must render no origin block; got %v", got)
	}
}
