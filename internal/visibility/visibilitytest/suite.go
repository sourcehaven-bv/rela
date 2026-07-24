// Package visibilitytest is the conformance suite for the
// internal/visibility contracts (the analog of storetest's
// RunVisibleSearchTests): any [visibility.Reader] implementation and any
// visibility-decorated [tracer.Tracer] must pass it. The suite owns a
// canonical world (memstore + declarative ACL + affordances resolver) and
// pins the security invariants from PLAN-RR12W4:
//
//   - hidden == missing (no existence oracle, RR-NGMI);
//   - stored-type check inside Get (RR-SRZK6X);
//   - redaction-on-copy, stored state never mutated (RR-6IL3X7);
//   - title never leaks past redaction (RR-5N4K35);
//   - both-endpoints relation rule (RR-Y7P4MQ);
//   - fail-closed on gate error / unstamped principal;
//   - allow-all and nop-policy parity with raw access.
package visibilitytest

import (
	"context"
	"maps"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// ReaderMaker builds the Reader under test from the suite's collaborators.
// Production impls compose exactly these three; a wiring under test (PR
// 2/3) adapts its own construction to this shape.
type ReaderMaker func(
	t *testing.T, gate visibility.RowGate, redact visibility.FieldRedactor, get visibility.EntityGetter,
) visibility.Reader

// TracerMaker builds the visibility-decorated tracer under test over the
// suite's base tracer and collaborators.
type TracerMaker func(
	t *testing.T, base tracer.Tracer,
	gate visibility.RowGate, redact visibility.FieldRedactor, get visibility.EntityGetter,
) tracer.Tracer

// world is the canonical fixture: a seeded memstore plus the real ACL
// engines (declarative gate + affordances redactor) for the canonical
// policy below.
type world struct {
	store  store.Store
	base   tracer.Tracer
	gate   visibility.DeclarativeGate
	redact visibility.PolicyRedactor
}

// Canonical policy. Principals:
//
//	alice — admin: reads everything, no field restrictions.
//	bob   — limited: reads project+person (NOT secret); on person only
//	        title+name are visible (salary hidden).
//	carol — notitle: reads project+person; on person only name+salary
//	        are visible (title hidden → ID fallback everywhere).
const policyYAML = `
roles:
  admin:
    read: ["*"]
  limited:
    read: [project, person]
    visible:
      person:
        - field: title
        - field: name
  notitle:
    read: [project, person]
    visible:
      person:
        - field: name
        - field: salary
assignments:
  alice: admin
  bob: limited
  carol: notitle
`

// suiteMeta declares the closed-world field universe the visible: blocks
// deny from.
func suiteMeta() *metamodel.Metamodel {
	str := metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"project": {Properties: map[string]metamodel.PropertyDef{"title": str}},
			"secret":  {Properties: map[string]metamodel.PropertyDef{"title": str}},
			"person": {Properties: map[string]metamodel.PropertyDef{
				"title":  str,
				"name":   str,
				"salary": {Type: metamodel.PropertyTypeInteger},
			}},
		},
	}
}

// newWorld seeds the canonical graph and wires the real engines.
//
// Graph (all edges relation type "relates"):
//
//	PRJ-1 → SEC-1 → PRJ-2        (bob's only PRJ-1..PRJ-2 path runs
//	PRJ-1 → P-1                   through a type he cannot read)
//	PRJ-4 → SEC-3 ⇄ SEC-4        (cycle entirely inside hidden types)
//	PRJ-3, SEC-2                  (orphans: one visible, one hidden)
func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()

	seed := []*entity.Entity{
		{ID: "PRJ-1", Type: "project", Properties: map[string]any{"title": "Alpha"}},
		{ID: "PRJ-2", Type: "project", Properties: map[string]any{"title": "Beta"}},
		{ID: "PRJ-3", Type: "project", Properties: map[string]any{"title": "Gamma"}},
		{ID: "PRJ-4", Type: "project", Properties: map[string]any{"title": "Delta"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "Hush"}},
		{ID: "SEC-2", Type: "secret", Properties: map[string]any{"title": "Shh"}},
		{ID: "SEC-3", Type: "secret", Properties: map[string]any{"title": "Loop A"}},
		{ID: "SEC-4", Type: "secret", Properties: map[string]any{"title": "Loop B"}},
		{ID: "P-1", Type: "person", Properties: map[string]any{"title": "Ann Ex", "name": "Ann", "salary": 100}},
	}
	for _, e := range seed {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed entity %s: %v", e.ID, err)
		}
	}
	for _, r := range [][3]string{
		{"PRJ-1", "relates", "SEC-1"},
		{"SEC-1", "relates", "PRJ-2"},
		{"PRJ-1", "relates", "P-1"},
		{"PRJ-4", "relates", "SEC-3"},
		{"SEC-3", "relates", "SEC-4"},
		{"SEC-4", "relates", "SEC-3"},
	} {
		if _, err := st.CreateRelation(ctx, r[0], r[1], r[2], nil); err != nil {
			t.Fatalf("seed relation %v: %v", r, err)
		}
	}

	var p acl.Policy
	if err := yaml.Unmarshal([]byte(policyYAML), &p); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	d, err := acl.NewDeclarative(&p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(suiteMeta(), storeLookup{st}, d)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		t.Fatalf("NewDeclarativeGate: %v", err)
	}
	redact, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		t.Fatalf("NewPolicyRedactor: %v", err)
	}
	return &world{store: st, base: tracer.New(st), gate: gate, redact: redact}
}

