package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/conflict"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// --- API v1 Types ---

func (a *App) toV1PropertyDef(meta *metamodel.Metamodel, propDef metamodel.PropertyDef) v1.PropertyDef {
	pd := v1.PropertyDef{
		Type:        propDef.Type,
		Required:    propDef.Required,
		Default:     propDef.Default,
		Description: propDef.Description,
		List:        propDef.List,
		Max:         propDef.Max,
	}
	// Labels mirror Values: a property naming a custom type inherits that
	// type's values and labels; an inline `labels` map on a custom-typed
	// property is ignored, exactly as inline `values` is.
	if ct, ok := meta.Types[propDef.Type]; ok {
		pd.Values = ct.Values
		pd.Labels = ct.Labels
	} else if len(propDef.Values) > 0 {
		pd.Values = propDef.Values
		pd.Labels = propDef.Labels
	}
	return pd
}

// toV1CustomType is the single serialization site for a metamodel custom type
// onto the wire, so the _schema types map and any other consumer stay in sync.
//
// It intentionally projects only Values/Labels/Default. The documentation-only
// fields — CustomType.Descriptions (per-value meaning), CustomType.Transitions,
// and TransitionDef.Help (TKT-0YBFT8) — are NOT serialized: their consumer is the
// offline `rela docs` generator (FEAT-G4VO53), not the SPA. Wiring any of them to
// the wire is a deliberate frontend-contract change (see internal/dataentry/
// CLAUDE.md), not a "helpful" one-liner here. TestToV1CustomType_OmitsDocFields
// pins this boundary.
func toV1CustomType(ct metamodel.CustomType) v1.CustomType {
	return v1.CustomType{
		Values:  ct.Values,
		Labels:  ct.Labels,
		Default: ct.Default,
	}
}

// --- API v1 Router ---

// registerAPIV1Routes registers all /api/v1/ routes.
// Note: /api/v1/_events is registered separately in NewRouter as it needs to be
// outside the reload-lock middleware (SSE long-lived connection).
//
// When adding a route, add a probe to the route table in
// router_walk_test.go so registration stays covered.
func (a *App) registerAPIV1Routes(mux *http.ServeMux) {
	// System endpoints (underscore prefix)
	mux.HandleFunc("/api/v1/_schema", a.handleV1Schema)
	mux.HandleFunc("/api/v1/_schema/", a.handleV1SchemaRoutes)
	mux.HandleFunc("/api/v1/_config", a.handleV1Config)
	mux.HandleFunc("/api/v1/_feeds/", a.handleV1Feed)
	mux.HandleFunc("/api/v1/_search", a.handleV1Search)
	mux.HandleFunc("/api/v1/_position", a.handleV1EntityPosition)
	mux.HandleFunc("/api/v1/_analyze", a.handleV1Analyze)
	mux.HandleFunc("/api/v1/_git/status", a.handleGitStatus)
	mux.HandleFunc("/api/v1/_git/sync", a.handleGitSync)
	mux.HandleFunc("/api/v1/_settings", a.handleAPISettingsCRUD)
	mux.HandleFunc("/api/v1/_palette", a.handleAPIPaletteCRUD)
	mux.HandleFunc("/api/v1/_theme/logo", a.handleAPIThemeLogo)
	mux.HandleFunc("/api/v1/_theme/export", a.handleAPIThemeExport)
	mux.HandleFunc("/api/v1/_theme/import", a.handleAPIThemeImport)
	mux.HandleFunc("/api/v1/_sidepanel/", a.views.handleV1SidePanel)
	mux.HandleFunc("/api/v1/_sidebar", a.views.handleV1Sidebar)
	mux.HandleFunc("/api/v1/_conflicts", a.handleV1Conflicts)
	mux.HandleFunc("/api/v1/_conflicts/", a.handleV1ConflictRoutes)
	mux.HandleFunc("/api/v1/_documents/", a.handleV1Documents)
	mux.HandleFunc("/api/v1/_history/", func(w http.ResponseWriter, r *http.Request) { handleV1History(a, w, r) })
	mux.HandleFunc("/api/v1/_relation_history/",
		func(w http.ResponseWriter, r *http.Request) { handleV1RelationHistory(a, w, r) })
	mux.HandleFunc("/api/v1/_openapi.json", a.handleV1OpenAPI)
	mux.HandleFunc("/api/v1/_commands", a.handleV1Commands)
	mux.HandleFunc("/api/v1/_transforms", a.export.handleV1Transforms)
	mux.HandleFunc("/api/v1/_templates/", a.handleV1Templates)
	mux.HandleFunc("/api/v1/_views/", a.views.handleV1Views)
	mux.HandleFunc("/api/v1/_action/", a.write.handleV1Action)
	mux.HandleFunc("/api/v1/_apps/", a.handleV1App)

	// Dynamic entity routes are handled by a catch-all
	mux.HandleFunc("/api/v1/", a.handleV1DynamicRoutes)
}

// Path-segment counts for the dynamic route dispatcher below. A
// four-segment path selects a sub-resource collection
// (relations/_actions/_attachments); a five-segment path targets a
// single member within one of those.
const (
	segmentsSubResource   = 4 // /{plural}/{id}/{sub}/{key}
	segmentsSubResourceID = 5 // /{plural}/{id}/{sub}/{key}/{member}
)

