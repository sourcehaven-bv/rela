package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/conflict"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// --- API v1 Types ---

// V1Entity is the JSON representation of an entity for API v1.
type V1Entity struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Title        string                 `json:"_title,omitempty"`
	Properties   map[string]interface{} `json:"properties"`
	Content      string                 `json:"content,omitempty"`
	Relations    map[string][]string    `json:"relations,omitempty"`
	Included     map[string]V1Entity    `json:"included,omitempty"`
	Self         string                 `json:"_self,omitempty"`
	Actions      map[string]bool        `json:"_actions,omitempty"`
	Inaccessible []V1InaccessibleField  `json:"inaccessible,omitempty"`
	// FieldAffordances carries per-field write affordances on per-entity
	// GET responses. Sparse: only fields whose verdict deviates from the
	// permissive default appear. Hidden fields are omitted from
	// `Properties` AND from this map entirely. Pointer semantics
	// distinguish "absent on the wire" (nil pointer; list / mutation
	// responses) from "present and empty" (`{}`; per-entity GET with no
	// deviations under nop resolver — closed-world signal matching the
	// `_actions` precedent).
	FieldAffordances *map[string]V1FieldAffordance `json:"_fields,omitempty"`
	// RelationAffordances carries per-relation-type affordances on
	// per-entity GET responses. Same pointer / closed-world semantics
	// as FieldAffordances.
	RelationAffordances *map[string]V1RelationAffordance `json:"_relations,omitempty"`
	// Warnings lists soft-condition findings surfaced by the write
	// path. Populated only by mutation responses (PATCH); read paths
	// leave it nil. Each warning has a stable `code`, an RFC 6901
	// JSON Pointer `path`, and a human-readable `detail`.
	Warnings []Warning `json:"warnings,omitempty"`
}

// V1FieldAffordance describes per-field write / option affordances on
// the wire. Sparse: `Writable` is nil when the default (writable)
// holds; `Options` lists only the false entries (allowed options are
// implicit via the metamodel). See the closed-world contract in
// docs/data-entry/api-reference.md.
type V1FieldAffordance struct {
	Writable *bool           `json:"writable,omitempty"`
	Options  map[string]bool `json:"options,omitempty"`
}

// V1RelationAffordance describes per-relation-type affordances on the
// wire. Sparse: `Creatable` and `Removable` are nil when the default
// (true) holds. `Fields` lists meta-field writability overrides, also
// sparse.
type V1RelationAffordance struct {
	Creatable *bool                        `json:"creatable,omitempty"`
	Removable *bool                        `json:"removable,omitempty"`
	Fields    map[string]V1FieldAffordance `json:"fields,omitempty"`
}

// V1InaccessibleField describes a property that is known to exist but
// whose value is unreadable by the holder of the entity (e.g. the file
// is git-crypt encrypted and the key is not present locally).
type V1InaccessibleField struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// V1ListResponse is the response for listing entities.
type V1ListResponse struct {
	Data    []V1Entity      `json:"data"`
	Meta    V1ListMeta      `json:"meta"`
	Actions map[string]bool `json:"_actions,omitempty"`
}

// V1ListMeta contains pagination metadata.
type V1ListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// V1Schema is the JSON representation of the metamodel.
type V1Schema struct {
	Entities  map[string]V1EntityType   `json:"entities"`
	Relations map[string]V1RelationType `json:"relations"`
	Types     map[string]V1CustomType   `json:"types,omitempty"`
}

// V1EntityType is the JSON representation of an entity type.
type V1EntityType struct {
	Label       string                   `json:"label"`
	Plural      string                   `json:"plural"`
	Description string                   `json:"description,omitempty"`
	Primary     string                   `json:"primary,omitempty"`
	IDType      string                   `json:"id_type,omitempty"`
	IDPrefix    string                   `json:"id_prefix,omitempty"`
	IDPrefixes  []string                 `json:"id_prefixes,omitempty"`
	Properties  map[string]V1PropertyDef `json:"properties"`
}

// V1PropertyDef is the JSON representation of a property definition.
type V1PropertyDef struct {
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Values      []string `json:"values,omitempty"`
	Description string   `json:"description,omitempty"`
	List        bool     `json:"list,omitempty"`
}

func (a *App) toV1PropertyDef(meta *metamodel.Metamodel, propDef metamodel.PropertyDef) V1PropertyDef {
	pd := V1PropertyDef{
		Type:        propDef.Type,
		Required:    propDef.Required,
		Default:     propDef.Default,
		Description: propDef.Description,
		List:        propDef.List,
	}
	if ct, ok := meta.Types[propDef.Type]; ok {
		pd.Values = ct.Values
	} else if len(propDef.Values) > 0 {
		pd.Values = propDef.Values
	}
	return pd
}

// V1RelationType is the JSON representation of a relation type.
type V1RelationType struct {
	Label       string                   `json:"label"`
	Description string                   `json:"description,omitempty"`
	From        []string                 `json:"from"`
	To          []string                 `json:"to"`
	Inverse     *V1InverseDef            `json:"inverse,omitempty"`
	Symmetric   bool                     `json:"symmetric,omitempty"`
	MinOutgoing *int                     `json:"min_outgoing,omitempty"`
	MaxOutgoing *int                     `json:"max_outgoing,omitempty"`
	MinIncoming *int                     `json:"min_incoming,omitempty"`
	MaxIncoming *int                     `json:"max_incoming,omitempty"`
	Properties  map[string]V1PropertyDef `json:"properties,omitempty"`
	// Orderable, when set, declares that the frontend may offer drag-to-reorder
	// controls on the corresponding side. The managed property names are
	// always the reserved `_order_out` (outgoing) and `_order_in` (incoming).
	Orderable *V1RelationOrderable `json:"orderable,omitempty"`
}

// V1RelationOrderable describes per-side orderability for a relation type.
type V1RelationOrderable struct {
	Outgoing bool `json:"outgoing,omitempty"`
	Incoming bool `json:"incoming,omitempty"`
}

// V1InverseDef mirrors metamodel.InverseDef on the wire. The SPA reads
// `inverse.id` to find the inverse body key for incoming-direction
// edits routed through the unified PATCH (TKT-GFQK).
type V1InverseDef struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// V1CustomType is the JSON representation of a custom type.
type V1CustomType struct {
	Values  []string `json:"values"`
	Default string   `json:"default,omitempty"`
}

// V1Config is the JSON representation of the UI config.
type V1Config struct {
	App         V1AppConfig                                 `json:"app"`
	Styles      map[string]map[string]string                `json:"styles"`
	Forms       map[string]dataentryconfig.Form             `json:"forms"`
	Lists       map[string]dataentryconfig.List             `json:"lists"`
	Views       map[string]dataentryconfig.ViewConfig       `json:"views"`
	EntityViews map[string]dataentryconfig.EntityViewConfig `json:"entity_views,omitempty"`
	Kanbans     map[string]dataentryconfig.Kanban           `json:"kanbans"`
	Dashboard   *dataentryconfig.DashboardConfig            `json:"dashboard,omitempty"`
	Actions     map[string]dataentryconfig.Action           `json:"actions,omitempty"`
	Navigation  []dataentryconfig.NavigationEntry           `json:"navigation"`
	Documents   map[string]dataentryconfig.DocumentConfig   `json:"documents,omitempty"`
	Palette     *dataentryconfig.ResolvedPalette            `json:"palette,omitempty"`
}

