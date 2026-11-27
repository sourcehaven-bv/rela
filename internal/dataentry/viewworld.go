package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// viewWorld is the world a view executes in, passed EXPLICITLY to
// [viewsHandler.executeView] rather than read from the request context
// (TKT-WRLDAPI item 4b).
//
// # Why a parameter and not ctx
//
// executeView is a shared engine with three callers, and only ONE of them is
// world-capable:
//
//   - `GET /_views/{type}/{id}` — world-capable (this ticket).
//   - `_sidepanel`, via executeSidePanel — NOT yet scoped.
//   - the command runner's `kind: view` — NOT yet scoped, and the most
//     consequential: `commands.go` marshals the viewResult to JSON and pipes
//     it to an operator shell script's stdin. A world applied there changes
//     what an EXTERNAL PROCESS receives, past the point where anything in this
//     codebase can observe that a world was involved at all.
//
// If executeView read the world off ctx, the moment a request carrying
// `?world=` reached the command path the engine would silently world-resolve
// its output — the two unscoped surfaces would inherit world-awareness rather
// than opting into it. That is the difference between an allowlist and an
// accident, and it is the same default-deny discipline [worldCapablePath]
// applies at the route table, moved one layer in.
//
// So the world is a value a caller must NAME. `defaultViewWorld()` is spelled
// out at the two unscoped call sites, so a reader sees a choice rather than an
// omission, and `TestExecuteViewCallersDeclareTheirWorld` fails if a new
// caller appears that does not make one.
//
// # The zero value is unset
//
// A zero viewWorld carries an unset scope, which every read refuses with
// store.ErrInvalidQuery (TKT-7IZHP0 design A4). Build one with
// defaultViewWorld or from the request's world handle; the trivial scope
// resolves every entity to its implicit face, as `store.GetEntity` did.
type viewWorld struct {
	// name is the declared world name, for provenance labeling. Empty means
	// the default world.
	name string
	// scope is the compiled resolution this world applies. It is always set;
	// the default world's is the trivial scope.
	scope store.WorldScope
	// denied marks a declared world this principal may not read. The handle's
	// scope is the default world's, so without this bit a denied world would
	// resolve as the default one and serve the default face's view — which
	// both discloses the denial (a permitted empty world answers
	// `_world_absent`) and shows content under a world the caller was refused.
	denied bool
}

// defaultViewWorld returns the world a surface that has not been scoped for
// worlds must use.
//
// Spelled as a named constructor rather than a bare `viewWorld{}` so the call
// sites read as a decision. `executeView(ctx, cfg, id, defaultViewWorld())`
// says "this surface serves the default world"; `executeView(ctx, cfg, id,
// viewWorld{})` says nothing, and a reader cannot tell whether the author
// considered worlds at all.
func defaultViewWorld() viewWorld { return viewWorld(defaultWorldHandle()) }

// viewWorldFromRequest builds the world for a world-capable view route.
//
// This is the ONLY constructor that reads the request's world. Keeping it
// distinct from [defaultViewWorld] is what makes the two populations legible:
// a grep for this function finds every surface that has opted in.
func viewWorldFromRequest(ctx context.Context) viewWorld {
	h := worldFromContext(ctx)
	return viewWorld(h)
}

// ranksNothing reports whether w reads every entity at its implicit face;
// see worldHandle.ranksNothing.
func (w viewWorld) ranksNothing() bool { return !w.denied && w.scope.IsTrivial() }

// viewEntry resolves the view's ENTRY entity to its face in world w through
// the same visibility.Resolver the entity GET uses, read as the view's type.
// The default world missing the id returns [errViewEntryNotFound]; a
// non-default world resolving nothing returns [errViewEntryNoFaceHere], which
// [viewsHandler.writeWorldAbsentView] turns into an answer. This discloses
// nothing: handleV1Views has already run the world-independent row gate, so
// existence is settled before this runs.
func (h *viewsHandler) viewEntry(
	ctx context.Context, entityType string, entry entityPkg.Ref, w viewWorld,
) (*entityPkg.Entity, error) {
	if w.denied {
		return nil, errNoFaceInWorld
	}
	world := worldHandle(w).visibility()
	if w.ranksNothing() || !entry.Face.IsImplicit() {
		e, ok, err := h.visible.refIn(ctx, world, entityType, entry)
		if err != nil {
			return nil, &gateFaultError{err: err}
		}
		if !ok {
			return nil, errViewEntryNotFound(entry.String())
		}
		return e, nil
	}
	e, ok, err := h.visible.inWorldOf(ctx, world, entityType, entry.ID)
	if err != nil {
		return nil, &gateFaultError{err: err}
	}
	if !ok {
		return nil, errViewEntryNoFaceHere(entry.ID)
	}
	return e, nil
}

