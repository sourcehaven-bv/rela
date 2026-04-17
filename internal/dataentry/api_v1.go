package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/conflict"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// --- API v1 Types ---

// V1Entity is the JSON representation of an entity for API v1.
type V1Entity struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Title      string                 `json:"_title,omitempty"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content,omitempty"`
	Relations  map[string][]string    `json:"relations,omitempty"`
	Included   map[string]V1Entity    `json:"included,omitempty"`
	Self       string                 `json:"_self,omitempty"`
	Actions    *V1Actions             `json:"_actions,omitempty"`
}

// V1Actions describes available actions for an entity.
type V1Actions struct {
	Delete      *V1ActionStatus `json:"delete,omitempty"`
	Transitions []string        `json:"transitions,omitempty"`
}

// V1ActionStatus describes whether an action is allowed.
type V1ActionStatus struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// V1ListResponse is the response for listing entities.
type V1ListResponse struct {
	Data []V1Entity `json:"data"`
	Meta V1ListMeta `json:"meta"`
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
	MinOutgoing *int                     `json:"min_outgoing,omitempty"`
	MaxOutgoing *int                     `json:"max_outgoing,omitempty"`
	MinIncoming *int                     `json:"min_incoming,omitempty"`
	MaxIncoming *int                     `json:"max_incoming,omitempty"`
	Properties  map[string]V1PropertyDef `json:"properties,omitempty"`
}

// V1CustomType is the JSON representation of a custom type.
type V1CustomType struct {
	Values  []string `json:"values"`
	Default string   `json:"default,omitempty"`
}

