package dataentry

import (
	"context"
	"iter"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// metaScopes classifies relation types from the test metamodel, mirroring
// what appbuild.RelationScopes supplies in production. Identity is the
// default, matching metamodelScopes.
type metaScopes struct{ meta *metamodel.Metamodel }

func (c metaScopes) IsContentScoped(relType string) bool {
	def, ok := c.meta.Relations[relType]
	return ok && def.Scope.IsContent()
}

// withWorldNeighbors wires the world-scoped link capability onto a test app,
// the way the composition root does.
func withWorldNeighbors(t *testing.T, app *App) *App {
	t.Helper()
	if err := SetWorldNeighbors(app, app.store, metaScopes{meta: app.Meta()}); err != nil {
		t.Fatalf("SetWorldNeighbors: %v", err)
	}
	return app
}

// publishedScope is the ISMS-shaped world: select the `published` face, and
// exclude anything that has none. Exclusion is what makes a link to an
// unpublished neighbor a link to nothing.
func publishedScope(types ...string) store.WorldScope {
	res := make(map[string]store.TypeResolution, len(types))
	for _, typ := range types {
		res[typ] = store.TypeResolution{
			Chain:    []entity.Pointer{entity.Pointer("published")},
			Fallback: store.FallbackExclude,
		}
	}
	return store.NewWorldScope(res)
}

// seedFace writes one content state of an entity.
func seedFace(t *testing.T, app *App, id, typ string, p entity.Pointer, title string) {
	t.Helper()
	if err := app.store.CreateEntity(context.Background(), &entity.Entity{
		ID: id, Type: typ, Pointer: p,
		Properties: map[string]any{"title": title},
	}); err != nil {
		t.Fatalf("seed %s@%s: %v", id, p, err)
	}
}

// worldCtx binds a world handle plus a permit-all read gate, which is the
// combination the handlers see for an unrestricted principal.
func worldCtx(scope store.WorldScope) context.Context {
	return withWorld(context.Background(), worldHandle{name: "published", scope: scope})
}

// TestWorldNeighbors_ExcludedHeadIsAbsent is the ISMS case from RULING 12,
// and the one the whole two-read design exists for.
//
// A published policy links to a control that has NO published face. The edge
// EXISTS in storage, so a design that resolved only the entry's edges would
// happily emit it — a link pointing at an entity that 404s in this world.
// Under `otherwise: exclude` the link must be ABSENT.
//
// This is the test that fails if head resolution is dropped, which is why it
// is written against the wire output rather than against the seam: the seam
// could be correct while the handler ignored it.
//
// # Mutation-checked (RULING 10), and the first attempt proved nothing
//
// The obvious mutation — deleting the `heads` lookup in headOnWire — leaves
// this test GREEN, because the visible set is built from heads, so an
// excluded neighbor is already gone by then. That check is defense in depth,
// not the mechanism.
//
// The mechanism is the world scope on the head-resolution query. Dropping
// `World: worldScopeFrom(ctx)` from worldNeighbors.resolveHeads makes
// FEAT-DRAFT resolve in the default world and survive, and THAT fails this
// test. Verified in both directions. If you are changing the exclusion
// behavior, mutate that line to confirm this test still bites.
func TestWorldNeighbors_ExcludedHeadIsAbsent(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))

	// TKT-1 has a published face and links to two features.
	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	seedFace(t, app, "TKT-1", "ticket", "published", "published policy")
	// FEAT-PUB is published; FEAT-DRAFT is not.
	seedEntity(app, &entity.Entity{
		ID: "FEAT-PUB", Type: "feature", Properties: map[string]any{"title": "pub draft"},
	})
	seedFace(t, app, "FEAT-PUB", "feature", "published", "published control")
	seedEntity(app, &entity.Entity{
		ID: "FEAT-DRAFT", Type: "feature", Properties: map[string]any{"title": "unpublished control"},
	})

	ctx := context.Background()
	for _, to := range []string{"FEAT-PUB", "FEAT-DRAFT"} {
		if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", to, nil); err != nil {
			t.Fatalf("seed edge to %s: %v", to, err)
		}
	}

	wctx := worldCtx(publishedScope("ticket", "feature"))
	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}
	outgoing, visible, err := worldOutgoingForEntity(wctx, app.worldNeighbors, app.visibleReader, face)
	if err != nil {
		t.Fatalf("worldOutgoingForEntity: %v", err)
	}

	got := map[string]bool{}
	for _, edge := range outgoing {
		got[edge.To] = true
	}
	if !got["FEAT-PUB"] {
		t.Errorf("a neighbor WITH a published face must be linked; got %v", got)
	}
	if got["FEAT-DRAFT"] {
		t.Errorf("a neighbor with NO published face must be absent — under "+
			"otherwise:exclude the link points at nothing in this world "+
			"(RULING 12, the ISMS case); got %v", got)
	}
	if visible["FEAT-DRAFT"] {
		t.Errorf("an excluded head must not reach the visible set either, or "+
			"the serializer would emit it; got %v", visible)
	}
}