// handleV1DynamicRoutes routes requests to the appropriate entity handler
// based on URL. Read operations work against the snapshot returned by
// a.State() with no locking; write operations take a.writeMu for the
// duration of the mutation.
func (a *App) handleV1DynamicRoutes(w http.ResponseWriter, r *http.Request) {
	// Skip system routes (already handled)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if strings.HasPrefix(path, "_") {
		http.NotFound(w, r)
		return
	}

	// Parse path: {plural}[/{id}[/relations[/{relType}[/{targetId}]]]]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		return
	}

	plural := parts[0]

	// Find entity type by plural
	var typeName string
	for name, def := range a.State().Meta.Entities {
		if def.GetPlural(name) == plural {
			typeName = name
			break
		}
	}

	if typeName == "" {
		writeV1Error(w, r, http.StatusNotFound, "unknown_type", "Unknown entity type", "")
		return
	}

	switch len(parts) {
	case 1:
		// /{plural} - collection
		a.handleV1EntityCollection(w, r, typeName, plural)
	case 2:
		// /{plural}/_export (collection export) or /{plural}/{id} (single entity).
		// _export is a reserved segment; no entity id may collide (ids never
		// start with the reserved underscore).
		if parts[1] == "_export" {
			a.export.handleV1ExportList(w, r, typeName)
		} else {
			a.handleV1SingleEntity(w, r, typeName, plural, parts[1])
		}
	case 3:
		// /{plural}/{id}/relations or /{plural}/{id}/_export
		switch parts[2] {
		case "relations":
			a.handleV1EntityRelations(w, r, typeName, parts[1])
		case "_export":
			a.export.handleV1ExportEntity(w, r, typeName, parts[1])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	case segmentsSubResource:
		// /{plural}/{id}/relations/{relType}, /{plural}/{id}/_actions/{action},
		// or /{plural}/{id}/_attachments/{property}
		switch parts[2] {
		case "relations":
			a.handleV1EntityRelationType(w, r, typeName, parts[1], parts[3])
		case "_actions":
			a.handleV1EntityAction(w, r, typeName, parts[1], parts[3])
		case "_attachments":
			a.attachments.handleV1AttachmentRoute(w, r, typeName, plural, parts[1], parts[3])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	case segmentsSubResourceID:
		// /{plural}/{id}/relations/{relType}/{targetId} or
		// /{plural}/{id}/_attachments/{property}/{fileName}
		switch parts[2] {
		case "relations":
			a.handleV1RelationTarget(w, r, typeName, parts[1], parts[3], parts[4])
		case "_attachments":
			a.attachments.handleV1AttachmentFileRoute(w, r, typeName, parts[1], parts[3], parts[4])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

// --- Collection Handlers ---

func (a *App) handleV1EntityCollection(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	switch r.Method {
	case http.MethodGet:
		a.handleV1ListEntities(w, r, typeName, plural)
	case http.MethodPost:
		// TKT-3I5U: ?dry_run=true evaluates affordances + soft validation
		// against the candidate WITHOUT persisting, so the create form can
		// gate fields / options / hidden as the user types. Read-shaped:
		// dispatched before handleV1CreateEntity acquires the write lock.
		if r.URL.Query().Get("dry_run") == "true" {
			a.write.handleV1DryRunCreate(w, r, typeName, plural)
			return
		}
		a.write.handleV1CreateEntity(w, r, typeName, plural)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// errACLListQuery wraps a store.GraphQuery failure during ACL list
// filtering so call sites can route it through writeGateError (500
// acl_query_failed / 504 / silent-on-cancel) instead of mislabeling
// it as a free-text search failure.
var errACLListQuery = errors.New("acl list query failed")

// errBadFilter marks a filter[...] query param the pipeline refuses to
// evaluate: malformed key shape or an operator outside the supported set.
// Mapped to HTTP 400 by writeListPipelineError. A rejected request beats
// the old behavior of dropping the clause with only a server-side log —
// that silently returned the UNFILTERED superset, which let a config typo
// (`=~`, BUG: active_tickets) masquerade as an empty/wrong result set for
// months. Malformed wire format is a hard 400 per DEC-HWZHA.
var errBadFilter = errors.New("invalid filter parameter")

// errListLoad wraps a store iterator failure while loading the
// unfiltered (AllowAll) list. Surfaced as 500 list_load_failed at the
// call sites: before TKT-VMD8 a mid-stream error silently truncated
// the result, which under ACL would make the AllowAll and Query paths
// observably asymmetric (truncated-200 vs 500) for the same backend
// fault — and a truncated list with authoritative-looking pagination
// is worse than an error either way.
var errListLoad = errors.New("list load failed")

// scopedSortedEntities runs the shared list pipeline — ACL read scope,
// load, free-text intersection (?q=), structured filters (filter[...]),
// then the configured sort — and returns the fully ordered result set
// *before* pagination. Both handleV1ListEntities and
// handleV1EntityPosition call this, so a list and its scope navigator
// are guaranteed to observe identical ordering (under ACL both operate
// on the same visible subset — hidden entities don't occupy ordinals).
//
// ACL ordering contract (TKT-VMD8, RR-X56H + RR-3IO2): the read-scope
// verdict resolves FIRST. DenyAll returns before the search backend,
// filters, or sort run — a denied principal must not be able to probe
// backend latency through ?q=. A composed Query loads the visible
// subset via store.GraphQuery; search/filter/sort then operate on that
// filtered slice only (search-after-ACL). AllowAll keeps the pre-ACL
// load path byte-identical.
//
// Errors: free-text search failures surface verbatim (HTTP 500
// search_failed at the call site); ACL query failures are wrapped in
// errACLListQuery so call sites map them via writeGateError. Everything
// else degrades to an empty/whole set as the list endpoint always did.
func (a *App) scopedSortedEntities(
	ctx context.Context,
	typeName string,
	query map[string][]string,
) ([]*entityPkg.Entity, error) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)

	var entities []*entityPkg.Entity
	switch {
	case rqr.DenyAll:
		return []*entityPkg.Entity{}, nil
	case rqr.AllowAll:
		// Inline iteration rather than listFromStoreByTypes: that
		// helper swallows iterator errors into a partial slice, and
		// the list pipeline must fail loud on both verdict paths.
		for e, err := range a.Services().Store.ListEntities(ctx, store.EntityQuery{Type: typeName}) {
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errListLoad, err)
			}
			entities = append(entities, e)
		}
	case rqr.Query == nil:
		// Defensive: a zero ReadQueryResult would otherwise alias
		// AllowAll. Fail loud instead of silently widening the list.
		return nil, fmt.Errorf("%w: zero ReadQueryResult for type %q", errACLListQuery, typeName)
	default:
		for e, err := range a.Services().Store.GraphQuery(ctx, *rqr.Query) {
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errACLListQuery, err)
			}
			entities = append(entities, e)
		}
	}

	// Free-text search: intersect with hits from the searcher when ?q=... is
	// present. Bleve scores are discarded — the list's configured sort wins
	// over relevance ranking, same approach SearchView uses for filtering.
	// Backend errors surface as HTTP 500 rather than rendering an empty list
	// and pretending the search succeeded.
	searchResult, err := a.freeTextIDsForType(ctx, queryGet(query, "q"), typeName)
	if err != nil {
		return nil, err
	}
	if searchResult.HasFilter {
		filtered := entities[:0]
		for _, e := range entities {
			if _, hit := searchResult.IDs[e.ID]; hit {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	// Classify each filter[<key>] param as property vs relation ONCE, up
	// front, so both passes agree on routing. The config's FilterControls are
	// authoritative (RR-0HWAS0 / RR-B0JPPL): a relation filter applies only
	// when a control on a list of this type configures it. Name-based
	// GetRelationDef is only a fallback and only ever routes AWAY from
	// properties, never toward relations without a control.
	isRelationKey := relationFilterClassifier(a.Meta(), a.Cfg(), typeName)
	entities, err = applyV1Filters(entities, query, typeName, isRelationKey)
	if err != nil {
		return nil, err
	}
	entities, err = a.applyRelationFilters(ctx, entities, query, typeName, isRelationKey)
	if err != nil {
		return nil, err
	}
	entities = applyV1Sorting(entities, query)
	return entities, nil
}

// relationFilterClassifier returns a predicate that decides, for a bare filter
// param name (the property/relation segment, no operator), whether it should be
// handled by the direction-aware relation pass rather than applyV1Filters.
//
// Relation filtering is restricted to configured filter_controls (RR-B0JPPL):
// a `filter[<rel>]` param routes to the relation pass ONLY when a relation
// FilterControl on a list of this entity type configures it. An arbitrary
// metamodel relation with no control is NOT filterable and falls through to
// applyV1Filters (where, absent a matching property, it fails closed rather
// than silently widening the set).
//
// The config discriminator also resolves the property/relation NAME COLLISION
// (RR-0HWAS0): properties (per-type) and relations (global) are disjoint
// namespaces, so a property can share a name with a relation. An explicit
// property FilterControl for the name forces the property pass; a name that is
// only ever a relation control routes to the relation pass. When a name is both
// a property of this type and a relation control with no disambiguating
// property control, the property wins (safe, backward-compatible) and
// CollectConfigWarnings flags the ambiguity at load.
//
// A free function (not an App method) taking the already-loaded meta/cfg
// snapshot: it needs no other App state, and keeping it off the receiver keeps
// App under its plimsoll method cap.
func relationFilterClassifier(
	meta *metamodel.Metamodel,
	cfg *dataentryconfig.Config,
	typeName string,
) func(string) bool {
	return func(key string) bool {
		// Explicit property control → property pass.
		if cfg.HasPropertyFilterControl(typeName, key) {
			return false
		}
		// A relation control is required for relation filtering.
		if _, ok := cfg.RelationFilterDirection(typeName, key); !ok {
			return false
		}
		// Relation control exists, but the collision rule: if the name is
		// also a property of this type (and no property control disambiguated
		// above), the property wins.
		if entDef, ok := meta.GetEntityDef(typeName); ok {
			if _, isProp := entDef.Properties[key]; isProp {
				return false
			}
		}
		return true
	}
}

// applyRelationFilters keeps only the entities that match every relation
// filter present in the query. isRelationKey (the config-backed classifier
// built in scopedSortedEntities) decides which `filter[<key>]` params are
// relation filters; property filters are handled by applyV1Filters. Matching is
// direction-aware: the FilterControl config on a list of this entity type
// supplies the direction (default outgoing), and an entity matches iff it has
// an edge of that (relation, direction) to a READABLE neighbor whose display
// title equals the requested value.
//
// Operators: only `eq` (the bare `filter[<rel>]` form) and `ne`
// (`filter[<rel>][ne]`) are supported. Any other operator segment is rejected
// with errBadFilter → HTTP 400, matching applyV1Filters. This supersedes the
// RR-6RF60V drop-to-zero-rows behavior: both were fail-closed, but zero rows
// reads as "no data" while a 400 names the broken operator to the caller.
// Never fail open.
//
// ACL (RR-HK1XNO): neighbor titles are resolved through the read gate, so a
// filter value matching a neighbor the caller cannot read does NOT include the
// edged rows (no title-match inference channel). Neighbors are gated in one
// batch per relation param via matchRelationFilter → visibleNeighborTitles.
// NOTE: when the sibling helper App.visibleRelationIDs (TKT-ODHV2D) merges,
// this should converge on it; today it uses the same readGate.PermitsReadMany
// batching pattern inline.
//
// Cost (RR-38K7K9): this runs over the entire type's visible set BEFORE
// pagination, doing ListRelations + a batched GetEntity per row. It is
// therefore O(N·edges) per request, uncached, where N is the unpaginated
// visible row count — acceptable for the current dataset sizes but NOT a
// store-side pushdown. A future optimization is to push the relation-title
// predicate into the store query. Each row short-circuits: matchRelationFilter
// stops scanning a row's edges as soon as one matches.
//
// This runs in scopedSortedEntities so BOTH the list handler and scope nav
// (_position, via resolveScope) observe identical relation filtering — they
// share this pipeline. Property filters are untouched (TKT-5U7QBR).
func (a *App) applyRelationFilters(
	ctx context.Context, entities []*entityPkg.Entity, query map[string][]string, typeName string,
	isRelationKey func(string) bool,
) ([]*entityPkg.Entity, error) {
	cfg := a.Cfg()
	for key, values := range query {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}
		relation, operator, ok := parseRelationFilterKey(key)
		if !ok || !isRelationKey(relation) {
			continue // malformed, or a property filter handled elsewhere
		}
		want := values[len(values)-1]
		if want == "" {
			continue // empty control value = no constraint
		}

		// Reject any operator we don't support for relations. Never fall
		// through and leave the set unfiltered (that would be the fail-open
		// bug RR-6RF60V); see the doc comment on the 400-over-zero-rows choice.
		switch operator {
		case "eq", "ne":
			// supported below
		default:
			return nil, fmt.Errorf("%w: unknown operator %q in relation filter key %q (supported: eq, ne)",
				errBadFilter, operator, key)
		}

		direction, _ := cfg.RelationFilterDirection(typeName, relation)
		filtered := entities[:0]
		for _, e := range entities {
			matched := a.matchRelationFilter(ctx, e.ID, relation, direction, want)
			if (operator == "eq" && matched) || (operator == "ne" && !matched) {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}
	return entities, nil
}

// parseRelationFilterKey parses a `filter[<rel>]` or `filter[<rel>][<op>]` key
// using the SAME `][`-split logic as applyV1Filters, so an operator segment is
// recognized rather than being swallowed into the relation name (RR-6RF60V,
// the old TrimPrefix/TrimSuffix bug). Returns (relation, operator, ok) with
// operator defaulting to "eq". ok is false for a malformed key (empty relation,
// too many segments).
func parseRelationFilterKey(key string) (relation, operator string, ok bool) {
	filterKey := strings.TrimPrefix(key, "filter[")
	filterKey = strings.TrimSuffix(filterKey, "]")
	filterKey = strings.TrimSuffix(filterKey, "][") // was "...[]"
	parts := strings.Split(filterKey, "][")
	if len(parts) > 2 || parts[0] == "" {
		return "", "", false
	}
	operator = "eq"
	if len(parts) == 2 {
		if parts[1] == "" {
			return "", "", false
		}
		operator = parts[1]
	}
	return parts[0], operator, true
}

// matchRelationFilter reports whether entityID has an edge of the given
// relation+direction to a neighbor the caller can READ whose display title
// equals want. It gates neighbor entities through the read gate so a hidden
// neighbor cannot be used as a title-match inference channel (RR-HK1XNO), and
// short-circuits as soon as a readable neighbor's title matches (RR-38K7K9).
//
// Neighbors are gated in batches grouped by type via readGate.PermitsReadMany
// (one call per neighbor type on the row), rather than a per-neighbor gate
// call. When the sibling helper App.visibleRelationIDs (TKT-ODHV2D) merges,
// this should converge on it.
func (a *App) matchRelationFilter(
	ctx context.Context, entityID, relation string, direction dataentryconfig.Direction, want string,
) bool {
	svc := a.Services()
	q := store.RelationQuery{
		EntityID:  entityID,
		Type:      relation,
		Direction: relationDirection(direction),
	}

	// Load each neighbor once, keeping only those whose title matches want.
	// Group the candidates by type so the read gate can batch per type.
	candidatesByType := map[string][]string{}
	entByID := map[string]*entityPkg.Entity{}
	for r, err := range svc.Store.ListRelations(ctx, q) {
		if err != nil {
			return false
		}
		targetID := r.To
		if direction.IsIncoming() {
			targetID = r.From
		}
		if _, seen := entByID[targetID]; seen {
			continue
		}
		e, err := svc.Store.GetEntity(ctx, targetID)
		if err != nil {
			continue // dangling relation
		}
		entByID[targetID] = e
		if svc.Meta.DisplayTitle(e.ID, e.Type, e.Properties) == want {
			candidatesByType[e.Type] = append(candidatesByType[e.Type], targetID)
		}
	}
	if len(candidatesByType) == 0 {
		return false
	}

	// A row matches iff at least one title-matching neighbor is READABLE. Gate
	// only the title-matching candidates (the minimal set) per type.
	gate := readGateFromContext(ctx)
	for typ, ids := range candidatesByType {
		perm, err := gate.PermitsReadMany(ctx, typ, ids)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if perm[id] {
				return true // readable title match (RR-HK1XNO honored)
			}
		}
	}
	return false
}

// queryGet returns the first value for key from a raw query map, or "".
// url.Values.Get over a plain map[string][]string without allocating.
func queryGet(query map[string][]string, key string) string {
	if vals, ok := query[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (a *App) handleV1ListEntities(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	query := r.URL.Query()

	entities, err := a.scopedSortedEntities(r.Context(), typeName, query)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}

	// Pagination
	total := len(entities)
	page, perPage := parseV1Pagination(query)
	start := (page - 1) * perPage
	end := start + perPage
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	entities = entities[start:end]

	// Check if includes are requested (for relation columns)
	includes := query.Get("include")
	wantIncludes := includes != ""

	// Load each row's edges once. List rows carry BOTH outgoing edges (keyed
	// by relation type) and incoming edges (keyed by the relation's inverse
	// name) so that `direction: incoming` relation columns have a wire key to
	// resolve against. The SPA computes the same inverse key and reads the
	// source entities from the ?include=* peers. See MECHANISM.md.
	outgoingByRow := make([][]*entityPkg.Relation, len(entities))
	incomingByRow := make([][]*entityPkg.Relation, len(entities))
	var pageNeighborIDs []string
	for i, e := range entities {
		outgoingByRow[i] = a.reader.outgoingRelations(r.Context(), e.ID)
		incomingByRow[i] = a.reader.incomingRelations(r.Context(), e.ID)
		pageNeighborIDs = append(pageNeighborIDs, neighborIDsOf(outgoingByRow[i], incomingByRow[i])...)
	}

	// Gate neighbor IDs for the WHOLE page in one type-batched pass
	// (RR-HJV8CP + RR-FRK1): a neighbor's ID may appear in a row's relations
	// map only if its entity is visible to the caller, so `relations` and the
	// visibility-filtered `included` map can never disagree. Outgoing targets
	// (edge.To) and incoming sources (edge.From) are gated together — an
	// entity's visibility is direction-independent.
	visibleNeighbors := visibleRelationIDs(r.Context(), a.reader, a.visibleReader, pageNeighborIDs)

	// Build response - always include relations for relation column support
	data := make([]v1.Entity, 0, len(entities))
	included := make(map[string]v1.Entity)
	for i, e := range entities {
		v1Entity := a.serializer.forWireRelated(
			r.Context(), e,
			outgoingByRow[i],
			incomingByRow[i],
			visibleNeighbors,
			a.Meta(), plural,
		)
		data = append(data, v1Entity)

		// Resolve includes if requested
		if wantIncludes {
			maps.Copy(included, a.resolveV1Includes(r.Context(), e, includes))
		}
	}

	resp := v1.ListResponse{
		Data: data,
		Meta: v1.ListMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: end < total,
		},
		Actions: a.affordances.computeCollectionActions(r.Context(), typeName),
	}

	// Add Link header for pagination (RFC 5988)
	addPaginationLinks(w, r, page, perPage, total, plural)

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Per-Page", strconv.Itoa(perPage))

	// If includes were requested, add them to response
	if len(included) > 0 {
		// For list responses with includes, we need a different response structure
		// Encode as JSON with additional "included" field
		type listWithIncludes struct {
			Data     []v1.Entity          `json:"data"`
			Meta     v1.ListMeta          `json:"meta"`
			Included map[string]v1.Entity `json:"included,omitempty"`
			Actions  map[string]bool      `json:"_actions,omitempty"`
		}
		writeV1JSON(w, http.StatusOK, listWithIncludes{
			Data:     resp.Data,
			Meta:     resp.Meta,
			Included: included,
			Actions:  resp.Actions,
		})
		return
	}

	writeV1JSON(w, http.StatusOK, resp)
}

// --- Single Entity Handlers ---

func (a *App) handleV1SingleEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	switch r.Method {
	case http.MethodGet:
		a.handleV1GetEntity(w, r, typeName, plural, entityID)
	case http.MethodPatch:
		a.write.handleV1UpdateEntity(w, r, typeName, plural, entityID)
	case http.MethodDelete:
		a.write.handleV1DeleteEntity(w, r, typeName, plural, entityID)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// gateReadOrNotFound runs the per-entity ACL read gate (PermitsRead)
// and writes the right response on deny: 404 with the same body shape
// as not-found (indistinguishable-from-not-found invariant) when the
// principal cannot read, or the relevant 5xx via writeGateError on
// store/timeout failure. Returns true if the handler should continue
// (gate allowed), false if a response was already written.
//
// Centralizing this means every read chokepoint (GET, PATCH, DELETE,
// clone, relations CRUD) handles the deny branch the same way — a
// future divergence in error code, body shape, or ETag suppression
// would otherwise be a per-handler oracle leak (RR-NGMI).
// entityNotFoundTitle is the title for EVERY read-path 404 — the ACL
// deny path (gateReadOrNotFound), a genuinely missing entity, and a
// missing attachment all share it so a denied read is byte-identical to
// a nonexistent one (the RR-NGMI indistinguishability invariant). Any
// handler that 404s on the read path MUST use this const, not a fresh
// literal, or the bodies drift and existence leaks.
const entityNotFoundTitle = "Entity not found"

func (a *App) gateReadOrNotFound(w http.ResponseWriter, r *http.Request, typeName, entityID string) bool {
	ok, err := readGateFromContext(r.Context()).PermitsRead(r.Context(), typeName, entityID)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

func (a *App) handleV1GetEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	ctx := r.Context()

	// ACL gate (TKT-VQGN). visibleReader.getVisible applies PermitsRead
	// BEFORE the store read so a hidden id and a nonexistent id spend the
	// same MatchingIDs roundtrip — otherwise the timing difference
	// (in-memory lookup ~1µs vs. DB roundtrip ~1ms) is an id-enumeration
	// side channel that defeats the indistinguishable-404-body invariant
	// (RR-NGMI). A gate error surfaces via writeGateError; a deny is
	// returned as (nil,false,nil), indistinguishable from a real miss.
	entity, found, err := a.visibleReader.getVisible(ctx, typeName, entityID)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	query := r.URL.Query()

	// Single per-entity serialization: strips hidden + attaches
	// `_fields` / `_relations` per docs/data-entry/api-reference.md.
	result := a.serializer.forWire(ctx, entity, a.reader.outgoingRelations(ctx, entity.ID), a.Meta(), plural)

	// Handle includes for related entities
	if includes := query.Get("include"); includes != "" {
		result.Included = a.resolveV1Includes(ctx, entity, includes)
	}

	// ETag for caching (visible-only path; deny-path above emits no ETag).
	etag := a.computeEntityETag(ctx, entity)
	w.Header().Set("ETag", etag)

	// Check If-None-Match
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	writeV1JSON(w, http.StatusOK, result)
}

// writeListPipelineError maps a scopedSortedEntities / resolveScope
// error to the right HTTP shape: ACL query failures route through
// writeGateError, store-load failures surface as list_load_failed,
// and anything else is the free-text search failure the pipeline
// always surfaced. Shared by the list and _position handlers so the
// two consumers of the pipeline can't drift.
//
// All branches log the raw error server-side and keep it out of the
// response body (RR-372L / IB-review PR939 #1): a store or search
// backend error string can name tables, columns, or index paths.
func writeListPipelineError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errBadFilter):
		// Server-side log stays alongside the 400 (RR-6RF60V's logging
		// intent, CONTROL-8-15): the response informs the caller, the log
		// informs operators — a broken config or a caller probing filter
		// params is an anomalous event worth a trace even when no one
		// watches the client.
		slog.Warn("dataentry: rejected invalid filter parameter",
			"err", err, "path", r.URL.Path, "method", r.Method)
		// Client-caused: echo the detail — it only restates the caller's own
		// filter key/operator, never store internals.
		writeV1Error(w, r, http.StatusBadRequest, "invalid_filter",
			"Invalid filter parameter", err.Error())
	case errors.Is(err, errACLListQuery):
		writeGateError(w, r, err)
	case errors.Is(err, errListLoad):
		slog.Warn("dataentry: list load failed",
			"err", err, "path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusInternalServerError, "list_load_failed",
			"Loading entities failed", "check server logs")
	default:
		slog.Warn("dataentry: list free-text search failed",
			"err", err, "path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusInternalServerError, "search_failed",
			"Free-text search failed", "check server logs")
	}
}

// writeGateError maps a readGate.PermitsRead / PermitsReadMany error
// to the right HTTP shape: client-disconnect emits nothing,
// deadline-exceeded is 504, everything else is 500 with the
// acl_query_failed code (RR-89XK). Centralized so every gate call
// site handles the error shape the same way.
//
// The raw error is logged server-side and never echoed in the
// response body (same RR-372L rationale as attachACLRequest): a
// backend error string can carry table/column names or other
// internals an API client has no business seeing.
func writeGateError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	slog.Warn("acl: read-gate query failed",
		"err", err, "path", r.URL.Path, "method", r.Method)
	if errors.Is(err, context.DeadlineExceeded) {
		writeV1Error(w, r, http.StatusGatewayTimeout, "acl_query_timeout",
			"ACL read-permission check timed out", "check server logs")
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError, "acl_query_failed",
		"ACL read-permission check failed", "check server logs")
}