// storeLookup implements affordances.RelationLookup over the store.
type storeLookup struct{ st store.Store }

func (l storeLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		counts[rel.Type]++
	}
	return counts
}

func (l storeLookup) HasEdge(ctx context.Context, fromID, relType, toID string) bool {
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Type: relType, To: toID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		return true
	}
	return false
}

// ctxFor stamps a principal ctx for one of the canonical users.
func ctxFor(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
}

// mustGet loads an entity straight from the store (bypassing everything)
// for parity and no-mutation assertions.
func mustGet(t *testing.T, st store.Store, id string) *entity.Entity {
	t.Helper()
	e, err := st.GetEntity(context.Background(), id)
	if err != nil {
		t.Fatalf("store.GetEntity(%s): %v", id, err)
	}
	return e
}

// RunReaderTests is the conformance suite for [visibility.Reader].
func RunReaderTests(t *testing.T, mk ReaderMaker) {
	t.Helper()
	t.Run("HiddenEqualsMissing", func(t *testing.T) { testHiddenEqualsMissing(t, mk) })
	t.Run("StoredTypeMismatchIsMiss", func(t *testing.T) { testStoredTypeMismatch(t, mk) })
	t.Run("RedactsOnCopy", func(t *testing.T) { testRedactsOnCopy(t, mk) })
	t.Run("HiddenTitleFallsBackToID", func(t *testing.T) { testHiddenTitleFallback(t, mk) })
	t.Run("FilterMixedVisibility", func(t *testing.T) { testFilterMixed(t, mk) })
	t.Run("FilterGateErrorDropsTypeFailClosed", func(t *testing.T) { testFilterGateError(t, mk) })
	t.Run("FilterRelationsBothEndpointsRule", func(t *testing.T) { testFilterRelations(t, mk) })
	t.Run("UnstampedPrincipalFailsClosed", func(t *testing.T) { testUnstampedPrincipal(t, mk) })
	t.Run("HideEverythingRedactor", func(t *testing.T) { testHideEverythingRedactor(t, mk) })
	t.Run("NopPolicyParity", func(t *testing.T) { testReaderNopParity(t, mk) })
	t.Run("RaceSmoke", func(t *testing.T) { testReaderRaceSmoke(t, mk) })
}

func testHiddenEqualsMissing(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)
	ctx := ctxFor("bob")

	eHidden, okHidden, errHidden := r.Get(ctx, "secret", "SEC-1")
	eMissing, okMissing, errMissing := r.Get(ctx, "secret", "SEC-404")
	if eHidden != nil || okHidden || errHidden != nil {
		t.Fatalf("hidden Get = (%v,%v,%v), want (nil,false,nil)", eHidden, okHidden, errHidden)
	}
	if eMissing != nil || okMissing || errMissing != nil {
		t.Fatalf("missing Get = (%v,%v,%v), want (nil,false,nil)", eMissing, okMissing, errMissing)
	}
}