// TestWorldNeighbors_PerLinkFallback is Jeroen's Spanish-page case: links
// resolve to the world's preferred face WHERE IT EXISTS and fall back
// PER LINK, not per page.
//
// The distinction is the whole point. A per-PAGE rule would decide once
// ("this page has no Dutch, so serve everything in English") and be
// self-consistent but wrong. This asserts one response carrying BOTH a
// chain-resolved neighbor and a fallback-resolved one — which is only
// possible if the decision is made per neighbor.
func TestWorldNeighbors_PerLinkFallback(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "en entry"},
	})
	seedFace(t, app, "TKT-1", "ticket", "nl", "nl entry")

	// FEAT-NL has a Dutch face; FEAT-EN has only the default (English) one.
	seedEntity(app, &entity.Entity{
		ID: "FEAT-NL", Type: "feature", Properties: map[string]any{"title": "english title"},
	})
	seedFace(t, app, "FEAT-NL", "feature", "nl", "nederlandse titel")
	seedEntity(app, &entity.Entity{
		ID: "FEAT-EN", Type: "feature", Properties: map[string]any{"title": "english only"},
	})

	ctx := context.Background()
	for _, to := range []string{"FEAT-NL", "FEAT-EN"} {
		if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", to, nil); err != nil {
			t.Fatalf("seed edge to %s: %v", to, err)
		}
	}

	// chain [nl], fallback DEFAULT — the multilingual shape: prefer Dutch,
	// serve English where no Dutch exists.
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket":  {Chain: []entity.Pointer{"nl"}, Fallback: store.FallbackDefaultState},
		"feature": {Chain: []entity.Pointer{"nl"}, Fallback: store.FallbackDefaultState},
	})
	wctx := withWorld(context.Background(), worldHandle{name: "site-nl", scope: scope})

	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}
	edges, heads, err := app.worldNeighbors.worldScopedNeighbors(
		wctx, resolvedFromStoreFace(wctx, face), store.DirectionOutgoing)
	if err != nil {
		t.Fatalf("worldScopedNeighbors: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("both links must survive — neither neighbor is excluded under "+
			"otherwise:default; got %d", len(edges))
	}

	// The load-bearing assertion: two neighbors, resolved DIFFERENTLY, in one
	// response.
	nl, ok := heads["FEAT-NL"]
	if !ok {
		t.Fatal("FEAT-NL must resolve")
	}
	if nl.Pointer != entity.Pointer("nl") {
		t.Errorf("a neighbor WITH a Dutch face must resolve to it; got pointer %q", nl.Pointer)
	}
	if got, _ := nl.Properties["title"].(string); got != "nederlandse titel" {
		t.Errorf("FEAT-NL title = %q, want the Dutch face's title", got)
	}
	en, ok := heads["FEAT-EN"]
	if !ok {
		t.Fatal("FEAT-EN must resolve via the fallback, not vanish")
	}
	if en.Pointer != entity.Pointer("") {
		t.Errorf("a neighbor with NO Dutch face must fall back to the default "+
			"face PER LINK, not drag the whole page to English; got pointer %q",
			en.Pointer)
	}
}