// V1AppConfig is the JSON representation of the app config.
type V1AppConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// V1Error is an RFC 7807 Problem Details response.
type V1Error struct {
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Status   int            `json:"status"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Errors   []V1FieldError `json:"errors,omitempty"`
}

// V1FieldError represents a validation error on a specific field.
type V1FieldError struct {
	Source V1ErrorSource `json:"source"`
	Code   string        `json:"code"`
	Detail string        `json:"detail"`
}

// V1ErrorSource points to the location of an error.
type V1ErrorSource struct {
	Pointer string `json:"pointer"`
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
	mux.HandleFunc("/api/v1/_sidepanel/", a.handleV1SidePanel)
	mux.HandleFunc("/api/v1/_sidebar", a.handleV1Sidebar)
	mux.HandleFunc("/api/v1/_conflicts", a.handleV1Conflicts)
	mux.HandleFunc("/api/v1/_conflicts/", a.handleV1ConflictRoutes)
	mux.HandleFunc("/api/v1/_documents/", a.handleV1Documents)
	mux.HandleFunc("/api/v1/_openapi.json", a.handleV1OpenAPI)
	mux.HandleFunc("/api/v1/_commands", a.handleV1Commands)
	mux.HandleFunc("/api/v1/_templates/", a.handleV1Templates)
	mux.HandleFunc("/api/v1/_views/", a.handleV1Views)
	mux.HandleFunc("/api/v1/_action/", a.handleV1Action)

	// Dynamic entity routes are handled by a catch-all
	mux.HandleFunc("/api/v1/", a.handleV1DynamicRoutes)
}

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
		// /{plural}/{id} - single entity
		a.handleV1SingleEntity(w, r, typeName, plural, parts[1])
	case 3:
		// /{plural}/{id}/relations
		if parts[2] == "relations" {
			a.handleV1EntityRelations(w, r, typeName, parts[1])
		} else {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	case 4:
		// /{plural}/{id}/relations/{relType} or /{plural}/{id}/_actions/{action}
		switch parts[2] {
		case "relations":
			a.handleV1EntityRelationType(w, r, typeName, parts[1], parts[3])
		case "_actions":
			a.handleV1EntityAction(w, r, typeName, parts[1], parts[3])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	case 5:
		// /{plural}/{id}/relations/{relType}/{targetId}
		if parts[2] == "relations" {
			a.handleV1RelationTarget(w, r, typeName, parts[1], parts[3], parts[4])
		} else {
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
			a.handleV1DryRunCreate(w, r, typeName, plural)
			return
		}
		a.handleV1CreateEntity(w, r, typeName, plural)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// scopedSortedEntities runs the shared list pipeline — load-by-type,
// free-text intersection (?q=), structured filters (filter[...]), then the
// configured sort — and returns the fully ordered result set *before*
// pagination. Both handleV1ListEntities and handleV1EntityPosition call this,
// so a list and its scope navigator are guaranteed to observe identical
// ordering. The only error returned is a free-text search failure (HTTP 500
// at the call site); everything else degrades to an empty/whole set as the
// list endpoint always did.
func (a *App) scopedSortedEntities(ctx context.Context, typeName string, query map[string][]string) ([]*entityPkg.Entity, error) {
	entities := listFromStoreByTypes(ctx, a.Services(), []string{typeName})

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

	entities = a.applyV1Filters(entities, query, typeName)
	entities = a.applyV1Sorting(entities, query)
	return entities, nil
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
		writeV1Error(w, r, http.StatusInternalServerError, "search_failed", "Free-text search failed", err.Error())
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

	// Build response - always include relations for relation column support
	data := make([]V1Entity, 0, len(entities))
	included := make(map[string]V1Entity)
	for _, e := range entities {
		v1Entity := a.serializeRelatedEntityForWire(r.Context(), e, plural, true)
		data = append(data, v1Entity)

		// Resolve includes if requested
		if wantIncludes {
			for id, inc := range a.resolveV1Includes(r.Context(), e, includes) {
				included[id] = inc
			}
		}
	}

	resp := V1ListResponse{
		Data: data,
		Meta: V1ListMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: end < total,
		},
		Actions: a.computeCollectionActions(r.Context(), typeName),
	}

	// Add Link header for pagination (RFC 5988)
	a.addPaginationLinks(w, r, page, perPage, total, plural)

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Per-Page", strconv.Itoa(perPage))

	// If includes were requested, add them to response
	if len(included) > 0 {
		// For list responses with includes, we need a different response structure
		// Encode as JSON with additional "included" field
		type listWithIncludes struct {
			Data     []V1Entity          `json:"data"`
			Meta     V1ListMeta          `json:"meta"`
			Included map[string]V1Entity `json:"included,omitempty"`
			Actions  map[string]bool     `json:"_actions,omitempty"`
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

func (a *App) handleV1CreateEntity(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	// Need write lock for creation
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	var req struct {
		ID         string                 `json:"id,omitempty"`
		Prefix     string                 `json:"prefix,omitempty"`
		Properties map[string]interface{} `json:"properties"`
		Content    string                 `json:"content,omitempty"`
		Relations  V1RelationsField       `json:"relations,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var werr *wireError
		if errors.As(err, &werr) {
			writeV1Error(w, r, http.StatusBadRequest, werr.Code, werr.Detail, werr.Path)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	entityDef, defOK := a.State().Meta.Entities[typeName]
	if !defOK {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity type not found", typeName)
		return
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Prefix = strings.TrimSpace(req.Prefix)
	if msg := validateCreateIDOpts(&entityDef, req.ID, req.Prefix); msg != "" {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", msg, "")
		return
	}

	// Affordance parity (BUG-Q60V): a `fields:` policy that hides or
	// freezes a field must gate it on create too, not just PATCH —
	// otherwise the value can be smuggled in at create time. Validate
	// against the candidate entity (type + proposed properties, no ID
	// yet). Relation-dependent predicates fail closed for an
	// unpersisted entity, which is the safe direction; only global-role
	// grants apply at create. Collection-level create authorization is
	// enforced separately inside CreateEntity (acl.OpCreate).
	candidate := &entityPkg.Entity{Type: typeName, Properties: req.Properties}
	if denial := a.validateFieldWrite(r.Context(), candidate, req.Properties, nil); denial != nil {
		a.denyAffordance(r.Context(), w, candidate, *denial)
		return
	}

	createResult, err := a.entityManager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: req.Properties,
			Content:    req.Content,
		},
		entityPkg.CreateOptions{ID: req.ID, Prefix: req.Prefix},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}
	created := createResult.Entity

	// Phase A: relation validation (mirrors the PATCH path). Soft
	// conditions surface as warnings; hard wire/structural failures
	// return immediately without applying.
	var relWarnings []Warning
	if req.Relations.Modern != nil {
		ws, err := a.validateRelationsModern(r.Context(), created.ID, created.Type, req.Relations.Modern)
		if err != nil {
			a.writeRelationsValidationError(w, r, err)
			return
		}
		relWarnings = ws
	}

	// Phase B: relation writes.
	if req.Relations.Modern != nil {
		ws, err := a.applyRelationsModern(r.Context(), created.ID, req.Relations.Modern)
		relWarnings = append(relWarnings, ws...)
		if err != nil {
			a.writeRelationsApplyError(w, r, err)
			return
		}
	}

	result := a.serializeEntityForWire(r.Context(), created, plural, true)
	if len(relWarnings) > 0 {
		result.Warnings = append(result.Warnings, relWarnings...)
	}
	// DEC-HWZHA: surface entity-level soft validation findings (e.g.
	// required-field-missing) as warnings on the 201 response.
	if len(createResult.Warnings) > 0 {
		result.Warnings = append(result.Warnings, createResult.Warnings...)
	}

	// Set Location header
	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, created.ID))

	// SSE broadcast is driven by the store-event bridge (see
	// App.startStoreEventBridge), not inline here — so a create by ANY process
	// reaches all connected browsers and a local create isn't double-broadcast.

	writeV1JSON(w, http.StatusCreated, result)
}