// --- Relation Handlers ---

func (a *App) handleV1EntityRelations(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	// ACL gate (TKT-VQGN CRIT-2): /relations on a hidden entity 404s
	// indistinguishably. Without the gate the endpoint confirms
	// existence (200 vs 404) AND leaks the full neighbor-id set —
	// closing one channel via /include filter while leaving this open
	// would defeat the per-entity-response invariant.
	if !a.gateReadOrNotFound(w, r, typeName, entityID) {
		return
	}

	s := a.State()
	entity, found := a.reader.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	outgoing := a.reader.outgoingRelations(r.Context(), entityID)
	incoming := a.reader.incomingRelations(r.Context(), entityID)

	// Gate hidden neighbors (BUG-ABXMAV / RR-HJV8CP). Without this, a hidden
	// peer's `type` (via the ungated entityType read) and edge `meta` leak past
	// the source-only read gate. Mirror the list path (handleV1ListEntities):
	// compute the visible neighbor set in one type-batched pass and drop any
	// edge whose peer is not visible BEFORE reading its type. A visible neighbor
	// stays; a hidden one is dropped (no over-filtering).
	// TODO(BUG-ABXMAV): the "gate a set of neighbor edges" step is now
	// replicated here, in handleV1GetRelationType, and in the list path — a
	// shared chokepoint (P3) would collapse the three; deferred to avoid
	// churning the list path in a security fix.
	visibleNeighbors := visibleRelationIDs(r.Context(), a.reader, a.visibleReader,
		neighborIDsOf(outgoing, incoming))

	relations := make(map[string][]map[string]any)

	// Track the sort property per group, derived from the relation type's
	// Orderable mode. Empty string disables sorting for that group.
	groupSortProp := make(map[string]string)

	// pendingStrips defers per-edge relation-meta redaction (TKT-B1F5Q1) until
	// AFTER sortRelationGroup runs (see relationMetaStrip). Each strip also carries
	// the relation type so the redaction resolves the right grant block.
	type typedStrip struct {
		relationMetaStrip
		relType string
	}
	var pendingStrips []typedStrip

	for _, edge := range outgoing {
		if !visibleNeighbors[edge.To] {
			continue
		}
		rel := map[string]any{
			"id":        edge.To,
			"type":      a.reader.entityType(r.Context(), edge.To),
			"direction": "outgoing",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
			pendingStrips = append(pendingStrips, typedStrip{
				relationMetaStrip: relationMetaStrip{rel: rel}, relType: edge.Type,
			})
		}
		relations[edge.Type] = append(relations[edge.Type], rel)
		if relDef, ok := s.Meta.Relations[edge.Type]; ok {
			if p := relDef.OutgoingOrderProperty(); p != "" {
				groupSortProp[edge.Type] = p
			}
		}
	}

	for _, edge := range incoming {
		if !visibleNeighbors[edge.From] {
			continue
		}
		relDef, ok := s.Meta.Relations[edge.Type]
		if !ok {
			continue
		}
		inverseName := inverseRelationKey(edge.Type, relDef)
		rel := map[string]any{
			"id":        edge.From,
			"type":      a.reader.entityType(r.Context(), edge.From),
			"direction": "incoming",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
			// The relation grant lives on the SOURCE entity type (edge.From for an
			// incoming edge — the peer). Resolved fail-closed at strip time.
			pendingStrips = append(pendingStrips, typedStrip{
				relationMetaStrip: relationMetaStrip{rel: rel, incoming: true, peerID: edge.From},
				relType:           edge.Type,
			})
		}
		relations[inverseName] = append(relations[inverseName], rel)
		if p := relDef.IncomingOrderProperty(); p != "" {
			groupSortProp[inverseName] = p
		}
	}

	// Sort each group by its managed order property; missing values last;
	// ties stable on insertion order.
	for groupName, prop := range groupSortProp {
		if prop == "" {
			continue
		}
		sortRelationGroup(relations[groupName], prop)
	}

	// Redact hidden relation meta AFTER sorting, through the shared chokepoint.
	for _, ps := range pendingStrips {
		a.affordances.redactRelationMetaStrip(r.Context(), ps.relationMetaStrip, entity, ps.relType)
	}

	writeV1JSON(w, http.StatusOK, relations)
}