// TestWorldNeighbors_ContentEdgesAreFaceSpecific pins the identity-vs-content
// dispatch that worldreader.RelationReader.Neighbors performs, observed
// through this package's seam.
//
// A CONTENT-scoped edge belongs to one face's tail, so the published face's
// links must not include an edge stored on the draft tail. An IDENTITY-scoped
// edge belongs to the entity and must be visible from EVERY face.
//
// Both halves are asserted together on purpose: a dispatch that returned
// everything would pass the identity half alone, and one that returned only
// the pointer-matched edges would pass the content half alone.
func TestWorldNeighbors_ContentEdgesAreFaceSpecific(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	seedFace(t, app, "TKT-1", "ticket", "published", "published")
	for _, id := range []string{"FEAT-IDENT", "FEAT-DRAFTCITE", "FEAT-PUBCITE"} {
		seedEntity(app, &entity.Entity{
			ID: id, Type: "feature", Properties: map[string]any{"title": id},
		})
		seedFace(t, app, id, "feature", "published", id+" published")
	}

	// An IDENTITY edge (implements): no tail, belongs to the entity.
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-IDENT", nil); err != nil {
		t.Fatalf("seed identity edge: %v", err)
	}
	// CONTENT edges (cites): one on the DRAFT tail, one on the PUBLISHED tail.
	draftTail := entity.Pointer("")
	pubTail := entity.Pointer("published")
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "cites", "FEAT-DRAFTCITE",
		&store.RelationData{FromPointer: draftTail}); err != nil {
		t.Fatalf("seed draft-tail content edge: %v", err)
	}
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "cites", "FEAT-PUBCITE",
		&store.RelationData{FromPointer: pubTail}); err != nil {
		t.Fatalf("seed published-tail content edge: %v", err)
	}

	wctx := worldCtx(publishedScope("ticket", "feature"))
	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}
	if face.Pointer != pubTail {
		t.Fatalf("precondition: the entry must resolve to the PUBLISHED face, "+
			"or the content-tail assertions below test the wrong tail; got %q", face.Pointer)
	}
	outgoing, _, err := worldOutgoingForEntity(wctx, app.worldNeighbors, app.visibleReader, face)
	if err != nil {
		t.Fatalf("worldOutgoingForEntity: %v", err)
	}

	got := map[string]bool{}
	for _, edge := range outgoing {
		got[edge.To] = true
	}
	if !got["FEAT-IDENT"] {
		t.Errorf("an IDENTITY edge belongs to the entity and must be visible "+
			"from every face, including a non-default one; got %v", got)
	}
	if !got["FEAT-PUBCITE"] {
		t.Errorf("a CONTENT edge on the published tail must appear on the "+
			"published face; got %v", got)
	}
	if got["FEAT-DRAFTCITE"] {
		t.Errorf("a CONTENT edge on the DRAFT tail must NOT appear on the "+
			"published face — that is the draft's citation, and showing it "+
			"is the mixed-face bug; got %v", got)
	}
}

