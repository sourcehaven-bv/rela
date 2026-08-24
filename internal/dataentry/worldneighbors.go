package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// worldNeighbors is the world-scoped neighbor seam: it answers "what does
// this entity link to IN THIS WORLD", for both the edges and the entities
// those edges point at (TKT-WRLDAPI item 4, RULING 12).
//
// # Why this exists at all
//
// A world resolves the entry AND every link through the same rule, per
// neighbor, independently. An ISMS `published` view links to published
// faces, and a linked control with no published face is ABSENT under
// `otherwise: exclude`. A preview world with chain [draft, published] links
// to drafts where drafts exist and published otherwise. A Spanish page links
// to Spanish where Spanish exists and English where it does not — per-LINK
// fallback, not per-page.
//
// Before this type, a world-bound response carried NO relations at all: the
// only relation reader on these handlers was the ungated, default-world
// [entityReader], and pairing a published entity with draft edges is the
// mixed-face bug that reads as correct. Emitting nothing was the honest
// placeholder; it is not the answer, and RULING 12 closed it.
//
// # Two reads, not one
//
// Resolving a link is TWO steps, and collapsing them is the trap:
//
//  1. The EDGES of the entry's resolved face, via
//     [worldreader.RelationReader.Neighbors]. Per design-doc §2.3 a content
//     edge carries a state-specific TAIL, so which edges exist depends on
//     which face you are standing on.
//  2. The HEADS those edges name, resolved through the world. §2.3 also
//     says heads are ENTITY-LEVEL — an edge points at `SPEC-9`, never at
//     `SPEC-9@published` — so step 1 alone yields an id, not a face.
//
// Step 2 is where `otherwise: exclude` bites. Without it a published view
// emits a link to a control that has no published face: a link pointing at a
// 404, which is precisely the case RULING 12 says must be absent. Step 1
// cannot do this itself, because the edge is genuinely there — it is the
// HEAD that this world does not resolve.
//
// # Ordering: world, THEN gate. Non-negotiable.
//
// [worldScopedNeighbors] resolves heads before the ACL row gate runs, never
// after. This is guard rule 1 (see the [worldreader] package doc) applied one
// layer out: resolution must be PRINCIPAL-INDEPENDENT.
//
// Gate-first would be an existence oracle. If the gate ran before the world,
// a head the principal may not read would be dropped early — and under a
// fallback chain the world would then have nothing to resolve, so the
// RESULT SET a caller sees would differ depending on what the ACL denied
// them. World-first means a denied neighbor is absent for exactly one
// reason (the gate said no), indistinguishable from a neighbor that has no
// face in this world. Pinned by TestWorldNeighbors_WorldResolvesBeforeGate.
type worldNeighbors struct {
	// relations is the world-scoped relation capability. It is a
	// *worldreader.RelationReader rather than a store handle DELIBERATELY:
	// that type exists so the identity-vs-content scope dispatch is
	// unrepresentable to omit, and taking a store here would hand this
	// package the raw nil-tail query the dispatch is written to prevent.
	relations *worldreader.RelationReader

	// store resolves neighbor HEADS through the world. This is the same
	// store path visibleReader.getWorldEntity uses — deliberately, so entity
	// resolution has ONE implementation and this type adds no second one.
	store store.Store
}

// SetWorldNeighbors enables world-scoped LINK resolution (`?world=` on a
// response's relations and `?include=`).
//
// A package-level FUNCTION rather than a method on App, for the reason the
// world code has taken this shape throughout: App carries a
// `//plimsoll:max-methods=104` directive pinning it at its current count, and
// the project rule is to split the type rather than raise the number. The
// world feature has added ONE method to App so far ([App.SetWorlds]) and five
// package functions (resolveWorld, attachWorld, worldCapablePath,
// worldRefusesSearch, and this), which is the discipline recorded on the App
// type doc.
//
// Not calling this is a valid state: relations then behave as they did before
// TKT-WRLDAPI item 4 — present under the default world, absent under any
// other. That is safe but incomplete, which is why the composition root wires
// it whenever it wires [App.SetWorlds].
//
// classes classifies a relation type as content- or identity-scoped; it is
// supplied by the wiring site because the dispatch it feeds
// ([worldreader.RelationReader]) must not be reimplemented here.
//
// Nil: rejected — a nil app, store or classifier returns an error rather than
// silently leaving link resolution off, which would present as "this world
// has no links" on every page.
func SetWorldNeighbors(a *App, s store.Store, classes worldreader.ScopeClassifier) error {
	if a == nil {
		return errors.New("dataentry: SetWorldNeighbors: app must be non-nil")
	}
	if s == nil {
		return errors.New("dataentry: SetWorldNeighbors: store must be non-nil")
	}
	rr, err := worldreader.NewRelationReader(s, classes)
	if err != nil {
		return fmt.Errorf("dataentry: SetWorldNeighbors: %w", err)
	}
	a.worldNeighbors = &worldNeighbors{relations: rr, store: s}
	return nil
}