// relationMetaStrip is a deferred relation-meta redaction (TKT-B1F5Q1): the built
// wire `rel` map plus what is needed to resolve the source's `visible:` grants.
// The strip runs AFTER sortRelationGroup because the sort reads the managed order
// property out of `meta`, so redacting a (possibly hidden) order key first would
// break ordering. An outgoing edge's source is the path entity; an incoming edge's
// source is the peer (peerID), resolved fail-closed at strip time.
type relationMetaStrip struct {
	rel      map[string]any
	incoming bool
	peerID   string // incoming only: the source (from) entity id
}

// buildRelationTypeRows builds the single-relation-type wire rows (id/type[/meta])
// for the visible edges of relType in the given direction, plus the deferred meta
// strips. It is the shared build step for handleV1GetRelationType; the caller
// sorts then applies [App.redactRelationMetaStrip].
func buildRelationTypeRows(
	ctx context.Context, reader entityReader, edges []*entityPkg.Relation,
	relType string, incoming bool, visibleNeighbors map[string]bool,
) (rows []map[string]any, strips []relationMetaStrip) {
	rows = make([]map[string]any, 0, len(edges))
	for _, edge := range edges {
		if edge.Type != relType {
			continue
		}
		peerID := edge.To
		if incoming {
			peerID = edge.From
		}
		if !visibleNeighbors[peerID] {
			continue
		}
		rel := map[string]any{"id": peerID, "type": reader.entityType(ctx, peerID)}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
			s := relationMetaStrip{rel: rel, incoming: incoming}
			if incoming {
				s.peerID = peerID
			}
			strips = append(strips, s)
		}
		rows = append(rows, rel)
	}
	return rows, strips
}

// sortRelationGroup sorts a relation group in place by a numeric meta key.
// Entries without a finite numeric value at prop sort last; ties stable.
func sortRelationGroup(group []map[string]any, prop string) {
	if len(group) < 2 || prop == "" {
		return
	}
	value := func(m map[string]any) (float64, bool) {
		meta, ok := m["meta"].(map[string]any)
		if !ok {
			return 0, false
		}
		return entitymanager.FiniteOrder(meta[prop])
	}
	sort.SliceStable(group, func(i, j int) bool {
		vi, oki := value(group[i])
		vj, okj := value(group[j])
		switch {
		case oki && !okj:
			return true
		case !oki && okj:
			return false
		case !oki && !okj:
			return false
		}
		return vi < vj
	})
}