func testStoredTypeMismatch(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)

	// bob may read type project (AllowAll scope), so the gate permits the
	// CLAIM — the stored entity is a secret he cannot read. The in-package
	// stored-type check must turn this into a miss (RR-SRZK6X, the
	// BUG-ZWTDH9 read-side analog).
	e, ok, err := r.Get(ctxFor("bob"), "project", "SEC-1")
	if e != nil || ok || err != nil {
		t.Fatalf("cross-type Get = (%v,%v,%v), want (nil,false,nil)", e, ok, err)
	}
	// Even a fully-privileged principal gets a miss on a wrong claim: the
	// check is Reader semantics, not policy.
	e, ok, err = r.Get(ctxFor("alice"), "project", "P-1")
	if e != nil || ok || err != nil {
		t.Fatalf("alice cross-type Get = (%v,%v,%v), want (nil,false,nil)", e, ok, err)
	}
}

func testRedactsOnCopy(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)
	beforeProps := maps.Clone(mustGet(t, w.store, "P-1").Properties)

	e, ok, err := r.Get(ctxFor("bob"), "person", "P-1")
	if err != nil || !ok {
		t.Fatalf("Get person P-1 = (ok=%v, err=%v)", ok, err)
	}
	if _, leaked := e.Properties["salary"]; leaked {
		t.Fatalf("salary visible to bob: %v", e.Properties)
	}
	if e.Properties["name"] != "Ann" || e.Properties["title"] != "Ann Ex" {
		t.Fatalf("granted fields damaged: %v", e.Properties)
	}
	after := mustGet(t, w.store, "P-1")
	if !reflect.DeepEqual(beforeProps, after.Properties) {
		t.Fatalf("stored entity mutated by redaction: before=%v after=%v", beforeProps, after.Properties)
	}
}

func testHiddenTitleFallback(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)

	e, ok, err := r.Get(ctxFor("carol"), "person", "P-1")
	if err != nil || !ok {
		t.Fatalf("Get person P-1 = (ok=%v, err=%v)", ok, err)
	}
	if _, leaked := e.Properties["title"]; leaked {
		t.Fatalf("title visible to carol: %v", e.Properties)
	}
	// The stripped copy has no secondary title channel: every DisplayTitle
	// derivation recomputes from Properties and lands on the ID.
	if got := e.Title(); got != "" {
		t.Fatalf("Title() over redacted copy = %q, want empty (→ ID fallback downstream)", got)
	}
}

func testFilterMixed(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)
	in := []*entity.Entity{
		mustGet(t, w.store, "PRJ-1"),
		mustGet(t, w.store, "SEC-1"),
		mustGet(t, w.store, "P-1"),
		mustGet(t, w.store, "PRJ-2"),
	}

	out := r.Filter(ctxFor("bob"), in)
	ids := make([]string, 0, len(out))
	for _, e := range out {
		ids = append(ids, e.ID)
	}
	want := []string{"PRJ-1", "P-1", "PRJ-2"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("Filter ids = %v, want %v (order-preserving, hidden dropped)", ids, want)
	}
	for _, e := range out {
		if e.ID == "P-1" {
			if _, leaked := e.Properties["salary"]; leaked {
				t.Fatalf("Filter survivor not redacted: %v", e.Properties)
			}
		}
	}
	if got := r.Filter(ctxFor("bob"), nil); got != nil {
		t.Fatalf("Filter(nil) = %v, want nil", got)
	}
}

func testFilterGateError(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, &erroringGate{inner: w.gate, failType: "person"}, w.redact, w.store)
	in := []*entity.Entity{
		mustGet(t, w.store, "PRJ-1"),
		mustGet(t, w.store, "P-1"),
	}
	out := r.Filter(ctxFor("alice"), in)
	if len(out) != 1 || out[0].ID != "PRJ-1" {
		t.Fatalf("Filter with erroring person gate = %v, want [PRJ-1] only", out)
	}
}

func testFilterRelations(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	counting := &countingGate{inner: w.gate}
	r := mk(t, counting, w.redact, w.store)
	rels := []*entity.Relation{
		{From: "PRJ-1", Type: "relates", To: "SEC-1"}, // TO hidden → dropped
		{From: "SEC-1", Type: "relates", To: "PRJ-2"}, // FROM hidden → dropped
		{From: "PRJ-1", Type: "relates", To: "P-1"},   // both visible → kept
	}

	out := r.FilterRelations(ctxFor("bob"), rels)
	if len(out) != 1 || out[0].From != "PRJ-1" || out[0].To != "P-1" {
		t.Fatalf("FilterRelations = %v, want only PRJ-1→P-1", out)
	}
	// Batched endpoint gating: one PermitsReadMany per distinct endpoint
	// type (project, secret, person = 3), not per endpoint.
	if counting.many > 3 {
		t.Fatalf("FilterRelations made %d PermitsReadMany calls, want ≤3 (one per distinct type)", counting.many)
	}
	if got := r.FilterRelations(ctxFor("bob"), nil); got != nil {
		t.Fatalf("FilterRelations(nil) = %v, want nil", got)
	}
}

