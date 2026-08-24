package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// WorldParam is the query parameter that selects a world on the read API:
// `?world=published`. Absent or empty means the DEFAULT world, bound
// explicitly at this boundary — the interior never sees "unspecified"
// (design doc §4.4).
const WorldParam = "world"

// defaultWorldName is the reserved name of the implicit default world,
// mirroring metamodel.DefaultWorldName and acl.DefaultWorldName. Spelled
// here so `?world=default` is accepted as an explicit way to ask for what
// an absent parameter already means.
const defaultWorldName = "default"

// errWorldUnknown names a `?world=` value no declared world matches. A
// CONFIG error, rendered as a named 400 — see [App.resolveWorld].
var errWorldUnknown = errors.New("no such world")

// errWorldDenied means the principal holds no read grant for a world that
// DOES exist. Rendered as an EMPTY RESULT, never a 403 — see
// [resolveWorld].
var errWorldDenied = errors.New("world not readable by this principal")

// errWorldDuplicated means the request carried more than one `?world=`.
// Rejected rather than resolved by a precedence rule nobody would remember.
var errWorldDuplicated = errors.New("duplicate world parameter")

// errWorldUnsupported names a route that cannot honor a non-default world.
// Rendered as a 422: the caller asked for something coherent that this
// surface cannot serve, and saying so is better than serving default-world
// data under a published-world request (Ruling 3).
var errWorldUnsupported = errors.New("this endpoint cannot serve a non-default world")

// WorldLookup resolves a declared world NAME to its compiled scope.
//
// Consumer-side interface: internal/dataentry may not import internal/worlds
// (arch-lint), and a store.WorldScope is metamodel-free by construction, so
// the compiled map is injected from the wiring site via [App.SetWorlds].
//
// It must FAIL CLOSED on an unknown name — returning ok=false rather than
// substituting the default world, which would silently widen a request that
// asked for a narrower view.
type WorldLookup interface {
	Lookup(name string) (store.WorldScope, bool)
}

// worldHandle is the per-request world binding: a NAME and its compiled
// SCOPE, resolved once at the top of the operation and reused for every read
// in it.
//
// Both fields are needed. The scope is what the store matches on; the name is
// what the grant was checked against and what diagnostics report — a
// store.WorldScope deliberately carries no name (internal/store cannot know
// one), so it cannot be recovered later.
type worldHandle struct {
	name  string
	scope store.WorldScope

	// denied marks a world that EXISTS but that this principal holds no
	// read grant for. The request still runs, so the handler produces its
	// ordinary response shape; the read seams simply find nothing. See the
	// errWorldDenied arm in attachWorld for why the alternative — writing a
	// synthetic empty body — leaked the denial.
	denied bool
}

// isDefault reports whether this handle is the default world. The zero
// handle is the default world, which is what makes an unstamped context and
// an explicit `?world=default` behave identically.
func (w worldHandle) isDefault() bool { return !w.denied && w.scope.IsDefaultWorld() }

// blocksAllReads reports a handle that must yield nothing at all.
func (w worldHandle) blocksAllReads() bool { return w.denied }

type worldCtxKey struct{}

// withWorld binds a resolved world to ctx for the rest of the operation.
//
// The handle is CONSTRUCTED ONCE, in middleware, and carried whole — this is
// deliberately not "a world name on ctx that each read re-resolves". §4.4
// forbids the latter: a world that travels as ambient data can be
// reinterpreted per call, and a trace that flips worlds mid-walk is
// incoherent. A fully-constructed handle cannot be reinterpreted.
//
// The key is typed and unexported so nothing outside this package can inject
// one.
func withWorld(ctx context.Context, w worldHandle) context.Context {
	return context.WithValue(ctx, worldCtxKey{}, w)
}

// worldFromContext returns the request's world handle, or the zero handle
// (the default world) when none was bound.
//
// Defaulting to the DEFAULT world rather than erroring is correct here and is
// not a fail-open: the default world is today's graph, so an unstamped
// context behaves exactly as it did before worlds existed. A non-default
// world can only ever arrive by passing the grant check in
// [resolveWorld].
func worldFromContext(ctx context.Context) worldHandle {
	w, _ := ctx.Value(worldCtxKey{}).(worldHandle)
	return w
}