// V1Config is the JSON representation of the UI config.
type V1Config struct {
	App        V1AppConfig                               `json:"app"`
	Styles     map[string]map[string]string              `json:"styles"`
	Forms      map[string]dataentryconfig.Form           `json:"forms"`
	Lists      map[string]dataentryconfig.List           `json:"lists"`
	Views      map[string]dataentryconfig.ViewConfig     `json:"views"`
	Kanbans    map[string]dataentryconfig.Kanban         `json:"kanbans"`
	Dashboard  *dataentryconfig.DashboardConfig          `json:"dashboard,omitempty"`
	Actions    map[string]dataentryconfig.Action         `json:"actions,omitempty"`
	Navigation []dataentryconfig.NavigationEntry         `json:"navigation"`
	Documents  map[string]dataentryconfig.DocumentConfig `json:"documents,omitempty"`
	Palette    *dataentryconfig.ResolvedPalette          `json:"palette,omitempty"`
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
func (a *App) registerAPIV1Routes(mux *http.ServeMux) {
	// System endpoints (underscore prefix)
	mux.HandleFunc("/api/v1/_schema", a.handleV1Schema)
	mux.HandleFunc("/api/v1/_schema/", a.handleV1SchemaRoutes)
	mux.HandleFunc("/api/v1/_config", a.handleV1Config)
	mux.HandleFunc("/api/v1/_search", a.handleV1Search)
	mux.HandleFunc("/api/v1/_analyze", a.handleV1Analyze)
	mux.HandleFunc("/api/v1/_git/status", a.handleGitStatus)
	mux.HandleFunc("/api/v1/_git/sync", a.handleGitSync)
	mux.HandleFunc("/api/v1/_settings", a.handleAPISettingsCRUD)
	mux.HandleFunc("/api/v1/_palette", a.handleAPIPaletteCRUD)
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
		if def.GetDirPlural(name) == plural {
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
		a.handleV1CreateEntity(w, r, typeName, plural)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (a *App) handleV1ListEntities(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	entities := listFromStoreByTypes(a.Services(), []string{typeName})

	// Apply filters
	query := r.URL.Query()
	entities = a.applyV1Filters(entities, query, typeName)

	// Apply sorting
	entities = a.applyV1Sorting(entities, query)

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
		v1Entity := a.entityToV1(e, plural, true, false)
		data = append(data, v1Entity)

		// Resolve includes if requested
		if wantIncludes {
			for id, inc := range a.resolveV1Includes(e, includes) {
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
		}
		writeV1JSON(w, http.StatusOK, listWithIncludes{
			Data:     resp.Data,
			Meta:     resp.Meta,
			Included: included,
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
		Properties map[string]interface{} `json:"properties"`
		Content    string                 `json:"content,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	createResult, err := a.ws.EntityManager().CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: req.Properties,
			Content:    req.Content,
		},
		entitymanager.CreateOptions{ID: req.ID},
	)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}
	created := createResult.Entity

	result := a.entityToV1(created, plural, false, false)

	// Set Location header
	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, created.ID))

	// Broadcast entity creation event
	a.broker.broadcastEntityEvent("created", typeName, created.ID)

	writeV1JSON(w, http.StatusCreated, result)
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
	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	query := r.URL.Query()
	includeRelations := true
	includeActions := strings.Contains(query.Get("include"), "_actions")

	result := a.entityToV1(entity, plural, includeRelations, includeActions)

	// Handle includes for related entities
	if includes := query.Get("include"); includes != "" {
		result.Included = a.resolveV1Includes(entity, includes)
	}

	// ETag for caching
	etag := a.computeEntityETag(entity)
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

	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Check If-Match for optimistic locking
	ifMatch := r.Header.Get("If-Match")
	if ifMatch != "" {
		currentETag := a.computeEntityETag(entity)
		if ifMatch != currentETag {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Entity has been modified", "ETag mismatch")
			return
		}
	}

	var req struct {
		Properties map[string]interface{} `json:"properties,omitempty"`
		Content    *string                `json:"content,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	if req.Properties != nil {
		for k, v := range req.Properties {
			entity.Properties[k] = v
		}
	}

	if req.Content != nil {
		entity.Content = *req.Content
	}

	if _, err := a.ws.EntityManager().UpdateEntity(r.Context(), entity); err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}

	result := a.entityToV1(entity, plural, true, false)
	newETag := a.computeEntityETag(entity)
	w.Header().Set("ETag", newETag)

	// Broadcast entity update event
	a.broker.broadcastEntityEvent("updated", typeName, entityID)

	writeV1JSON(w, http.StatusOK, result)
}

func (a *App) handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, _, entityID string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	if _, err := a.ws.EntityManager().DeleteEntity(r.Context(), entityID, true); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "delete_failed", "Failed to delete entity", err.Error())
		return
	}

	// Broadcast entity deletion event
	a.broker.broadcastEntityEvent("deleted", typeName, entityID)

	w.WriteHeader(http.StatusNoContent)
}

// --- Relation Handlers ---

func (a *App) handleV1EntityRelations(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	s := a.State()
	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	outgoing := a.outgoingRelations(entityID)
	incoming := a.incomingRelations(entityID)

	relations := make(map[string][]map[string]interface{})

	for _, edge := range outgoing {
		rel := map[string]interface{}{
			"id":        edge.To,
			"direction": "outgoing",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
		}
		relations[edge.Type] = append(relations[edge.Type], rel)
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
			"direction": "incoming",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
		}
		relations[inverseName] = append(relations[inverseName], rel)
	}

	writeV1JSON(w, http.StatusOK, relations)
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
	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	incoming := r.URL.Query().Get("direction") == string(DirectionIncoming)

	var edges []*entityPkg.Relation
	if incoming {
		edges = a.incomingRelations(entityID)
	} else {
		edges = a.outgoingRelations(entityID)
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
			"id": peerID,
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
		}
		relations = append(relations, rel)
	}

	writeV1JSON(w, http.StatusOK, relations)
}