func testUnstampedPrincipal(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)

	// No principal on ctx → ForPrincipal rejects → gate error → deny.
	e, ok, err := r.Get(context.Background(), "project", "PRJ-1")
	if err == nil {
		t.Fatalf("unstamped Get = (%v,%v,nil), want gate error (fail closed, never open)", e, ok)
	}
	if e != nil || ok {
		t.Fatalf("unstamped Get leaked entity: (%v,%v)", e, ok)
	}
	if out := r.Filter(context.Background(), []*entity.Entity{mustGet(t, w.store, "PRJ-1")}); len(out) != 0 {
		t.Fatalf("unstamped Filter = %v, want empty (fail closed)", out)
	}
}

func testHideEverythingRedactor(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, hideAllRedactor{}, w.store)

	e, ok, err := r.Get(ctxFor("alice"), "person", "P-1")
	if err != nil || !ok {
		t.Fatalf("Get = (ok=%v, err=%v)", ok, err)
	}
	if len(e.Properties) != 0 {
		t.Fatalf("hide-everything redactor left properties: %v", e.Properties)
	}
}

func testReaderNopParity(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, visibility.NopGate{}, visibility.NopRedactor{}, w.store)
	ctx := context.Background() // parity must hold even without a principal

	raw := mustGet(t, w.store, "P-1")
	e, ok, err := r.Get(ctx, "person", "P-1")
	if err != nil || !ok {
		t.Fatalf("nop Get = (ok=%v, err=%v)", ok, err)
	}
	if !reflect.DeepEqual(raw, e) {
		t.Fatalf("nop-policy Get differs from raw store read:\nraw=%+v\ngot=%+v", raw, e)
	}
	in := []*entity.Entity{mustGet(t, w.store, "PRJ-1"), mustGet(t, w.store, "SEC-1")}
	out := r.Filter(ctx, in)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("nop-policy Filter differs from input: %v vs %v", in, out)
	}
}

func testReaderRaceSmoke(t *testing.T, mk ReaderMaker) {
	t.Helper()
	w := newWorld(t)
	r := mk(t, w.gate, w.redact, w.store)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()
			ctx := ctxFor(user) // independent ctx per goroutine: fresh acl.Request per call
			_, _, _ = r.Get(ctx, "person", "P-1")
			_ = r.Filter(ctx, []*entity.Entity{mustGet(t, w.store, "PRJ-1")})
		}([]string{"alice", "bob", "carol"}[i%3])
	}
	wg.Wait()
}

// RunTracerTests is the conformance suite for a visibility-decorated
// [tracer.Tracer].
func RunTracerTests(t *testing.T, mk TracerMaker) {
	t.Helper()
	t.Run("HiddenNodePrunesSubtree", func(t *testing.T) { testHiddenNodePrunes(t, mk) })
	t.Run("HiddenRootEqualsUnknownRoot", func(t *testing.T) { testHiddenRoot(t, mk) })
	t.Run("PathThroughHiddenNodeWithheld", func(t *testing.T) { testPathWithheld(t, mk) })
	t.Run("OrphansFiltered", func(t *testing.T) { testOrphansFiltered(t, mk) })
	t.Run("HasCycleHiddenStartEqualsMissingStart", func(t *testing.T) { testHasCycleHiddenStart(t, mk) })
	t.Run("CycleThroughHiddenNodesPruneTerminates", func(t *testing.T) { testHiddenCycleTerminates(t, mk) })
	t.Run("NodePropertiesRedactedAliasSafe", func(t *testing.T) { testNodeRedactionAliasSafe(t, mk) })
	t.Run("NodeAndStepTitleFallback", func(t *testing.T) { testTitleFallbacks(t, mk) })
	t.Run("NopPolicyParity", func(t *testing.T) { testTracerNopParity(t, mk) })
}

