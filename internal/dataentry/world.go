package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// WorldParam is the query parameter that selects a world on the read API:
// `?world=published`. Absent or empty means the DEFAULT world, bound
// explicitly at this boundary — the interior never sees "unspecified"
// (design doc §4.4).
const WorldParam = "world"

// errWorldUnknown names a `?world=` value no declared world matches. A
// CONFIG error, rendered as a named 400 — see [resolveWorld].
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
// the compiled map is injected from the wiring site via [NewApp].
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

// defaultWorldHandle is the handle of the default world: the trivial scope,
// every entity at its implicit face. It is the generated default world, and
// the scope an unstamped context falls back to.
//
// The zero handle is NOT this: its scope is unset, and a query built from it
// fails with store.ErrInvalidQuery (TKT-7IZHP0 design A4).
func defaultWorldHandle() worldHandle { return worldHandle{scope: store.TrivialScope()} }

// ranksNothing reports whether this world reads every entity at its implicit
// face, so a bare id needs no resolution. That is the generated default
// world of a metamodel that declares none. A declared default world ranks,
// and a denied world never reads at all.
func (w worldHandle) ranksNothing() bool { return !w.denied && w.scope.IsTrivial() }

// blocksAllReads reports a handle that must yield nothing at all.
func (w worldHandle) blocksAllReads() bool { return w.denied }

// visibility converts the handle to the world a [visibility.Resolver] reads
// in. It is the one place the request's world crosses into that package.
func (w worldHandle) visibility() visibility.World {
	if w.denied {
		return visibility.DeniedWorld()
	}
	return visibility.WorldOf(w.scope)
}

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

// worldFromContext returns the request's world handle, or
// [defaultWorldHandle] when none was bound.
//
// Every API request has a world stamped by attachWorld, so the fallback is
// reached only by code that runs outside an API request (non-API routes,
// tests, background callers). The fallback is the trivial scope, not the project's declared
// default world: every id resolves to its implicit face, so an entity of a
// faced type, which has no implicit face, appears to be missing. It narrows
// rather than widens what is read. A non-default world can only arrive by
// passing the grant check in [resolveWorld].
func worldFromContext(ctx context.Context) worldHandle {
	w, ok := ctx.Value(worldCtxKey{}).(worldHandle)
	if !ok {
		return defaultWorldHandle()
	}
	return w
}

// worldScopeFrom returns the store scope to stamp onto a query built from
// ctx. Spelled as its own helper so query-construction sites read as
// `Faces: store.InWorld(worldScopeFrom(ctx))` and a reviewer can grep for the ones that
// forgot.
func worldScopeFrom(ctx context.Context) store.WorldScope {
	return worldFromContext(ctx).scope
}

// setWorlds injects the compiled world map, enabling `?world=` selection.
//
// Until this is called the App serves the default world only and REFUSES any
// `?world=` naming something else — which is the correct posture for a
// surface whose wiring never opted in, and means a deployment that does not
// use worlds cannot accidentally acquire the parameter.
//
// It also rebuilds the base tracer, whose node titles come from the default
// world (BUG-95W7MV): NewApp built it before any world lookup existed.
//
// It also rewires the validator, whose scripts resolve bare ids in the
// default world (RR-HKVULG).
func (a *App) setWorlds(w WorldLookup) {
	a.worlds = w
	world := defaultWorldScope(w)
	tr, err := tracer.New(a.store, world)
	if err != nil { // coverage-ignore: invariant: store is non-nil and the scope is always set
		panic("dataentry: setWorlds: " + err.Error())
	}
	a.tracer = tr
	// coverage-ignore: invariant: NewApp built the same validator
	if err := wireValidation(a, a.Meta(), world); err != nil {
		panic("dataentry: setWorlds: " + err.Error())
	}
}

// defaultWorlder is the optional capability of a [WorldLookup] that names
// the schema's default world (worlds.Compiled.DefaultWorld).
type defaultWorlder interface {
	DefaultWorld() store.WorldScope
}

// familiesProvider is the optional capability of a [WorldLookup] that
// supplies the families scope, which ranks each faced type's faces in
// declaration order (worlds.Compiled.Families, G18).
type familiesProvider interface {
	Families() store.WorldScope
}

// defaultWorldScope is the scope of the default world in w, the world a
// non-HTTP surface uses: the lookup's DefaultWorld when it offers one, else
// its `default` entry. A nil lookup, or an unset scope, yields the trivial
// scope, so the result is always set.
func defaultWorldScope(w WorldLookup) store.WorldScope {
	if w == nil {
		return defaultWorldHandle().scope
	}
	if dw, ok := w.(defaultWorlder); ok {
		if scope := dw.DefaultWorld(); scope.IsSet() {
			return scope
		}
		return defaultWorldHandle().scope
	}
	scope, ok := w.Lookup(metamodel.DefaultWorldName)
	if !ok || !scope.IsSet() {
		return defaultWorldHandle().scope
	}
	return scope
}