// errViewEntryNoFaceHere marks an entry that EXISTS but has no face in the
// requested world — the ordinary state of an unpublished draft, not an error.
//
// A distinct sentinel rather than a flag on the not-found error, so the
// handler's branch is a type-directed `errors.Is` rather than a string match,
// and so a caller that forgets to handle it degrades to the old 422 rather than
// to a silent 200.
func errViewEntryNoFaceHere(entryID string) error {
	return fmt.Errorf("%w: %s", errNoFaceInWorld, entryID)
}

// errNoFaceInWorld is the sentinel [errViewEntryNoFaceHere] wraps. Spelled
// separately so handlers match on identity, not on message text.
var errNoFaceInWorld = errors.New("entry entity has no face in this world")

// errViewEntryNotFound is the entry-missing error, spelled once so the
// default-world and world paths cannot drift in wording. The `_views` route
// answers it with the uniform 404, never with this message.
func errViewEntryNotFound(entryID string) error {
	return fmt.Errorf("%w: %s", errEntryMissing, entryID)
}

// errEntryMissing is the sentinel [errViewEntryNotFound] wraps, so a handler
// can tell a missing entry from other view failures by identity.
var errEntryMissing = errors.New("entry entity not found")

// loadViewEntities resolves neighbor ids to their faces in world w in one
// batch, preserving the caller's id order and collapsing duplicates. An id
// that resolves to no face is dropped silently. A store fault is logged, and
// the first fault of the load or the gate is returned beside the partial
// result.
func (h *viewsHandler) loadViewEntities(
	ctx context.Context, ids []string, w viewWorld,
) ([]*entityPkg.Entity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var fault error
	byID := make(map[string]*entityPkg.Entity, len(ids))

	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	for hdr, err := range store.ListEntityHeaders(ctx, h.store, store.EntityQuery{
		IDs:   unique,
		Faces: store.InWorld(w.scope),
	}) {
		if err != nil {
			slog.Warn("dataentry: view traversal: loading collected entities failed; "+
				"collection truncated",
				"world", w.name, "ids", len(unique), "err", err)
			fault = err
			break
		}
		byID[hdr.ID] = headerEntity(hdr)
	}

	if err := h.gateLoadedEntities(ctx, ids, byID); err != nil && fault == nil {
		fault = err
	}

	out := make([]*entityPkg.Entity, 0, len(byID))
	emitted := make(map[string]struct{}, len(byID))
	for _, id := range ids {
		e, ok := byID[id]
		if !ok {
			continue
		}
		if _, dup := emitted[id]; dup {
			continue
		}
		emitted[id] = struct{}{}
		out = append(out, e)
	}
	return out, fault
}

// gateLoadedEntities deletes from byID every entity the principal may not
// read. The frontier gate, [viewsHandler.readableViewIDs], is the one that
// closes reachability; this one covers the non-recursive path and keeps
// hidden entities out of collections and where: filters. Each entity is kept
// only when the row verdict holds on its face and [faceReadable] allows it.
// It fails closed: a probe error drops the whole type, with a log.
func (h *viewsHandler) gateLoadedEntities(
	ctx context.Context, ids []string, byID map[string]*entityPkg.Entity,
) error {
	if len(byID) == 0 {
		return nil
	}
	var fault error
	idsByType := make(map[string][]string, 4)
	for _, id := range ids {
		if e, ok := byID[id]; ok {
			idsByType[e.Type] = append(idsByType[e.Type], e.ID)
		}
	}
	gate := readGateFromContext(ctx)
	for typ, typeIDs := range idsByType {
		verdicts, err := gate.ReadableFacesMany(ctx, typ, typeIDs)
		if err != nil {
			slog.Warn("dataentry: view traversal: collection read gate failed; "+
				"dropping type", "type", typ, "ids", len(typeIDs), "err", err)
			for _, id := range typeIDs {
				delete(byID, id)
			}
			if fault == nil {
				fault = err
			}
			continue
		}
		for _, id := range typeIDs {
			e, ok := byID[id]
			if !ok {
				continue
			}
			if !verdicts.For(id).Contains(e.Face) || !faceReadable(ctx, e.Type, e.Face) {
				delete(byID, id)
			}
		}
	}
	return fault
}

