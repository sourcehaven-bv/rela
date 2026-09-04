package dataentry

import (
	"context"
	"errors"
	"log/slog"

	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// visibleReader is the ACL-bounded entity-read seam for the data-entry
// handlers. It composes a raw [store.Store] with the per-request read gate
// ([readGate]) so that every read it exposes is filtered by the principal's
// read-ACL. It is the entity-read analog of [search.VisibleSearcher]: the
// gate produces a per-type scope/verdict, and the reader consumes (store
// handle + verdict) to emit only visible rows — never a raw, ungated entity.
//
// Why a dedicated type rather than ad-hoc gate calls at each handler: the read
// gate already exists ([readGateFromContext]) but is applied by *convention* —
// a handler can reach `a.store.GetEntity` directly and forget to gate. That
// "gate by convention" is the read-ACL bug class (TKT-N26KLB, #1010). A type
// that holds the store privately and exposes only gated reads makes the gating
// structural: a consumer that takes a visibleReader cannot bypass it.
//
// The gate is resolved from the request context at call time (not held on the
// struct) so it keeps the per-request binding that [attachACLRequest] sets up.
// Under no ACL, [readGateFromContext] returns the permit-all [nopReadGate], so
// behavior is byte-identical to the pre-ACL path.
//
// Scope (TKT-N26KLB M5.0b): this type absorbs the *already-gated* read paths
// (single-entity GET, list-with-verdict, include-filtering). Ungated reads that
// the audit surfaced (e.g. nav badge counts, BUG-ZM7SBI) are deliberately NOT
// migrated here yet — closing those changes ACL behavior and is tracked
// separately.
type visibleReader struct {
	store store.Store
}

// newVisibleReader constructs a visibleReader over s. s must be non-nil; the
// data-entry composition root always has a store, so a nil here is a wiring
// bug, not a runtime condition.
func newVisibleReader(s store.Store) visibleReader {
	return visibleReader{store: s}
}

// getVisible looks up an entity by ID, applying the read gate FIRST so a
// hidden id and a nonexistent id are indistinguishable (same MatchingIDs
// roundtrip, no existence side channel — the RR-NGMI invariant). It returns:
//
//   - (entity, true, nil)  — readable and present
//   - (nil, false, nil)    — denied OR absent (caller cannot tell which, by design)
//   - (nil, false, err)    — the gate itself failed (caller surfaces via writeGateError)
//
// This mirrors the gate-then-read ordering of the former gateReadOrNotFound +
// getEntity pair: callers translate (nil,false,nil) into the same not-found
// wire response a genuinely-missing entity produces.
func (vr visibleReader) getVisible(ctx context.Context, entityType, id string) (*entitypkg.Entity, bool, error) {
	if worldFromContext(ctx).blocksAllReads() {
		// Same not-found a genuine miss produces — the caller cannot tell a
		// denied world from an entity with no face in this one.
		return nil, false, nil
	}
	ok, err := readGateFromContext(ctx).PermitsRead(ctx, entityType, id)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	e, gerr := vr.getWorldEntity(ctx, entityType, id)
	if gerr != nil && !errors.Is(gerr, errWorldEntityAbsent) &&
		!worldScopeFrom(ctx).IsDefaultWorld() {
		// On the WORLD path the underlying read is a ListEntities iterator,
		// whose error is always an infrastructure failure — a genuine miss
		// is errWorldEntityAbsent. Folding it into not-found would render a
		// backend outage as "this entity has no published face", which is
		// the same mistake resolveWorld refuses to make one file over
		// (RR-4TFZNL). The default-world branch keeps GetEntity's inherited
		// miss-is-not-found contract unchanged.
		return nil, false, gerr
	}
	if gerr != nil {
		// A store miss is treated as not-found, matching the former
		// App.getEntity contract (err -> not found). The store error is
		// deliberately not surfaced as the gate error: callers map
		// (nil,false,nil) to the same indistinguishable 404 a real miss
		// produces. Only a *gate* error (above) is propagated.
		return nil, false, nil //nolint:nilerr // store miss == not-found, by design
	}
	// The FACE gate (TKT-O7R2A1). The row gate above decided WHICH entities
	// this principal reads; this decides which of their content states.
	//
	// It consults the SAME ReadQueryResult.Faces the list path pushes into
	// its query — one source, two consumers. Deriving it independently here
	// is how the two paths come to disagree about which faces exist, which a
	// parity test pins.
	//
	// A denied face returns the (nil,false,nil) a miss produces, so it is
	// indistinguishable from "this entity has no such face". Reporting a
	// distinct refusal would disclose that the face exists — the row-level
	// rule applied one level down.
	if !faceReadable(ctx, entityType, e.Face) {
		return nil, false, nil
	}
	return e, true, nil
}

// getVisibleRef is [visibleReader.getVisible] for a parsed address.
//
// A bare address takes the ordinary path: the request's world resolves it. An
// EXPLICIT address (`ID@face`, including the bare face by its declared name)
// is served literally, whatever world the request is in — the caller named
// the row, so there is nothing for the world to resolve (see [entityRef]).
//
// The gates are the same two the world path applies, in the same order: the
// face-blind row gate on the BARE id first (a suffixed string handed to a
// query-shaped policy matches nothing, which would 404 every explicit address
// under a non-wildcard grant), then the face gate on the row that came back.
// A denied face returns the (nil,false,nil) a missing one produces, so a
// `type@face` grant still withholds the existence of what it withholds.
//
// A denied WORLD blocks the read as well, even though the address does not
// use the world: a principal who may not select `?world=editorial` must get
// the same empty answer whatever they append to the path, or the world grant
// would be bypassable by spelling the face.
func (vr visibleReader) getVisibleRef(
	ctx context.Context, entityType string, ref entityRef,
) (*entitypkg.Entity, bool, error) {
	if !ref.Explicit {
		return vr.getVisible(ctx, entityType, ref.ID)
	}
	if worldFromContext(ctx).blocksAllReads() {
		return nil, false, nil
	}
	ok, err := readGateFromContext(ctx).PermitsRead(ctx, entityType, ref.ID)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	e, gerr := vr.store.GetEntityState(ctx, ref.ID, ref.Face)
	if gerr != nil {
		// A store miss is not-found, as on the default-world branch of
		// getVisible: GetEntityState reports ErrNotFound for a missing state
		// even when sibling states exist, which is exactly "no such row".
		return nil, false, nil //nolint:nilerr // store miss == not-found, by design
	}
	if !faceReadable(ctx, entityType, e.Face) {
		return nil, false, nil
	}
	return e, true, nil
}

// faceReadable reports whether this principal's read grants cover the given
// content state.
//
// Shared rather than inlined because it is the check every read-out path owes
// an entity AFTER the row gate has cleared it: the row gate decides WHICH
// entities a principal reads, this decides which of their faces. The two are
// separate questions and answering only the first is how [TKT-O7R2A1]'s gate
// was missed on the view surface, which served a draft body to a principal
// holding `policy@published` while the entity route 404'd the same face.
//
// An empty Faces slice means "every face" — see [acl.ReadQueryResult.Faces] for
// why a bare grant widens rather than narrows.
//
// It delegates to [visibility.FaceAllowed] through the same [ctxRowGate] the
// visibility wrappers are built over, so this package holds NO second copy of
// the rule. The two exist because a handler that reads the raw store (the view
// ENTRY, which is deliberately not routed through a redacting Reader) still
// owes the entity a face verdict, and it must be the same verdict
// [visibility.PolicyReader] would reach.
func faceReadable(ctx context.Context, entityType string, face entitypkg.Face) bool {
	return visibility.FaceAllowed(ctx, ctxRowGate{}, entityType, face)
}

// getWorldEntity fetches the entity's face in the request's world.
//
// For the DEFAULT world this is exactly the previous GetEntity call — the
// zero WorldScope is the default world, so a faceless project and every
// existing caller are byte-identical.
//
// For a non-default world it goes through a one-id ListEntities query
// carrying the world scope AND the principal's face allowlist, so the
// BACKEND resolves the chain and the fallback verdict over the faces this
// reader may see — exactly the query the list path runs. That keeps ONE
// resolution site and ONE ordering: the ACL trims the candidates first, the
// world ranks what is left, so a `policy@published` reader under `select:
// [review, published]` is served the published face here just as the list
// serves it, rather than a 404 beside a listed row (the parity bug this
// closed). It also means an entity the world EXCLUDES is simply absent
// from the result, which the caller renders as the same not-found a
// genuine miss produces.
func (vr visibleReader) getWorldEntity(
	ctx context.Context, entityType, id string,
) (*entitypkg.Entity, error) {
	scope := worldScopeFrom(ctx)
	if scope.IsDefaultWorld() {
		return vr.store.GetEntity(ctx, id)
	}
	for e, err := range vr.store.ListEntities(ctx, store.EntityQuery{
		IDs:    []string{id},
		World:  scope,
		FaceIn: readGateFromContext(ctx).ReadQuery(ctx, entityType).Faces,
	}) {
		if err != nil {
			return nil, err
		}
		return e, nil
	}
	return nil, errWorldEntityAbsent
}

// errWorldEntityAbsent means the world resolved the id to no face — either
// no state matched the chain under `otherwise: exclude`, or the entity does
// not exist. The caller cannot tell the two apart, which is the point:
// existence in a world IS the publication bit (§4.1).
var errWorldEntityAbsent = errors.New("entity has no face in this world")

// filterVisible drops every candidate the principal cannot read, batching the
// gate probe by entity type — one PermitsReadMany per distinct type, turning a
// worst case of O(N) per-id probes into O(distinct-types) (RR-FRK1). Order is
// preserved and a fresh slice is returned (RR-I2SI). On a gate error for a
// type, that whole type is dropped fail-closed (RR-7TIU) — a read-ACL failure
// must never widen visibility — and logged loud so operators see the cause
// rather than a silently-empty include block.
//
// This is the extraction of the former App.filterVisibleIncludes; behavior is
// preserved, including the nil return for empty input.
func (vr visibleReader) filterVisible(ctx context.Context, candidates []*entitypkg.Entity) []*entitypkg.Entity {
	if len(candidates) == 0 {
		return nil
	}
	gate := readGateFromContext(ctx)

	byType := make(map[string][]*entitypkg.Entity)
	for _, c := range candidates {
		byType[c.Type] = append(byType[c.Type], c)
	}

	allowed := make(map[string]bool, len(candidates))
	for typeName, group := range byType {
		ids := make([]string, 0, len(group))
		for _, c := range group {
			ids = append(ids, c.ID)
		}
		perm, err := gate.PermitsReadMany(ctx, typeName, ids)
		if err != nil {
			slog.Warn("dataentry: visibleReader.filterVisible: PermitsReadMany failed; dropping type",
				"type", typeName,
				"candidates", len(ids),
				"err", err)
			continue
		}
		for id, ok := range perm {
			if ok {
				allowed[id] = true
			}
		}
	}

	// Preserve original candidate order; allocate a fresh slice. The face
	// half is owed HERE too (TKT-O7R2A1): the row gate is face-blind, and a
	// neighbor arrives as a resolved face — under a world, possibly a
	// within-chain fallback to a face a `type@face` grant withholds. Without
	// this a principal granted only `feature@published` saw a draft-only
	// neighbor's title through `?include=` while its own GET 404'd.
	out := make([]*entitypkg.Entity, 0, len(candidates))
	for _, c := range candidates {
		if allowed[c.ID] && faceReadable(ctx, c.Type, c.Face) {
			out = append(out, c)
		}
	}
	return out
}