func testHiddenNodePrunes(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)

	got := tr.TraceFrom(ctxFor("bob"), "PRJ-1", 0)
	if got == nil {
		t.Fatal("TraceFrom(PRJ-1) = nil for bob, want visible tree")
	}
	ids := collectIDs(got)
	if ids["SEC-1"] || ids["PRJ-2"] {
		t.Fatalf("hidden node or its subtree leaked: %v (SEC-1 hidden ⇒ PRJ-2 unreachable)", keys(ids))
	}
	if !ids["P-1"] {
		t.Fatalf("visible branch missing: %v", keys(ids))
	}
}

func testHiddenRoot(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)
	ctx := ctxFor("bob")

	if got := tr.TraceFrom(ctx, "SEC-1", 0); got != nil {
		t.Fatalf("TraceFrom(hidden root) = %+v, want nil", got)
	}
	if got := tr.TraceFrom(ctx, "NOPE", 0); got != nil {
		t.Fatalf("TraceFrom(unknown root) = %+v, want nil (base shape)", got)
	}
}

func testPathWithheld(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)
	ctx := ctxFor("bob")

	// Sanity: the path exists for a privileged principal.
	if p := tr.FindPath(ctxFor("alice"), "PRJ-1", "PRJ-2"); len(p) == 0 {
		t.Fatal("premise broken: alice sees no PRJ-1..PRJ-2 path")
	}
	hidden := tr.FindPath(ctx, "PRJ-1", "PRJ-2") // exists, runs through SEC-1
	missing := tr.FindPath(ctx, "PRJ-1", "NOPE") // genuinely absent
	if !reflect.DeepEqual(hidden, missing) {
		t.Fatalf("withheld path (%v) distinguishable from no-path (%v)", hidden, missing)
	}
	if hidden != nil {
		t.Fatalf("withheld path = %v, want nil", hidden)
	}
}

func testOrphansFiltered(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)

	got, err := tr.FindOrphans(ctxFor("bob"))
	if err != nil {
		t.Fatalf("FindOrphans: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"PRJ-3"}) {
		t.Fatalf("orphans for bob = %v, want [PRJ-3] (SEC-2 hidden)", got)
	}
}

func testHasCycleHiddenStart(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)

	if !tr.HasCycle(ctxFor("alice"), "SEC-3") {
		t.Fatal("premise broken: alice sees no SEC-3 cycle")
	}
	got, want := tr.HasCycle(ctxFor("bob"), "SEC-3"), tr.HasCycle(ctxFor("bob"), "NOPE")
	if got != want || got {
		t.Fatalf("HasCycle(hidden start) = %v, HasCycle(missing start) = %v, want both false", got, want)
	}
}

func testHiddenCycleTerminates(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)

	got := tr.TraceFrom(ctxFor("bob"), "PRJ-4", 0)
	if got == nil {
		t.Fatal("TraceFrom(PRJ-4) = nil for bob, want the root alone")
	}
	if len(got.Children) != 0 {
		t.Fatalf("hidden cycle leaked below PRJ-4: %+v", got.Children)
	}
}

func testNodeRedactionAliasSafe(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)
	beforeProps := maps.Clone(mustGet(t, w.store, "P-1").Properties)

	got := tr.TraceFrom(ctxFor("bob"), "PRJ-1", 0)
	node := findNode(got, "P-1")
	if node == nil {
		t.Fatal("P-1 node missing from bob's trace")
	}
	if _, leaked := node.Properties["salary"]; leaked {
		t.Fatalf("salary leaked into trace node: %v", node.Properties)
	}
	after := mustGet(t, w.store, "P-1")
	if !reflect.DeepEqual(beforeProps, after.Properties) {
		t.Fatalf("trace redaction mutated the stored entity: before=%v after=%v",
			beforeProps, after.Properties)
	}
}

func testTitleFallbacks(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, w.gate, w.redact, w.store)
	ctx := ctxFor("carol") // title hidden on person

	got := tr.TraceFrom(ctx, "PRJ-1", 0)
	node := findNode(got, "P-1")
	if node == nil {
		t.Fatal("P-1 node missing from carol's trace")
	}
	if node.Title != "P-1" {
		t.Fatalf("trace node title = %q, want ID fallback %q", node.Title, "P-1")
	}
	steps := tr.FindPath(ctx, "PRJ-1", "P-1")
	if len(steps) == 0 {
		t.Fatal("FindPath(PRJ-1, P-1) empty for carol")
	}
	last := steps[len(steps)-1]
	if last.ID != "P-1" || last.Title != "P-1" {
		t.Fatalf("path step = %+v, want Title == ID fallback", last)
	}
}

