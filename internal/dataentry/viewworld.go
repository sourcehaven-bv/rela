package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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
// # The zero value is the default world
//
// A zero viewWorld is the default world, which is what makes a faceless
// project and every pre-item-4b caller behave identically. That is safe
// BECAUSE it is also inert: the zero scope resolves every entity to its
// default state, exactly as `store.GetEntity` did.
type viewWorld struct {
	// name is the declared world name, for provenance labeling. Empty means
	// the default world.
	name string
	// scope is the compiled resolution this world applies. The zero scope is
	// the default world.
	scope store.WorldScope
	// denied marks a declared world this principal may not read. The handle's
	// scope is the ZERO scope, so without this bit a denied world would
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
func defaultViewWorld() viewWorld { return viewWorld{} }

// viewWorldFromRequest builds the world for a world-capable view route.
//
// This is the ONLY constructor that reads the request's world. Keeping it
// distinct from [defaultViewWorld] is what makes the two populations legible:
// a grep for this function finds every surface that has opted in.
func viewWorldFromRequest(ctx context.Context) viewWorld {
	h := worldFromContext(ctx)
	return viewWorld(h)
}

// isDefault reports whether this is the default world. A denied world is
// never the default one, whatever its (zero) scope says.
func (w viewWorld) isDefault() bool { return !w.denied && w.scope.IsDefaultWorld() }

// viewEntry resolves the view's ENTRY entity to its face in world w.
//
// Under the default world this is exactly the previous `store.GetEntity` call,
// error text included, so every existing caller is byte-identical.
//
// Under a non-default world it goes through the same one-id ListEntities query
// [visibleReader.getWorldEntity] uses, so the BACKEND resolves the chain and
// the fallback verdict. One resolution site, not two — reimplementing the walk
// here would be a second copy of the semantics deciding which face a reader
// sees.
//
// The entry is NOT row-gated here, deliberately: the handler already gated it
// (`views_handler.go`, TKT-BNX2PN) and re-gating could 404 an entry the caller
// was just cleared to read. That reasoning is unchanged by worlds.
//
// # Two absences, not one
//
// A world that excludes the entry is NOT the same answer as "no such entity",
// and conflating them was a defect (BUG-1): with `app.default_world` set to a
// filtering world, every newly created entity became unreachable from creation
// until publication — and publishing requires opening it. The LIST path already
// drew this distinction correctly by excluding the row rather than erroring.
//
// So the two are now separate returns:
//
//   - the DEFAULT world cannot find the id -> [errViewEntryNotFound], unchanged.
//   - a non-default world resolves nothing -> [errViewEntryNoFaceHere], which
//     the handler resolves into one of two answers: a 200 carrying
//     `_world_absent` plus the entity's `_faces` when the entity exists (so the
//     page can offer the face that DOES exist), or the unchanged not-found when
//     it does not. See [viewsHandler.writeWorldAbsentView] for why that single
//     decision lives there rather than here.
//
// # Why this discloses nothing
//
// The distinction is drawn strictly AFTER the ACL row gate. `handleV1Views`
// calls `gateRead` (acl.Request.PermitsRead) before executeView ever runs, and
// that gate is world-INDEPENDENT by guard rule 1 (see the [worldreader] package
// doc). So a principal who reaches this function has already been cleared to
// read this entity id, and a caller who has not been cleared receives the
// uniform 404 from the gate and never gets here. The new answer therefore
// separates two states that are both already visible to this caller — it
// cannot become an existence oracle, because existence was settled one layer up
// by a check that does not consult the world.

func (h *viewsHandler) viewEntry(
	ctx context.Context, entry entityRef, w viewWorld,
) (*entityPkg.Entity, error) {
	entryID := entry.ID
	if w.denied {
		// Byte-identical to a permitted world in which the entity has no
		// face: `_world_absent` over the default face. That is the only
		// answer that does not disclose the denial.
		//
		// An EXPLICIT address gets the same answer: the world grant must not
		// be bypassable by spelling the face (see visibleReader.getVisibleRef).
		return nil, errNoFaceInWorld
	}
	if w.isDefault() || entry.Explicit {
		// An explicit address (`ID@face`) is served literally under every
		// world — the caller named the row, so there is nothing to resolve
		// (see entityRef). Under the default world every address is literal.
		e, err := h.store.GetEntityState(ctx, entry.ID, entry.Face)
		if err != nil {
			return nil, errViewEntryNotFound(entry.String())
		}
		// The row gate one layer up cleared the ENTITY; it says nothing about
		// which FACE this principal may read (TKT-O7R2A1). Without this the
		// view surface served a draft body to a principal granted only
		// `policy@published`, while the entity route 404'd the same face.
		//
		// Reported as the ordinary not-found so a denied face stays
		// indistinguishable from an absent one, matching
		// [visibleReader.getVisible].
		if !faceReadable(ctx, e.Type, e.Face) {
			return nil, errViewEntryNotFound(entryID)
		}
		return e, nil
	}
	for e, err := range h.store.ListEntities(ctx, store.EntityQuery{
		IDs:   []string{entryID},
		World: w.scope,
	}) {
		if err != nil {
			// An infrastructure failure is NOT "this entity has no face in
			// this world". Surfacing it keeps a backend outage from reading
			// as a correctly-empty published view (RR-4TFZNL).
			return nil, err
		}
		// A world chose this face, but a world is a VIEW, never a permission
		// (guard rule 1) — so the grant still has to cover what it resolved to.
		//
		// Deliberately NOT errViewEntryNoFaceHere: that answers with a 200
		// carrying the entity's other `_faces` so the page can offer one that
		// exists, which for a DENIED face would disclose that it exists. A
		// denial has to read as absence.
		if !faceReadable(ctx, e.Type, e.Face) {
			return nil, errViewEntryNotFound(entryID)
		}
		return e, nil
	}
	// The world resolved nothing. Whether that is a draft awaiting publication
	// or a typo depends on whether the ENTITY exists at all — a question this
	// function deliberately does not answer.
	//
	// Answering it here would mean probing the store and then probing it AGAIN
	// in [viewsHandler.writeWorldAbsentView], which must load the default face
	// to build the response anyway. Two probes can disagree (a delete landing
	// between them) and the second is the one that decides, so the first is
	// dead weight that reads as a guarantee. So this reports the world-absence
	// unconditionally and the handler, which has to do the read regardless,
	// makes the single existence decision.
	return nil, errViewEntryNoFaceHere(entryID)
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
// default-world and world paths cannot drift in wording. The message is
// preserved verbatim from before item 4b: it reaches the wire as a 422
// `view_execution_failed` detail, and callers' tests match on it.
func errViewEntryNotFound(entryID string) error {
	return fmt.Errorf("entry entity not found: %s", entryID)
}

// loadViewEntities resolves neighbor ids to their faces in world w, preserving
// the caller's id order and dropping ids that resolve to nothing.
//
// # One batch per rule application
//
// The traversal collects ids across every source and every recursion depth
// before calling this, so a rule that walks a hundred edges costs ONE
// resolution rather than a hundred. That ordering is deliberate and is why
// traverseViewOnce returns ids: the per-hop shape would multiply an N+1 by the
// 10-pass fixpoint AND by the recursive depth.
//
// # Order and duplicates
//
// Input order is preserved because the caller's dedup-into-collection step
// runs afterwards and callers observe collection order. Duplicate ids collapse
// to one entity, which is what the dedup step would have done anyway.
//
// # Absence is normal
//
// An id that resolves to no face is DROPPED, exactly as the previous
// `GetEntity`-error-skip did. Under a world that is the `otherwise: exclude`
// verdict — the neighbor has no face here — and under the default world it is
// a dangling edge. Both were silent before and stay silent, because a view is
// a presentation surface and a missing neighbor is not an error a page can act
// on.
//
// A store FAULT is different and is logged: it would otherwise read as "this
// world excludes everything", the RR-4TFZNL shape.
func (h *viewsHandler) loadViewEntities(
	ctx context.Context, ids []string, w viewWorld,
) []*entityPkg.Entity {
	if len(ids) == 0 {
		return nil
	}
	byID := make(map[string]*entityPkg.Entity, len(ids))

	if w.isDefault() {
		// Point reads, as before. The default world has no chain to resolve, so
		// a batch query would buy nothing and would change the error handling
		// of a path this ticket must leave byte-identical.
		for _, id := range ids {
			if _, seen := byID[id]; seen {
				continue
			}
			if e, err := h.store.GetEntity(ctx, id); err == nil {
				byID[id] = e
			}
		}
	} else {
		unique := make([]string, 0, len(ids))
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
		for e, err := range h.store.ListEntities(ctx, store.EntityQuery{
			IDs:   unique,
			World: w.scope,
		}) {
			if err != nil {
				slog.Warn("dataentry: view traversal: world resolution failed; "+
					"collection truncated",
					"world", w.name, "ids", len(unique), "err", err)
				break
			}
			byID[e.ID] = e
		}
	}

	out := make([]*entityPkg.Entity, 0, len(byID))
	emitted := make(map[string]struct{}, len(byID))
	for _, id := range ids {
		e, ok := byID[id]
		if !ok {
			continue // no face in this world, or a dangling edge
		}
		if _, dup := emitted[id]; dup {
			continue
		}
		emitted[id] = struct{}{}
		out = append(out, e)
	}
	return out
}

// provenanceFor labels how this world resolved e's face, or nil under the
// default world (TKT-WRLDAPI item 4b).
//
// # Labeling, not re-resolving
//
// Resolution happened once, in the store: the world scope rode the query and
// the backend picked the face. This reads the answer back — the coordinate the
// returned row was stored at — and names the rule that must have produced it.
// It never fetches and never walks the chain, so it cannot disagree with the
// store about WHICH face was served, because it is handed that face.
//
// It shares [resolutionRule] with the entity GET's `_world` block rather than
// deriving the rule again, so a view row and a GET of the same entity under
// the same world report identical provenance. Two implementations of that
// mapping would be free to drift, and the drift would be invisible — both
// would look plausible.
//
// # Nil under the default world, deliberately
//
// Every entity resolves to its default state there by definition, so a block
// on every row of every existing view would be noise — and, worse, would imply
// a world was applied when none was. The entity GET makes the opposite choice
// (it always labels, including `via: unscoped`) because a single-entity
// response has room for one block and a client asking for provenance asked
// about that entity. A collection is a different shape with a different cost.
//
// # It reports the CHAIN POSITION too
//
// `via` alone cannot say whether a row got the world's FIRST choice: under
// `select: [published, draft]` a genuine published face and a draft standing
// in for a missing one both report "chain". A view row is exactly where that
// matters — a section can mix a first-choice hit and a stand-in, and the two
// are byte-identical. So this shares [resolutionRuleAt] with the entity GET
// rather than the rule-only [resolutionRule], for the same
// one-implementation reason the paragraph above gives.
//
// # The face is DECLARED, not stored
//
// Same mapping the entity GET applies, and for the same reason — see
// [worldProvenance]. A row resolved to the type's `bare_face:` is stored at
// the zero coordinate, so reporting the raw coordinate emitted `face: ""`
// with `via: "chain"` and every consumer printed the WORLD name instead
// (TKT-PI17Z6). m may be nil, in which case the coordinate passes through.
//
// e must be non-nil; callers hold a resolved entity by construction.
func (w viewWorld) provenanceFor(m *metamodel.Metamodel, e *entityPkg.Entity) *v1.EntityWorld {
	if w.isDefault() || e == nil {
		return nil
	}
	rule, position := resolutionRuleAt(w.scope, e.Type, e.Face)
	return &v1.EntityWorld{
		Name:          w.name,
		Face:          metamodel.DeclaredFace(m, e.Type, e.Face.String()),
		Via:           rule,
		ChainPosition: position,
	}
}

// writeWorldAbsentView answers a view request for an entity that exists but
// has no face in the requested world (BUG-1).
//
// # Why 200 and not a 4xx
//
// The caller was already cleared to read this entity: `handleV1Views` runs the
// ACL row gate before view execution, and that gate is world-independent
// (guard rule 1). What is missing is not permission and not the entity — only
// this world's face of it. The useful answer is therefore "not here, and here
// is where it is", which a 4xx cannot carry: the SPA's error path discards the
// body, so a 422 leaves the user who just created something being told it does
// not exist. That was the reported failure, and it produced duplicate entities
// because the natural response to "not found" is to create it again.
//
// # What it serves
//
// The entity's DEFAULT face, which is the face that exists and the face writes
// land on — so the page has a title, a body, and (via the affordance maps) the
// `_faces` list and the copy offers that let the user promote it into the world
// they asked for. `_world_absent` marks the response so the SPA renders that
// state deliberately rather than as an ordinary page that silently lost its
// world.
//
// Sections are deliberately EMPTY. A section is a traversal through the
// requested world, and that world resolves nothing here; running the traversal
// against the default face would present default-world neighbors under a
// published-world request, which is the mixed-face serve this arc refuses
// (Ruling 3). The page has enough to offer a way through without them.
//
// A read failure here is reported as the ordinary not-found this surface has
// always returned for a missing entry, not as an absence: the entity was
// present a moment ago, so a failure now is a fault, and claiming "no face in
// this world" would launder it into a routine answer.
func (h *viewsHandler) writeWorldAbsentView(
	w http.ResponseWriter, r *http.Request, entityType, entityID string,
) {
	ctx := r.Context()
	e, err := h.store.GetEntity(ctx, entityID)
	// The face check rides the same condition as the miss, deliberately: this
	// loads the DEFAULT face to describe an entity absent from the requested
	// world, and a principal who may not read that face must not receive its
	// edges or its `_faces` list — which would disclose exactly the content
	// states the grant withholds (TKT-O7R2A1).
	if err != nil || e == nil || e.Type != entityType ||
		!faceReadable(ctx, e.Type, e.Face) {

		writeV1Error(w, r, http.StatusUnprocessableEntity, "view_execution_failed",
			"View execution failed", errViewEntryNotFound(entityID).Error())
		return
	}

	// The default face's own edges, via the same face-scoped seam every other
	// surface uses — never the bare-id reader, which returns the union of every
	// face's edges (the face-merging bug fixed in 8b476cd1).
	rels, ferr := h.faceEdges(ctx, e)
	if ferr != nil {
		writeGateError(w, r, ferr)
		return
	}

	m := h.schema().Meta
	def := m.Entities[e.Type]
	resp := v1.ViewResponse{
		Entry:           h.serializer.forWire(ctx, e, rels, m, def.GetPlural(e.Type)),
		Sections:        []v1.ViewSection{},
		WorldAbsent:     true,
		WorldAbsentName: worldFromContext(ctx).name,
	}
	writeV1JSON(w, http.StatusOK, resp)
}