func (a *App) handleV1EntityRelationType(w http.ResponseWriter, r *http.Request, typeName, entityID, relType string) {
	switch r.Method {
	case http.MethodGet:
		a.handleV1GetRelationType(w, r, typeName, entityID, relType)
	case http.MethodPost:
		a.write.handleV1CreateRelation(w, r, typeName, entityID, relType)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// resolveRelationEndpoints returns the from/to entity IDs for a relation operation,
// swapping them when direction is incoming.
func resolveRelationEndpoints(entityID, peerID, direction string) (from, to string) {
	if direction == string(DirectionIncoming) {
		return peerID, entityID
	}
	return entityID, peerID
}

func (a *App) handleV1GetRelationType(w http.ResponseWriter, r *http.Request, typeName, entityID, relType string) {
	// ACL gate (TKT-VQGN CRIT-2): see handleV1EntityRelations.
	if !a.gateReadOrNotFound(w, r, typeName, entityID) {
		return
	}

	entity, found := a.reader.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	incoming := r.URL.Query().Get("direction") == string(DirectionIncoming)

	var edges []*entityPkg.Relation
	if incoming {
		edges = a.reader.incomingRelations(r.Context(), entityID)
	} else {
		edges = a.reader.outgoingRelations(r.Context(), entityID)
	}

	// Gate hidden neighbors (BUG-ABXMAV / RR-HJV8CP): same rationale as
	// handleV1EntityRelations. Collect this relation-type's peers and compute
	// the visible set in one batched pass, then skip any hidden peer BEFORE
	// reading its type (so its type/meta never reach the wire). See the
	// shared-chokepoint TODO in handleV1EntityRelations.
	var peerIDs []string
	for _, edge := range edges {
		if edge.Type != relType {
			continue
		}
		peerID := edge.To
		if incoming {
			peerID = edge.From
		}
		peerIDs = append(peerIDs, peerID)
	}
	visibleNeighbors := visibleRelationIDs(r.Context(), a.reader, a.visibleReader, peerIDs)

	relations, pendingStrips := buildRelationTypeRows(r.Context(), a.reader, edges, relType, incoming, visibleNeighbors)

	// Apply orderable sort when the type declares the relevant side.
	if relDef, ok := a.State().Meta.Relations[relType]; ok {
		var prop string
		if incoming {
			prop = relDef.IncomingOrderProperty()
		} else {
			prop = relDef.OutgoingOrderProperty()
		}
		if prop != "" {
			sortRelationGroup(relations, prop)
		}
	}

	// Redact hidden relation meta after sorting (see relationMetaStrip).
	for _, ps := range pendingStrips {
		a.affordances.redactRelationMetaStrip(r.Context(), ps, entity, relType)
	}

	writeV1JSON(w, http.StatusOK, relations)
}

func (a *App) handleV1RelationTarget(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string,
) {
	switch r.Method {
	case http.MethodPatch:
		a.write.handleV1UpdateRelation(w, r, typeName, entityID, relType, targetID)
	case http.MethodDelete:
		a.write.handleV1DeleteRelation(w, r, typeName, entityID, relType, targetID)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// --- Action Handlers ---

func (a *App) handleV1EntityAction(w http.ResponseWriter, r *http.Request, typeName, entityID, action string) {
	if r.Method != http.MethodPost {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	switch action {
	case "clone":
		a.write.handleV1CloneEntity(w, r, typeName, entityID)
	default:
		writeV1Error(w, r, http.StatusNotFound, "unknown_action", "Unknown action", "")
	}
}

// --- System Handlers ---

func (a *App) handleV1Schema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	s := a.State()
	schema := v1.Schema{
		Entities:  make(map[string]v1.EntityType),
		Relations: make(map[string]v1.RelationType),
		Types:     make(map[string]v1.CustomType),
	}

	for name, def := range s.Meta.Entities {
		et := v1.EntityType{
			Label:       def.Label,
			Plural:      def.GetPlural(name),
			Description: def.Description,
			Primary:     def.GetPrimaryProperty(),
			IDType:      def.GetIDType(),
			Properties:  make(map[string]v1.PropertyDef),
		}
		prefixes := def.GetIDPrefixes()
		if len(prefixes) > 0 {
			et.IDPrefix = prefixes[0]
			et.IDPrefixes = prefixes
		}
		for propName, propDef := range def.Properties {
			et.Properties[propName] = a.toV1PropertyDef(s.Meta, propDef)
		}
		schema.Entities[name] = et
	}

	for name, def := range s.Meta.Relations {
		rt := v1.RelationType{
			Label:       def.Label,
			Description: def.Description,
			From:        def.From,
			To:          def.To,
			Symmetric:   def.Symmetric,
			MinOutgoing: def.MinOutgoing,
			MaxOutgoing: def.MaxOutgoing,
			MinIncoming: def.MinIncoming,
			MaxIncoming: def.MaxIncoming,
		}
		if def.Inverse != nil && def.Inverse.ID != "" {
			rt.Inverse = &v1.InverseDef{ID: def.Inverse.ID, Label: def.Inverse.Label}
		}
		if len(def.Properties) > 0 {
			rt.Properties = make(map[string]v1.PropertyDef, len(def.Properties))
			for propName, propDef := range def.Properties {
				rt.Properties[propName] = a.toV1PropertyDef(s.Meta, propDef)
			}
		}
		if def.OutgoingOrderProperty() != "" || def.IncomingOrderProperty() != "" {
			rt.Orderable = &v1.RelationOrderable{
				Outgoing: def.OutgoingOrderProperty() != "",
				Incoming: def.IncomingOrderProperty() != "",
			}
		}
		schema.Relations[name] = rt
	}

	for name, def := range s.Meta.Types {
		schema.Types[name] = toV1CustomType(def)
	}

	writeV1JSON(w, http.StatusOK, schema)
}

func (a *App) handleV1SchemaRoutes(w http.ResponseWriter, r *http.Request) {
	s := a.State()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_schema/")

	switch {
	case path == "types":
		// List entity type names
		names := make([]string, 0, len(s.Meta.Entities))
		for name := range s.Meta.Entities {
			names = append(names, name)
		}
		sort.Strings(names)
		writeV1JSON(w, http.StatusOK, names)

	case strings.HasPrefix(path, "types/"):
		// Get specific entity type
		typeName := strings.TrimPrefix(path, "types/")
		def, ok := s.Meta.Entities[typeName]
		if !ok {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity type not found", "")
			return
		}
		et := v1.EntityType{
			Label:       def.Label,
			Plural:      def.GetPlural(typeName),
			Description: def.Description,
			Primary:     def.GetPrimaryProperty(),
			IDType:      def.GetIDType(),
			Properties:  make(map[string]v1.PropertyDef),
		}
		if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
			et.IDPrefix = prefixes[0]
			et.IDPrefixes = prefixes
		}
		for propName, propDef := range def.Properties {
			et.Properties[propName] = a.toV1PropertyDef(s.Meta, propDef)
		}
		writeV1JSON(w, http.StatusOK, et)

	case path == "relations":
		writeV1JSON(w, http.StatusOK, s.Meta.Relations)

	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

// resolveRelationWidgets returns a copy of rels with any empty Widget set to
// "cards" when the relation type has edge properties/content. Shared by the
// flat and wizard-step relation lists in handleV1Config.
func resolveRelationWidgets(s *Schema, rels []dataentryconfig.FormRelation) []dataentryconfig.FormRelation {
	resolved := make([]dataentryconfig.FormRelation, len(rels))
	copy(resolved, rels)
	for i := range resolved {
		if resolved[i].Widget == "" {
			if def, ok := s.Meta.GetRelationDef(resolved[i].Relation); ok && def.HasAdvancedFeatures() {
				resolved[i].Widget = WidgetCards
			}
		}
	}
	return resolved
}

// aboutDescription is the deployment description shown by the SPA's global
// "About" help (TKT-DUQBD0). The data-entry.yaml `app.description` wins when set
// (it is the UI-app-specific text); otherwise it falls back to the metamodel's
// top-level `description` (the schema-level prose added in TKT-0YBFT8). Empty
// when neither is set — the SPA hides the About button.
//
// This is a SEPARATE field from AppConfig.Description on the wire: that field is
// also rendered by SettingsView as a plain one-line value, and pushing the
// (possibly multi-paragraph markdown) metamodel description into it would
// regress that view. The About surface gets its own field, `about_description`.
func aboutDescription(s *Schema) string {
	if s.Cfg != nil && s.Cfg.App.Description != "" {
		return s.Cfg.App.Description
	}
	if s.Meta != nil {
		return s.Meta.Description
	}
	return ""
}

func (a *App) handleV1Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	s := a.State()
	// Resolve relation widgets: auto-detect "cards" for relations with
	// properties/content, for both single-page (flat) and wizard (step) forms.
	forms := make(map[string]dataentryconfig.Form, len(s.Cfg.Forms))
	for id, form := range s.Cfg.Forms {
		f := form
		f.Relations = resolveRelationWidgets(s, f.Relations)
		if len(f.Steps) > 0 {
			steps := make([]dataentryconfig.FormStep, len(f.Steps))
			copy(steps, f.Steps)
			for i := range steps {
				steps[i].Relations = resolveRelationWidgets(s, steps[i].Relations)
			}
			f.Steps = steps
		}
		forms[id] = f
	}

	config := v1.Config{
		App: v1.AppConfig{
			Name:              s.Cfg.App.Name,
			Description:       s.Cfg.App.Description,
			PlantUMLServerURL: s.Cfg.App.PlantUMLServerURL,
		},
		AboutDescription: aboutDescription(s),
		Styles:           s.StyleMap,
		Forms:            forms,
		Lists:            s.Cfg.Lists,
		Views:            s.Cfg.Views,
		EntityViews:      s.Cfg.EntityViews,
		Kanbans:          s.Cfg.Kanbans,
		Dashboard:        s.Cfg.Dashboard,
		Actions:          s.Cfg.Actions,
		Navigation:       visibleNavigation(r.Context(), s),
		Documents:        visibleDocuments(r.Context(), s),
		Apps:             appsToV1(a.scanAppsOrLog()),
		Palette:          a.palette.Resolved(),
	}

	writeV1JSON(w, http.StatusOK, config)
}

func (a *App) handleV1Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeV1JSON(w, http.StatusOK, v1.ListResponse{Data: []v1.Entity{}, Meta: v1.ListMeta{}})
		return
	}

	// executeQuery is read-gated (TKT-BA8BSX): only entities the
	// request principal may read come back, and gate/load/search
	// failures surface instead of silently truncating.
	entities, err := a.executeQuery(r.Context(), query)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}

	// Apply type filter if provided
	if typeFilter := r.URL.Query().Get("type"); typeFilter != "" {
		filtered := make([]*entityPkg.Entity, 0)
		for _, e := range entities {
			if e.Type == typeFilter {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	meta := a.State().Meta
	data := make([]v1.Entity, 0, len(entities))
	for _, e := range entities {
		entityDef := meta.Entities[e.Type]
		plural := entityDef.GetPlural(e.Type)
		// includeRelations stays false on search results: the read gate
		// covers root entities only, and a relation map would expose
		// {ID, Title} of related entities this principal may not read.
		// Flipping this requires per-target gating first (RR-QO01XY) —
		// TestACLSearch_VisibleHitRelatedToHidden pins the invariant.
		data = append(data, a.serializer.forWireRelated(r.Context(), e, nil, nil, nil, a.Meta(), plural))
	}

	resp := v1.ListResponse{
		Data: data,
		Meta: v1.ListMeta{
			Total:   len(data),
			Page:    1,
			PerPage: len(data),
		},
	}

	writeV1JSON(w, http.StatusOK, resp)
}

func (a *App) handleV1Analyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	analysisResult := a.analyze.runAnalysis(r.Context(), a.State().Meta)

	// runAnalysis now reads GATED per the requesting principal (TKT-3FL2S6), so
	// a hidden entity never enters a check and a visible entity is redacted
	// before its value can reach an issue title or message. No post-hoc output
	// filter is needed — the issues are already the requester's visible slice.
	// Flatten sections to (issue, section) pairs for the wire builder.
	visible := flattenIssues(analysisResult.Sections)

	result := APIAnalysisResult{
		Issues:  make([]APIIssue, 0, len(visible)),
		ByCheck: make(map[string]int),
	}

	// Loopback gate: same policy as action / document surfaces.
	// Non-loopback callers get a degraded envelope on script-error
	// issues (no source slice, no stack, no captured output).
	fullDetail := a.allowFullScriptDetail(r)

	for _, vi := range visible {
		issue, section := vi.issue, vi.section
		api := APIIssue{
			EntityID:   issue.EntityID,
			EntityType: issue.EntityType,
			Title:      issue.Title,
			Message:    issue.Message,
			Severity:   issue.Severity,
			CheckType:  section,
			Detail:     issue.Detail,
		}
		if issue.ScriptError != nil {
			env := buildScriptErrorEnvelope(issue.ScriptError, fullDetail, "")
			api.ScriptError = &env
		}
		result.Issues = append(result.Issues, api)
		result.ByCheck[section]++
		switch issue.Severity {
		case "error":
			result.Errors++
		case "warning":
			result.Warnings++
		}
	}

	writeV1JSON(w, http.StatusOK, result)
}

// visibleIssue pairs an analysis issue with its section name, carried
// through the ACL filter so the wire builder keeps the issue→check
// association without re-walking sections.
type visibleIssue struct {
	issue   AnalysisIssue
	section string
}

// flattenIssues walks the analysis sections into (issue, section) pairs for the
// wire builder. No filtering: runAnalysis already read gated, so the issues are
// the requester's visible slice — a hidden entity produced no issue and a
// visible entity was redacted before its value could reach a title or message
// (TKT-3FL2S6, replacing the former visibleAnalysisIssues output filter).
func flattenIssues(sections []AnalysisSection) []visibleIssue {
	var out []visibleIssue
	for _, section := range sections {
		for _, issue := range section.Issues {
			out = append(out, visibleIssue{issue: issue, section: section.Name})
		}
	}
	return out
}

// --- Helper Functions ---

func (a *App) resolveV1Includes(ctx context.Context, entity *entityPkg.Entity, includes string) map[string]v1.Entity {
	s := a.State()
	included := make(map[string]v1.Entity)

	// Collect candidate target entities first; filter via the ACL
	// gate per-type (batched), then serialize the survivors. The
	// two-phase shape exists to satisfy RR-M84L (the include channel
	// must respect the per-entity visibility rule) AND RR-FRK1 (a
	// hub entity with 50 neighbors must not cost 50 GraphCount
	// round-trips).
	var candidates []*entityPkg.Entity
	// nestedFor maps target.ID → the remaining nested-include
	// expression (e.g. "implements.requires" → "requires" stored
	// against the implements target). Recurses after the visibility
	// filter so hidden neighbors don't trigger hidden nested probes.
	nestedFor := make(map[string]string)

	if includes == "*" { //nolint:nestif // include-all vs. named-include expansion is inherently nested; flattening would obscure the two-mode logic.
		for _, edge := range a.reader.outgoingRelations(ctx, entity.ID) {
			if target, found := a.reader.getEntity(ctx, edge.To); found {
				candidates = append(candidates, target)
			}
		}
		for _, edge := range a.reader.incomingRelations(ctx, entity.ID) {
			if source, found := a.reader.getEntity(ctx, edge.From); found {
				candidates = append(candidates, source)
			}
		}
	} else {
		for part := range strings.SplitSeq(includes, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			relParts := strings.SplitN(part, ".", 2)
			relType := relParts[0]
			for _, edge := range a.reader.outgoingRelations(ctx, entity.ID) {
				if edge.Type != relType {
					continue
				}
				target, found := a.reader.getEntity(ctx, edge.To)
				if !found {
					continue
				}
				candidates = append(candidates, target)
				if len(relParts) > 1 {
					nestedFor[target.ID] = relParts[1]
				}
			}
		}
	}

	visible := a.filterVisibleIncludes(ctx, candidates)
	for _, target := range visible {
		entityDef := s.Meta.Entities[target.Type]
		plural := entityDef.GetPlural(target.Type)
		included[target.ID] = a.serializer.forWireRelated(ctx, target, nil, nil, nil, a.Meta(), plural)

		if nested, ok := nestedFor[target.ID]; ok {
			maps.Copy(included, a.resolveV1Includes(ctx, target, nested))
		}
	}
	return included
}

// filterVisibleIncludes drops any candidate the principal cannot read,
// batched by entity type. For each distinct type ONE gate call
// (PermitsReadMany over every candidate of that type) — turning a
// worst case of O(N) per-id probes into O(distinct-types). RR-FRK1.
//
// On gate error: drop the whole type's candidates (fail-closed) and
// log loud so operators see the underlying failure rather than just
// "the include block is empty." The include channel is a "best
// effort" affordance; the explicit GET / list endpoints are the
// authoritative read surface.
func (a *App) filterVisibleIncludes(ctx context.Context, candidates []*entityPkg.Entity) []*entityPkg.Entity {
	return a.visibleReader.filterVisible(ctx, candidates)
}

// applyV1Filters applies `filter[...]` query params to the entity slice. Pure
// data transform — a free function, not App behavior (TKT-N26KLB M5.5).
// A malformed key or unsupported operator returns errBadFilter (HTTP 400)
// instead of silently dropping the clause, which returned the unfiltered
// superset and hid config/caller typos.
//
// isRelationKey identifies filter keys that target a metamodel relation rather
// than an entity property; those are skipped here and handled by the relation
// pass in scopedSortedEntities (which has ctx + config for direction-aware
// traversal). A relation key would otherwise match no property and nuke the
// whole result set (TKT-5U7QBR). A nil predicate treats every key as a
// property filter (the pre-TKT-5U7QBR behavior).
//
//nolint:gocognit,funlen // dispatches over every supported filter operator and value type; each branch is one operator's semantics, not extractable shared logic.
func applyV1Filters(
	entities []*entityPkg.Entity, query map[string][]string, _ string, isRelationKey func(string) bool,
) ([]*entityPkg.Entity, error) {
	filtered := entities

	for key, values := range query {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}

		// Parse filter[property] or filter[property][operator] or
		// filter[property][operator][] (multi-value array form). Strip the
		// optional `[]` array suffix before splitting so we get clean parts.
		filterKey := strings.TrimPrefix(key, "filter[")
		filterKey = strings.TrimSuffix(filterKey, "]")
		filterKey = strings.TrimSuffix(filterKey, "][") // was "...[]"
		parts := strings.Split(filterKey, "][")

		// Validate parsed shape. A malformed key like `filter[prop][][weird]`
		// produces parts=["prop", "", "weird"] — more than 2 parts or an
		// empty property/operator means the URL is bogus. Reject the request:
		// the old skip-with-a-log "fail closed" still returned the unfiltered
		// superset, which is fail-open at the result level.
		if len(parts) > 2 {
			return nil, fmt.Errorf("%w: key %q has too many segments", errBadFilter, key)
		}
		property := parts[0]
		if property == "" {
			return nil, fmt.Errorf("%w: key %q has an empty property", errBadFilter, key)
		}

		// Relation filter keys are handled by the direction-aware relation
		// pass in scopedSortedEntities. Skip them here so a relation key
		// (which matches no property) doesn't nuke the result set.
		if isRelationKey != nil && isRelationKey(property) {
			continue
		}

		operator := "eq"
		if len(parts) == 2 {
			if parts[1] == "" {
				return nil, fmt.Errorf("%w: key %q has an empty operator segment", errBadFilter, key)
			}
			operator = parts[1]
		}

		// Reject unknown operators BEFORE the per-entity loop. A typo like
		// `filter[status][equals]=done` used to be skipped with only a log —
		// which returned every entity, the very silent inclusion the skip
		// claimed to prevent. A 400 makes the typo visible to the caller
		// (and to a config author whose list filter reached the wire).
		switch operator {
		case "eq", "ne", "contains", "in", "lt", "lte", "gt", "gte":
			// known
		default:
			return nil, fmt.Errorf("%w: unknown operator %q in key %q (supported: eq, ne, contains, in, lt, lte, gt, gte)",
				errBadFilter, operator, key)
		}

		// Multi-value support: `in`/`ne` collect ALL repeated values from the
		// query (e.g. `filter[tags][in][]=a&filter[tags][in][]=b`) and join
		// them with commas, matching the comma-separated form. Other
		// operators stay last-write-wins on values[len-1] for predictability.
		var value string
		if operator == "in" || operator == "ne" {
			value = resolveFilterVariablesInList(strings.Join(values, ","))
		} else {
			value = resolveFilterVariable(values[len(values)-1])
		}

		var newFiltered []*entityPkg.Entity
		for _, e := range filtered {
			propVal, ok := e.Properties[property]
			if !ok {
				if operator == "eq" && value == "" {
					newFiltered = append(newFiltered, e)
				}
				continue
			}

			propStr := fmt.Sprintf("%v", propVal)

			switch operator {
			case "eq":
				if propStr == value {
					newFiltered = append(newFiltered, e)
				}
			case "ne":
				// Support comma-separated values as NOT IN
				vals := strings.Split(value, ",")
				excluded := false
				for _, v := range vals {
					if propStr == strings.TrimSpace(v) {
						excluded = true
						break
					}
				}
				if !excluded {
					newFiltered = append(newFiltered, e)
				}
			case "contains":
				if strings.Contains(strings.ToLower(propStr), strings.ToLower(value)) {
					newFiltered = append(newFiltered, e)
				}
			case "in":
				vals := strings.Split(value, ",")
				for _, v := range vals {
					if propStr == strings.TrimSpace(v) {
						newFiltered = append(newFiltered, e)
						break
					}
				}
			case "lt", "lte", "gt", "gte":
				match, err := compareValues(propStr, value, operator)
				if err != nil {
					// Type mismatch (e.g. property is a date, filter value isn't).
					// Exclude the entity rather than silently lying via lexicographic
					// fallback. Log so users notice.
					slog.Warn("filter compare error", "property", property, "error", err)
					continue
				}
				if match {
					newFiltered = append(newFiltered, e)
				}
			}
		}
		filtered = newFiltered
	}

	return filtered, nil
}

// parseSortParam parses the `sort=` query param into ordered sort specs:
// "-created,title" means descending created, then ascending title. Returns nil
// when no sort was requested.
//
// Shared by every consumer of the grammar — the list pipeline that APPLIES the
// sort and the export context that REPORTS it — so the two cannot drift into
// describing different orderings.
func parseSortParam(query map[string][]string) []filter.SortSpec {
	sortParam := queryGet(query, "sort")
	if sortParam == "" {
		return nil
	}
	var specs []filter.SortSpec
	for part := range strings.SplitSeq(sortParam, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		spec := filter.SortSpec{Direction: "asc"}
		if after, found := strings.CutPrefix(part, "-"); found {
			spec.Direction = "desc"
			part = after
		}
		spec.Property = part
		specs = append(specs, spec)
	}
	return specs
}

// applyV1Sorting applies `sort=` query params to the entity slice. Pure data
// transform — a free function, not App behavior (TKT-N26KLB M5.5).
func applyV1Sorting(entities []*entityPkg.Entity, query map[string][]string) []*entityPkg.Entity {
	sortSpecs := parseSortParam(query)
	if len(sortSpecs) == 0 {
		return entities
	}

	sorted := make([]*entityPkg.Entity, len(entities))
	copy(sorted, entities)

	sort.Slice(sorted, func(i, j int) bool {
		for _, spec := range sortSpecs {
			vi := sorted[i].Properties[spec.Property]
			vj := sorted[j].Properties[spec.Property]

			si := fmt.Sprintf("%v", vi)
			sj := fmt.Sprintf("%v", vj)

			if si == sj {
				continue
			}

			if spec.IsDescending() {
				return si > sj
			}
			return si < sj
		}
		return false
	})

	return sorted
}

func parseV1Pagination(query map[string][]string) (page, perPage int) {
	page = 1
	perPage = 25

	if vals, ok := query["page"]; ok && len(vals) > 0 {
		if p, err := strconv.Atoi(vals[0]); err == nil && p > 0 {
			page = p
		}
	}

	if vals, ok := query["per_page"]; ok && len(vals) > 0 {
		if pp, err := strconv.Atoi(vals[0]); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	return page, perPage
}

// addPaginationLinks writes RFC 8288 Link headers for the collection page.
// Pure data transform over its args — a free function (TKT-N26KLB M5.5).
func addPaginationLinks(w http.ResponseWriter, _ *http.Request, page, perPage, total int, plural string) {
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}

	baseURL := "/api/v1/" + plural
	var links []string

	// First
	links = append(links, fmt.Sprintf("<%s?page=1&per_page=%d>; rel=\"first\"", baseURL, perPage))

	// Prev
	if page > 1 {
		links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"prev\"", baseURL, page-1, perPage))
	}

	// Next
	if page < totalPages {
		links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"next\"", baseURL, page+1, perPage))
	}

	// Last
	links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"last\"", baseURL, totalPages, perPage))

	w.Header().Set("Link", strings.Join(links, ", "))
}