// worldScopedNeighbors returns the entity's edges under the request's world,
// together with the world-resolved heads those edges name.
//
// Returns (edges, headsByID, error). headsByID holds ONLY the neighbors this
// world resolves to a face: an id present in an edge but absent from the map
// is a link the world excludes, and the caller MUST drop that edge rather
// than emit it. That asymmetry is the point — see the type doc.
//
// The heads are resolved in ONE batched query over the whole id set, not a
// point read per neighbor. A hub entity with fifty links must not cost fifty
// round-trips (the RR-FRK1 shape, applied to world resolution).
//
// The ACL row gate is NOT applied here. Callers gate the returned heads —
// world first, then gate, per the type doc.
func (wn *worldNeighbors) worldScopedNeighbors(
	ctx context.Context, res worldreader.Resolved, dir store.Direction,
) (edges []*entityPkg.Relation, heads map[string]*entityPkg.Entity, err error) {
	if !res.Found || res.Entity == nil {
		// The world excludes the ENTRY, so it has no edges in this world.
		// Neighbors() makes the same judgement internally; restating the
		// guard here keeps the two-read contract legible at this level.
		return nil, nil, nil
	}

	edges, err = wn.relations.Neighbors(ctx, res, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("world-scoped neighbors for %s: %w", res.Entity.ID, err)
	}
	if len(edges) == 0 {
		return nil, nil, nil
	}

	ids := headIDsOf(edges, res.Entity.ID)
	if len(ids) == 0 {
		return edges, nil, nil
	}

	heads, err = wn.resolveHeads(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	return edges, heads, nil
}

// resolveHeads resolves neighbor ids to their face in the request's world.
//
// One ListEntities carrying the world scope, so the BACKEND resolves the
// chain and the fallback verdict. This is the same route
// visibleReader.getWorldEntity takes for the entry, which is what keeps
// entity resolution to a single implementation: a chain walk written here
// would be a second copy of the semantics that decide which face a reader
// sees, free to drift from the store's.
//
// An id absent from the result is a neighbor this world excludes. That is a
// normal outcome, not an error — under `otherwise: exclude` it IS the
// publication bit.
//
// An iterator error is returned, never swallowed into a short map. Truncating
// here would silently drop links and read as "this world excludes them",
// which is a backend outage wearing the costume of a correct answer — the
// mistake resolveWorld refuses to make one file over (RR-4TFZNL).
func (wn *worldNeighbors) resolveHeads(
	ctx context.Context, ids []string,
) (map[string]*entityPkg.Entity, error) {
	out := make(map[string]*entityPkg.Entity, len(ids))
	for e, err := range wn.store.ListEntities(ctx, store.EntityQuery{
		IDs:   ids,
		World: worldScopeFrom(ctx),
	}) {
		if err != nil {
			return nil, fmt.Errorf("resolving neighbor heads: %w", err)
		}
		out[e.ID] = e
	}
	return out, nil
}

// headIDsOf collects the DISTINCT neighbor ids an edge set names, from
// whichever endpoint is not selfID.
//
// Keying on selfID rather than on the direction is what makes this correct
// for [store.DirectionBoth], where an edge may name the entry on either side.
// A self-referential edge (From == To == selfID) contributes nothing, which
// is right: the entry's own face is already resolved.
func headIDsOf(edges []*entityPkg.Relation, selfID string) []string {
	seen := make(map[string]struct{}, len(edges))
	var ids []string
	add := func(id string) {
		if id == "" || id == selfID {
			return
		}
		if _, dup := seen[id]; dup {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, edge := range edges {
		add(edge.From)
		add(edge.To)
	}
	return ids
}

// visibleWorldNeighbors runs the ACL row gate over already-world-resolved
// heads and returns the id set that may appear on the wire.
//
// This is the world-path counterpart of [visibleRelationIDs], and the split
// is deliberate rather than duplication: visibleRelationIDs loads its
// candidates through the ungated, DEFAULT-world entityReader, which is
// exactly what a world-bound response must not do. Here the candidates are
// already the world's faces, so this only gates them.
//
// Gating happens AFTER resolution, never before — see the [worldNeighbors]
// type doc for why the order is load-bearing.
func visibleWorldNeighbors(
	ctx context.Context, visible visibleReader, heads map[string]*entityPkg.Entity,
) map[string]bool {
	if len(heads) == 0 {
		return map[string]bool{}
	}
	candidates := make([]*entityPkg.Entity, 0, len(heads))
	for _, e := range heads {
		candidates = append(candidates, e)
	}
	vis := visible.filterVisible(ctx, candidates)
	out := make(map[string]bool, len(vis))
	for _, e := range vis {
		out[e.ID] = true
	}
	return out
}

// worldEdgesForWire splits world-resolved edges into the outgoing/incoming
// pair the serializer takes, dropping every edge whose head this world does
// not resolve or whose head the principal may not read.
//
// Dropping an unresolved head is the ISMS case from RULING 12: a published
// view must not link to a control that has no published face. Dropping a
// gated head is the ordinary row-gate rule. The wire cannot distinguish
// them, which is correct — a neighbor absent because the world excludes it
// and one absent because the ACL hid it must look the same, or the response
// becomes an oracle for whichever is the rarer cause.
func worldEdgesForWire(
	edges []*entityPkg.Relation, selfID string,
	heads map[string]*entityPkg.Entity, visible map[string]bool,
) (outgoing, incoming []*entityPkg.Relation) {
	for _, edge := range edges {
		switch {
		case edge.From == selfID:
			if headOnWire(edge.To, heads, visible) {
				outgoing = append(outgoing, edge)
			}
		case edge.To == selfID:
			if headOnWire(edge.From, heads, visible) {
				incoming = append(incoming, edge)
			}
		default:
			// An edge naming neither endpoint cannot be attributed to a
			// direction for this entity. Logged rather than guessed: a
			// silent drop here would look like a world exclusion.
			slog.Warn("dataentry: world neighbors: edge names neither endpoint; dropped",
				"entity", selfID, "from", edge.From, "type", edge.Type, "to", edge.To)
		}
	}
	return outgoing, incoming
}

// headOnWire reports whether a neighbor id may be emitted: the world must
// resolve it to a face AND the principal must be permitted to read it.
//
// # The heads check is defense in depth, and saying so is the point
//
// On the production paths the `visible` set is BUILT FROM `heads`
// ([visibleWorldNeighbors] gates the resolved faces), so an id the world
// excluded is already absent from `visible` and the first condition is
// redundant there. Removing it changes nothing today — verified by mutation:
// deleting this check leaves every test in this package green.
//
// It stays for two reasons. It makes the function total over any
// (heads, visible) pair rather than correct only for the pair the current
// callers happen to build, which is what lets
// TestWorldEdgesForWire_DropsUnresolvedAndHidden pin all four combinations
// directly. And the redundancy runs in the SAFE direction: the failure it
// guards against is emitting a link to a face this world excluded, and the
// cost of keeping it is one map lookup.
//
// What it is NOT is the mechanism that implements RULING 12's exclusion rule.
// That is the world scope on [worldNeighbors.resolveHeads] — mutating THAT is
// what fails TestWorldNeighbors_ExcludedHeadIsAbsent. A future reader tracing
// the exclusion should look there, not here.
func headOnWire(id string, heads map[string]*entityPkg.Entity, visible map[string]bool) bool {
	if _, resolved := heads[id]; !resolved {
		return false
	}
	return visible[id]
}

// resolvedFromStoreFace rebuilds a [worldreader.Resolved] from a face the
// STORE already resolved.
//
// The entity on these handlers does not come from a worldreader.Resolver — it
// comes from the store path (store.EntityQuery.World), which is the second of
// the two resolution mechanisms worldreader's package doc names. So the
// Resolved handed to Neighbors is reconstructed rather than produced, and
// exactly ONE field of it is load-bearing here: Pointer, the coordinate the
// returned row was stored at, which Neighbors uses as the content-edge tail.
//
// Via is filled for completeness and is NOT consulted by Neighbors — it
// labels provenance for callers that render a badge. It is computed by the
// same total mapping [worldProvenance] uses, so a Resolved built here and a
// `_world` block on the same response cannot disagree.
//
// Found is true unconditionally: this is only ever called with a face the
// store actually returned. A world that excluded the entity produces no
// entity at all, and the handler has already rendered a 404 by then.
func resolvedFromStoreFace(ctx context.Context, e *entityPkg.Entity) worldreader.Resolved {
	return worldreader.Resolved{
		Entity:  e,
		Pointer: e.Pointer,
		Via:     ruleFromName(resolutionRule(worldScopeFrom(ctx), e.Type, e.Pointer)),
		Found:   true,
	}
}

// ruleFromName maps the wire vocabulary back to [worldreader.Rule].
//
// The two vocabularies are deliberately the same strings (see the
// resolutionRule constants in world.go), so this is a lookup rather than a
// translation. RuleUnscoped is the default arm because it is the rule for a
// type the world applies no resolution to — the same direction
// resolutionRule takes for an unknown type, and the one that keeps a
// pointerless project unchanged.
func ruleFromName(name string) worldreader.Rule {
	switch name {
	case ruleChain:
		return worldreader.RuleChain
	case ruleFallbackDefault:
		return worldreader.RuleFallbackDefault
	default:
		return worldreader.RuleUnscoped
	}
}

// worldOutgoingForEntity resolves one entity's OUTGOING links under the
// request's world, returning the edges plus the gated neighbor-id set the
// serializer filters against.
//
// Outgoing only, matching what a per-entity response has always carried:
// incoming edges reach the SPA through the dedicated /relations endpoint,
// which is not a world-capable route.
//
// Returns (nil, nil, nil) when no world-neighbor capability is wired. That is
// the pre-item-4 behavior — a world-bound response with no relations — and it
// is reachable only in a deployment that never called [SetWorldNeighbors],
// not through any request path.
//
// A package FUNCTION taking its three seams explicitly, not an App method:
// App is pinned at its plimsoll method cap, and every world helper in this
// package has taken the same shape for that reason. It also reads better —
// the seams a world-scoped link read depends on are named in the signature
// rather than reached through a god object.
func worldOutgoingForEntity(
	ctx context.Context, wn *worldNeighbors, visReader visibleReader, e *entityPkg.Entity,
) (outgoing []*entityPkg.Relation, visible map[string]bool, err error) {
	if wn == nil {
		return nil, nil, nil
	}
	edges, heads, err := wn.worldScopedNeighbors(
		ctx, resolvedFromStoreFace(ctx, e), store.DirectionOutgoing)
	if err != nil {
		return nil, nil, err
	}
	// World FIRST, gate SECOND. See the worldNeighbors type doc: the reverse
	// order makes the served result set depend on what the ACL denied, which
	// is the existence oracle guard rule 1 exists to close.
	visible = visibleWorldNeighbors(ctx, visReader, heads)
	outgoing, _ = worldEdgesForWire(edges, e.ID, heads, visible)
	return outgoing, visible, nil
}

// worldNeighborsForPage resolves the links of a whole list page under the
// request's world, returning per-row outgoing/incoming edge slices plus the
// one gated neighbor-id set the serializer filters every row against.
//
// # Why the page is one batch, not a loop of single-entity calls
//
// The obvious implementation — call worldOutgoingForEntity per row — would
// issue one head-resolution query AND one ACL gate pass per row, turning a
// 50-row page into 50 of each. That is the RR-FRK1 shape the default-world
// path already avoids by collecting the page's neighbor ids before gating,
// and the world path has no reason to be worse. So: edges per row, then ONE
// head resolution and ONE gate pass over the union.
//
// # What is NOT batched, stated plainly
//
// The EDGE queries are still per row, and [worldreader.RelationReader.Neighbors]
// issues two apiece (identity-tail and content-tail), so a 50-row page costs
// 100 relation queries. That is not a regression — the default-world path also
// queries per row (outgoingRelations + incomingRelations) — but it is not an
// improvement either, and the doc should not imply the whole thing is one
// round-trip.
//
// It is not batchable through Neighbors as it stands: the content-tail query
// filters on the row's OWN resolved pointer, so fifty rows can carry fifty
// different tails, and store.RelationQuery has no multi-endpoint selector to
// express that in one shot. Widening it is a store-layer change (and DOFYR1's
// FromPointer contract is deliberately frozen), so it belongs in its own
// ticket rather than being smuggled in here.
//
// Rows are matched to their edges positionally, so the returned slices are
// index-aligned with entities — a row with no links keeps a nil entry rather
// than shifting its neighbors onto another row.
//
// Both directions, unlike the single-entity GET: list rows carry incoming
// edges too, keyed by the relation's inverse name, so a `direction: incoming`
// relation column has a wire key to resolve against.
func worldNeighborsForPage(
	ctx context.Context, wn *worldNeighbors, visReader visibleReader,
	entities []*entityPkg.Entity,
) (outgoing, incoming [][]*entityPkg.Relation, visible map[string]bool, err error) {
	outgoing = make([][]*entityPkg.Relation, len(entities))
	incoming = make([][]*entityPkg.Relation, len(entities))
	if wn == nil {
		return outgoing, incoming, nil, nil
	}

	// Pass 1: each row's edges, under that row's own resolved face.
	edgesByRow := make([][]*entityPkg.Relation, len(entities))
	var headIDs []string
	seen := make(map[string]struct{})
	for i, e := range entities {
		edges, nerr := wn.relations.Neighbors(
			ctx, resolvedFromStoreFace(ctx, e), store.DirectionBoth)
		if nerr != nil {
			return nil, nil, nil, fmt.Errorf("world neighbors for %s: %w", e.ID, nerr)
		}
		edgesByRow[i] = edges
		for _, id := range headIDsOf(edges, e.ID) {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			headIDs = append(headIDs, id)
		}
	}

	// Pass 2: ONE world resolution over the page's whole neighbor set, then
	// ONE gate pass. World first, gate second — see the worldNeighbors doc.
	var heads map[string]*entityPkg.Entity
	if len(headIDs) > 0 {
		heads, err = wn.resolveHeads(ctx, headIDs)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	visible = visibleWorldNeighbors(ctx, visReader, heads)

	// Pass 3: split each row's edges by direction, dropping heads this world
	// does not resolve and heads the principal may not read.
	for i, e := range entities {
		outgoing[i], incoming[i] = worldEdgesForWire(edgesByRow[i], e.ID, heads, visible)
	}
	return outgoing, incoming, visible, nil
}

// includeCandidates collects the neighbor entities an `?include=` expression
// names, resolved through the request's world.
//
// Returns the candidates (UNGATED — the caller gates them) and the nested
// include expression to recurse with per candidate id.
//
// Two collection paths, chosen by world, sharing one expression parser:
//
//   - DEFAULT world: the ungated reader, exactly as before.
//   - NON-DEFAULT world: the world-scoped seam, so a candidate is that
//     world's face of the neighbor. A neighbor the world excludes never
//     becomes a candidate, which is what makes an ISMS published view's
//     `include` agree with its `relations` map instead of listing a peer the
//     relations map dropped.
//
// The agreement between `relations` and `included` is the invariant worth
// naming: RR-HJV8CP made them agree under the ACL gate, and a world that
// filtered one but not the other would break it again from the other side.
func includeCandidates(
	ctx context.Context, reader entityReader, wn *worldNeighbors,
	e *entityPkg.Entity, includes string,
) (candidates []*entityPkg.Entity, nestedFor map[string]string, err error) {
	wanted, all := parseIncludeSpec(includes)
	if worldScopeFrom(ctx).IsDefaultWorld() {
		candidates, nestedFor = defaultWorldCandidates(ctx, reader, e, wanted, all)
		return candidates, nestedFor, nil
	}
	return worldCandidates(ctx, wn, e, wanted, all)
}

// defaultWorldCandidates collects include peers through the ungated reader —
// the pre-item-4 path, byte-identical.
//
// `all` (the `*` form) pulls INCOMING peers too, so a list row's
// inverse-keyed relation columns can resolve their sources. A NAMED include
// stays outgoing-only, which is the pre-existing contract: `include=blocks`
// means "the things this blocks".
func defaultWorldCandidates(
	ctx context.Context, reader entityReader, e *entityPkg.Entity,
	wanted map[string]string, all bool,
) (candidates []*entityPkg.Entity, nestedFor map[string]string) {
	nestedFor = make(map[string]string)
	if !worldScopeFrom(ctx).IsDefaultWorld() {
		// Unreachable through includeCandidates, which branches on exactly
		// this. Restated HERE because the reader below is default-world-only
		// and this function is now the one that touches it: a defense that
		// lives only in the caller is one refactor away from being bypassed,
		// and the failure would be a published entity wrapped in draft peers.
		// TestWorldCapableRoutesDoNotUseUngatedReader requires it too — it
		// caught the absence of this check when the collection loop was
		// extracted out of includeCandidates, which is the guard working.
		return nil, nestedFor
	}
	for _, edge := range reader.outgoingRelations(ctx, e.ID) {
		nested, want := wanted[edge.Type]
		if !all && !want {
			continue
		}
		target, found := reader.getEntity(ctx, edge.To)
		if !found {
			continue
		}
		candidates = append(candidates, target)
		if nested != "" {
			nestedFor[target.ID] = nested
		}
	}
	if !all {
		return candidates, nestedFor
	}
	for _, edge := range reader.incomingRelations(ctx, e.ID) {
		if source, found := reader.getEntity(ctx, edge.From); found {
			candidates = append(candidates, source)
		}
	}
	return candidates, nestedFor
}

// worldCandidates collects include peers through the world-scoped seam, so a
// candidate is THIS world's face of the neighbor and a neighbor the world
// excludes never becomes one.
//
// That exclusion is what keeps `included` agreeing with the `relations` map:
// both reach the same verdict from the same resolution, rather than one being
// filtered and the other not (the RR-HJV8CP invariant, now spanning the world
// filter as well as the ACL one).
func worldCandidates(
	ctx context.Context, wn *worldNeighbors, e *entityPkg.Entity,
	wanted map[string]string, all bool,
) (candidates []*entityPkg.Entity, nestedFor map[string]string, err error) {
	nestedFor = make(map[string]string)
	if wn == nil {
		return nil, nestedFor, nil
	}
	dir := store.DirectionOutgoing
	if all {
		dir = store.DirectionBoth
	}
	edges, heads, err := wn.worldScopedNeighbors(
		ctx, resolvedFromStoreFace(ctx, e), dir)
	if err != nil {
		return nil, nil, err
	}
	for _, edge := range edges {
		nested, want := wanted[edge.Type]
		if !all && !want {
			continue
		}
		head, resolved := heads[peerOf(edge, e.ID)]
		if !resolved {
			// No face in this world — not a candidate. Same verdict
			// worldEdgesForWire reaches for the relations map.
			continue
		}
		candidates = append(candidates, head)
		if nested != "" {
			nestedFor[head.ID] = nested
		}
	}
	return candidates, nestedFor, nil
}

// peerOf returns the endpoint of edge that is not selfID.
//
// Derived rather than assumed to be edge.To, because a DirectionBoth query
// returns edges naming the entry on either side. A self-edge yields selfID,
// which resolves to the entry's own already-resolved face and is harmless.
func peerOf(edge *entityPkg.Relation, selfID string) string {
	if edge.To == selfID {
		return edge.From
	}
	return edge.To
}

// parseIncludeSpec splits an `?include=` expression into the relation types
// it names and the nested expression under each.
//
// Returns (wanted, all). `all` is the `*` form, which takes every relation
// type and both directions. Otherwise `wanted` maps a relation type to the
// remainder after the first dot ("implements.requires" → wanted["implements"]
// = "requires"), or "" when there is no nesting.
//
// A duplicate relation type keeps the FIRST nested expression rather than the
// last, matching the pre-existing loop order — `include=a.b,a.c` was never a
// documented form, and silently switching which one wins would be a behavior
// change smuggled in under a refactor.
//
// One deliberate divergence from the pre-refactor loop, recorded because it
// IS a difference even though nothing observes it: a TRAILING DOT
// (`include=implements.`) used to set an empty nested expression, which
// recursed once and returned an empty map (the recursion splits "" into a
// single empty part and skips it). Here an empty remainder simply records no
// nesting, so the pointless recursion does not happen. The `included` map is
// identical either way; only the wasted call is gone.
func parseIncludeSpec(includes string) (wanted map[string]string, all bool) {
	if strings.TrimSpace(includes) == "*" {
		return nil, true
	}
	wanted = make(map[string]string)
	for part := range strings.SplitSeq(includes, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		relType, nested, _ := strings.Cut(part, ".")
		if _, dup := wanted[relType]; dup {
			continue
		}
		wanted[relType] = nested
	}
	return wanted, false
}