// TestWorldNeighbors_WorldResolvesBeforeGate is the ORDERING test RULING 10
// governs, and it is written to be mutation-sensitive in a specific way.
//
// # Why this ordering is a security property, not a preference
//
// Guard rule 1: world resolution must be PRINCIPAL-INDEPENDENT. If the ACL
// gate ran first, the set of ids handed to the world would depend on what the
// principal may read — so a neighbor's presence would confound two facts
// ("no face in this world" and "you may not read it") in a way the caller
// could pull apart by comparing across principals.
//
// # The injection point, and why it is this one
//
// The assertion is that the WORLD-RESOLUTION query sees the id of a neighbor
// the principal cannot read. That is the only observation that distinguishes
// the two orderings: under gate-first, that id is filtered out BEFORE
// resolveHeads runs, so the query never sees it. Asserting only on the final
// wire output would prove nothing — the neighbor is absent under BOTH
// orderings (gated out either way), which is exactly the "passes trivially"
// shape RULING 10 names.
//
// Mutation-checked: moving visibleWorldNeighbors ahead of resolveHeads in
// worldOutgoingForEntity fails this test and no other.
func TestWorldNeighbors_WorldResolvesBeforeGate(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "entry"},
	})
	seedFace(t, app, "TKT-1", "ticket", "published", "published entry")
	// FEAT-HIDDEN is published, so the WORLD resolves it happily; the ACL is
	// what removes it. That separation is what the test turns on.
	seedEntity(app, &entity.Entity{
		ID: "FEAT-HIDDEN", Type: "feature", Properties: map[string]any{"title": "secret"},
	})
	seedFace(t, app, "FEAT-HIDDEN", "feature", "published", "published secret")
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-HIDDEN", nil); err != nil {
		t.Fatalf("seed edge: %v", err)
	}

	// A principal who may read tickets but NOT features.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	req, rerr := d.ForPrincipal(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	if rerr != nil {
		t.Fatalf("ForPrincipal: %v", rerr)
	}
	gate, gerr := newACLReadGate(req)
	if gerr != nil {
		t.Fatalf("newACLReadGate: %v", gerr)
	}

	// Observe the ids the WORLD resolution is asked about, by wrapping the
	// store the neighbor seam reads through.
	spy := &headSpyStore{Store: app.store}
	app.worldNeighbors.store = spy

	wctx := withWorld(withReadGate(aliceCtx(), gate),
		worldHandle{name: "published", scope: publishedScope("ticket", "feature")})
	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}
	outgoing, visible, err := worldOutgoingForEntity(wctx, app.worldNeighbors, app.visibleReader, face)
	if err != nil {
		t.Fatalf("worldOutgoingForEntity: %v", err)
	}

	// THE ordering assertion.
	if !spy.asked("FEAT-HIDDEN") {
		t.Errorf("the world must resolve a neighbor BEFORE the ACL gate sees "+
			"it (guard rule 1: resolution is principal-independent). "+
			"FEAT-HIDDEN never reached the world-resolution query, which "+
			"means the gate ran first. Asked about: %v", spy.ids)
	}
	// And the gate still removes it — world-first must not widen anything.
	if visible["FEAT-HIDDEN"] {
		t.Error("resolving before the gate must not let a hidden neighbor " +
			"through; the gate still decides what reaches the wire")
	}
	for _, edge := range outgoing {
		if edge.To == "FEAT-HIDDEN" {
			t.Error("a neighbor the principal may not read must be absent from the wire")
		}
	}
}

// headSpyStore records the ids passed to the world head-resolution query.
// It wraps rather than replaces the store so every other read behaves
// normally — a stub returning nothing would make the test pass for the wrong
// reason.
type headSpyStore struct {
	store.Store
	ids []string
}

func (s *headSpyStore) ListEntities(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	s.ids = append(s.ids, q.IDs...)
	return s.Store.ListEntities(ctx, q)
}

func (s *headSpyStore) asked(id string) bool {
	return slices.Contains(s.ids, id)
}

// TestWorldNeighbors_IncludeAgreesWithRelations pins the invariant RR-HJV8CP
// established for the ACL gate, now that a SECOND filter (the world) can
// remove a neighbor.
//
// `relations` and `included` must never disagree: a peer named in the
// relations map must be resolvable in `included`, and a peer absent from
// `included` must not be named. A world that filtered one but not the other
// would reopen the leak from the other side — the relations map would carry
// the raw id of an entity the response cannot show.
func TestWorldNeighbors_IncludeAgreesWithRelations(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	seedFace(t, app, "TKT-1", "ticket", "published", "published")
	seedEntity(app, &entity.Entity{
		ID: "FEAT-PUB", Type: "feature", Properties: map[string]any{"title": "d"},
	})
	seedFace(t, app, "FEAT-PUB", "feature", "published", "published feature")
	seedEntity(app, &entity.Entity{
		ID: "FEAT-DRAFT", Type: "feature", Properties: map[string]any{"title": "unpublished"},
	})
	for _, to := range []string{"FEAT-PUB", "FEAT-DRAFT"} {
		if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", to, nil); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}

	wctx := worldCtx(publishedScope("ticket", "feature"))
	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}
	outgoing, visible, err := worldOutgoingForEntity(wctx, app.worldNeighbors, app.visibleReader, face)
	if err != nil {
		t.Fatalf("worldOutgoingForEntity: %v", err)
	}
	wire := app.serializer.forWireScoped(wctx, face, outgoing, visible, app.Meta(), "tickets")
	included := app.resolveV1Includes(wctx, face, "*")

	for _, ids := range wire.Relations {
		for _, id := range ids {
			if _, ok := included[id]; !ok {
				t.Errorf("relations names %q but included cannot resolve it — "+
					"the two must agree (RR-HJV8CP, now across the world filter "+
					"as well as the ACL one)", id)
			}
		}
	}
	if _, leaked := included["FEAT-DRAFT"]; leaked {
		t.Error("a neighbor with no face in this world must not appear in included")
	}
	if len(included) == 0 {
		t.Error("the published neighbor must be included — an empty map would " +
			"satisfy the agreement check vacuously")
	}
}