// worldScopeFrom returns the store scope to stamp onto a query built from
// ctx. Spelled as its own helper so query-construction sites read as
// `q.World = worldScopeFrom(ctx)` and a reviewer can grep for the ones that
// forgot.
func worldScopeFrom(ctx context.Context) store.WorldScope {
	return worldFromContext(ctx).scope
}

// SetWorlds injects the compiled world map, enabling `?world=` selection.
//
// Until this is called the App serves the default world only and REFUSES any
// `?world=` naming something else — which is the correct posture for a
// surface whose wiring never opted in, and means a deployment that does not
// use worlds cannot accidentally acquire the parameter.
func (a *App) SetWorlds(w WorldLookup) { a.worlds = w }

// resolveWorld resolves the request's `?world=` parameter into a handle,
// applying the per-world read grant BEFORE any resolver is constructed.
//
// A free function taking the lookup rather than an App method: it needs
// exactly one collaborator, and App is at its plimsoll method cap because it
// has accreted for years — adding to it is the habit that got it there.
//
// Returns (handle, nil) on success. The two failure modes are DELIBERATELY
// DIFFERENT, and the difference is not an inconsistency to tidy away:
//
//   - UNKNOWN world -> a named 400. A world name is operator-authored
//     CONFIG (schema.yaml), and CLAUDE.md is explicit that config names are
//     not secret: naming the missing world is more useful to whoever is
//     debugging it than a uniform silence, and conceals nothing that the
//     operator's own repo does not already state.
//
//     `/api/v1/_schema` now enumerates the declared worlds (TKT-WRLDAPI), so
//     this 400 names something the same caller can already list. Note the
//     ORDER of that argument: the conclusion does not rest on the
//     enumeration. Config names are not secret whether or not this API
//     happens to serve them, and the enumeration exists because a client
//     cannot build a world selector by guessing — not to make the 400 safe.
//     An earlier revision of this comment asserted the enumeration before it
//     was built; it is recorded here as fact only now that it is one.
//
//   - Known world the principal may NOT read -> (zero handle, errWorldDenied),
//     which callers render as an EMPTY RESULT, never a 403. What a world
//     CONTAINS — and whether any given entity exists in it — is exactly the
//     secret this feature exists to keep (§4.4). A 403 here would tell the
//     caller that a world exists holding things they may not see, which is
//     the existence oracle the row-level rule forbids. Indistinguishable
//     from a world with nothing in it, by design.
//
// Do NOT "unify" these two into one response shape. They protect different
// things: one is config, the other is content.
func resolveWorld(r *http.Request, lookup WorldLookup) (worldHandle, error) {
	values := r.URL.Query()[WorldParam]
	if len(values) > 1 {
		// Get() would silently take the FIRST, so `?world=default&world=published`
		// would serve the default world under a request that also asked for
		// published — a client-side param-append bug becoming a silent
		// wrong-face serve, which is the direction this arc refuses.
		return worldHandle{}, errWorldDuplicated
	}
	name := r.URL.Query().Get(WorldParam)
	if name == "" || name == defaultWorldName {
		// The default world needs no grant beyond the ordinary read gates
		// that already run per entity: it IS today's graph.
		return worldHandle{}, nil
	}
	if lookup == nil {
		return worldHandle{}, errWorldUnknown
	}
	scope, ok := lookup.Lookup(name)
	if !ok {
		return worldHandle{}, errWorldUnknown
	}
	permitted, err := readGateFromContext(r.Context()).PermitsWorld(r.Context(), name)
	if err != nil {
		// An infrastructure failure is NOT a denial. Rendering it as an
		// empty result would hide an outage behind a page that looks like a
		// correctly-empty world, with no operator signal (RR-4TFZNL).
		return worldHandle{}, err
	}
	if !permitted {
		return worldHandle{}, errWorldDenied
	}
	return worldHandle{name: name, scope: scope}, nil
}

