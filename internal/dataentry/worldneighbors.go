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
// world feature has added ONE method to App so far ([App.SetWorlds]) and four
// package functions (resolveWorld, attachWorld, worldCapablePath, and this),
// which is the discipline recorded on the App type doc.
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
	if len(ids) > 0 {
		heads, err = wn.resolveHeads(ctx, ids)
		if err != nil {
			return nil, nil, err
		}
	} else {
		heads = make(map[string]*entityPkg.Entity, 1)
	}

	// SEED THE ENTRY'S OWN FACE. A SELF-EDGE (from == to == this entity) names
	// the entry as its own head, and headIDsOf deliberately omits it — there
	// is nothing to look up, the face is already in hand. But `heads` is also
	// the CONSUMER'S test for "does this world resolve that link", so an
	// unseeded self id reads as EXCLUDED and the edge is dropped.
	//
	// That was a real bug, not a theoretical one: `blocks: ticket -> ticket`
	// is an ordinary shape (dependency, supersedes, related-to), and a
	// self-edge visible in the default world silently vanished under every
	// non-default one. The two functions disagreed about what an absent key
	// meant — headIDsOf said "already known", headOnWire said "unresolvable".
	// Seeding here makes them agree, at the one place that holds the face.
	heads[res.Entity.ID] = res.Entity
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
// exactly ONE field of it is load-bearing here: Face, the coordinate the
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
		Entity: e,
		Face:   e.Face,
		Via:    ruleFromName(resolutionRule(worldScopeFrom(ctx), e.Type, e.Face)),
		Found:  true,
	}
}

