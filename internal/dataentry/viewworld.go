package dataentry

import (
	"context"
	"fmt"
	"log/slog"

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
// # The zero value is the default world
//
// A zero viewWorld is the default world, which is what makes a pointerless
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
	return viewWorld{name: h.name, scope: h.scope}
}

// isDefault reports whether this is the default world.
func (w viewWorld) isDefault() bool { return w.scope.IsDefaultWorld() }

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
// was just cleared to read. That reasoning is unchanged by worlds — but note
// what IS changed: a world that EXCLUDES the entry yields a not-found here,
// which is correct (in this world the entity does not exist) and is not an ACL
// decision.
func (h *viewsHandler) viewEntry(
	ctx context.Context, entryID string, w viewWorld,
) (*entityPkg.Entity, error) {
	if w.isDefault() {
		e, err := h.store.GetEntity(ctx, entryID)
		if err != nil {
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
		return e, nil
	}
	return nil, errViewEntryNotFound(entryID)
}

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
// e must be non-nil; callers hold a resolved entity by construction.
func (w viewWorld) provenanceFor(e *entityPkg.Entity) *v1.EntityWorld {
	if w.isDefault() || e == nil {
		return nil
	}
	return &v1.EntityWorld{
		Name:    w.name,
		Pointer: e.Pointer.String(),
		Via:     resolutionRule(w.scope, e.Type, e.Pointer),
	}
}