// provenanceFor labels how this world resolved e's face, or nil under the
// default world. It reads back the face the store served and names the rule
// through [resolutionRuleAt], the mapping the entity GET shares, including
// the chain position. It never re-resolves.
//
// Nil: e is rejected by contract; callers hold a resolved entity.
func (w viewWorld) provenanceFor(e *entityPkg.Entity) *v1.EntityWorld {
	if w.ranksNothing() || e == nil {
		return nil
	}
	rule, position := resolutionRuleAt(w.scope, e.Type, e.Face)
	return &v1.EntityWorld{
		Name:          w.name,
		Face:          e.Face.String(),
		Via:           rule,
		ChainPosition: position,
	}
}

// writeWorldAbsentView answers a view request for an entity that exists but
// has no face in the requested world. It answers 200, since the caller was
// already cleared by the world-independent row gate, and a 4xx body is
// discarded by the SPA. It serves the entity's default face marked
// `_world_absent`, with empty sections: traversing the default face would
// mix faces under a world request. A read failure here returns the ordinary
// not-found, not an absence.
func (h *viewsHandler) writeWorldAbsentView(
	w http.ResponseWriter, r *http.Request, entityType, entityID string,
) {
	ctx := r.Context()
	e, err := h.readableFaceOf(ctx, entityType, entityID)
	if err != nil || e == nil || e.Type != entityType {
		viewMiss(w, r, h.viewReader, entityID, func() { writeViewEntryMissing(w, r) })
		return
	}

	rels, ferr := h.faceEdges.edges(ctx, e)
	if ferr != nil {
		writeGateError(w, r, ferr)
		return
	}

	sch := h.schema()
	m := sch.Meta
	def := m.Entities[e.Type]
	view := ViewConfig{Entry: ViewEntry{Type: e.Type}}
	resp := v1.ViewResponse{
		Entry:                h.serializer.forWire(ctx, e, rels, m, def.GetPlural(e.Type)),
		Sections:             []v1.ViewSection{},
		WorldAbsent:          true,
		HideMentionedByPanel: hideMentionedByPanel(sch.Cfg, view),
		HideMentionedByID:    hideMentionedByID(sch.Cfg, view),
	}
	h.rollups.add(ctx, e.Type, []rollupRow{{ref: e.Ref(), props: resp.Entry.Properties}}, worldScopeFrom(ctx), nil)
	writeV1JSON(w, http.StatusOK, resp)
}

// readableFaceOf returns one stored row of an entity that this principal may
// read, for surfaces that describe the entity rather than a content state.
// The read gate applies while choosing, so an unreadable face and a missing
// one are indistinguishable. Faces are tried in a stable order: the implicit
// face, then the declared faces.
//
// Nil: returns (nil, store.ErrNotFound) when no row is readable.
func (h *viewsHandler) readableFaceOf(
	ctx context.Context, entityType, entityID string,
) (*entityPkg.Entity, error) {
	fam, ok, err := h.visible.family(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, store.ErrNotFound
	}
	declared := h.schema().Meta.Entities[entityType].Faces
	world := defaultWorldHandle().visibility()
	for _, face := range fam.Faces {
		if _, isDeclared := declared[string(face)]; !face.IsImplicit() && !isDeclared {
			continue
		}
		e, found, rerr := h.visible.refIn(ctx, world, entityType,
			entityPkg.Ref{ID: entityID, Face: face})
		if rerr != nil {
			return nil, rerr
		}
		if found {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}