// TestWorldNeighbors_DefaultWorldUnchanged is the compat assertion: a
// pointerless request must behave exactly as it did before item 4.
//
// Worth its own test because every change here is guarded by an
// IsDefaultWorld() branch, and a branch written the wrong way round would
// send ordinary traffic through the world path — where a zero WorldScope
// resolves everything to its default face and would LOOK correct while
// taking a different route with different error handling.
func TestWorldNeighbors_DefaultWorldUnchanged(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t"},
	})
	seedEntity(app, &entity.Entity{
		ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "f"},
	})
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatalf("seed edge: %v", err)
	}

	// No world on the context at all — the zero handle, i.e. the default world.
	e, found, err := app.visibleReader.getVisible(ctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	outgoing := app.reader.outgoingRelations(ctx, e.ID)
	wire := app.serializer.forWire(ctx, e, outgoing, app.Meta(), "tickets")
	if got := wire.Relations["implements"]; len(got) != 1 || got[0] != "FEAT-1" {
		t.Errorf("the default world must carry relations exactly as before; got %v", got)
	}
	if included := app.resolveV1Includes(ctx, e, "*"); len(included) != 1 {
		t.Errorf("the default world's include block is unchanged; got %d entries", len(included))
	}
}

// TestParseIncludeSpec covers the expression parser the two collection paths
// share, including the duplicate-type rule that preserves the pre-refactor
// first-wins order.
func TestParseIncludeSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantAll    bool
		wantWanted map[string]string
	}{
		{"star", "*", true, nil},
		{"star with spaces", " * ", true, nil},
		{"single", "implements", false, map[string]string{"implements": ""}},
		{"nested", "implements.requires", false, map[string]string{"implements": "requires"}},
		{"several", "a,b", false, map[string]string{"a": "", "b": ""}},
		{"spaces trimmed", " a , b ", false, map[string]string{"a": "", "b": ""}},
		{"empty parts skipped", "a,,b", false, map[string]string{"a": "", "b": ""}},
		{"deep nesting keeps remainder", "a.b.c", false, map[string]string{"a": "b.c"}},
		// First wins, matching the pre-refactor loop order.
		{"duplicate keeps first", "a.b,a.c", false, map[string]string{"a": "b"}},
		// A trailing dot records NO nesting. The pre-refactor loop recorded an
		// empty one and recursed once for nothing; see parseIncludeSpec.
		{"trailing dot is not nesting", "a.", false, map[string]string{"a": ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wanted, all := parseIncludeSpec(tc.in)
			if all != tc.wantAll {
				t.Fatalf("all = %v, want %v", all, tc.wantAll)
			}
			if all {
				return
			}
			if len(wanted) != len(tc.wantWanted) {
				t.Fatalf("wanted = %v, want %v", wanted, tc.wantWanted)
			}
			for k, v := range tc.wantWanted {
				if wanted[k] != v {
					t.Errorf("wanted[%q] = %q, want %q", k, wanted[k], v)
				}
			}
		})
	}
}

// TestHeadIDsOf_SkipsSelfAndDedupes covers the id collector, including the
// self-reference case that DirectionBoth makes reachable.
func TestHeadIDsOf_SkipsSelfAndDedupes(t *testing.T) {
	t.Parallel()
	edges := []*entity.Relation{
		{From: "TKT-1", Type: "implements", To: "FEAT-1"},
		{From: "TKT-1", Type: "blocks", To: "FEAT-1"}, // duplicate head
		{From: "TKT-2", Type: "blocks", To: "TKT-1"},  // incoming: peer is From
		{From: "TKT-1", Type: "blocks", To: "TKT-1"},  // self-edge: no peer
	}
	got := headIDsOf(edges, "TKT-1")
	want := map[string]bool{"FEAT-1": true, "TKT-2": true}
	if len(got) != len(want) {
		t.Fatalf("headIDsOf = %v, want exactly %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected id %q", id)
		}
	}
}