func (a *App) computeEntityETag(ctx context.Context, e *entityPkg.Entity) string {
	h := sha256.New()
	_, _ = h.Write([]byte(e.ID))
	_, _ = h.Write([]byte(e.Type))
	_, _ = h.Write([]byte(e.Content))

	// Sort properties so the hash is stable across map iteration order.
	propKeys := make([]string, 0, len(e.Properties))
	for k := range e.Properties {
		propKeys = append(propKeys, k)
	}
	sort.Strings(propKeys)
	for _, k := range propKeys {
		_, _ = h.Write([]byte(k))
		_, _ = fmt.Fprintf(h, "=%v;", e.Properties[k])
	}

	// Fold outgoing relations into the hash so PATCHes that only change
	// edges also change the ETag — otherwise If-Match / If-None-Match
	// round-trips poison client caches.
	edges := a.reader.outgoingRelations(ctx, e.ID)
	edgeKeys := make([]string, 0, len(edges))
	for _, edge := range edges {
		edgeKeys = append(edgeKeys, edge.Type+"|"+edge.To)
	}
	sort.Strings(edgeKeys)
	for _, k := range edgeKeys {
		_, _ = h.Write([]byte("r:"))
		_, _ = h.Write([]byte(k))
	}

	sum := h.Sum(nil)
	return fmt.Sprintf("\"%s\"", base64.StdEncoding.EncodeToString(sum[:8]))
}