// familiesScope is the families scope a resolver orders faces by: the
// lookup's own when it offers one, else one built from meta's face
// declaration order. The fallback serves an App whose wiring set no worlds
// (tests), and orders faces exactly as the compiled families scope would.
func familiesScope(w WorldLookup, meta *metamodel.Metamodel) store.WorldScope {
	if fp, ok := w.(familiesProvider); ok {
		return fp.Families()
	}
	byType := map[string]store.TypeResolution{}
	if meta == nil {
		return store.NewWorldScope(byType)
	}
	for typ := range meta.Entities {
		order := metamodel.FaceOrderOf(meta, typ)
		if len(order) == 0 {
			continue
		}
		chain := make([]entity.Face, len(order))
		for i, name := range order {
			chain[i] = entity.Face(name)
		}
		byType[typ] = store.TypeResolution{Chain: chain, Fallback: store.FallbackExclude}
	}
	return store.NewWorldScope(byType)
}

// resolveWorld resolves the request's `?world=` parameter into a handle,
// applying the per-world read grant BEFORE any resolver is constructed.
//
// The two failure modes differ on purpose. An unknown world is a named 400:
// world names are config, which is not secret. A known world the principal
// may not read returns errWorldDenied, which callers render as an EMPTY
// result, never a 403: a 403 would be an existence oracle for the world's
// content. Do not unify the two.
func resolveWorld(r *http.Request, lookup WorldLookup, defaultName string) (worldHandle, error) {
	values := r.URL.Query()[WorldParam]
	if len(values) > 1 {
		return worldHandle{}, errWorldDuplicated
	}
	name := r.URL.Query().Get(WorldParam)
	if name == "" {
		name = defaultName
	}
	return resolveNamedWorld(r.Context(), lookup, name, defaultName)
}

// resolveNamedWorld is [resolveWorld] after the name is known: the lookup and
// the per-world read grant. The remote MCP endpoint shares it, so an MCP read
// and a data-entry read of the same world are resolved and authorized by one
// function.
// A gate failure is returned as an error, never as a denial, so an outage does
// not look like an empty world.
func resolveNamedWorld(ctx context.Context, lookup WorldLookup, name, defaultName string) (worldHandle, error) {
	if lookup == nil {
		if name == defaultName {
			return worldHandle{name: name, scope: defaultWorldHandle().scope}, nil
		}
		return worldHandle{}, errWorldUnknown
	}
	scope, ok := lookup.Lookup(name)
	if !ok {
		return worldHandle{}, errWorldUnknown
	}
	if name == defaultName {
		return worldHandle{name: name, scope: scope}, nil
	}
	permitted, err := readGateFromContext(ctx).PermitsWorld(ctx, name)
	if err != nil {
		return worldHandle{}, err
	}
	if !permitted {
		return worldHandle{}, errWorldDenied
	}
	return worldHandle{name: name, scope: scope}, nil
}

// worldCapablePath reports whether path may serve a non-default world.
//
// A world-bound request must never be served default-world data, and most
// store call sites in this package are not world-aware. So the default is
// deny: a route joins this allowlist only when its whole read path is
// world-scoped and tested, so a leak needs someone to widen this predicate
// rather than forget a call site.
func worldCapablePath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	if trimmed == path {
		return false
	}
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return false
	}
	if isWorldCapableViewPath(trimmed) {
		return true
	}
	if isWorldCapableHistoryPath(trimmed) {
		return true
	}
	if trimmed == "_next_action" {
		return true
	}
	if trimmed == "_search" || trimmed == "_position" {
		return true
	}
	if trimmed == "_page_status" {
		return true
	}
	if isWorldCapablePilesPath(trimmed) {
		return true
	}
	if trimmed == "_dashboard/card" {
		return true
	}
	if isGanttPath(trimmed) || isSankeyPath(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, "_") {
		return false
	}
	if isWorldCapableEntityExportPath(trimmed) {
		return true
	}
	if isWorldCapableMentionedByPath(trimmed) {
		return true
	}
	return strings.Count(trimmed, "/") <= 1 &&
		!strings.Contains(trimmed, "/_")
}