// # Why an allowlist rather than teaching every read path
//
// A world-bound request must never be served default-world data (Ruling 3).
// internal/dataentry reaches entity content through roughly forty store call
// sites across a dozen files — the list pipeline, the ungated entityReader
// used for relations and serialization, view traversal, document render,
// feeds, CalDAV, sync, commands. Teaching all of them in one change would be
// a large diff in which a single missed site is a silent leak, and "silence
// in the direction of serving the wrong face" is the failure this arc keeps
// hitting.
//
// So the default is DENY, and a route joins this predicate only when its
// whole read path is world-scoped and tested. That is the DEC-ZBI39P stance —
// structurally incapable rather than "defaults to safe" — applied at the
// route table: a leak requires someone to WIDEN this predicate, a visible and
// reviewable act, rather than to forget a call site.
//
// worldCapablePath reports whether path may serve a non-default world.
//
// Deliberately conservative: only the collection list (`/{plural}`) and the
// single-entity GET (`/{plural}/{id}`) qualify today, because those are the
// two paths whose reads this PR made world-scoped end to end. Every
// underscore-prefixed endpoint — search, analyze, views, documents, feeds,
// history, sync, position, next-action — is refused, along with every
// sub-resource of an entity (relations, attachments, export), because each
// reaches content through a path that is still world-blind.
//
// Refusing is not a permanent verdict on those routes; it is the honest one
// until each is scoped and tested.
func worldCapablePath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	if trimmed == path {
		// Not under the versioned API (e.g. /api/sync/...). Refuse.
		return false
	}
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "_") {
		return false
	}
	// `{plural}` or `{plural}/{id}` only. A third segment is a
	// sub-resource (relations, attachments, _export) and is refused.
	return strings.Count(trimmed, "/") <= 1 &&
		!strings.Contains(trimmed, "/_")
}

// worldRefusesSearch reports whether this request combines a non-default
// world with free-text search, which must be REFUSED rather than served.
//
// `?q=` on a list intersects the ACL-scoped rows with hits from the raw
// searcher, and no searcher can honor a world: internal/search is
// world-agnostic BY CONSTRUCTION (its Query type carries no world), and
// per-world indexing is Step 5 (TKT-9KZGJO). worldreader.NewSurface already
// refuses to CONSTRUCT a non-default-world surface over a searcher for this
// reason; this is the same refusal at the HTTP boundary.
//
// Silent degradation is the specific thing forbidden: the ACL row gate
// cannot catch a wrongly-scoped hit, because guard rule 1 makes it
// world-independent. A draft surfacing through the search box would reach
// the user with nothing downstream to stop it.
func worldRefusesSearch(r *http.Request) bool {
	return r.URL.Query().Get("q") != ""
}

// attachWorld resolves `?world=` once per request and binds the handle to
// the context, refusing every combination this surface cannot serve.
//
// Runs INSIDE attachACLRequest (so the read gate it needs for the grant
// check is already on the context) and before any handler, which is what
// makes "the grant check happens before a resolver is constructed"
// structural rather than a convention each handler must remember.
func attachWorld(next http.Handler, a *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) || r.URL.Query().Get(WorldParam) == "" {
			next.ServeHTTP(w, r)
			return
		}
		// Worlds are a READ-side concept: §9.4 makes "writes never pass
		// through worlds" a hard rule, because a world can answer a read
		// with a fallback face and a read-modify-write through that answer
		// saves the wrong state's content. Nothing on this API honors a
		// world on a write, so accepting the parameter would let
		// `PATCH ...?world=published` silently edit the DRAFT — and
		// `DELETE ...?world=published` delete the entity outright while the
		// caller believed they were unpublishing.
		readOnly := r.Method == http.MethodGet || r.Method == http.MethodHead ||
			r.Method == http.MethodOptions
		if !readOnly {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "world_read_only",
				"worlds are read-only on this API",
				"omit ?world= — writes always address the default state")
			return
		}
		handle, err := resolveWorld(r, a.worlds)
		switch {
		case errors.Is(err, errWorldDuplicated):
			writeV1Error(w, r, http.StatusBadRequest, "duplicate_world",
				"more than one ?world= parameter", "pass it exactly once")
			return
		case errors.Is(err, errWorldUnknown):
			// A config name, not a secret: name it. See resolveWorld.
			writeV1Error(w, r, http.StatusBadRequest, "unknown_world",
				fmt.Sprintf("no world named %q is declared",
					r.URL.Query().Get(WorldParam)),
				"check the `worlds:` block in schema.yaml")
			return
		case errors.Is(err, errWorldDenied):
			// NOT a 403, and NOT a synthetic body either. The request
			// continues with a handle marked `denied`, so the ORDINARY
			// handler runs and renders its own empty result — same JSON
			// shape, same `meta`, same `_actions`, same pagination headers
			// as a world that genuinely holds nothing this caller may read.
			//
			// Writing a hand-built `{"data":[]}` here was the first attempt
			// and it was wrong: the real list response carries meta/_actions/
			// X-Total-Count, so the bare body announced the denial on the
			// first byte — turning the thing designed to close an existence
			// oracle into one.
			next.ServeHTTP(w, r.WithContext(withWorld(r.Context(),
				worldHandle{name: r.URL.Query().Get(WorldParam), denied: true})))
			return
		case err != nil:
			// Infrastructure failure, not a denial.
			writeGateError(w, r, err)
			return
		}
		if !handle.isDefault() {
			if !worldCapablePath(r.URL.Path) {
				writeV1Error(w, r, http.StatusUnprocessableEntity, "world_unsupported",
					errWorldUnsupported.Error(),
					"this endpoint serves the default world only; omit ?world=")
				return
			}
			if r.URL.Query().Get("include") != "" {
				// `?include=` resolves NEIGHBORS, and every neighbor read on
				// these routes goes through the ungated, default-world
				// entityReader. Serving them under a world would return a
				// published entity wrapped in DRAFT neighbors — the mixed-face
				// response that is hardest to spot, because the entity looks
				// right and only its neighbors are wrong.
				writeV1Error(w, r, http.StatusUnprocessableEntity, "world_include_unsupported",
					"?include= cannot be resolved in a non-default world",
					"omit ?include= — neighbor resolution is not world-scoped yet")
				return
			}
			if worldRefusesSearch(r) {
				writeV1Error(w, r, http.StatusUnprocessableEntity, "world_search_unsupported",
					"free-text search cannot be scoped to a non-default world",
					"omit ?q=, or omit ?world= — per-world search indexing is not built yet")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(withWorld(r.Context(), handle)))
	})
}

