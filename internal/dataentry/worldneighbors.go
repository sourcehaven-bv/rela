package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
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
// attachWorld refused `?include=` on a world-bound request outright until
// TKT-WRLDAPI item 4, for that reason. Do not restore that refusal without also
// reverting this file: a bare refusal would leave the resolution below built
// and unreachable.
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
// # Ordering: the ACL trims, THEN the world ranks
//
// A head is the prime among the faces the principal may read, as for every
// other bare-id read (TKT-7IZHP0, design section 2.3): the per-face row gate
// trims each neighbor's candidate faces, then the world ranks what is left.
// A neighbor whose world-preferred face is denied therefore falls through to
// the next readable face in the chain, instead of vanishing from the links
// while the same principal can GET it, list it and find it in search.
//
// This is not the existence oracle the old world-first order guarded
// against. Only served rows come out, and a served row is one the principal
// may read, so the answer depends on what the ACL denied only among faces
// the caller cannot see. A neighbor with no readable face in this world is
// absent, indistinguishable from one the world excludes. Pinned by
// TestWorldNeighbors_DeniedPrimeFallsThrough and
// TestFaceGateParity_SurfacesAgreeUnderOneWorld.
type worldNeighbors struct {
	// relations is the world-scoped relation capability. It is a
	// *worldreader.RelationReader rather than a store handle DELIBERATELY:
	// that type exists so the identity-vs-content scope dispatch is
	// unrepresentable to omit, and taking a store here would hand this
	// package the raw nil-tail query the dispatch is written to prevent.
	relations *worldreader.RelationReader

	// heads resolves neighbor HEADS: the ACL-trimmed, world-ranked face of
	// each id (visibleReader.servedIDs), the same resolution the list and
	// the single-entity read use, so this type adds no second one.
	heads visibleReader
}