// handleV1DryRunCreate evaluates field/option/relation affordances and
// soft validation against a candidate entity WITHOUT persisting it, so
// the SPA create form can disable read-only fields, hide hidden fields,
// filter enum options, and show as-you-type validation feedback before
// commit (TKT-3I5U).
//
// It is READ-shaped (RR-R8OR): it never takes a.writeMu and snapshots
// state once like a GET. It is verdict-only (RR-4O6E): it computes
// affordances and warnings but emits NO `denied-write` audit row and
// performs NO write — so live re-derivation per keystroke can't flood
// the audit log or contend the writer lock.
//
// The verdicts are ADVISORY (RR-Y85M): the real create (POST without
// ?dry_run) re-runs the BUG-Q60V affordance gate and is the sole
// authorization point. A client that ignores these hints and POSTs a
// denied field still 403s.
//
// Scope: fields + options + relations + soft warnings. Relation edges
// are not staged (a candidate has no real ID); relation affordances
// reflect the per-type verdict only.
func (a *App) handleV1DryRunCreate(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	s := a.State()

	// Mirror of handleV1CreateEntity's request body MINUS `relations`
	// — staged relations are deferred (a candidate has no real source
	// ID to hang edges on). When a new field is added to the real
	// create body, decide explicitly whether dry-run should accept it
	// and update both structs together (RR-GOR8 drift guard).
	var req struct {
		ID         string                 `json:"id,omitempty"`
		Prefix     string                 `json:"prefix,omitempty"`
		Properties map[string]interface{} `json:"properties"`
		Content    string                 `json:"content,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	entityDef, ok := s.Meta.Entities[typeName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity type not found", typeName)
		return
	}

	reqID := strings.TrimSpace(req.ID)
	reqPrefix := strings.TrimSpace(req.Prefix)

	// RR-9JOH: surface ID/prefix problems as a soft warning rather than
	// 422 so the create form learns at typing time instead of at submit.
	// The real commit's validateCreateIDOpts still hard-rejects — this
	// is advisory parity with the rest of the dry-run.
	var idWarning *Warning
	if msg := validateCreateIDOpts(&entityDef, reqID, reqPrefix); msg != "" {
		idWarning = &Warning{Code: "id_opts_invalid", Path: "/id", Detail: msg}
	}

	// Resolve the would-be entity (post template / status defaults) and
	// soft warnings via the shared create-path validation — no persist,
	// no audit, no automation. Hard structural errors surface as 422.
	candidate, warnings, err := a.entityManager.ValidateCreate(r.Context(),
		&entityPkg.Entity{Type: typeName, Properties: req.Properties, Content: req.Content},
		entityPkg.CreateOptions{ID: reqID, Prefix: reqPrefix},
	)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}

	// Seed missing-but-declared property keys with nil values BEFORE
	// serialization. The SPA's create-mode field filter uses the
	// response's `properties` keys to know which declared fields are
	// visible (hidden fields get stripped by serializeEntityForWire's
	// hidden-property filter). Without this, a visible-by-default field
	// whose value the user hasn't set yet (e.g. a required `title`)
	// would be absent from both `_fields` (sparse: no deviation) and
	// `properties` (no value yet), so the filter would drop it.
	if def, ok := s.Meta.Entities[typeName]; ok {
		if candidate.Properties == nil {
			candidate.Properties = make(map[string]interface{})
		}
		for name := range def.Properties {
			if _, present := candidate.Properties[name]; !present {
				candidate.Properties[name] = nil
			}
		}
	}

	// Affordances are computed against the candidate's CURRENT values, so
	// value-dependent predicates (e.g. field B read-only when A == x)
	// re-derive as the form changes. includeRelations=false: no edges
	// exist for an unsaved entity.
	result := a.serializeEntityForWire(r.Context(), candidate, plural, false)
	if idWarning != nil {
		result.Warnings = append(result.Warnings, *idWarning)
	}
	if len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	// writeV1JSON already sets `Cache-Control: no-cache, no-store,
	// must-revalidate` and no ETag, which is what a per-request,
	// value-dependent, never-persisted response needs (RR-7PL4).
	writeV1JSON(w, http.StatusOK, result)
}

// --- Single Entity Handlers ---

func (a *App) handleV1SingleEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	switch r.Method {
	case http.MethodGet:
		a.handleV1GetEntity(w, r, typeName, plural, entityID)
	case http.MethodPatch:
		a.handleV1UpdateEntity(w, r, typeName, plural, entityID)
	case http.MethodDelete:
		a.handleV1DeleteEntity(w, r, typeName, plural, entityID)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (a *App) handleV1GetEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	query := r.URL.Query()
	includeRelations := true

	// Single per-entity serialization: strips hidden + attaches
	// `_fields` / `_relations` per docs/data-entry/api-reference.md.
	result := a.serializeEntityForWire(r.Context(), entity, plural, includeRelations)

	// Handle includes for related entities
	if includes := query.Get("include"); includes != "" {
		result.Included = a.resolveV1Includes(r.Context(), entity, includes)
	}

	// ETag for caching
	etag := a.computeEntityETag(r.Context(), entity)
	w.Header().Set("ETag", etag)

	// Check If-None-Match
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	writeV1JSON(w, http.StatusOK, result)
}

func (a *App) handleV1UpdateEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	s := a.State()

	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Refuse to write through an inaccessible entity. The on-disk file
	// is unreadable (e.g. git-crypt encrypted, no key locally) — writing
	// would replace the ciphertext with whatever the SPA had on hand.
	if entity.IsLocked() {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "encrypted_inaccessible",
			"Cannot edit an inaccessible entity", "File is git-crypt encrypted; run `git-crypt unlock` first.")
		return
	}

	// Check If-Match for optimistic locking
	ifMatch := r.Header.Get("If-Match")
	if ifMatch != "" {
		currentETag := a.computeEntityETag(r.Context(), entity)
		if ifMatch != currentETag {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Entity has been modified", "ETag mismatch")
			return
		}
	}

	var req struct {
		Properties      map[string]interface{} `json:"properties,omitempty"`
		PropertiesUnset []string               `json:"properties_unset,omitempty"`
		Content         *string                `json:"content,omitempty"`
		Relations       V1RelationsField       `json:"relations,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// V1RelationsField's UnmarshalJSON returns *wireError for
		// shape errors; surface them as 400 with the structured code.
		var werr *wireError
		if errors.As(err, &werr) {
			writeV1Error(w, r, http.StatusBadRequest, werr.Code,
				werr.Detail, werr.Path)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Affordance parity (TKT-G7N5): reject writes that conflict with
	// what the resolver would have surfaced on GET. Runs before any
	// other validation so the failure mode is identical regardless of
	// what else the PATCH body would have triggered.
	if denial := a.validateFieldWrite(r.Context(), entity, req.Properties, req.PropertiesUnset); denial != nil {
		a.denyAffordance(r.Context(), w, entity, *denial)
		return
	}
	if req.Relations.Modern != nil {
		if denial := a.validateRelationsModernAffordances(r.Context(), entityID, entity, req.Relations.Modern); denial != nil {
			a.denyAffordance(r.Context(), w, entity, *denial)
			return
		}
	}

	// Phase A: validate relations (no writes). Returns warnings (will
	// be merged into the success response) and err (hard 400/422).
	// Validation runs BEFORE entity update so a structural relation
	// error doesn't leave the entity half-written. (DEC-HWZHA atomicity.)
	var warnings []Warning
	if req.Relations.Modern != nil {
		ws, err := a.validateRelationsModern(r.Context(), entityID, entity.Type, req.Relations.Modern)
		if err != nil {
			a.writeRelationsValidationError(w, r, err)
			return
		}
		warnings = ws
	}

	// Phase B: entity update. Skipped when only relations changed,
	// to avoid bumping the file mtime and broadcasting a misleading
	// "entity updated" SSE event with no byte-level change.
	if req.Properties != nil {
		for k, v := range req.Properties {
			entity.Properties[k] = v
		}
	}
	// Apply properties_unset AFTER property upserts so a body that
	// both sets and unsets the same key behaves like the last-write-
	// wins of property merging followed by the explicit unset.
	// (TKT-E6094 / autosave: maps the "user cleared this field" intent
	// to a wire-level delete that's distinct from "field was untouched".)
	if len(req.PropertiesUnset) > 0 {
		entityTypeDef, hasType := s.Meta.Entities[entity.Type]
		for i, k := range req.PropertiesUnset {
			if hasType {
				if _, declared := entityTypeDef.Properties[k]; !declared {
					warnings = append(warnings, Warning{
						Code:   "unknown_property_unset_key",
						Path:   fmt.Sprintf("/properties_unset/%d", i),
						Detail: fmt.Sprintf("property %q is not declared on entity type %q", k, entity.Type),
					})
				}
			}
			delete(entity.Properties, k)
		}
	}
	if req.Content != nil {
		entity.Content = *req.Content
	}
	entityChanged := req.Properties != nil || len(req.PropertiesUnset) > 0 || req.Content != nil
	if entityChanged {
		updateResult, err := a.entityManager.UpdateEntity(r.Context(), entity)
		if err != nil {
			if writeForbiddenIfACLDenied(w, err) {
				return
			}
			writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
			return
		}
		// DEC-HWZHA: soft validation findings ride on the result as
		// warnings. Merge them into the response alongside any
		// relation warnings already collected.
		if updateResult != nil {
			warnings = append(warnings, updateResult.Warnings...)
		}
	}

	// Phase C: relation writes. Produces warnings on soft conditions
	// and structured errors on hard failures.
	if req.Relations.Modern != nil {
		ws, err := a.applyRelationsModern(r.Context(), entityID, req.Relations.Modern)
		warnings = append(warnings, ws...)
		if err != nil {
			a.writeRelationsApplyError(w, r, err)
			return
		}
	}

	result := a.serializeEntityForWire(r.Context(), entity, plural, true)
	if len(warnings) > 0 {
		result.Warnings = warnings
	}
	newETag := a.computeEntityETag(r.Context(), entity)
	w.Header().Set("ETag", newETag)

	// SSE broadcast is driven by the store-event bridge: an entity update only
	// fires EventEntityUpdated when the store's entity row actually changed,
	// which matches the prior "if entityChanged" gate (relation-only edits emit
	// no entity event). So a remote update reaches all browsers and a local one
	// isn't double-broadcast.

	writeV1JSON(w, http.StatusOK, result)
}

// writeRelationsValidationError maps a Phase A validation error from
// the modern reconciler to the corresponding HTTP response. wireError
// → 400 (caller bug); structuralError → 422 (storage can't represent).
func (a *App) writeRelationsValidationError(w http.ResponseWriter, r *http.Request, err error) {
	var werr *wireError
	if errors.As(err, &werr) {
		writeV1Error(w, r, http.StatusBadRequest, werr.Code, werr.Detail, werr.Path)
		return
	}
	if se, ok := asStructuralError(err); ok {
		writeV1Error(w, r, http.StatusUnprocessableEntity, se.Code, se.Detail, se.Path)
		return
	}
	writeV1Error(w, r, http.StatusUnprocessableEntity,
		"relation_failed", "Failed to validate relations", err.Error())
}

// writeRelationsApplyError maps a Phase C write error to a 500 — the
// entity may already have been updated, so a partial state is on disk.
// This is the documented atomicity gap. ACL denials short-circuit to
// the structured 403 path; everything else falls through to the
// 500-with-detail body.
func (a *App) writeRelationsApplyError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, err) {
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError,
		"relation_write_failed",
		"Failed to apply relation changes after entity update; the entity may have been updated",
		reconcileDetail(err))
}

func (a *App) handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, _, entityID string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	if _, err := a.entityManager.DeleteEntity(r.Context(), entityID, true); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "delete_failed", "Failed to delete entity", err.Error())
		return
	}

	// SSE broadcast is driven by the store-event bridge (see
	// App.startStoreEventBridge); a delete by any process reaches all browsers,
	// and a local delete isn't double-broadcast.

	w.WriteHeader(http.StatusNoContent)
}

// --- Relation Handlers ---

func (a *App) handleV1EntityRelations(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	s := a.State()
	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	outgoing := a.outgoingRelations(r.Context(), entityID)
	incoming := a.incomingRelations(r.Context(), entityID)

	relations := make(map[string][]map[string]interface{})

	// Track the sort property per group, derived from the relation type's
	// Orderable mode. Empty string disables sorting for that group.
	groupSortProp := make(map[string]string)

	for _, edge := range outgoing {
		rel := map[string]interface{}{
			"id":        edge.To,
			"type":      a.peerType(r.Context(), edge.To),
			"direction": "outgoing",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
		}
		relations[edge.Type] = append(relations[edge.Type], rel)
		if relDef, ok := s.Meta.Relations[edge.Type]; ok {
			if p := relDef.OutgoingOrderProperty(); p != "" {
				groupSortProp[edge.Type] = p
			}
		}
	}

	for _, edge := range incoming {
		relDef, ok := s.Meta.Relations[edge.Type]
		if !ok {
			continue
		}
		inverseName := edge.Type + "_inverse"
		if relDef.Inverse != nil && relDef.Inverse.ID != "" {
			inverseName = relDef.Inverse.ID
		}
		rel := map[string]interface{}{
			"id":        edge.From,
			"type":      a.peerType(r.Context(), edge.From),
			"direction": "incoming",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
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

	writeV1JSON(w, http.StatusOK, relations)
}

// sortRelationGroup sorts a relation group in place by a numeric meta key.
// Entries without a finite numeric value at prop sort last; ties stable.
func sortRelationGroup(group []map[string]interface{}, prop string) {
	if len(group) < 2 || prop == "" {
		return
	}
	value := func(m map[string]interface{}) (float64, bool) {
		meta, ok := m["meta"].(map[string]interface{})
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
		a.handleV1CreateRelation(w, r, typeName, entityID, relType)
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
	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	incoming := r.URL.Query().Get("direction") == string(DirectionIncoming)

	var edges []*entityPkg.Relation
	if incoming {
		edges = a.incomingRelations(r.Context(), entityID)
	} else {
		edges = a.outgoingRelations(r.Context(), entityID)
	}

	relations := make([]map[string]interface{}, 0, len(edges))

	for _, edge := range edges {
		if edge.Type != relType {
			continue
		}
		peerID := edge.To
		if incoming {
			peerID = edge.From
		}
		rel := map[string]interface{}{
			"id":   peerID,
			"type": a.peerType(r.Context(), peerID),
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
		}
		relations = append(relations, rel)
	}

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

	writeV1JSON(w, http.StatusOK, relations)
}

func (a *App) handleV1CreateRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	var req struct {
		ID        string                 `json:"id"`
		Meta      map[string]interface{} `json:"meta,omitempty"`
		Direction string                 `json:"direction,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	if req.ID == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_id", "Target ID is required", "")
		return
	}

	// Affordance gates: creatable + meta-writable, evaluated against
	// the SOURCE of the new edge (not necessarily the path entity —
	// for incoming-direction creates the path entity is the target).
	source := a.relationSourceEntity(r.Context(), entity, req.ID, req.Direction)
	// Audit subject is the source of the new edge, matching the
	// entity whose policy gated the write.
	if denial := a.validateRelationOp(r.Context(), source, relType, RelationOpCreate); denial != nil {
		a.denyAffordance(r.Context(), w, source, *denial)
		return
	}
	if denial := a.validateRelationMetaWrite(r.Context(), source, relType, req.Meta, nil); denial != nil {
		a.denyAffordance(r.Context(), w, source, *denial)
		return
	}

	from, to := resolveRelationEndpoints(entity.ID, req.ID, req.Direction)

	_, err := a.entityManager.CreateRelation(r.Context(), from, relType, to, entityPkg.RelationOptions{Properties: req.Meta})
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "relation_failed", "Failed to create relation", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *App) handleV1RelationTarget(w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string) {
	switch r.Method {
	case http.MethodPatch:
		a.handleV1UpdateRelation(w, r, typeName, entityID, relType, targetID)
	case http.MethodDelete:
		a.handleV1DeleteRelation(w, r, typeName, entityID, relType, targetID)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (a *App) handleV1UpdateRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	var req struct {
		Meta      map[string]interface{} `json:"meta"`
		Direction string                 `json:"direction,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Affordance gate: meta-writable, evaluated against the SOURCE of
	// the edge (the path entity for outgoing; the peer for incoming).
	// The edge already exists (PATCH is meta-only), so the create /
	// remove gates don't apply.
	source := a.relationSourceEntity(r.Context(), entity, targetID, req.Direction)
	if denial := a.validateRelationMetaWrite(r.Context(), source, relType, req.Meta, nil); denial != nil {
		a.denyAffordance(r.Context(), w, source, *denial)
		return
	}

	// Managed order properties must be finite numbers when present. Fast
	// 400 here so wire-format errors don't surface as 422-from-manager.
	if relDef, ok := a.State().Meta.Relations[relType]; ok {
		for _, prop := range []string{metamodel.OrderPropertyOut, metamodel.OrderPropertyIn} {
			if (prop == metamodel.OrderPropertyOut && relDef.OutgoingOrderProperty() == "") ||
				(prop == metamodel.OrderPropertyIn && relDef.IncomingOrderProperty() == "") {

				continue
			}
			v, present := req.Meta[prop]
			if !present {
				continue
			}
			if _, ok := entitymanager.FiniteOrder(v); !ok {
				writeV1Error(w, r, http.StatusBadRequest, "order_value_invalid",
					"managed order property must be a finite number", prop)
				return
			}
		}
	}

	from, to := resolveRelationEndpoints(entity.ID, targetID, req.Direction)

	rel, err := a.entityManager.UpdateRelation(r.Context(), from, relType, to, entityPkg.RelationOptions{
		Properties: req.Meta,
	})
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	result := map[string]interface{}{
		"from": rel.From,
		"type": rel.Type,
		"to":   rel.To,
	}
	if len(rel.Properties) > 0 {
		result["meta"] = rel.Properties
	}

	writeV1JSON(w, http.StatusOK, result)
}

func (a *App) handleV1DeleteRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Affordance gate: removable check, evaluated against the SOURCE
	// of the edge (the path entity for outgoing; the peer for
	// incoming). Per-relation-type uniform — a removable=false
	// verdict applies to every link of this type.
	direction := r.URL.Query().Get("direction")
	source := a.relationSourceEntity(r.Context(), entity, targetID, direction)
	if denial := a.validateRelationOp(r.Context(), source, relType, RelationOpRemove); denial != nil {
		a.denyAffordance(r.Context(), w, source, *denial)
		return
	}

	from, to := resolveRelationEndpoints(entity.ID, targetID, direction)

	if err := a.entityManager.DeleteRelation(r.Context(), from, relType, to); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Action Handlers ---

func (a *App) handleV1EntityAction(w http.ResponseWriter, r *http.Request, typeName, entityID, action string) {
	if r.Method != http.MethodPost {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	switch action {
	case "clone":
		a.handleV1CloneEntity(w, r, typeName, entityID)
	default:
		writeV1Error(w, r, http.StatusNotFound, "unknown_action", "Unknown action", "")
	}
}

func (a *App) handleV1CloneEntity(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	s := a.State()
	entity, found := a.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Clone properties
	props := make(map[string]interface{})
	for k, v := range entity.Properties {
		props[k] = v
	}

	cloneResult, err := a.entityManager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: props,
			Content:    entity.Content,
		},
		entityPkg.CreateOptions{},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "Failed to clone entity", err.Error())
		return
	}
	newEntity := cloneResult.Entity

	entityDef := s.Meta.Entities[typeName]
	plural := entityDef.GetPlural(typeName)
	result := a.serializeEntityForWire(r.Context(), newEntity, plural, false)

	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, newEntity.ID))
	writeV1JSON(w, http.StatusCreated, result)
}

// --- System Handlers ---

func (a *App) handleV1Schema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	s := a.State()
	schema := V1Schema{
		Entities:  make(map[string]V1EntityType),
		Relations: make(map[string]V1RelationType),
		Types:     make(map[string]V1CustomType),
	}

	for name, def := range s.Meta.Entities {
		et := V1EntityType{
			Label:       def.Label,
			Plural:      def.GetPlural(name),
			Description: def.Description,
			Primary:     def.GetPrimaryProperty(),
			IDType:      def.GetIDType(),
			Properties:  make(map[string]V1PropertyDef),
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
		rt := V1RelationType{
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
			rt.Inverse = &V1InverseDef{ID: def.Inverse.ID, Label: def.Inverse.Label}
		}
		if len(def.Properties) > 0 {
			rt.Properties = make(map[string]V1PropertyDef, len(def.Properties))
			for propName, propDef := range def.Properties {
				rt.Properties[propName] = a.toV1PropertyDef(s.Meta, propDef)
			}
		}
		if def.OutgoingOrderProperty() != "" || def.IncomingOrderProperty() != "" {
			rt.Orderable = &V1RelationOrderable{
				Outgoing: def.OutgoingOrderProperty() != "",
				Incoming: def.IncomingOrderProperty() != "",
			}
		}
		schema.Relations[name] = rt
	}

	for name, def := range s.Meta.Types {
		schema.Types[name] = V1CustomType{
			Values:  def.Values,
			Default: def.Default,
		}
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
		et := V1EntityType{
			Label:       def.Label,
			Plural:      def.GetPlural(typeName),
			Description: def.Description,
			Primary:     def.GetPrimaryProperty(),
			IDType:      def.GetIDType(),
			Properties:  make(map[string]V1PropertyDef),
		}
		if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
			et.IDPrefix = prefixes[0]
			et.IDPrefixes = prefixes
		}
		for propName, propDef := range def.Properties {
			pd := V1PropertyDef{
				Type:     propDef.Type,
				Required: propDef.Required,
				Default:  propDef.Default,
			}
			if ct, ok := s.Meta.Types[propDef.Type]; ok {
				pd.Values = ct.Values
			}
			et.Properties[propName] = pd
		}
		writeV1JSON(w, http.StatusOK, et)

	case path == "relations":
		writeV1JSON(w, http.StatusOK, s.Meta.Relations)

	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

func (a *App) handleV1Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	s := a.State()
	// Resolve relation widgets: auto-detect "cards" for relations with properties/content
	forms := make(map[string]dataentryconfig.Form, len(s.Cfg.Forms))
	for id, form := range s.Cfg.Forms {
		f := form
		resolved := make([]dataentryconfig.FormRelation, len(f.Relations))
		copy(resolved, f.Relations)
		for i := range resolved {
			if resolved[i].Widget == "" {
				if def, ok := s.Meta.GetRelationDef(resolved[i].Relation); ok && def.HasAdvancedFeatures() {
					resolved[i].Widget = WidgetCards
				}
			}
		}
		f.Relations = resolved
		forms[id] = f
	}

	config := V1Config{
		App: V1AppConfig{
			Name:        s.Cfg.App.Name,
			Description: s.Cfg.App.Description,
		},
		Styles:      s.StyleMap,
		Forms:       forms,
		Lists:       s.Cfg.Lists,
		Views:       s.Cfg.Views,
		EntityViews: s.Cfg.EntityViews,
		Kanbans:     s.Cfg.Kanbans,
		Dashboard:   s.Cfg.Dashboard,
		Actions:     s.Cfg.Actions,
		Navigation:  s.Cfg.Navigation,
		Documents:   s.Cfg.Documents,
		Palette:     s.Palette,
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
		writeV1JSON(w, http.StatusOK, V1ListResponse{Data: []V1Entity{}, Meta: V1ListMeta{}})
		return
	}

	entities := a.executeQuery(r.Context(), query)

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
	data := make([]V1Entity, 0, len(entities))
	for _, e := range entities {
		entityDef := meta.Entities[e.Type]
		plural := entityDef.GetPlural(e.Type)
		data = append(data, a.serializeRelatedEntityForWire(r.Context(), e, plural, false))
	}

	resp := V1ListResponse{
		Data: data,
		Meta: V1ListMeta{
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

	analysisResult := a.runAnalysis(r.Context())

	result := APIAnalysisResult{
		Errors:   analysisResult.ErrorCount,
		Warnings: analysisResult.WarningCount,
		Issues:   make([]APIIssue, 0),
		ByCheck:  make(map[string]int),
	}

	// Loopback gate: same policy as action / document surfaces.
	// Non-loopback callers get a degraded envelope on script-error
	// issues (no source slice, no stack, no captured output).
	fullDetail := a.allowFullScriptDetail(r)

	for _, section := range analysisResult.Sections {
		for _, issue := range section.Issues {
			api := APIIssue{
				EntityID:   issue.EntityID,
				EntityType: issue.EntityType,
				Title:      issue.Title,
				Message:    issue.Message,
				Severity:   issue.Severity,
				CheckType:  section.Name,
			}
			if issue.ScriptError != nil {
				env := buildScriptErrorEnvelope(issue.ScriptError, fullDetail, "")
				api.ScriptError = &env
			}
			result.Issues = append(result.Issues, api)
			result.ByCheck[section.Name]++
		}
	}

	writeV1JSON(w, http.StatusOK, result)
}

// --- Helper Functions ---

func (a *App) entityToV1(ctx context.Context, e *entityPkg.Entity, plural string, includeRelations bool) V1Entity {
	s := a.State()
	v1 := V1Entity{
		ID:         e.ID,
		Type:       e.Type,
		Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Properties: make(map[string]interface{}),
		Content:    e.Content,
		Self:       fmt.Sprintf("/api/v1/%s/%s", plural, e.ID),
		Actions:    a.computeActions(ctx, e),
	}

	for k, v := range e.Properties {
		v1.Properties[k] = v
	}

	if e.IsLocked() {
		v1.Inaccessible = make([]V1InaccessibleField, 0, len(e.Inaccessible))
		for _, f := range e.Inaccessible {
			v1.Inaccessible = append(v1.Inaccessible, V1InaccessibleField{
				Name:   f.Name,
				Reason: string(f.Reason),
			})
		}
	}

	if includeRelations {
		v1.Relations = make(map[string][]string)
		for _, edge := range a.outgoingRelations(ctx, e.ID) {
			v1.Relations[edge.Type] = append(v1.Relations[edge.Type], edge.To)
		}
	}

	return v1
}

func (a *App) resolveV1Includes(ctx context.Context, entity *entityPkg.Entity, includes string) map[string]V1Entity {
	s := a.State()
	included := make(map[string]V1Entity)

	// Support include=* to include all related entities
	if includes == "*" {
		// Include all outgoing relations
		for _, edge := range a.outgoingRelations(ctx, entity.ID) {
			target, found := a.getEntity(ctx, edge.To)
			if !found {
				continue
			}
			entityDef := s.Meta.Entities[target.Type]
			plural := entityDef.GetPlural(target.Type)
			included[target.ID] = a.serializeRelatedEntityForWire(ctx, target, plural, false)
		}
		// Include all incoming relations
		for _, edge := range a.incomingRelations(ctx, entity.ID) {
			source, found := a.getEntity(ctx, edge.From)
			if !found {
				continue
			}
			entityDef := s.Meta.Entities[source.Type]
			plural := entityDef.GetPlural(source.Type)
			included[source.ID] = a.serializeRelatedEntityForWire(ctx, source, plural, false)
		}
		return included
	}

	parts := strings.Split(includes, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle nested includes like "implements.requires"
		relParts := strings.SplitN(part, ".", 2)
		relType := relParts[0]

		for _, edge := range a.outgoingRelations(ctx, entity.ID) {
			if edge.Type != relType {
				continue
			}
			target, found := a.getEntity(ctx, edge.To)
			if !found {
				continue
			}
			entityDef := s.Meta.Entities[target.Type]
			plural := entityDef.GetPlural(target.Type)
			included[target.ID] = a.serializeRelatedEntityForWire(ctx, target, plural, false)

			// Handle nested includes
			if len(relParts) > 1 {
				nested := a.resolveV1Includes(ctx, target, relParts[1])
				for k, v := range nested {
					included[k] = v
				}
			}
		}
	}

	return included
}

func (a *App) applyV1Filters(entities []*entityPkg.Entity, query map[string][]string, _ string) []*entityPkg.Entity {
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
		// empty property/operator means the URL is bogus. Fail CLOSED by
		// skipping the filter entirely (logging so users notice) rather
		// than silently including every entity via the switch's default
		// case, which would be a fail-open scope bypass.
		if len(parts) > 2 {
			slog.Warn("filter key has too many segments", "key", key)
			continue
		}
		property := parts[0]
		if property == "" {
			slog.Warn("filter key has empty property", "key", key)
			continue
		}
		operator := "eq"
		if len(parts) == 2 {
			if parts[1] == "" {
				slog.Warn("filter key has empty operator segment", "key", key)
				continue
			}
			operator = parts[1]
		}

		// Reject unknown operators BEFORE the per-entity loop. A typo like
		// `filter[status][equals]=done` used to fall through to the switch's
		// default case and include every entity, silently bypassing the
		// configured scope. Fail closed instead.
		switch operator {
		case "eq", "ne", "contains", "in", "lt", "lte", "gt", "gte":
			// known
		default:
			slog.Warn("filter uses unknown operator", "key", key, "operator", operator)
			continue
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

	return filtered
}

func (a *App) applyV1Sorting(entities []*entityPkg.Entity, query map[string][]string) []*entityPkg.Entity {
	sortParam := ""
	if vals, ok := query["sort"]; ok && len(vals) > 0 {
		sortParam = vals[0]
	}
	if sortParam == "" {
		return entities
	}

	// Parse sort param: "-created,title" means descending created, ascending title
	sortSpecs := make([]filter.SortSpec, 0)
	for _, part := range strings.Split(sortParam, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		spec := filter.SortSpec{Direction: "asc"}
		if strings.HasPrefix(part, "-") {
			spec.Direction = "desc"
			part = part[1:]
		}
		spec.Property = part
		sortSpecs = append(sortSpecs, spec)
	}

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

func (a *App) addPaginationLinks(w http.ResponseWriter, _ *http.Request, page, perPage, total int, plural string) {
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
	edges := a.outgoingRelations(ctx, e.ID)
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
	for _, p := range def.GetIDPrefixes() {
		if p == prefix {
			return ""
		}
	}
	return fmt.Sprintf("prefix %q is not valid; allowed: %v", prefix, def.GetIDPrefixes())
}

func writeV1JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeV1Error(w http.ResponseWriter, r *http.Request, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	err := V1Error{
		Type:     "https://rela.dev/errors/" + errType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}

	_ = json.NewEncoder(w).Encode(err)
}

// --- Side Panel API ---

// V1SidePanelSection represents a section in the side panel response.
type V1SidePanelSection struct {
	Heading      string              `json:"heading"`
	SectionID    string              `json:"sectionId"`
	Display      string              `json:"display"`
	IsEmpty      bool                `json:"isEmpty"`
	EmptyMessage string              `json:"emptyMessage,omitempty"`
	Fields       []V1SectionField    `json:"fields,omitempty"`
	Entities     []V1SidePanelEntity `json:"entities,omitempty"`
	AddInfo      *V1ViewAddInfo      `json:"addInfo,omitempty"`
	LinkInfo     *V1ViewLinkInfo     `json:"linkInfo,omitempty"`
}

// V1SectionField represents a field in a side panel section.
// Values is always an array so that list-typed properties retain per-item
// structure; scalar properties become a one-element array. Empty fields emit
// an empty array (omitted via omitempty when nil).
//
// Property carries the raw property name so consumers can correlate the
// field with metamodel data (e.g. inaccessibility lookup); Label is the
// human-readable rendering. Inaccessible is true when the underlying entity
// is git-crypt encrypted — the field is known to exist in the schema but
// its value cannot be read.
type V1SectionField struct {
	Property     string   `json:"property,omitempty"`
	Label        string   `json:"label"`
	Values       []string `json:"values,omitempty"`
	PropType     string   `json:"propType,omitempty"`
	Inaccessible bool     `json:"inaccessible,omitempty"`
}

// V1SidePanelEntity represents an entity in a side panel section.
type V1SidePanelEntity struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Type       string           `json:"type"`
	EditFormID string           `json:"editFormId,omitempty"`
	Fields     []V1SectionField `json:"fields,omitempty"`
	Content    string           `json:"content,omitempty"`
	HasContent bool             `json:"hasContent"`
}

// handleV1SidePanel handles GET /api/v1/_sidepanel/{formId}/{entityId}.
func (a *App) handleV1SidePanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_sidepanel/{formId}/{entityId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_sidepanel/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_sidepanel/{formId}/{entityId}", "")
		return
	}

	formID := parts[0]
	entityID := parts[1] // Get form config
	s := a.State()
	form, ok := s.Cfg.Forms[formID]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "form_not_found", "Form not found", "")
		return
	}

	// Check if form has side panel
	if form.SidePanel == nil {
		writeV1JSON(w, http.StatusOK, []V1SidePanelSection{})
		return
	}

	// Get the entry entity
	entry, found := a.getEntity(r.Context(), entityID)
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "entity_not_found", "Entity not found", "")
		return
	}

	// Execute side panel traversal
	sections := a.executeSidePanel(r.Context(), form.SidePanel, entityID, form.EntityType)
	if sections == nil {
		writeV1JSON(w, http.StatusOK, []V1SidePanelSection{})
		return
	}

	// Build a synthetic ViewConfig to resolve add/link buttons
	viewConfig := ViewConfig{
		Entry:    ViewEntry{Type: form.EntityType},
		Traverse: form.SidePanel.Traverse,
		Sections: form.SidePanel.Sections,
	}
	a.resolveSectionButtonsWithTraverse(viewConfig, sections, entry)

	// Convert to API response format
	result := make([]V1SidePanelSection, 0, len(sections))
	for _, sec := range sections {
		apiSec := V1SidePanelSection{
			Heading:      sec.Heading,
			SectionID:    sec.SectionID,
			Display:      sec.Display,
			IsEmpty:      sec.IsEmpty,
			EmptyMessage: sec.EmptyMessage,
		}

		// Convert fields
		for _, f := range sec.Fields {
			apiSec.Fields = append(apiSec.Fields, V1SectionField(f))
		}

		// Convert entities
		for _, e := range sec.Entities {
			apiEnt := V1SidePanelEntity{
				ID:         e.ID,
				Title:      e.Title,
				Type:       e.Type,
				EditFormID: e.EditFormID,
				Content:    e.Content,
				HasContent: e.HasContent,
			}
			for _, f := range e.Fields {
				apiEnt.Fields = append(apiEnt.Fields, V1SectionField(f))
			}
			apiSec.Entities = append(apiSec.Entities, apiEnt)
		}

		// Convert add/link info
		if sec.AddInfo != nil {
			apiSec.AddInfo = &V1ViewAddInfo{
				Relation: sec.AddInfo.Relation,
				LinkAs:   sec.AddInfo.LinkAs,
				PeerID:   sec.AddInfo.PeerID,
			}
			for _, t := range sec.AddInfo.Targets {
				apiSec.AddInfo.Targets = append(apiSec.AddInfo.Targets, V1ViewAddTarget(t))
			}
		}
		if sec.LinkInfo != nil {
			apiSec.LinkInfo = &V1ViewLinkInfo{
				Relation:    sec.LinkInfo.Relation,
				LinkAs:      sec.LinkInfo.LinkAs,
				PeerID:      sec.LinkInfo.PeerID,
				EntityTypes: sec.LinkInfo.EntityTypes,
			}
		}

		result = append(result, apiSec)
	}

	writeV1JSON(w, http.StatusOK, result)
}

// --- Sidebar API ---

// V1SidebarItem represents a navigation item with count.
type V1SidebarItem struct {
	Label  string `json:"label"`
	Href   string `json:"href"`
	Icon   string `json:"icon,omitempty"`
	Count  *int   `json:"count,omitempty"`
	Action string `json:"action,omitempty"`
}

// V1SidebarGroup represents a navigation group with items.
type V1SidebarGroup struct {
	Group     string          `json:"group,omitempty"`
	Collapsed bool            `json:"collapsed,omitempty"`
	Items     []V1SidebarItem `json:"items"`
}

// V1SidebarResponse contains the sidebar data with app info and navigation.
type V1SidebarResponse struct {
	App        V1AppConfig      `json:"app"`
	Navigation []V1SidebarGroup `json:"navigation"`
	// LogoURL is the cache-busted URL of the user-uploaded sidebar logo,
	// or nil when no logo is set. Included here (rather than in
	// `_settings`) so the SPA can render the logo on first paint without
	// blocking on a settings fetch.
	LogoURL *string `json:"logoUrl,omitempty"`
}

// handleV1Sidebar returns denormalized sidebar data with entity counts.
func (a *App) handleV1Sidebar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	s := a.State()
	svc := a.Services()

	// Build entity type counts as a fallback for items without filters.
	typeCounts := make(map[string]int)
	for _, entityType := range s.Meta.EntityTypes() {
		n, err := svc.Store.CountEntities(r.Context(), store.EntityQuery{Type: entityType})
		if err != nil {
			typeCounts[entityType] = 0
			continue
		}
		typeCounts[entityType] = n
	}

	counts := sidebarCounts{
		typeCounts:  typeCounts,
		filterCache: make(map[string]int),
		app:         a,
	}

	// Build navigation with counts
	navigation := make([]V1SidebarGroup, 0)

	for _, entry := range s.Cfg.Navigation {
		if entry.IsGroup() {
			group := V1SidebarGroup{
				Group:     entry.Group,
				Collapsed: entry.Collapsed,
				Items:     make([]V1SidebarItem, 0),
			}
			for _, item := range entry.Items {
				sidebarItem := a.navEntryToSidebarItem(r.Context(), item, counts)
				group.Items = append(group.Items, sidebarItem)
			}
			navigation = append(navigation, group)
		} else {
			// Top-level item without group
			item := a.navEntryToSidebarItem(r.Context(), entry, counts)
			navigation = append(navigation, V1SidebarGroup{
				Items: []V1SidebarItem{item},
			})
		}
	}

	resp := V1SidebarResponse{
		App: V1AppConfig{
			Name:        s.Cfg.App.Name,
			Description: s.Cfg.App.Description,
		},
		Navigation: navigation,
	}
	resp.LogoURL = s.LogoURL()

	writeV1JSON(w, http.StatusOK, resp)
}

// sidebarCounts caches sidebar entity counts, applying list- or kanban-
// level filters when present. Unfiltered views fall back to the
// type-level total.
type sidebarCounts struct {
	typeCounts  map[string]int
	filterCache map[string]int // key: "list:<id>" or "kanban:<id>"
	app         *App
}

// listCount returns the entity count for the given list, applying any
// configured filters. Results are cached per call.
func (c *sidebarCounts) listCount(ctx context.Context, listID string, list dataentryconfig.List) int {
	key := "list:" + listID
	if n, ok := c.filterCache[key]; ok {
		return n
	}
	n := c.countWithFilters(ctx, list.EntityType, list.Filters)
	c.filterCache[key] = n
	return n
}

// kanbanCount returns the entity count for the given kanban, applying
// any configured filters. Results are cached per call.
func (c *sidebarCounts) kanbanCount(ctx context.Context, kanbanID string, kanban dataentryconfig.Kanban) int {
	key := "kanban:" + kanbanID
	if n, ok := c.filterCache[key]; ok {
		return n
	}
	n := c.countWithFilters(ctx, kanban.EntityType, kanban.Filters)
	c.filterCache[key] = n
	return n
}

// countWithFilters returns the count of entities of the given type that
// pass the supplied filters. When filters is empty, the precomputed
// type total is returned directly.
func (c *sidebarCounts) countWithFilters(ctx context.Context, entityType string, filters []dataentryconfig.FilterConfig) int {
	if len(filters) == 0 {
		return c.typeCounts[entityType]
	}
	entities := listFromStoreByTypes(ctx, c.app.Services(), []string{entityType})
	return len(applyFilters(entities, filters))
}

// navEntryToSidebarItem converts a navigation entry to a sidebar item with count.
func (a *App) navEntryToSidebarItem(ctx context.Context, entry dataentryconfig.NavigationEntry, counts sidebarCounts) V1SidebarItem {
	s := a.State()
	item := V1SidebarItem{
		Label: entry.Label,
	}

	switch {
	case entry.List != "":
		item.Href = "/list/" + entry.List
		item.Icon = "list"
		if list, ok := s.Cfg.Lists[entry.List]; ok {
			count := counts.listCount(ctx, entry.List, list)
			item.Count = &count
		}
	case entry.Kanban != "":
		item.Href = "/kanban/" + entry.Kanban
		item.Icon = "kanban"
		if kanban, ok := s.Cfg.Kanbans[entry.Kanban]; ok {
			count := counts.kanbanCount(ctx, entry.Kanban, kanban)
			item.Count = &count
		}
	case entry.Dashboard:
		item.Href = "/"
		item.Icon = "dashboard"
	case entry.Search:
		item.Href = "/search"
		item.Icon = "search"
	case entry.Settings:
		item.Href = "/settings"
		item.Icon = "settings"
	case entry.Action != "":
		item.Action = entry.Action
		// Href stays empty — frontend renders this as a button
	}

	return item
}

// --- Conflicts API ---

// V1ConflictItem represents a conflicted file.
type V1ConflictItem struct {
	Path        string `json:"path"`
	EntityType  string `json:"entity_type,omitempty"`
	EntityID    string `json:"entity_id,omitempty"`
	MarkerCount int    `json:"marker_count"`
}

// V1ConflictsResponse contains the list of conflicts.
type V1ConflictsResponse struct {
	Conflicts []V1ConflictItem `json:"conflicts"`
	Count     int              `json:"count"`
}

// V1PropertyDiff represents a property difference.
type V1PropertyDiff struct {
	Property    string `json:"property"`
	OursValue   string `json:"ours_value"`
	TheirsValue string `json:"theirs_value"`
	IsSame      bool   `json:"is_same"`
}

// V1ConflictDetail contains detailed info for resolving a conflict.
type V1ConflictDetail struct {
	Path          string           `json:"path"`
	EntityType    string           `json:"entity_type,omitempty"`
	EntityID      string           `json:"entity_id,omitempty"`
	PropertyDiffs []V1PropertyDiff `json:"property_diffs"`
	ContentSame   bool             `json:"content_same"`
	ContentOurs   string           `json:"content_ours,omitempty"`
	ContentTheirs string           `json:"content_theirs,omitempty"`
}

// V1ConflictResolveRequest contains the resolution choices.
type V1ConflictResolveRequest struct {
	Path            string            `json:"path"`
	PropertyChoices map[string]string `json:"property_choices"`
	ContentChoice   string            `json:"content_choice"`
	ManualContent   string            `json:"manual_content,omitempty"`
}

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

	items := make([]V1ConflictItem, 0, len(result.Files))
	for _, cf := range result.Files {
		relPath, _ := filepath.Rel(ctx.Root, cf.Path)
		items = append(items, V1ConflictItem{
			Path:        relPath,
			EntityType:  cf.EntityType,
			EntityID:    cf.EntityID,
			MarkerCount: len(cf.Markers),
		})
	}

	writeV1JSON(w, http.StatusOK, V1ConflictsResponse{
		Conflicts: items,
		Count:     len(items),
	})
}

// handleV1ConflictRoutes handles GET /api/v1/_conflicts/{path} and POST /api/v1/_conflicts/resolve.
func (a *App) handleV1ConflictRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_conflicts/")

	if path == "resolve" && r.Method == http.MethodPost {
		a.handleV1ConflictResolve(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Get conflict details. The path is caller-supplied — contain it to
	// the project root before any filesystem access.
	absPath, ok := a.resolveConflictPath(w, r, path)
	if !ok {
		return
	}

	cf, err := conflict.ParseConflictedFile(absPath, a.State().Meta)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "parse_failed", "Failed to parse conflict", err.Error())
		return
	}

	info := conflict.AnalyzeConflict(cf)

	diffs := make([]V1PropertyDiff, 0, len(info.PropertyDiffs))
	for _, d := range info.PropertyDiffs {
		diffs = append(diffs, V1PropertyDiff{
			Property:    d.Property,
			OursValue:   fmt.Sprintf("%v", d.OursValue),
			TheirsValue: fmt.Sprintf("%v", d.TheirsValue),
			IsSame:      d.IsSame,
		})
	}

	detail := V1ConflictDetail{
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

// handleV1ConflictResolve applies a conflict resolution.
func (a *App) handleV1ConflictResolve(w http.ResponseWriter, r *http.Request) {
	var req V1ConflictResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", err.Error())
		return
	}

	if req.Path == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_path", "Path is required", "")
		return
	}

	// The path is caller-supplied — contain it to the project root
	// before any filesystem access.
	absPath, ok := a.resolveConflictPath(w, r, req.Path)
	if !ok {
		return
	}

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	st := a.State()

	cf, err := conflict.ParseConflictedFile(absPath, st.Meta)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "parse_failed", "Failed to parse conflict", err.Error())
		return
	}

	resolution := &conflict.Resolution{
		PropertyChoices: make(map[string]conflict.Side),
	}

	// Map property choices
	for prop, choice := range req.PropertyChoices {
		if choice == "theirs" {
			resolution.PropertyChoices[prop] = conflict.SideTheirs
		} else {
			resolution.PropertyChoices[prop] = conflict.SideOurs
		}
	}

	// Map content choice
	switch req.ContentChoice {
	case "theirs":
		resolution.ContentChoice = conflict.SideTheirs
	case "manual":
		resolution.ManualContent = req.ManualContent
	default:
		resolution.ContentChoice = conflict.SideOurs
	}

	// Resolve first so the ACL gate evaluates the actual write target
	// (entity vs relation, post-choice identity), then authorize, then
	// write. The write is file-level marker removal and cannot route
	// through entitymanager — the store can't parse a file that still
	// contains conflict markers — so this handler re-authorizes and
	// audits explicitly. The store's file watcher picks the change up
	// as an external edit, keeping index/SSE consumers in sync.
	resolvedEntity, resolvedRelation, err := conflict.Resolve(cf, resolution)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
		return
	}
	if !a.authorizeConflictResolve(r.Context(), w, resolvedEntity, resolvedRelation) {
		return
	}
	if err := conflict.ValidateResolved(resolvedEntity, st.Meta); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
		return
	}
	if err := conflict.WriteResolved(absPath, resolvedEntity, resolvedRelation); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
		return
	}
	a.recordConflictResolveAudit(r.Context(), req.Path, resolvedEntity, resolvedRelation)

	writeV1JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    req.Path,
	})
}

// resolveConflictPath contains the caller-supplied conflict-file path
// to the project root. On failure it writes the error response (404
// when the path is inside the project but missing, 403 when it escapes
// the root) and returns ok=false.
func (a *App) resolveConflictPath(w http.ResponseWriter, r *http.Request, p string) (string, bool) {
	resolved, err := containedProjectPath(a.paths.Root, p)
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

// authorizeConflictResolve re-authorizes the write a conflict
// resolution performs. Conflict resolution bypasses entitymanager, so
// the gate the manager would normally apply lives here: entity files
// gate like an entity update; relation files gate like a relation
// update (source-entity type, mirroring entitymanager.UpdateRelation —
// type left empty when the source entity can't be loaded, the same
// fallback the manager uses). A deny records a `denied-write` audit
// row and writes the standard 403 body; returns true when the write
// may proceed.
func (a *App) authorizeConflictResolve(ctx context.Context, w http.ResponseWriter, e *entityPkg.Entity, rel *entityPkg.Relation) bool {
	var aclReq acl.WriteRequest
	if rel != nil {
		var fromType string
		if fromEntity, ok := a.getEntity(ctx, rel.From); ok {
			fromType = fromEntity.Type
		}
		aclReq = translateRelationWrite(rel.Type, fromType, rel.From)
	} else {
		aclReq = translateVerb("update", e.Type, e.ID)
	}
	decision := a.acl.AuthorizeWrite(ctx, aclReq)
	if decision.Allow {
		return true
	}
	a.auditSink.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     conflictAuditSubject(e, rel),
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s op=conflict-resolve)",
			decision.Reason, decision.RuleKind, decision.RuleID),
	})
	writeForbiddenIfACLDenied(w, &acl.ForbiddenError{Decision: decision})
	return false
}

// recordConflictResolveAudit emits the audit row for a successful
// conflict resolution — the direct-file-write counterpart of the
// records entitymanager emits for manager-routed writes.
func (a *App) recordConflictResolveAudit(ctx context.Context, relPath string, e *entityPkg.Entity, rel *entityPkg.Relation) {
	op := audit.OpUpdateEntity
	if rel != nil {
		op = audit.OpUpdateRelation
	}
	a.auditSink.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          op,
		Subject:     conflictAuditSubject(e, rel),
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     "resolved git conflict in " + relPath,
	})
}

// --- Documents API ---

// V1DocumentResponse contains the rendered document content.
type V1DocumentResponse struct {
	HTML      string   `json:"html"`
	Cached    bool     `json:"cached"`
	EntityIDs []string `json:"entity_ids"` // IDs of entities involved in this document (for SSE filtering)
}

// handleV1Documents handles GET /api/v1/_documents/{docName}/{entityId}.
// Returns JSON with rendered HTML content for Vue SPA consumption.
func (a *App) handleV1Documents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_documents/{docName}/{entityId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_documents/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_documents/{docName}/{entityId}", "")
		return
	}

	docName, entityID := parts[0], parts[1]

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
			writeV1JSON(w, http.StatusOK, V1DocumentResponse{
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
	writeV1JSON(w, http.StatusOK, V1DocumentResponse{
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

// V1Command is the JSON representation of an available command.
type V1Command struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Confirm  string `json:"confirm,omitempty"`
	Context  string `json:"context"`
	AutoOpen *bool  `json:"auto_open,omitempty"`
}

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

	resolved := a.resolveCommands(pageType, qualifier, entityType)

	commands := make([]V1Command, 0, len(resolved))
	for _, cmd := range resolved {
		commands = append(commands, V1Command(cmd))
	}

	writeV1JSON(w, http.StatusOK, commands)
}

// V1Template represents a template for API responses.
type V1Template struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content"`
	Relations  []V1TemplateRelation   `json:"relations"`
}

// V1TemplateRelation represents a pre-filled relation in a template.
type V1TemplateRelation struct {
	Relation string `json:"relation"`
	Target   string `json:"target"`
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
	result := make([]V1Template, 0, len(templates))

	for _, t := range templates {
		relations := make([]V1TemplateRelation, 0, len(t.Relations))
		for _, rel := range t.Relations {
			relations = append(relations, V1TemplateRelation{
				Relation: rel.Type,
				Target:   rel.Target,
			})
		}
		result = append(result, V1Template{
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

// V1ViewResponse contains the executed view data.
//
// Mentions carries the implicit-relation set discovered by scanning the
// entry and section markdown contents for bare-content entity-ID code
// spans (see collectMentions). The SPA's markdown renderer consumes this
// map to rewrite those code spans into titled in-app links. Mirrors the
// Lua-side `rela.md.entity_refs` shape (TKT-LXYHQ) for SPA consumers
// that don't go through the Lua document path.
//
// Wire stability: `mentions` is part of the public v1 API. The set of
// `inaccessible_reason` values may grow as new locking mechanisms are
// added (today: `git-crypt`); clients must treat unknown reasons as
// opaque rather than enumerating them.
type V1ViewResponse struct {
	Entry    V1Entity           `json:"entry"`
	Sections []V1ViewSection    `json:"sections"`
	Mentions map[string]Mention `json:"mentions,omitempty"`
}

// V1ViewSection represents a section with resolved data.
type V1ViewSection struct {
	Heading      string           `json:"heading"`
	SectionID    string           `json:"sectionId"`
	Display      string           `json:"display"`
	IsEmpty      bool             `json:"isEmpty"`
	EmptyMessage string           `json:"emptyMessage,omitempty"`
	Fields       []V1SectionField `json:"fields,omitempty"`
	Entities     []V1ViewEntity   `json:"entities,omitempty"`
	Columns      []V1ViewColumn   `json:"columns,omitempty"`
	Rows         []V1ViewRow      `json:"rows,omitempty"`
	Groups       []V1ViewGroup    `json:"groups,omitempty"`
	IsGrouped    bool             `json:"isGrouped"`
	Content      string           `json:"content,omitempty"`
	HasContent   bool             `json:"hasContent"`
}

// V1ViewEntity represents an entity in a view section.
//
// Props and FieldAffordances (TKT-IHC7D) carry the typed property
// values and per-cell writability verdict that inline-edit hosts on
// cards/list view sections consume. Both are hidden-property-stripped
// — the consumer can assume:
//
//   - `keys(Props) ∩ hidden(e) == ∅` (hidden properties never leak via
//     this surface)
//   - `keys(FieldAffordances) ∩ hidden(e) == ∅` (same for the verdict)
//   - `FieldAffordances` may have keys absent from `Props` when the
//     property has no stored value but a non-default verdict (e.g.
//     `writable: false` on an unset field)
//
// The pointer-to-map idiom on `FieldAffordances` mirrors
// `V1Entity.FieldAffordances`: `nil` means "absent on the wire"
// (table rows / non-cards paths), `&{}` means "evaluated, no
// deviations" (closed-world signal matching `_actions`).
//
// `Props` is a plain map with `omitempty`: presence/absence is
// sufficient, no closed-world semantic is needed.
type V1ViewEntity struct {
	ID               string                        `json:"id"`
	Title            string                        `json:"title"`
	Type             string                        `json:"type"`
	EditFormID       string                        `json:"editFormId,omitempty"`
	Fields           []V1SectionField              `json:"fields,omitempty"`
	Content          string                        `json:"content,omitempty"`
	HasContent       bool                          `json:"hasContent"`
	Props            map[string]any                `json:"_props,omitempty"`
	FieldAffordances *map[string]V1FieldAffordance `json:"_fields,omitempty"`
}

// sectionEntityToV1 lifts a section's row entity (template-side data)
// onto the wire shape. Centralises the `V1ViewEntity` construction so
// the typed `_props` and per-row `_fields` (TKT-IHC7D) stay consistent
// across both the top-level entities path and the (currently dormant)
// grouped-card entities path.
func sectionEntityToV1(e SectionEntityData) V1ViewEntity {
	v1Ent := V1ViewEntity{
		ID:         e.ID,
		Title:      e.Title,
		Type:       e.Type,
		EditFormID: e.EditFormID,
		Content:    e.Content,
		HasContent: e.HasContent,
		Props:      e.Props,
	}
	for _, f := range e.Fields {
		v1Ent.Fields = append(v1Ent.Fields, V1SectionField(f))
	}
	if e.FieldVerdicts != nil {
		fa := e.FieldVerdicts
		v1Ent.FieldAffordances = &fa
	}
	return v1Ent
}

// V1ViewColumn represents a column definition.
type V1ViewColumn struct {
	Property string `json:"property,omitempty"`
	Label    string `json:"label,omitempty"`
	Relation string `json:"relation,omitempty"`
	Link     string `json:"link,omitempty"`
}

// V1ViewRow represents a table row.
type V1ViewRow struct {
	EntityID   string       `json:"entityId"`
	EntityType string       `json:"entityType"`
	EditFormID string       `json:"editFormId,omitempty"`
	Cells      []V1ViewCell `json:"cells"`
	Content    string       `json:"content,omitempty"`
}

// V1ViewCell represents a table cell.
type V1ViewCell struct {
	Values     []string `json:"values"`
	PropType   string   `json:"propType,omitempty"`
	Widget     string   `json:"widget,omitempty"`
	Link       string   `json:"link,omitempty"`
	EntityID   string   `json:"entityId,omitempty"`
	EntityType string   `json:"entityType,omitempty"`
}

// V1ViewGroup represents a group of rows.
type V1ViewGroup struct {
	GroupName string         `json:"groupName"`
	Rows      []V1ViewRow    `json:"rows,omitempty"`
	Entities  []V1ViewEntity `json:"entities,omitempty"`
}

// V1ViewAddInfo describes an add button configuration. Despite the "View"
// prefix this is now used only by V1SidePanelSection — see TKT-6ETQ for
// the rename to V1SidePanelAddInfo. Do not reach for this type from a new
// view-related response: the read-only-view invariant established by
// TKT-651W means no view section should carry add affordances.
type V1ViewAddInfo struct {
	Relation string            `json:"relation"`
	LinkAs   string            `json:"linkAs"`
	PeerID   string            `json:"peerId"`
	Targets  []V1ViewAddTarget `json:"targets"`
}

// V1ViewAddTarget represents a possible target for add action.
// Side-panel-only post TKT-651W; see TKT-6ETQ for the rename plan.
type V1ViewAddTarget struct {
	EntityType string `json:"entityType"`
	FormID     string `json:"formId"`
	Label      string `json:"label"`
}

// V1ViewLinkInfo describes a link existing button configuration.
// Side-panel-only post TKT-651W; see TKT-6ETQ for the rename plan.
type V1ViewLinkInfo struct {
	Relation    string   `json:"relation"`
	LinkAs      string   `json:"linkAs"`
	PeerID      string   `json:"peerId"`
	EntityTypes []string `json:"entityTypes"`
}

// handleV1Views handles GET /api/v1/_views/{entityType}/{entityId}.
// Returns JSON with executed view data including entry and sections.
//
// View configs are looked up by entry.type. When no explicit ViewConfig
// is registered for entityType, a default is synthesized from the
// metamodel (see buildDefaultViewConfig) and executed through the same
// pipeline so the response shape is identical.
func (a *App) handleV1Views(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_views/{entityType}/{entityId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_views/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_views/{entityType}/{entityId}", "")
		return
	}

	entityType, entityID := parts[0], parts[1]
	s := a.State()

	if _, ok := s.Meta.GetEntityDef(entityType); !ok {
		writeV1Error(w, r, http.StatusNotFound, "entity_type_not_found", "Entity type not found", entityType)
		return
	}

	viewCfg, ok := findViewByEntityType(s.Cfg.Views, entityType)
	if !ok {
		viewCfg, ok = buildDefaultViewConfig(s.Meta, entityType)
		if !ok {
			// Cannot happen — entity type already validated above —
			// but handled defensively to keep the contract clear.
			writeV1Error(w, r, http.StatusNotFound, "entity_type_not_found", "Entity type not found", entityType)
			return
		}
	}

	// Execute view
	result, err := a.executeView(r.Context(), viewCfg, entityID)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "view_execution_failed", "View execution failed", err.Error())
		return
	}

	// Build sections
	sections := a.buildSections(r.Context(), viewCfg.Sections, result)

	// Build response
	entityDef := s.Meta.Entities[result.Entry.Type]
	plural := entityDef.GetPlural(result.Entry.Type)

	resp := V1ViewResponse{
		Entry:    a.serializeEntityForWire(r.Context(), result.Entry, plural, true),
		Sections: make([]V1ViewSection, 0, len(sections)),
	}

	for _, sec := range sections {
		v1Sec := V1ViewSection{
			Heading:      sec.Heading,
			SectionID:    sec.SectionID,
			Display:      sec.Display,
			IsEmpty:      sec.IsEmpty,
			EmptyMessage: sec.EmptyMessage,
			IsGrouped:    sec.IsGrouped,
			Content:      sec.Content,
			HasContent:   sec.HasContent,
		}

		// Convert fields
		for _, f := range sec.Fields {
			v1Sec.Fields = append(v1Sec.Fields, V1SectionField(f))
		}

		// Convert entities
		for _, e := range sec.Entities {
			v1Sec.Entities = append(v1Sec.Entities, sectionEntityToV1(e))
		}

		// Convert columns
		for _, col := range sec.Columns {
			v1Sec.Columns = append(v1Sec.Columns, V1ViewColumn{
				Property: col.Property,
				Label:    col.Label,
				Relation: col.Relation,
				Link:     col.Link,
			})
		}

		// Convert rows
		for _, row := range sec.Rows {
			v1Row := V1ViewRow{
				EntityID:   row.EntityID,
				EntityType: row.EntityType,
				EditFormID: row.EditFormID,
				Content:    row.Content,
			}
			for _, cell := range row.Cells {
				v1Row.Cells = append(v1Row.Cells, V1ViewCell(cell))
			}
			v1Sec.Rows = append(v1Sec.Rows, v1Row)
		}

		// Convert groups
		for _, grp := range sec.Groups {
			v1Grp := V1ViewGroup{
				GroupName: grp.GroupName,
			}
			for _, row := range grp.Rows {
				v1Row := V1ViewRow{
					EntityID:   row.EntityID,
					EntityType: row.EntityType,
					EditFormID: row.EditFormID,
					Content:    row.Content,
				}
				for _, cell := range row.Cells {
					v1Row.Cells = append(v1Row.Cells, V1ViewCell(cell))
				}
				v1Grp.Rows = append(v1Grp.Rows, v1Row)
			}
			for _, e := range grp.Entities {
				v1Grp.Entities = append(v1Grp.Entities, sectionEntityToV1(e))
			}
			v1Sec.Groups = append(v1Sec.Groups, v1Grp)
		}

		resp.Sections = append(resp.Sections, v1Sec)
	}

	resp.Mentions = collectMentions(r.Context(), a.store, s.Meta, viewContentBlobs(result.Entry, sections)...)

	writeV1JSON(w, http.StatusOK, resp)
}

// viewContentBlobs gathers every markdown body that will be rendered by
// the SPA for a single view response: the entry's content, every section's
// own content, and every entity card's content (sections with display
// "content"/"cards" surface related entities, each carrying its own
// `Content` markdown that EntityDetail.vue renders with the same
// `refResolver`). Used to scope the mentions scan to text the user
// actually sees on this screen.
func viewContentBlobs(entry *entityPkg.Entity, sections []SectionData) []string {
	blobs := make([]string, 0, 1+len(sections))
	if entry != nil && entry.Content != "" {
		blobs = append(blobs, entry.Content)
	}
	for _, sec := range sections {
		if sec.HasContent && sec.Content != "" {
			blobs = append(blobs, sec.Content)
		}
		for _, ent := range sec.Entities {
			if ent.HasContent && ent.Content != "" {
				blobs = append(blobs, ent.Content)
			}
		}
		for _, grp := range sec.Groups {
			for _, ent := range grp.Entities {
				if ent.HasContent && ent.Content != "" {
					blobs = append(blobs, ent.Content)
				}
			}
		}
	}
	return blobs
}