// ruleFromName maps the wire vocabulary back to [worldreader.Rule].
//
// The two vocabularies are deliberately the same strings (see the
// resolutionRule constants in world.go), so this is a lookup rather than a
// translation. RuleUnscoped is the default arm because it is the rule for a
// type the world applies no resolution to — the same direction
// resolutionRule takes for an unknown type, and the one that keeps a
// faceless project unchanged.
func ruleFromName(name string) worldreader.Rule {
	return store.ParseResolutionRule(name)
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
// filters on the row's OWN resolved face, so fifty rows can carry fifty
// different tails, and store.RelationQuery has no multi-endpoint selector to
// express that in one shot. Widening it is a store-layer change (and DOFYR1's
// FromFace contract is deliberately frozen), so it belongs in its own
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
	if heads == nil {
		heads = make(map[string]*entityPkg.Entity, len(entities))
	}
	// Seed each row's OWN face, for the same reason worldScopedNeighbors does:
	// a self-edge names its row as its own head, headIDsOf omits it, and an
	// unseeded id reads as "excluded by this world" to worldEdgesForWire. A row
	// already IS a resolved face, so this needs no query.
	//
	// This path does not call worldScopedNeighbors (it batches across rows
	// instead), so the seeding cannot be inherited and has to be restated —
	// which is precisely how the self-edge bug reached two call sites.
	for _, e := range entities {
		heads[e.ID] = e
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
	if wn == nil {
		// Unwired deployment only — no content states exist, so the bare-id
		// reader cannot merge faces. See [servedFaceEdges] for why this is a
		// wiring-shaped fallback rather than a per-request branch.
		candidates, nestedFor = defaultWorldCandidates(ctx, reader, e, wanted, all)
		return candidates, nestedFor, nil
	}
	return worldCandidates(ctx, wn, e, wanted, all)
}

// defaultWorldCandidates collects include peers through the ungated, BARE-ID
// reader. It is reachable only when world link resolution was never wired
// (see [servedFaceEdges]) — a build with no content states, where a bare-id
// query cannot merge faces.
//
// It is NOT "the default-world path". A default-world request on a wired
// deployment goes through the face-scoped seam like every other request:
// the default world is the DEFAULT FACE, not the union of all of them.
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
	if worldBoundRelations(ctx) {
		// Unreachable through includeCandidates, which only reaches this
		// function when world link resolution was never wired — and a
		// request cannot then be world-bound. Restated HERE because the
		// reader below is bare-id and this function is the one that touches
		// it: a defense living only in the caller is one refactor away from
		// being bypassed, and the failure would be a published entity
		// wrapped in draft peers.
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

// peerOf returns the endpoint of edge that is not selfID, or "" when the edge
// names selfID on neither side.
//
// Derived rather than assumed to be edge.To, because a DirectionBoth query
// returns edges naming the entry on either side.
//
// A SELF-EDGE yields selfID, and that is correct rather than merely harmless:
// worldScopedNeighbors seeds the entry's own face into `heads`, so the lookup
// finds it and the self-link is included. (An earlier revision of this comment
// called it harmless when it was not — the face was unseeded, so a self-edge
// was silently dropped under every non-default world. Fixed at the seeding
// site; recorded here because this doc asserted the opposite.)
//
// An edge naming NEITHER endpoint returns "" rather than defaulting to
// edge.To. Unreachable — the Neighbors query is endpoint-scoped — but
// worldEdgesForWire already treats that case as worth refusing, and a sibling
// that silently admitted a third party instead would be the divergence a
// reader is entitled to assume does not exist.
func peerOf(edge *entityPkg.Relation, selfID string) string {
	switch selfID {
	case edge.From:
		return edge.To
	case edge.To:
		return edge.From
	default:
		return ""
	}
}

// parseIncludeSpec splits an `?include=` expression into the relation types
// it names and the nested expression under each.
//
// Returns (wanted, all). `all` is the `*` form, which takes every relation
// type and both directions. Otherwise `wanted` maps a relation type to the
// remainder after the first dot ("implements.requires" → wanted["implements"]
// = "requires"), or "" when there is no nesting.
//
// # Two rules preserved from the loop this replaced, both verified rather than assumed
//
// A DUPLICATE relation type keeps the LAST nested expression, not the first.
// The pre-refactor inner loop wrote `nestedFor[target.ID] = relParts[1]`
// unconditionally on every pass, so a later clause overwrote an earlier one:
// `include=a.b,a.c` recursed with "c". An earlier revision of this function
// kept the FIRST and claimed in a comment that doing so "matched the
// pre-existing loop order" — it did not, and code review caught the
// contradiction. Undocumented behavior is still behavior; a refactor does not
// get to change it silently in either direction.
//
// The `*` form is matched WITHOUT trimming, likewise deliberately. The old
// code tested `includes == "*"` on the raw string, so `" * "` fell through to
// the named branch, trimmed to "*", matched no relation type, and produced an
// EMPTY include block. Trimming here would turn that stray space into a full
// expansion — including incoming peers — which is a widening of the default
// world triggered by a client's query-string builder. Whether `" * "` ought to
// mean `*` is a real question; it is not one this PR gets to answer by
// accident.
func parseIncludeSpec(includes string) (wanted map[string]string, all bool) {
	if includes == "*" {
		return nil, true
	}
	wanted = make(map[string]string)
	for part := range strings.SplitSeq(includes, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		relType, nested, _ := strings.Cut(part, ".")
		// Last wins, matching the pre-refactor unconditional map write.
		wanted[relType] = nested
	}
	return wanted, false
}

// worldBoundRelations reports whether this request's relation reads must go
// through the world-scoped seam rather than the ungated, default-world reader.
//
// # Why this is not `worldScopeFrom(ctx).IsDefaultWorld()`
//
// A DENIED world handle carries a ZERO scope (see [worldHandle]), so the raw
// scope says "default world" while the handle says otherwise. The two
// predicates therefore disagree for exactly one input, and code that mixed
// them would send a denied request down the default-world relation path while
// a sibling site sent it down the world path.
//
// That is unreachable today — getVisible and scopedSortedEntities both bail on
// blocksAllReads() before any relation code runs — so this is latent rather
// than live. It is still worth one named predicate: the alternative is three
// call sites that happen to agree, held together by an invariant enforced
// somewhere else entirely.
//
// A denied handle answers TRUE (world-bound). That is the safe direction: the
// world seam resolves nothing for a request that may read nothing, whereas the
// ungated reader would happily return default-world edges.
func worldBoundRelations(ctx context.Context) bool {
	return !worldFromContext(ctx).isDefault()
}

// etagEdges returns the outgoing edges a cache validator must hash for e
// under world w.
//
// It mirrors the branch App.handleV1GetEntity takes for the response body,
// because a validator computed from a different edge set than the body
// describes a different document — which is the whole failure mode an ETag
// exists to prevent.
//
// Errors are RETURNED, not swallowed. The tempting shortcut is to treat a
// resolution fault as "no edges", but the hash of an edge-less entity is a
// perfectly valid validator for a real document state, so a fault would mint
// a validator that a later If-None-Match matches against a body that does have
// edges. The caller folds a sentinel instead. See the call site.
func etagEdges(
	ctx context.Context, reader entityReader, wn *worldNeighbors,
	visReader visibleReader, e *entityPkg.Entity,
) ([]*entityPkg.Relation, error) {
	edges, _, err := servedFaceEdges(ctx, reader, wn, visReader, e)
	return edges, err
}

// servedFaceEdges returns the outgoing edges OF THE FACE BEING SERVED, in
// every world including the default one.
//
// # The rule, and why it has no second arm
//
// A response renders exactly one face of an entity, and that face's edges
// are the ones it owns: its content-scoped edges (tail == this face) plus
// the identity-scoped edges the whole entity shares. "Which face am I
// rendering" has an answer whether or not a world was named, so the rule is
// unconditional and the branch that used to stand here is gone.
//
// # The bug this replaces
//
// The predecessor branched on [worldBoundRelations] and sent DEFAULT-world
// requests to `reader.outgoingRelations(ctx, e.ID)` — a query by BARE
// entity id, which matches every face's tail at once. An entity with a
// draft and a published face therefore returned the UNION of both edge
// sets on its draft, with any shared target duplicated. It presented as a
// write bug ("editing the draft changed published") even though storage was
// correct: the read was merging faces that the write had kept apart.
//
// The mistake was the QUESTION. `worldBoundRelations` asks "is this request
// world-bound", when the edge set depends on which face is being served —
// and the default world is not "all faces", it is the DEFAULT FACE. Keeping
// the pre-worlds path "byte-identical" was safe only while entities had one
// face each; the moment they have several it is a correctness bug, and this
// is the third defect in this epic from that same assumption.
//
// # Why the world seam serves the default world too
//
// [worldreader.RelationReader.Neighbors] already implements exactly this
// rule — two queries, one nil-tail for identity edges and one filtered to
// the face's own face for content edges — and it is face-parametric, not
// world-parametric. Handing it the default face is the same operation as
// handing it a published one, so reusing it keeps ONE implementation of the
// identity-vs-content dispatch rather than growing a second, subtly
// different copy on the default path.
//
// # The one surviving fallback
//
// wn is nil only in a deployment that never called [SetWorldNeighbors]. Such
// a build has no content states wired at all, so a bare-id query cannot
// merge faces and the pre-worlds reader remains exactly right. That is a
// wiring-shaped fallback, not a per-request one.
//
// Returns the edges plus the gated neighbor-id set (nil on the unwired
// path, where the caller's pre-existing gating applies unchanged).
func servedFaceEdges(
	ctx context.Context, reader entityReader, wn *worldNeighbors,
	visReader visibleReader, e *entityPkg.Entity,
) (outgoing []*entityPkg.Relation, visible map[string]bool, err error) {
	if wn == nil {
		if !worldScopeFrom(ctx).IsDefaultWorld() {
			// A world-bound request on a build with no neighbor wiring: the
			// bare-id read would return the UNION of every face's edges
			// beside a world-resolved entry — the mixed-face response this
			// whole file exists to prevent. Nothing is the honest answer.
			return nil, nil, nil
		}
		return reader.outgoingRelations(ctx, e.ID), nil, nil
	}
	return worldOutgoingForEntity(ctx, wn, visReader, e)
}

// servedFacePageEdges is [servedFaceEdges] for a whole list page: each row's
// edges come from THAT ROW'S OWN FACE, in every world including the default
// one.
//
// Both directions, unlike the single-entity GET — list rows carry incoming
// edges keyed by the relation's inverse name so a `direction: incoming`
// relation column has a wire key to resolve against.
//
// The returned slices are index-aligned with entities: a row with no links
// keeps a nil entry rather than shifting its neighbors onto another row.
//
// The nil-wn arm is the same wiring-shaped fallback [servedFaceEdges]
// documents — a deployment with no content states, where a bare-id query
// cannot merge faces. It gates the page's neighbor ids in ONE type-batched
// pass (RR-HJV8CP + RR-FRK1) rather than per row: a neighbor id may appear in
// a row's relations map only if its entity is visible to the caller, so
// `relations` and the visibility-filtered `included` map can never disagree.
// Outgoing targets and incoming sources are gated together — an entity's
// visibility is direction-independent.
func servedFacePageEdges(
	ctx context.Context, reader entityReader, wn *worldNeighbors,
	visReader visibleReader, entities []*entityPkg.Entity,
) (outgoing, incoming [][]*entityPkg.Relation, visible map[string]bool, err error) {
	if wn != nil {
		return worldNeighborsForPage(ctx, wn, visReader, entities)
	}
	outgoing = make([][]*entityPkg.Relation, len(entities))
	incoming = make([][]*entityPkg.Relation, len(entities))
	var neighborIDs []string
	for i, e := range entities {
		outgoing[i] = reader.outgoingRelations(ctx, e.ID)
		incoming[i] = reader.incomingRelations(ctx, e.ID)
		neighborIDs = append(neighborIDs, neighborIDsOf(outgoing[i], incoming[i])...)
	}
	return outgoing, incoming, visibleRelationIDs(ctx, reader, visReader, neighborIDs), nil
}