// TestWorldEdgesForWire_DropsUnresolvedAndHidden pins that BOTH conditions
// are required, and that they are indistinguishable on the wire.
//
// A table rather than one case because the interesting content is the
// combination: resolved-but-hidden and hidden-but-resolved must both drop,
// and a single-case test would pin only whichever the author thought of.
func TestWorldEdgesForWire_DropsUnresolvedAndHidden(t *testing.T) {
	t.Parallel()
	edge := func(to string) *entity.Relation {
		return &entity.Relation{From: "TKT-1", Type: "implements", To: to}
	}
	tests := []struct {
		name     string
		resolved bool
		visible  bool
		wantKept bool
	}{
		{"resolved and visible", true, true, true},
		{"resolved but hidden", true, false, false},
		{"visible but unresolved in this world", false, true, false},
		{"neither", false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			heads := map[string]*entity.Entity{}
			if tc.resolved {
				heads["FEAT-1"] = &entity.Entity{ID: "FEAT-1", Type: "feature"}
			}
			visible := map[string]bool{"FEAT-1": tc.visible}
			out, in := worldEdgesForWire([]*entity.Relation{edge("FEAT-1")}, "TKT-1", heads, visible)
			if len(in) != 0 {
				t.Errorf("an edge FROM the entry is outgoing, not incoming; got %d", len(in))
			}
			if got := len(out) == 1; got != tc.wantKept {
				t.Errorf("kept = %v, want %v", got, tc.wantKept)
			}
		})
	}
}

// TestResolvedFromStoreFace_CarriesPointer pins the one field Neighbors
// actually consults.
//
// Pointer is the content-edge tail. If it were dropped (or defaulted to the
// zero pointer), a published face would query the DEFAULT tail — and per the
// "fallback trap" in worldreader.RelationReader.Neighbors, the zero pointer
// as a FromPointer value is not "unfiltered", it is "default-tail only". So
// the failure would be a published face silently showing the draft's content
// edges: wrong, and quiet.
func TestResolvedFromStoreFace_CarriesPointer(t *testing.T) {
	t.Parallel()
	scope := publishedScope("ticket")
	ctx := withWorld(context.Background(), worldHandle{name: "published", scope: scope})
	e := &entity.Entity{ID: "TKT-1", Type: "ticket", Pointer: entity.Pointer("published")}

	res := resolvedFromStoreFace(ctx, e)
	if res.Pointer != entity.Pointer("published") {
		t.Errorf("Pointer = %q, want the coordinate the face was stored at — "+
			"it is the content-edge tail Neighbors queries", res.Pointer)
	}
	if !res.Found || res.Entity != e {
		t.Error("a face the store returned is Found, carrying that entity")
	}
	if res.Via != worldreader.RuleChain {
		t.Errorf("Via = %v, want chain: `published` is in this world's chain", res.Via)
	}
}

// TestRuleFromName_MatchesWireVocabulary pins that the two vocabularies stay
// in step. They are separate constants in separate packages, so nothing but a
// test stops them drifting — and a drift would mislabel provenance rather
// than fail, which is the quiet kind.
func TestRuleFromName_MatchesWireVocabulary(t *testing.T) {
	t.Parallel()
	cases := map[string]worldreader.Rule{
		ruleUnscoped:        worldreader.RuleUnscoped,
		ruleChain:           worldreader.RuleChain,
		ruleFallbackDefault: worldreader.RuleFallbackDefault,
	}
	for name, want := range cases {
		if got := ruleFromName(name); got != want {
			t.Errorf("ruleFromName(%q) = %v, want %v", name, got, want)
		}
		if want.String() != name {
			t.Errorf("worldreader.Rule %v stringifies as %q but the wire "+
				"constant is %q — the two vocabularies must match",
				want, want.String(), name)
		}
	}
}