// refuseWorldConfigError writes the 400 for a malformed or undeclared
// `?world=` and reports true, or reports false for every other outcome.
//
// The two it handles are CONFIG errors: they are true of a request on any
// route, for any principal, and are settled without consulting the grant. That
// is why they come first — and why they must not be shadowed by the
// route-capability 422 that follows. An operator who typos a world name is
// better served by `no world named "pubished" is declared` than by a refusal
// telling them the route cannot serve worlds at all, especially since a typo
// is likeliest on the routes they are experimenting with.
func refuseWorldConfigError(w http.ResponseWriter, r *http.Request, requested string, err error) bool {
	switch {
	case errors.Is(err, errWorldDuplicated):
		writeV1Error(w, r, http.StatusBadRequest, "duplicate_world",
			"more than one ?world= parameter", "pass it exactly once")
		return true
	case errors.Is(err, errWorldUnknown):
		writeV1Error(w, r, http.StatusBadRequest, "unknown_world",
			fmt.Sprintf("no world named %q is declared", requested),
			"check the `worlds:` block in schema.yaml")
		return true
	}
	return false
}

// refuseWorldIncapablePath writes the `world_unsupported` 422 and reports true
// when `requested` names a non-default world on a route [worldCapablePath]
// refuses. It reports false — write nothing, carry on — for the default world
// or a capable route.
//
// # Why this is decided BEFORE the grant check's outcome
func refuseWorldIncapablePath(w http.ResponseWriter, r *http.Request, requested, defaultName string) bool {
	if requested == "" || requested == defaultName {
		return false
	}
	if worldCapablePath(r.URL.Path) {
		return false
	}
	writeV1Error(w, r, http.StatusUnprocessableEntity, "world_unsupported",
		errWorldUnsupported.Error(),
		"this endpoint serves the default world only; omit ?world=")
	return true
}

// isWorldCapablePilesPath matches the piles READ shapes exactly: `_piles`,
// `_piles/{id}` and `_piles/{id}/_export`.
func isWorldCapablePilesPath(trimmed string) bool {
	parts := strings.Split(trimmed, "/")
	if parts[0] != "_piles" {
		return false
	}
	switch len(parts) {
	case 1:
		return true
	case 2:
		return parts[1] != ""
	case 3:
		return parts[1] != "" && parts[2] == "_export"
	}
	return false
}

// isWorldCapableViewPath matches `_views/{type}/{id}` — the entity view, whose
// whole read path was world-scoped in TKT-WRLDAPI item 4b.
//
// EXACTLY three segments, and the id must be non-empty. `_views/{name}` (a
// standalone view by config name) is a DIFFERENT surface reached through a
// different handler, and it is not scoped: it is refused here rather than
// admitted by a looser prefix match.
//
// `_sidepanel` and the command runner's `kind: view` share executeView but are
// not routes this admits — and they pass defaultViewWorld() explicitly, so
// even a future routing mistake could not hand them a world. See [viewWorld].
func isWorldCapableViewPath(trimmed string) bool {
	parts := strings.Split(trimmed, "/")
	return len(parts) == 3 && parts[0] == "_views" && parts[1] != "" && parts[2] != ""
}