// worldProvenance labels HOW the request's world resolved this face
// (TKT-WRLDAPI item 2).
//
// # Labeling, not re-resolving
//
// Resolution itself happens exactly once, in the store: the world scope rides
// the query ([store.EntityQuery.World]) and the backend picks the prime. This
// function reads the answer back — the coordinate the returned row was stored
// at — and names the rule that must have produced it. It never fetches, never
// walks the chain, and cannot disagree with the store about WHICH face was
// served, because it is handed that face.
//
// That distinction matters: a second chain walk here would be a second
// implementation of the semantics deciding which face a reader sees, free to
// drift from the store's. The mapping below is total over the states the
// store can return, so there is nothing left to decide:
//
//   - type absent from the scope -> rule 1, "unscoped".
//   - pointer present in the chain -> rule 2, "chain".
//   - otherwise -> rule 3 under `otherwise: default`, "fallback-default".
//
// The third arm is reachable only when the fallback actually fired: under
// `otherwise: exclude` the store returns NOTHING, so there is no response to
// label and the handler has already rendered a 404.
//
// Returns nil for a nil entity, so a caller may pass a not-found result
// through without branching.
func worldProvenance(ctx context.Context, e *entity.Entity) *v1.EntityWorld {
	if e == nil {
		return nil
	}
	handle := worldFromContext(ctx)
	name := handle.name
	if name == "" {
		name = defaultWorldName
	}
	return &v1.EntityWorld{
		Name:    name,
		Pointer: e.Pointer.String(),
		Via:     resolutionRule(handle.scope, e.Type, e.Pointer),
	}
}

// Resolution rule names, mirroring worldreader.Rule's String(). Spelled here
// rather than imported because internal/dataentry does not depend on
// internal/worldreader (the HTTP path resolves through the store query, not
// through a Resolver) — but the VOCABULARY is deliberately the same one, so a
// wire value and a server log line about the same resolution read alike.
const (
	ruleUnscoped        = "unscoped"
	ruleChain           = "chain"
	ruleFallbackDefault = "fallback-default"
)

// resolutionRule names the rule that produced a face stored at p, for an
// entity of entityType, under scope. See [worldProvenance] for why this is a
// total mapping rather than a walk.
//
// entityType MUST be canonical: [store.WorldScope] is keyed on canonical
// names only, and an alias reaching For() reads as an unknown type, which is
// rule 1. The route resolves the type by iterating the metamodel's own keys,
// so every caller on this path is canonical by construction.
func resolutionRule(scope store.WorldScope, entityType string, p entity.Pointer) string {
	res, scoped := scope.For(entityType)
	if !scoped {
		return ruleUnscoped
	}
	if slices.Contains(res.Chain, p) {
		return ruleChain
	}
	return ruleFallbackDefault
}