func (a *App) handleV1CreateRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType string) {
	// Need write lock
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entity, found := a.getEntity(entityID)
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

	from, to := resolveRelationEndpoints(entity.ID, req.ID, req.Direction)

	_, err := a.ws.EntityManager().CreateRelation(r.Context(), from, relType, to, entitymanager.RelationOptions{Properties: req.Meta})
	if err != nil {
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

	entity, found := a.getEntity(entityID)
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

	from, to := resolveRelationEndpoints(entity.ID, targetID, req.Direction)

	rel, err := a.ws.EntityManager().UpdateRelation(r.Context(), from, relType, to, entitymanager.RelationOptions{
		Properties: req.Meta,
	})
	if err != nil {
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

	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	from, to := resolveRelationEndpoints(entity.ID, targetID, r.URL.Query().Get("direction"))

	if err := a.ws.EntityManager().DeleteRelation(r.Context(), from, relType, to); err != nil {
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
	entity, found := a.getEntity(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Clone properties
	props := make(map[string]interface{})
	for k, v := range entity.Properties {
		props[k] = v
	}

	cloneResult, err := a.ws.EntityManager().CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: props,
			Content:    entity.Content,
		},
		entitymanager.CreateOptions{},
	)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "Failed to clone entity", err.Error())
		return
	}
	newEntity := cloneResult.Entity

	entityDef := s.Meta.Entities[typeName]
	plural := entityDef.GetDirPlural(typeName)
	result := a.entityToV1(newEntity, plural, false, false)

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
			Plural:      def.GetDirPlural(name),
			Description: def.Description,
			Primary:     def.GetPrimaryProperty(),
			IDType:      def.GetIDType(),
			Properties:  make(map[string]V1PropertyDef),
		}
		if len(def.GetIDPrefixes()) > 0 {
			et.IDPrefix = def.GetIDPrefixes()[0]
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
			MinOutgoing: def.MinOutgoing,
			MaxOutgoing: def.MaxOutgoing,
			MinIncoming: def.MinIncoming,
			MaxIncoming: def.MaxIncoming,
		}
		if len(def.Properties) > 0 {
			rt.Properties = make(map[string]V1PropertyDef, len(def.Properties))
			for propName, propDef := range def.Properties {
				rt.Properties[propName] = a.toV1PropertyDef(s.Meta, propDef)
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
			Plural:      def.GetDirPlural(typeName),
			Description: def.Description,
			Primary:     def.GetPrimaryProperty(),
			IDType:      def.GetIDType(),
			Properties:  make(map[string]V1PropertyDef),
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
		Styles:     s.StyleMap,
		Forms:      forms,
		Lists:      s.Cfg.Lists,
		Views:      s.Cfg.Views,
		Kanbans:    s.Cfg.Kanbans,
		Dashboard:  s.Cfg.Dashboard,
		Actions:    s.Cfg.Actions,
		Navigation: s.Cfg.Navigation,
		Documents:  s.Cfg.Documents,
		Palette:    s.Palette,
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

	entities := a.executeQuery(query)

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
		plural := entityDef.GetDirPlural(e.Type)
		data = append(data, a.entityToV1(e, plural, false, false))
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

	analysisResult := a.runAnalysis()

	result := APIAnalysisResult{
		Errors:   analysisResult.ErrorCount,
		Warnings: analysisResult.WarningCount,
		Issues:   make([]APIIssue, 0),
		ByCheck:  make(map[string]int),
	}

	for _, section := range analysisResult.Sections {
		for _, issue := range section.Issues {
			result.Issues = append(result.Issues, APIIssue{
				EntityID:   issue.EntityID,
				EntityType: issue.EntityType,
				Message:    issue.Message,
				Severity:   issue.Severity,
				CheckType:  section.Name,
			})
			result.ByCheck[section.Name]++
		}
	}

	writeV1JSON(w, http.StatusOK, result)
}

// --- Helper Functions ---

func (a *App) entityToV1(e *entityPkg.Entity, plural string, includeRelations, includeActions bool) V1Entity {
	s := a.State()
	v1 := V1Entity{
		ID:         e.ID,
		Type:       e.Type,
		Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Properties: make(map[string]interface{}),
		Content:    e.Content,
		Self:       fmt.Sprintf("/api/v1/%s/%s", plural, e.ID),
	}

	for k, v := range e.Properties {
		v1.Properties[k] = v
	}

	if includeRelations {
		v1.Relations = make(map[string][]string)
		for _, edge := range a.outgoingRelations(e.ID) {
			v1.Relations[edge.Type] = append(v1.Relations[edge.Type], edge.To)
		}
	}

	if includeActions {
		v1.Actions = a.computeEntityActions(s, e)
	}

	return v1
}

func (a *App) computeEntityActions(s *AppState, e *entityPkg.Entity) *V1Actions {
	actions := &V1Actions{}

	// Delete is always allowed; cascade removes associated relations.
	actions.Delete = &V1ActionStatus{Allowed: true}

	// Get valid status transitions
	if status, ok := e.Properties["status"].(string); ok {
		entityDef := s.Meta.Entities[e.Type]
		if statusProp, ok := entityDef.Properties["status"]; ok {
			if ct, ok := s.Meta.Types[statusProp.Type]; ok {
				actions.Transitions = ct.Values
			} else if len(statusProp.Values) > 0 {
				actions.Transitions = statusProp.Values
			}
		}
		// Filter out current status
		filtered := make([]string, 0)
		for _, t := range actions.Transitions {
			if t != status {
				filtered = append(filtered, t)
			}
		}
		actions.Transitions = filtered
	}

	return actions
}

func (a *App) resolveV1Includes(entity *entityPkg.Entity, includes string) map[string]V1Entity {
	s := a.State()
	included := make(map[string]V1Entity)

	// Support include=* to include all related entities
	if includes == "*" {
		// Include all outgoing relations
		for _, edge := range a.outgoingRelations(entity.ID) {
			target, found := a.getEntity(edge.To)
			if !found {
				continue
			}
			entityDef := s.Meta.Entities[target.Type]
			plural := entityDef.GetDirPlural(target.Type)
			included[target.ID] = a.entityToV1(target, plural, false, false)
		}
		// Include all incoming relations
		for _, edge := range a.incomingRelations(entity.ID) {
			source, found := a.getEntity(edge.From)
			if !found {
				continue
			}
			entityDef := s.Meta.Entities[source.Type]
			plural := entityDef.GetDirPlural(source.Type)
			included[source.ID] = a.entityToV1(source, plural, false, false)
		}
		return included
	}

	parts := strings.Split(includes, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "_actions" {
			continue
		}

		// Handle nested includes like "implements.requires"
		relParts := strings.SplitN(part, ".", 2)
		relType := relParts[0]

		for _, edge := range a.outgoingRelations(entity.ID) {
			if edge.Type != relType {
				continue
			}
			target, found := a.getEntity(edge.To)
			if !found {
				continue
			}
			entityDef := s.Meta.Entities[target.Type]
			plural := entityDef.GetDirPlural(target.Type)
			included[target.ID] = a.entityToV1(target, plural, false, false)

			// Handle nested includes
			if len(relParts) > 1 {
				nested := a.resolveV1Includes(target, relParts[1])
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

	baseURL := fmt.Sprintf("/api/v1/%s", plural)
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

func (a *App) computeEntityETag(e *entityPkg.Entity) string {
	h := sha256.New()
	_, _ = h.Write([]byte(e.ID))
	_, _ = h.Write([]byte(e.Type))
	_, _ = h.Write([]byte(e.Content))
	for k, v := range e.Properties {
		_, _ = h.Write([]byte(k))
		_, _ = fmt.Fprintf(h, "%v", v)
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("\"%s\"", base64.StdEncoding.EncodeToString(sum[:8]))
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
		Type:     fmt.Sprintf("https://rela.dev/errors/%s", errType),
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
type V1SectionField struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	PropType string `json:"propType,omitempty"`
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
	entry, found := a.getEntity(entityID)
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "entity_not_found", "Entity not found", "")
		return
	}

	// Execute side panel traversal
	sections := a.executeSidePanel(form.SidePanel, entityID, form.EntityType)
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
		n, err := svc.Store.CountEntities(context.Background(), store.EntityQuery{Type: entityType})
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
				sidebarItem := a.navEntryToSidebarItem(item, counts)
				group.Items = append(group.Items, sidebarItem)
			}
			navigation = append(navigation, group)
		} else {
			// Top-level item without group
			item := a.navEntryToSidebarItem(entry, counts)
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
func (c *sidebarCounts) listCount(listID string, list dataentryconfig.List) int {
	key := "list:" + listID
	if n, ok := c.filterCache[key]; ok {
		return n
	}
	n := c.countWithFilters(list.EntityType, list.Filters)
	c.filterCache[key] = n
	return n
}

// kanbanCount returns the entity count for the given kanban, applying
// any configured filters. Results are cached per call.
func (c *sidebarCounts) kanbanCount(kanbanID string, kanban dataentryconfig.Kanban) int {
	key := "kanban:" + kanbanID
	if n, ok := c.filterCache[key]; ok {
		return n
	}
	n := c.countWithFilters(kanban.EntityType, kanban.Filters)
	c.filterCache[key] = n
	return n
}

// countWithFilters returns the count of entities of the given type that
// pass the supplied filters. When filters is empty, the precomputed
// type total is returned directly.
func (c *sidebarCounts) countWithFilters(entityType string, filters []dataentryconfig.FilterConfig) int {
	if len(filters) == 0 {
		return c.typeCounts[entityType]
	}
	entities := listFromStoreByTypes(c.app.Services(), []string{entityType})
	return len(applyFilters(entities, filters))
}

// navEntryToSidebarItem converts a navigation entry to a sidebar item with count.
func (a *App) navEntryToSidebarItem(entry dataentryconfig.NavigationEntry, counts sidebarCounts) V1SidebarItem {
	s := a.State()
	item := V1SidebarItem{
		Label: entry.Label,
	}

	switch {
	case entry.List != "":
		item.Href = "/list/" + entry.List
		item.Icon = "list"
		if list, ok := s.Cfg.Lists[entry.List]; ok {
			count := counts.listCount(entry.List, list)
			item.Count = &count
		}
	case entry.Kanban != "":
		item.Href = "/kanban/" + entry.Kanban
		item.Icon = "kanban"
		if kanban, ok := s.Cfg.Kanbans[entry.Kanban]; ok {
			count := counts.kanbanCount(entry.Kanban, kanban)
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
		Root:         a.ws.Paths().Root,
		EntitiesDir:  a.ws.Paths().EntitiesDir,
		RelationsDir: a.ws.Paths().RelationsDir,
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

	// Get conflict details
	ctx := a.ws.Paths()
	absPath := filepath.Join(ctx.Root, path)

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

	ctx := a.ws.Paths()
	absPath := filepath.Join(ctx.Root, req.Path)

	cf, err := conflict.ParseConflictedFile(absPath, a.State().Meta)
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

	if err := conflict.ResolveAndWrite(cf, resolution, a.State().Meta); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
		return
	}

	writeV1JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    req.Path,
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
	} // Get document config
	docCfg, ok := a.State().Cfg.Documents[docName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "document_not_found", "Document config not found", "")
		return
	}

	// Convert config to workspace format
	wsCfg := a.toWorkspaceDocConfig(&docCfg)

	// Check for refresh param - skip cache if present
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	// Try to get cached content (unless refresh requested)
	if !forceRefresh {
		result := a.ws.GetCachedDocument(entityID, wsCfg)
		if result != nil {
			html := workspace.RewriteDocumentLinks(result.HTML, "")
			writeV1JSON(w, http.StatusOK, V1DocumentResponse{
				HTML:      html,
				Cached:    true,
				EntityIDs: extractEntityIDs(result.Entities),
			})
			return
		}
	}

	// Render the document
	result, err := a.ws.RenderDocument(entityID, wsCfg)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "render_failed", "Document rendering failed", err.Error())
		return
	}

	html := workspace.RewriteDocumentLinks(result.HTML, "")
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

	templates, _ := a.ws.Templater().EntityTemplates(r.Context(), entityType)
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
type V1ViewResponse struct {
	Entry    V1Entity        `json:"entry"`
	Sections []V1ViewSection `json:"sections"`
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
	AddInfo      *V1ViewAddInfo   `json:"addInfo,omitempty"`
	LinkInfo     *V1ViewLinkInfo  `json:"linkInfo,omitempty"`
}

// V1ViewEntity represents an entity in a view section.
type V1ViewEntity struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Type       string           `json:"type"`
	EditFormID string           `json:"editFormId,omitempty"`
	Fields     []V1SectionField `json:"fields,omitempty"`
	Content    string           `json:"content,omitempty"`
	HasContent bool             `json:"hasContent"`
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

// V1ViewAddInfo describes an add button configuration.
type V1ViewAddInfo struct {
	Relation string            `json:"relation"`
	LinkAs   string            `json:"linkAs"`
	PeerID   string            `json:"peerId"`
	Targets  []V1ViewAddTarget `json:"targets"`
}

// V1ViewAddTarget represents a possible target for add action.
type V1ViewAddTarget struct {
	EntityType string `json:"entityType"`
	FormID     string `json:"formId"`
	Label      string `json:"label"`
}

// V1ViewLinkInfo describes a link existing button configuration.
type V1ViewLinkInfo struct {
	Relation    string   `json:"relation"`
	LinkAs      string   `json:"linkAs"`
	PeerID      string   `json:"peerId"`
	EntityTypes []string `json:"entityTypes"`
}

// handleV1Views handles GET /api/v1/_views/{viewId}/{entityId}.
// Returns JSON with executed view data including entry and sections.
func (a *App) handleV1Views(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_views/{viewId}/{entityId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_views/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_views/{viewId}/{entityId}", "")
		return
	}

	viewID, entityID := parts[0], parts[1] // Get view config
	s := a.State()
	viewCfg, ok := s.Cfg.Views[viewID]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "view_not_found", "View not found", "")
		return
	}

	// Execute view
	result, err := a.executeView(viewCfg, entityID)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "view_execution_failed", "View execution failed", err.Error())
		return
	}

	// Build sections
	sections := a.buildSections(viewCfg.Sections, result)

	// Resolve add/link info for sections
	a.resolveSectionButtonsWithTraverse(viewCfg, sections, result.Entry)

	// Build response
	entityDef := s.Meta.Entities[result.Entry.Type]
	plural := entityDef.GetDirPlural(result.Entry.Type)

	resp := V1ViewResponse{
		Entry:    a.entityToV1(result.Entry, plural, true, false),
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
			v1Ent := V1ViewEntity{
				ID:         e.ID,
				Title:      e.Title,
				Type:       e.Type,
				EditFormID: e.EditFormID,
				Content:    e.Content,
				HasContent: e.HasContent,
			}
			for _, f := range e.Fields {
				v1Ent.Fields = append(v1Ent.Fields, V1SectionField(f))
			}
			v1Sec.Entities = append(v1Sec.Entities, v1Ent)
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
				v1Ent := V1ViewEntity{
					ID:         e.ID,
					Title:      e.Title,
					Type:       e.Type,
					EditFormID: e.EditFormID,
					Content:    e.Content,
					HasContent: e.HasContent,
				}
				for _, f := range e.Fields {
					v1Ent.Fields = append(v1Ent.Fields, V1SectionField(f))
				}
				v1Grp.Entities = append(v1Grp.Entities, v1Ent)
			}
			v1Sec.Groups = append(v1Sec.Groups, v1Grp)
		}

		// Convert add/link info
		if sec.AddInfo != nil {
			v1Sec.AddInfo = &V1ViewAddInfo{
				Relation: sec.AddInfo.Relation,
				LinkAs:   sec.AddInfo.LinkAs,
				PeerID:   sec.AddInfo.PeerID,
			}
			for _, t := range sec.AddInfo.Targets {
				v1Sec.AddInfo.Targets = append(v1Sec.AddInfo.Targets, V1ViewAddTarget(t))
			}
		}

		if sec.LinkInfo != nil {
			v1Sec.LinkInfo = &V1ViewLinkInfo{
				Relation:    sec.LinkInfo.Relation,
				LinkAs:      sec.LinkInfo.LinkAs,
				PeerID:      sec.LinkInfo.PeerID,
				EntityTypes: sec.LinkInfo.EntityTypes,
			}
		}

		resp.Sections = append(resp.Sections, v1Sec)
	}

	writeV1JSON(w, http.StatusOK, resp)
}