func testTracerNopParity(t *testing.T, mk TracerMaker) {
	t.Helper()
	w := newWorld(t)
	tr := mk(t, w.base, visibility.NopGate{}, visibility.NopRedactor{}, w.store)
	ctx := context.Background()

	baseTree := w.base.TraceFrom(ctx, "PRJ-1", 0)
	gotTree := tr.TraceFrom(ctx, "PRJ-1", 0)
	if !reflect.DeepEqual(baseTree, gotTree) {
		t.Fatalf("nop-policy TraceFrom differs from base:\nbase=%+v\ngot=%+v", baseTree, gotTree)
	}
	basePath := w.base.FindPath(ctx, "PRJ-1", "PRJ-2")
	gotPath := tr.FindPath(ctx, "PRJ-1", "PRJ-2")
	if !reflect.DeepEqual(basePath, gotPath) {
		t.Fatalf("nop-policy FindPath differs from base: %v vs %v", basePath, gotPath)
	}
	baseOrphans, _ := w.base.FindOrphans(ctx)
	gotOrphans, err := tr.FindOrphans(ctx)
	if err != nil || !reflect.DeepEqual(baseOrphans, gotOrphans) {
		t.Fatalf("nop-policy FindOrphans differs: %v vs %v (err=%v)", baseOrphans, gotOrphans, err)
	}
	if b, g := w.base.HasCycle(ctx, "SEC-3"), tr.HasCycle(ctx, "SEC-3"); b != g {
		t.Fatalf("nop-policy HasCycle differs: base=%v got=%v", b, g)
	}
}

// --- suite stubs -----------------------------------------------------------

// erroringGate fails PermitsRead/Many for one type and delegates the rest.
type erroringGate struct {
	inner    visibility.RowGate
	failType string
}

func (g *erroringGate) PermitsRead(ctx context.Context, entityType, id string) (bool, error) {
	if entityType == g.failType {
		return false, errGate
	}
	return g.inner.PermitsRead(ctx, entityType, id)
}

func (g *erroringGate) PermitsReadMany(
	ctx context.Context, entityType string, ids []string,
) (map[string]bool, error) {
	if entityType == g.failType {
		return nil, errGate
	}
	return g.inner.PermitsReadMany(ctx, entityType, ids)
}

var errGate = &gateError{}

type gateError struct{}

func (*gateError) Error() string { return "visibilitytest: deliberate gate failure" }

// countingGate counts PermitsReadMany calls (batching assertions).
type countingGate struct {
	inner visibility.RowGate
	many  int
}

func (g *countingGate) PermitsRead(ctx context.Context, entityType, id string) (bool, error) {
	return g.inner.PermitsRead(ctx, entityType, id)
}

func (g *countingGate) PermitsReadMany(
	ctx context.Context, entityType string, ids []string,
) (map[string]bool, error) {
	g.many++
	return g.inner.PermitsReadMany(ctx, entityType, ids)
}

// hideAllRedactor hides every property — the FieldRedactor fail-closed
// contract shape (RR-FJUQSF).
type hideAllRedactor struct{}

func (hideAllRedactor) HiddenProperties(_ context.Context, e *entity.Entity) map[string]struct{} {
	out := make(map[string]struct{}, len(e.Properties))
	for k := range e.Properties {
		out[k] = struct{}{}
	}
	return out
}

// --- helpers ---------------------------------------------------------------

func collectIDs(n *tracer.TraceResult) map[string]bool {
	out := map[string]bool{}
	var walk func(*tracer.TraceResult)
	walk = func(x *tracer.TraceResult) {
		if x == nil {
			return
		}
		out[x.ID] = true
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(n)
	return out
}

func findNode(n *tracer.TraceResult, id string) *tracer.TraceResult {
	if n == nil {
		return nil
	}
	if n.ID == id {
		return n
	}
	for _, c := range n.Children {
		if f := findNode(c, id); f != nil {
			return f
		}
	}
	return nil
}

func keys(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return strings.Join(out, ",")
}