// validateCreateIDOpts enforces that `id` is only accepted for manual-ID types
// and that `prefix` is only accepted for non-manual types with a known prefix.
// For manual-ID types that declare one or more prefixes, the `id` must start
// with one of them AND include a non-empty suffix. Surrounding whitespace on
// the inputs is trimmed at the boundary so the error message lines up with
// what the user actually typed. Returns an empty string when valid.
func validateCreateIDOpts(def *metamodel.EntityDef, id, prefix string) string {
	id = strings.TrimSpace(id)
	prefix = strings.TrimSpace(prefix)

	if id != "" && !def.IsManualID() {
		return "id not accepted for non-manual ID type; use 'prefix' instead"
	}
	if id != "" && def.IsManualID() {
		if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
			matched := false
			for _, p := range prefixes {
				// Reject the bare prefix (id == p) — the entity needs a
				// distinguishing suffix or it conflicts with the prefix
				// concept itself. (RR-…)
				if strings.HasPrefix(id, p) && len(id) > len(p) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Sprintf("id %q must start with one of %v and include a suffix", id, prefixes)
			}
		}
	}
	if prefix == "" {
		return ""
	}
	if def.IsManualID() {
		return "prefix not applicable to manual ID type"
	}
	if slices.Contains(def.GetIDPrefixes(), prefix) {
		return ""
	}
	return fmt.Sprintf("prefix %q is not valid; allowed: %v", prefix, def.GetIDPrefixes())
}

func writeV1JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeV1Error(w http.ResponseWriter, r *http.Request, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	err := v1.Error{
		Type:     "https://rela.dev/errors/" + errType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}

	_ = json.NewEncoder(w).Encode(err)
}

// --- Side Panel API ---

// --- Sidebar API ---

// --- Conflicts API ---

// handleV1Conflicts returns the list of conflicted files as JSON.
func (a *App) handleV1Conflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	ctx := &project.Context{
		Root:         a.paths.Root,
		EntitiesDir:  a.paths.EntitiesDir,
		RelationsDir: a.paths.RelationsDir,
	}

	result, err := conflict.DetectAll(ctx)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "conflict_detection_failed", "Failed to detect conflicts", err.Error())
		return
	}

	items := make([]v1.ConflictItem, 0, len(result.Files))
	for _, cf := range result.Files {
		relPath, _ := filepath.Rel(ctx.Root, cf.Path)
		items = append(items, v1.ConflictItem{
			Path:        relPath,
			EntityType:  cf.EntityType,
			EntityID:    cf.EntityID,
			MarkerCount: len(cf.Markers),
		})
	}

	writeV1JSON(w, http.StatusOK, v1.ConflictsResponse{
		Conflicts: items,
		Count:     len(items),
	})
}

// handleV1ConflictRoutes handles GET /api/v1/_conflicts/{path} and POST /api/v1/_conflicts/resolve.
func (a *App) handleV1ConflictRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_conflicts/")

	if path == "resolve" && r.Method == http.MethodPost {
		a.write.handleV1ConflictResolve(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Get conflict details. The path is caller-supplied — contain it to
	// the project root before any filesystem access.
	absPath, ok := resolveConflictPath(a.paths, w, r, path)
	if !ok {
		return
	}

	cf, err := conflict.ParseConflictedFile(absPath, a.State().Meta)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "parse_failed", "Failed to parse conflict", err.Error())
		return
	}

	info := conflict.AnalyzeConflict(cf)

	diffs := make([]v1.PropertyDiff, 0, len(info.PropertyDiffs))
	for _, d := range info.PropertyDiffs {
		diffs = append(diffs, v1.PropertyDiff{
			Property:    d.Property,
			OursValue:   fmt.Sprintf("%v", d.OursValue),
			TheirsValue: fmt.Sprintf("%v", d.TheirsValue),
			IsSame:      d.IsSame,
		})
	}

	detail := v1.ConflictDetail{
		Path:          path,
		EntityType:    cf.EntityType,
		EntityID:      cf.EntityID,
		PropertyDiffs: diffs,
		ContentSame:   info.ContentSame,
		ContentOurs:   info.ContentDiffOurs,
		ContentTheirs: info.ContentDiffTheirs,
	}

	writeV1JSON(w, http.StatusOK, detail)
}

// resolveConflictPath contains the caller-supplied conflict-file path
// to the project root. On failure it writes the error response (404
// when the path is inside the project but missing, 403 when it escapes
// the root) and returns ok=false.
func resolveConflictPath(paths *project.Context, w http.ResponseWriter, r *http.Request, p string) (string, bool) {
	resolved, err := containedProjectPath(paths.Root, p)
	switch {
	case errors.Is(err, errPathNotFound):
		writeV1Error(w, r, http.StatusNotFound, "conflict_not_found", "Conflicted file not found", "")
		return "", false
	case err != nil:
		writeV1Error(w, r, http.StatusForbidden, "path_outside_project", "Path is outside the project root", "")
		return "", false
	}
	return resolved, true
}