// SetWorldNeighbors enables world-scoped LINK resolution (`?world=` on a
// response's relations and `?include=`).
//
// A package-level FUNCTION rather than a method on App, for the reason the
// world code has taken this shape throughout: App carries a
// `//plimsoll:max-methods` directive pinning it at its current count, and
// the project rule is to split the type rather than raise the number. The
// world feature has added ONE method to App so far (setWorlds) and four
// package functions (resolveWorld, attachWorld, worldCapablePath, and this),
// which is the discipline recorded on the App type doc.
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
	if a.visibleReader.resolver == nil {
		return errors.New("dataentry: SetWorldNeighbors: app has no visible reader")
	}
	a.worldNeighbors = &worldNeighbors{relations: rr, heads: a.visibleReader}
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
func (wn *worldNeighbors) worldScopedNeighbors(
	ctx context.Context, res worldreader.Resolved, dir store.Direction,
) (edges []*entityPkg.Relation, heads map[string]*entityPkg.Entity, err error) {
	if !res.Found || res.Entity == nil {
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

	heads[res.Entity.ID] = res.Entity
	return edges, heads, nil
}

// resolveHeads resolves neighbor ids to the face the principal is served
// in the request's world: the ACL trims each id's faces, then the world
// ranks what is left ([visibility.Resolver.ResolveIDs]). One batched header
// read and one gate query per type, whatever the number of ids.
//
// An id absent from the result has no readable face this world serves.
// That is a normal outcome, not an error: under `otherwise: exclude` it IS
// the publication bit.
func (wn *worldNeighbors) resolveHeads(ctx context.Context, ids []string) (map[string]*entityPkg.Entity, error) {
	served, err := wn.heads.servedIDsErr(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolving neighbor heads: %w", err)
	}
	out := make(map[string]*entityPkg.Entity, len(served))
	for id, h := range served {
		out[id] = headerEntity(h)
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

// visibleWorldNeighbors returns the id set that may appear on the wire: every
// resolved head. resolveHeads already applied the per-face row gate and the
// type's face grant before the world ranked, and the seeded entry rows were
// gated when the entry was read, so a second gate pass would repeat a query
// and could not remove anything.
func visibleWorldNeighbors(heads map[string]*entityPkg.Entity) map[string]bool {
	out := make(map[string]bool, len(heads))
	for id := range heads {
		out[id] = true
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
	edges []*entityPkg.Relation, self *entityPkg.Entity,
	heads map[string]*entityPkg.Entity, visible map[string]bool,
	owns func(*entityPkg.Relation, entityPkg.Face) bool,
) (outgoing, incoming []*entityPkg.Relation) {
	for _, edge := range edges {
		switch {
		case edge.From == self.ID:
			if owns(edge, self.Face) && headOnWire(edge.To, heads, visible) {
				outgoing = append(outgoing, edge)
			}
		case edge.To == self.ID:
			if sourceServes(edge, heads, owns) && headOnWire(edge.From, heads, visible) {
				incoming = append(incoming, edge)
			}
		default:
			slog.Warn("dataentry: world neighbors: edge names neither endpoint; dropped",
				"entity", self.ID, "from", edge.From, "type", edge.Type, "to", edge.To)
		}
	}
	return outgoing, incoming
}

// sourceServes reports whether this world serves edge's SOURCE at the face
// that owns the edge. A source the world does not resolve serves nothing.
func sourceServes(
	edge *entityPkg.Relation, heads map[string]*entityPkg.Entity,
	owns func(*entityPkg.Relation, entityPkg.Face) bool,
) bool {
	src, ok := heads[edge.From]
	return ok && owns(edge, src.Face)
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
// comes from the store path (an InWorld selection on store.EntityQuery.Faces), which is the second of
// the two resolution mechanisms worldreader's package doc names. So the
// Resolved handed to Neighbors is reconstructed rather than produced, and
// exactly ONE field of it is load-bearing here: Face, the coordinate the
// returned row was stored at, which Neighbors uses as the content-edge tail.
func resolvedFromStoreFace(ctx context.Context, e *entityPkg.Entity) worldreader.Resolved {
	via, _ := worldScopeFrom(ctx).RuleAt(e.Type, e.Face)
	return worldreader.Resolved{
		Entity: e,
		Face:   e.Face,
		Via:    via,
		Found:  true,
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
func worldOutgoingForEntity(
	ctx context.Context, wn *worldNeighbors, e *entityPkg.Entity,
) (outgoing []*entityPkg.Relation, visible map[string]bool, err error) {
	if wn == nil {
		return nil, nil, nil
	}
	edges, heads, err := wn.worldScopedNeighbors(
		ctx, resolvedFromStoreFace(ctx, e), store.DirectionOutgoing)
	if err != nil {
		return nil, nil, err
	}
	visible = visibleWorldNeighbors(heads)
	outgoing, _ = worldEdgesForWire(edges, e, heads, visible, wn.relations.Owns)
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
func worldNeighborsForPage(
	ctx context.Context, wn *worldNeighbors, entities []*entityPkg.Entity,
) (outgoing, incoming [][]*entityPkg.Relation, visible map[string]bool, err error) {
	outgoing = make([][]*entityPkg.Relation, len(entities))
	incoming = make([][]*entityPkg.Relation, len(entities))
	if wn == nil {
		return outgoing, incoming, nil, nil
	}

	rows := make([]worldreader.Resolved, len(entities))
	for i, e := range entities {
		rows[i] = resolvedFromStoreFace(ctx, e)
	}
	edgesByRow, nerr := wn.relations.NeighborsForPage(ctx, rows, store.DirectionBoth)
	if nerr != nil {
		return nil, nil, nil, fmt.Errorf("world neighbors for page: %w", nerr)
	}
	var headIDs []string
	seen := make(map[string]struct{})
	for i, e := range entities {
		edges := edgesByRow[i]
		for _, id := range headIDsOf(edges, e.ID) {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			headIDs = append(headIDs, id)
		}
	}

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
	for _, e := range entities {
		heads[e.ID] = e
	}
	visible = visibleWorldNeighbors(heads)

	for i, e := range entities {
		outgoing[i], incoming[i] = worldEdgesForWire(edgesByRow[i], e, heads, visible, wn.relations.Owns)
	}
	return outgoing, incoming, visible, nil
}

// includeCandidates collects the neighbor entities an `?include=` expression
// names for a whole set of rows, resolved through the request's world.
//
// Returns the candidates (UNGATED — the caller gates them) in row order, then
// edge order, and the nested include expressions to recurse with.
//
// The rows are resolved as ONE batch (TASK-550RD): a list page with
// `include=*` used to call this once per row, which cost an edge read, a head
// resolution and a gate pass per row — thousands of queries for a few hundred
// rows. A single entity is a batch of one, so the GET and the list share the
// rule.
func includeCandidates(
	ctx context.Context, reader entityReader, wn *worldNeighbors,
	rows []*entityPkg.Entity, includes string,
) (candidates []*entityPkg.Entity, nested []includeNest, err error) {
	wanted, all := parseIncludeSpec(includes)
	if wn == nil {
		var nests includeNests
		for _, e := range rows {
			c, nestedFor := defaultWorldCandidates(ctx, reader, e, wanted, all)
			candidates = append(candidates, c...)
			nests.addRow(nestedFor)
		}
		return candidates, nests.list, nil
	}
	return worldCandidates(ctx, wn, rows, wanted, all)
}

// includeNest is one nested include to resolve: the remaining expression
// for the candidate with this id ("implements.requires" leaves "requires"
// against each implements target).
type includeNest struct {
	id, expr string
}

// includeNests collects the distinct (id, expr) pairs of a batch. Within one
// row the last relation naming a target wins, as the per-row loop did; across
// rows each row's choice is kept, since each row used to recurse on its own.
type includeNests struct {
	list []includeNest
	seen map[includeNest]struct{}
}

// addRow adds one row's nested expressions in a stable order.
func (n *includeNests) addRow(nestedFor map[string]string) {
	ids := slices.Sorted(maps.Keys(nestedFor))
	for _, id := range ids {
		k := includeNest{id: id, expr: nestedFor[id]}
		if _, dup := n.seen[k]; dup {
			continue
		}
		if n.seen == nil {
			n.seen = make(map[includeNest]struct{})
		}
		n.seen[k] = struct{}{}
		n.list = append(n.list, k)
	}
}

// defaultWorldCandidates collects include peers through the ungated, BARE-ID
// reader. It is reachable only when world link resolution was never wired
// (see [servedFaceEdges]) — a build with no content states, where a bare-id
// query cannot merge faces.
//
// It is NOT "the default-world path". A default-world request on a wired
// deployment goes through the face-scoped seam like every other request:
// the default world is the DEFAULT FACE, not the union of all of them.
func defaultWorldCandidates(
	ctx context.Context, reader entityReader, e *entityPkg.Entity,
	wanted map[string]string, all bool,
) (candidates []*entityPkg.Entity, nestedFor map[string]string) {
	nestedFor = make(map[string]string)
	if worldBoundRelations(ctx) {
		return nil, nestedFor
	}
	var outgoing, incoming []*entityPkg.Relation
	var ids []string
	for _, edge := range reader.outgoingRelations(ctx, e.ID) {
		if _, want := wanted[edge.Type]; all || want {
			outgoing = append(outgoing, edge)
			ids = append(ids, edge.To)
		}
	}
	if all {
		incoming = reader.incomingRelations(ctx, e.ID)
		for _, edge := range incoming {
			ids = append(ids, edge.From)
		}
	}
	rows := reader.defaultWorldRows(ctx, ids)
	for _, edge := range outgoing {
		target, found := rows[edge.To]
		if !found {
			continue
		}
		candidates = append(candidates, target)
		if nested := wanted[edge.Type]; nested != "" {
			nestedFor[target.ID] = nested
		}
	}
	for _, edge := range incoming {
		if source, found := rows[edge.From]; found {
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
	ctx context.Context, wn *worldNeighbors, rows []*entityPkg.Entity,
	wanted map[string]string, all bool,
) (candidates []*entityPkg.Entity, nested []includeNest, err error) {
	if wn == nil || len(rows) == 0 {
		return nil, nil, nil
	}
	dir := store.DirectionOutgoing
	if all {
		dir = store.DirectionBoth
	}
	resolved := make([]worldreader.Resolved, len(rows))
	for i, e := range rows {
		resolved[i] = resolvedFromStoreFace(ctx, e)
	}
	edgesByRow, err := wn.relations.NeighborsForPage(ctx, resolved, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("world-scoped include neighbors: %w", err)
	}
	var heads map[string]*entityPkg.Entity
	if headIDs := pageHeadIDs(rows, edgesByRow); len(headIDs) > 0 {
		if heads, err = wn.resolveHeads(ctx, headIDs); err != nil {
			return nil, nil, err
		}
	}
	var nests includeNests
	for i, e := range rows {
		rowCands, nestedFor := wn.rowCandidates(e, edgesByRow[i], heads, wanted, all)
		candidates = append(candidates, rowCands...)
		nests.addRow(nestedFor)
	}
	return candidates, nests.list, nil
}

// pageHeadIDs returns the union of the rows' neighbor ids, in first-seen
// order.
func pageHeadIDs(rows []*entityPkg.Entity, edgesByRow [][]*entityPkg.Relation) []string {
	var headIDs []string
	seen := make(map[string]struct{})
	for i, e := range rows {
		for _, id := range headIDsOf(edgesByRow[i], e.ID) {
			if _, dup := seen[id]; !dup {
				seen[id] = struct{}{}
				headIDs = append(headIDs, id)
			}
		}
	}
	return headIDs
}

// rowCandidates is one row's share of [worldCandidates]: its include
// candidates, and the nested include expression of each candidate id.
func (wn *worldNeighbors) rowCandidates(
	e *entityPkg.Entity, edges []*entityPkg.Relation, heads map[string]*entityPkg.Entity,
	wanted map[string]string, all bool,
) (candidates []*entityPkg.Entity, nestedFor map[string]string) {
	headOf := func(id string) (*entityPkg.Entity, bool) {
		if id == e.ID {
			return e, true
		}
		h, ok := heads[id]
		return h, ok
	}
	nestedFor = make(map[string]string)
	for _, edge := range edges {
		expr, want := wanted[edge.Type]
		if !all && !want {
			continue
		}
		head, ok := headOf(peerOf(edge, e.ID))
		if !ok {
			continue
		}
		if src, ok := headOf(edge.From); !ok || !wn.relations.Owns(edge, src.Face) {
			continue
		}
		candidates = append(candidates, head)
		if expr != "" {
			nestedFor[head.ID] = expr
		}
	}
	return candidates, nestedFor
}

// peerOf returns the endpoint of edge that is not selfID, or "" when the edge
// names selfID on neither side.
//
// Derived rather than assumed to be edge.To, because a DirectionBoth query
// returns edges naming the entry on either side.
//
// A SELF-EDGE yields selfID, and that is correct rather than merely harmless:
// worldScopedNeighbors seeds the entry's own face into `heads`, so the lookup
// finds it and the self-link is included.
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
		wanted[relType] = nested
	}
	return wanted, false
}

// worldBoundRelations reports whether this request's relation reads must go
// through the world-scoped seam rather than the ungated, default-world reader.
//
// # Why this is not `worldScopeFrom(ctx).IsTrivial()`
//
// A DENIED world handle carries a ZERO scope (see [worldHandle]), so the raw
// scope says "default world" while the handle says otherwise. The two
// predicates therefore disagree for exactly one input, and code that mixed
// them would send a denied request down the default-world relation path while
// a sibling site sent it down the world path.
func worldBoundRelations(ctx context.Context) bool {
	return !worldFromContext(ctx).ranksNothing()
}

// faceEdgeReader is the one gated read of the outgoing edges of the face an
// entity is ([servedFaceEdges]), bound to the App's neighbor wiring. Every
// response that serves one entity reads its edges through it: the GET, the
// create, update, attachment and restore responses, the view entry, and the
// ETag an If-Match is checked against. They cannot drift apart because
// there is one binding.
//
// neighbors is late-bound because [SetWorldNeighbors] runs after NewApp
// builds the handlers that hold this value. A captured pointer would keep
// the nil it held at construction.
//
// The zero value fails closed: with no neighbor wiring every read is an
// error, never an ungated read.
type faceEdgeReader struct {
	neighbors func() *worldNeighbors
}

// faceEdgesOf binds the face-edge read to a's neighbor wiring.
func faceEdgesOf(a *App) faceEdgeReader {
	return faceEdgeReader{neighbors: func() *worldNeighbors { return a.worldNeighbors }}
}

// read returns e's served-face edges and the neighbor ids the caller may
// see. See [servedFaceEdges].
func (f faceEdgeReader) read(
	ctx context.Context, e *entityPkg.Entity,
) ([]*entityPkg.Relation, map[string]bool, error) {
	var wn *worldNeighbors
	if f.neighbors != nil {
		wn = f.neighbors()
	}
	return servedFaceEdges(ctx, wn, e)
}

// edges is [faceEdgeReader.read] without the neighbor set, for a caller
// that hashes or renders the edges only.
//
// Errors are RETURNED, not swallowed. The tempting shortcut is to treat a
// resolution fault as "no edges", but the hash of an edge-less entity is a
// perfectly valid validator for a real document state, so a fault would mint
// a validator that a later If-None-Match matches against a body that does have
// edges. The ETag caller folds a sentinel instead (see etagUnresolved).
func (f faceEdgeReader) edges(ctx context.Context, e *entityPkg.Entity) ([]*entityPkg.Relation, error) {
	edges, _, err := f.read(ctx, e)
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
func servedFaceEdges(
	ctx context.Context, wn *worldNeighbors, e *entityPkg.Entity,
) (outgoing []*entityPkg.Relation, visible map[string]bool, err error) {
	if wn == nil {
		return nil, nil, errNeighborsUnwired
	}
	return worldOutgoingForEntity(ctx, wn, e)
}

// errNeighborsUnwired reports an App whose neighbor gate was never wired
// ([SetWorldNeighbors]); see [servedFaceEdges].
var errNeighborsUnwired = errors.New("dataentry: neighbor gate not wired (SetWorldNeighbors)")

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
func servedFacePageEdges(
	ctx context.Context, reader entityReader, wn *worldNeighbors,
	visReader visibleReader, entities []*entityPkg.Entity,
) (outgoing, incoming [][]*entityPkg.Relation, visible map[string]bool, err error) {
	if wn != nil {
		return worldNeighborsForPage(ctx, wn, entities)
	}
	outgoing, incoming = reader.pageRelations(ctx, entities)
	var neighborIDs []string
	for i := range entities {
		neighborIDs = append(neighborIDs, neighborIDsOf(outgoing[i], incoming[i])...)
	}
	return outgoing, incoming, visibleRelationIDs(ctx, visReader, neighborIDs), nil
}