// isWorldCapableEntityExportPath matches `{plural}/{id}/_export`, the export
// of one entity.
//
// It renders the page the reader is looking at, so it has to read in the same
// world. Refused, it resolved the entry's links in the zero world, where a
// type with faces stores nothing: every link to a faced entity vanished from
// the export while the detail page listed it. Its whole read path takes the
// world from ctx: the entry through getVisibleRef (world deny, row gate, face
// gate), its edges and neighbor rows through servedFaceNeighbors.
//
// The LIST export (`{plural}/_export`) is not admitted. It reads through
// scopedSortedEntities and has not been world-scoped or tested.
func isWorldCapableEntityExportPath(trimmed string) bool {
	parts := strings.Split(trimmed, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" &&
		!strings.HasPrefix(parts[1], "_") && parts[2] == "_export"
}

// isWorldCapableMentionedByPath matches `{plural}/{id}/mentioned-by`. The
// addressed entity is read with readAddressedOr404 and every source through
// visibility.Resolver.MentionedBy, both in the world on ctx, so a source is
// listed at the face the page's world serves. Refused, a world-bound page
// would list backlinks computed in the default world.
func isWorldCapableMentionedByPath(trimmed string) bool {
	parts := strings.Split(trimmed, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" &&
		!strings.HasPrefix(parts[1], "_") && parts[2] == "mentioned-by"
}

// isWorldCapableHistoryPath matches `_history/{type}/{id}` and
// `_history/{type}/{id}/{version}` — the entity-history reads.
//
// The RESTORE sub-path (`.../{version}/restore`) is a POST and is therefore
// already refused one layer up: attachWorld rejects `?world=` on any
// non-GET/HEAD/OPTIONS request with `world_read_only`, because a world can
// answer a read with a fallback face and writing through that answer saves the
// wrong state. So this predicate does not need to exclude it, and deliberately
// does not try — duplicating a rule that is enforced structurally elsewhere
// invites the two copies to drift.
func isWorldCapableHistoryPath(trimmed string) bool {
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || len(parts) > 4 || parts[0] != "_history" {
		return false
	}
	return parts[1] != "" && parts[2] != ""
}

// effectiveDefaultWorld is the world a request that names none reads in:
// schema.yaml's `default_world:`, else the first declared world, else the
// generated default world. The deprecated `app.default_world` alias must
// agree with it (config load refuses one that does not), so it is not read.
func effectiveDefaultWorld(a *App) string {
	if a == nil {
		return metamodel.DefaultWorldName
	}
	state := a.State()
	if state == nil {
		return metamodel.DefaultWorldName
	}
	return metamodel.EffectiveDefaultWorld(state.Meta)
}

// declaredDefaultWorld is the default world's name for `/_config`, or ""
// when the schema declares no worlds and the generated default world ranks
// nothing. The SPA treats "" as "no world binds this page".
func declaredDefaultWorld(meta *metamodel.Metamodel) string {
	if meta == nil || len(meta.Worlds) == 0 {
		return ""
	}
	return metamodel.EffectiveDefaultWorld(meta)
}

// readOnlyMethod reports whether a method only reads. Worlds are a read-side
// routing rule, so this gates BOTH the refusal of an explicit `?world=` on a
// write and the application of the operator's browsing default — a write must
// address a face by id, never by world.
func readOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead ||
		method == http.MethodOptions
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
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		explicit := len(r.URL.Query()[WorldParam]) > 0
		if explicit && !readOnlyMethod(r.Method) {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "world_read_only",
				"worlds are read-only on this API",
				"omit ?world= — address the face directly by id (`ID` or `ID@face`)")
			return
		}
		defaultName := effectiveDefaultWorld(a)
		requested := r.URL.Query().Get(WorldParam)
		if requested == "" {
			requested = defaultName
		}
		handle, err := resolveWorld(r, a.worlds, defaultName)
		if refuseWorldConfigError(w, r, requested, err) {
			return
		}
		if refuseWorldIncapablePath(w, r, requested, defaultName) {
			return
		}
		switch {
		case errors.Is(err, errWorldDenied):
			next.ServeHTTP(w, r.WithContext(withWorld(r.Context(),
				worldHandle{name: requested, scope: defaultWorldHandle().scope, denied: true})))
			return
		case err != nil:
			writeGateError(w, r, err)
			return
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
// the query (an InWorld selection on [store.EntityQuery.Faces]) and the backend picks the prime. This
// function reads the answer back — the coordinate the returned row was stored
// at — and names the rule that must have produced it. It never fetches, never
// walks the chain, and cannot disagree with the store about WHICH face was
// served, because it is handed that face.
func worldProvenance(ctx context.Context, e *entity.Entity) *v1.EntityWorld {
	if e == nil {
		return nil
	}
	handle := worldFromContext(ctx)
	name := handle.name
	if name == "" {
		name = metamodel.DefaultWorldName
	}
	rule, position := resolutionRuleAt(handle.scope, e.Type, e.Face)
	return &v1.EntityWorld{
		Name:          name,
		Face:          e.Face.String(),
		Via:           rule,
		ChainPosition: position,
	}
}

// The wire vocabulary for a resolution rule is [store.ResolutionRule]'s own
// String(); these names exist so a handler and its tests spell it once.
var (
	ruleUnscoped        = store.ResolutionUnscoped.String()
	ruleChain           = store.ResolutionChain.String()
	ruleFallbackDefault = store.ResolutionFallbackDefault.String()
)

// resolutionRuleAt names the rule that produced a face stored at p, for an
// entity of entityType, under scope, plus the chain POSITION of the served
// coordinate — the rank the world got, not merely that it got something. See
// [worldProvenance] for why this is a total mapping rather than a walk.
//
// entityType MUST be canonical: [store.WorldScope] is keyed on canonical
// names only, and an alias reaching For() reads as an unknown type, which is
// rule 1. The route resolves the type by iterating the metamodel's own keys,
// so every caller on this path is canonical by construction.
func resolutionRuleAt(
	scope store.WorldScope, entityType string, p entity.Face,
) (rule string, position *int) {
	r, pos := scope.RuleAt(entityType, p)
	if r == store.ResolutionChain {
		return r.String(), &pos
	}
	return r.String(), nil
}

// isGanttPath matches `_gantts/{id}`, one gantt chart.
func isGanttPath(trimmed string) bool {
	id, ok := strings.CutPrefix(trimmed, "_gantts/")
	return ok && id != "" && !strings.Contains(id, "/")
}