// conflictAuditSubject derives the audit subject for a resolved
// conflict write. Exactly one of e / rel is non-nil ([conflict.Resolve]
// errors otherwise).
func conflictAuditSubject(e *entityPkg.Entity, rel *entityPkg.Relation) *audit.Subject {
	if rel != nil {
		return &audit.Subject{
			Kind:         "relation",
			RelationType: rel.Type,
			FromID:       rel.From,
			ToID:         rel.To,
		}
	}
	return &audit.Subject{Kind: "entity", Type: e.Type, ID: e.ID}
}

// --- Documents API ---

// handleV1Documents handles GET /api/v1/_documents/{docName}/{entityId} and
// GET /api/v1/_documents/{docName}. Returns JSON with rendered HTML content
// for Vue SPA consumption.
//
// The two path shapes serve the two document kinds
// (dataentryconfig.DocumentConfig): the two-segment form renders an
// entity-anchored document, the one-segment form a standalone one. Each shape
// REJECTS the other kind rather than inventing or ignoring an entry id — see
// handleV1StandaloneDocument.
func (a *App) handleV1Documents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_documents/{docName}[/{entityId}], then dispatch on
	// the shape. The two branches live in separate functions so neither grows
	// the other's guards by accident — they differ in their ACL story, not
	// just in whether an id is present.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_documents/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 1 || parts[1] == "" {
		handleV1StandaloneDocument(a, w, r, parts[0])
		return
	}
	if parts[0] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_documents/{docName}/{entityId}", "")
		return
	}
	handleV1AnchoredDocument(a, w, r, parts[0], parts[1])
}

// handleV1AnchoredDocument serves GET /api/v1/_documents/{docName}/{entityId}
// — a document declared WITH an `entity_type:`, rendered about one entity.
//
// Gate ordering here is load-bearing; see the comments inline. A plain
// function taking *App for the same reason handleV1StandaloneDocument is one:
// App sits on its plimsoll load line.
func handleV1AnchoredDocument(a *App, w http.ResponseWriter, r *http.Request, docName, entityID string) {
	// Both segments flow into the on-disk document cache filename
	// (workspace/document.go). Reject anything that could escape the cache
	// directory before any filesystem work happens.
	if !isSafePathSegment(docName) || !isSafePathSegment(entityID) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path segment contains forbidden characters", "")
		return
	}

	// Get document config
	docCfg, ok := a.State().Cfg.Documents[docName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "document_not_found", "Document config not found", "")
		return
	}

	// A standalone document has no entry entity, so an entity-anchored
	// request for one is a category error, not a document to render against
	// the supplied id. Reject rather than silently ignoring the id.
	if docCfg.IsStandalone() {
		writeV1Error(w, r, http.StatusBadRequest, "document_kind_mismatch",
			fmt.Sprintf("document %q has no entity_type; request it at /_documents/%s without an entity id",
				docName, docName), "")
		return
	}

	// Enforce the doc's entity_type before running the renderer: a
	// release-notes script authored for releases must not run against a
	// ticket. The frontend already filters the docs shown for an entity,
	// but an HTTP caller can hit /_documents/<doc>/<wrong-type-id>
	// directly; reject here.
	ent, entErr := a.store.GetEntity(r.Context(), entityID)
	if entErr != nil {
		writeV1Error(w, r, http.StatusNotFound, "entity_not_found",
			fmt.Sprintf("entity %q not found", entityID), "")
		return
	}

	// ACL gate (TKT-C0R07J): document rendering serves entity-derived content
	// (HTML + EntityIDs) and may run a Lua script that reads related entities.
	// Gate on the document's declared entity_type BEFORE the type-mismatch
	// branch (so a denied principal gets a uniform 404, not a 400 oracle) and
	// BEFORE any rendering runs — a denied caller must never trigger the
	// (possibly Lua) renderer.
	if !a.gateReadOrNotFound(w, r, docCfg.EntityType, entityID) {
		return
	}

	// A doc-level `permission:` applies IN ADDITION to the per-entity gate
	// above — it narrows, never widens (a holder still needs to pass the
	// entity read gate). Same uniform-404 treatment for the same reason.
	if !gateDocumentPermission(w, r, docCfg) {
		return
	}

	if ent.Type != docCfg.EntityType {
		writeV1Error(w, r, http.StatusBadRequest, "entity_type_mismatch",
			fmt.Sprintf("document %q is for entity_type %q, but %q is a %q",
				docName, docCfg.EntityType, entityID, ent.Type), "")
		return
	}

	renderCfg := a.toDocumentRenderConfig(docName, &docCfg)

	// Check for refresh param - skip cache if present
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	// return_to is the URL the caller is currently on. The rewriter uses
	// it to inject a `return_to` query param into form links so the form
	// redirects back here after submit. isSafeReturnPath enforces the
	// open-redirect guard — it rejects protocol-relative (//evil.com),
	// backslash-tricks (/\evil.com), and any absolute/schemed URLs, and
	// returns the normalised same-origin path on success.
	returnPath := isSafeReturnPath(r.URL.Query().Get("return_to"))

	// Try to get cached content (unless refresh requested). Disk cache
	// is only populated for command: renders (see doRender); skip the
	// read for script: docs so we don't serve a stale command:-era file
	// after a doc is switched to a Lua script.
	if !forceRefresh && docCfg.Script == "" {
		result := a.documents.GetCached(r.Context(), entityID)
		if result != nil {
			html := RewriteDocumentLinks(result.HTML, returnPath, nil)
			writeV1JSON(w, http.StatusOK, v1.DocumentResponse{
				HTML:      html,
				Cached:    true,
				EntityIDs: extractEntityIDs(result.Entities),
			})
			return
		}
	}

	// Render the document
	result, err := a.documents.Render(r.Context(), entityID, renderCfg)
	if err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			correlationID := newCorrelationID()
			slog.Warn("document render failed",
				"document", docName, "entity", entityID,
				"correlation", correlationID, "error", err)
			writeV1ScriptError(w, se, a.allowFullScriptDetail(r), correlationID)
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "render_failed", "Document rendering failed", err.Error())
		return
	}

	html := RewriteDocumentLinks(result.HTML, returnPath, nil)
	writeV1JSON(w, http.StatusOK, v1.DocumentResponse{
		HTML:      html,
		Cached:    false,
		EntityIDs: extractEntityIDs(result.Entities),
	})
}

// extractEntityIDs extracts IDs from a slice of entities.
func extractEntityIDs(entities []*entityPkg.Entity) []string {
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.ID
	}
	return ids
}

// --- Commands API ---

// handleV1Commands returns available commands for a given page context.
// Query params:
//   - page_type: "entity", "list", "view", or "dashboard"
//   - qualifier: specific list ID or view ID (optional)
//   - entity_type: the entity type shown on the page (optional)
func (a *App) handleV1Commands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	query := r.URL.Query()
	pageType := query.Get("page_type")
	qualifier := query.Get("qualifier")
	entityType := query.Get("entity_type")

	resolved := a.commands.resolveCommands(r.Context(), pageType, qualifier, entityType)

	commands := make([]v1.Command, 0, len(resolved))
	for _, cmd := range resolved {
		commands = append(commands, v1.Command(cmd))
	}

	writeV1JSON(w, http.StatusOK, commands)
}

// handleV1Templates returns templates for an entity type.
// GET /api/v1/_templates/{entityType}
func (a *App) handleV1Templates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	} // Extract entity type from path: /api/v1/_templates/{entityType}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_templates/")
	entityType := strings.TrimSuffix(path, "/")

	if entityType == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_entity_type", "Entity type is required", "")
		return
	}

	// Check if entity type exists
	if _, ok := a.State().Meta.Entities[entityType]; !ok {
		writeV1Error(w, r, http.StatusNotFound, "entity_type_not_found",
			fmt.Sprintf("Entity type '%s' not found", entityType), "")
		return
	}

	templates, _ := a.templater.EntityTemplates(r.Context(), entityType)
	result := make([]v1.Template, 0, len(templates))

	for _, t := range templates {
		relations := make([]v1.TemplateRelation, 0, len(t.Relations))
		for _, rel := range t.Relations {
			relations = append(relations, v1.TemplateRelation{
				Relation: rel.Type,
				Target:   rel.Target,
			})
		}
		result = append(result, v1.Template{
			Name:       t.Name,
			Properties: t.Properties,
			Content:    t.Content,
			Relations:  relations,
		})
	}

	writeV1JSON(w, http.StatusOK, result)
}

// --- OpenAPI Spec ---

// handleV1OpenAPI serves the OpenAPI 3.1 specification.
func (a *App) handleV1OpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	data, err := a.State().OpenAPIGen.GenerateJSON()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate OpenAPI spec", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(data)
}

// --- Views API ---

// sectionEntityToV1 lifts a section's row entity (template-side data)
// onto the wire shape. Centralizes the `v1.ViewEntity` construction so
// the typed `_props` and per-row `_fields` (TKT-IHC7D) stay consistent
// across both the top-level entities path and the (currently dormant)
// grouped-card entities path.
func sectionEntityToV1(e SectionEntityData) v1.ViewEntity {
	v1Ent := v1.ViewEntity{
		ID:         e.ID,
		Title:      e.Title,
		Type:       e.Type,
		EditFormID: e.EditFormID,
		Content:    e.Content,
		HasContent: e.HasContent,
		Props:      e.Props,
	}
	for _, f := range e.Fields {
		v1Ent.Fields = append(v1Ent.Fields, v1.SectionField(f))
	}
	if e.FieldVerdicts != nil {
		fa := e.FieldVerdicts
		v1Ent.FieldAffordances = &fa
	}
	return v1Ent
}