// TestIncludeTrailingDot_MatchesPreRefactorOutput pins the one place this
// refactor diverges from the loop it replaced.
//
// `include=implements.` used to record an EMPTY nested expression and recurse
// once; it now records no nesting and does not recurse. The claim is that the
// `included` map is identical either way — the old recursion split "" into a
// single empty part and skipped it, so it contributed nothing. A comment
// asserting that would be a claim; this checks it, by comparing the trailing-
// dot form against the plain form that has always been equivalent.
func TestIncludeTrailingDot_MatchesPreRefactorOutput(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t"},
	})
	seedEntity(app, &entity.Entity{
		ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "f"},
	})
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	e, found, err := app.visibleReader.getVisible(ctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}

	plain := app.resolveV1Includes(ctx, e, "implements")
	dotted := app.resolveV1Includes(ctx, e, "implements.")
	if len(plain) != len(dotted) {
		t.Fatalf("a trailing dot must not change the included set: "+
			"plain=%d dotted=%d", len(plain), len(dotted))
	}
	for id := range plain {
		if _, ok := dotted[id]; !ok {
			t.Errorf("included %q present without the trailing dot, absent with it", id)
		}
	}
	if len(plain) != 1 {
		t.Fatalf("precondition: the plain form must include exactly the one "+
			"neighbor, or this proves nothing; got %d", len(plain))
	}
}

// TestWorldETag_ReflectsWorldResolvedEdges pins that the cache validator
// describes the document actually served.
//
// Before TKT-WRLDAPI item 4 a world-bound response carried no relations, so
// computeEntityETag folded no edges under a world and that was correct. Item 4
// made those responses carry world-resolved links WITHOUT updating the ETag,
// which meant an edge change under a world did not move the validator — a
// stale 304 on a response whose links had changed. This is the regression
// test for that.
//
// # Why it asserts a DIFFERENCE rather than a specific hash
//
// Pinning a literal hash would break on any unrelated hashing change and
// prove nothing about the property. The property is "changing a world-visible
// edge changes the validator", so the test changes one and compares.
//
// Mutation-checked (RULING 10): reverting the world arm to fold no edges
// makes both hashes identical and fails this test. A test that only checked
// "the ETag is non-empty" would have passed against that.
func TestWorldETag_ReflectsWorldResolvedEdges(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	seedFace(t, app, "TKT-1", "ticket", "published", "published")
	seedEntity(app, &entity.Entity{
		ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "f"},
	})
	seedFace(t, app, "FEAT-1", "feature", "published", "published f")

	wctx := worldCtx(publishedScope("ticket", "feature"))
	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}

	before := app.computeEntityETag(wctx, face)

	// Add an edge that IS visible in this world (both endpoints published).
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	after := app.computeEntityETag(wctx, face)

	if before == after {
		t.Error("adding a world-visible edge must change the ETag — otherwise " +
			"a client holding the old validator gets a 304 for a response " +
			"whose relations map has changed")
	}
}

// TestWorldETag_DoesNotFoldDefaultWorldEdges is the mirror half: a world-bound
// validator must not be moved by an edge that is invisible in that world.
//
// Both halves are needed. Folding the DEFAULT-world reader under a world would
// pass the test above (any edge moves the hash) while being wrong in the other
// direction: publishing a face would not invalidate the validator, and an
// unrelated draft-only edit would.
func TestWorldETag_DoesNotFoldDefaultWorldEdges(t *testing.T) {
	app := withWorldNeighbors(t, newTestAppV1(t))
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	seedFace(t, app, "TKT-1", "ticket", "published", "published")
	// FEAT-DRAFT has NO published face, so an edge to it is invisible in the
	// published world.
	seedEntity(app, &entity.Entity{
		ID: "FEAT-DRAFT", Type: "feature", Properties: map[string]any{"title": "unpublished"},
	})

	wctx := worldCtx(publishedScope("ticket", "feature"))
	face, found, err := app.visibleReader.getVisible(wctx, "ticket", "TKT-1")
	if err != nil || !found {
		t.Fatalf("resolve entry: found=%v err=%v", found, err)
	}

	before := app.computeEntityETag(wctx, face)
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-DRAFT", nil); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	after := app.computeEntityETag(wctx, face)

	if before != after {
		t.Error("an edge INVISIBLE in this world must not move the world's " +
			"ETag — folding default-world edges into a published validator " +
			"means a draft-only edit invalidates a published cache entry, " +
			"and publishing a face does not")
	}
}
