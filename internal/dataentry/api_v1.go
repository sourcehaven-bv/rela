package dataentry

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// --- API v1 Types ---

// V1Entity is the JSON representation of an entity for API v1.
type V1Entity struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
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

// V1RelationType is the JSON representation of a relation type.
type V1RelationType struct {
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	From        []string `json:"from"`
	To          []string `json:"to"`
	MinOutgoing *int     `json:"min_outgoing,omitempty"`
	MaxOutgoing *int     `json:"max_outgoing,omitempty"`
	MinIncoming *int     `json:"min_incoming,omitempty"`
	MaxIncoming *int     `json:"max_incoming,omitempty"`
}

// V1CustomType is the JSON representation of a custom type.
type V1CustomType struct {
	Values  []string `json:"values"`
	Default string   `json:"default,omitempty"`
}

// V1Config is the JSON representation of the UI config.
type V1Config struct {
	App        V1AppConfig                           `json:"app"`
	Forms      map[string]dataentryconfig.Form       `json:"forms"`
	Lists      map[string]dataentryconfig.List       `json:"lists"`
	Views      map[string]dataentryconfig.ViewConfig `json:"views"`
	Kanbans    map[string]dataentryconfig.Kanban     `json:"kanbans"`
	Dashboard  *dataentryconfig.DashboardConfig      `json:"dashboard,omitempty"`
	Navigation []dataentryconfig.NavigationEntry     `json:"navigation"`
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
	mux.HandleFunc("/api/v1/_sidepanel/", a.handleV1SidePanel)

	// Dynamic entity routes are handled by a catch-all
	mux.HandleFunc("/api/v1/", a.handleV1DynamicRoutes)
}

// handleV1DynamicRoutes routes requests to the appropriate entity handler based on URL.
func (a *App) handleV1DynamicRoutes(w http.ResponseWriter, r *http.Request) {
	// Skip system routes (already handled)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if strings.HasPrefix(path, "_") {
		http.NotFound(w, r)
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Parse path: {plural}[/{id}[/relations[/{relType}[/{targetId}]]]]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		return
	}

	plural := parts[0]

	// Find entity type by plural
	var typeName string
	for name, def := range a.meta.Entities {
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
	entities := a.g.NodesByType(typeName)

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

	// Build response
	data := make([]V1Entity, 0, len(entities))
	for _, e := range entities {
		data = append(data, a.entityToV1(e, plural, false, false))
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

	writeV1JSON(w, http.StatusOK, resp)
}

func (a *App) handleV1CreateEntity(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	// Need write lock for creation
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	var req struct {
		ID         string                 `json:"id,omitempty"`
		Properties map[string]interface{} `json:"properties"`
		Content    string                 `json:"content,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	entity, _, err := a.ws.CreateEntity(typeName, workspace.CreateOptions{
		ID:         req.ID,
		Properties: req.Properties,
		Content:    req.Content,
	})
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}

	result := a.entityToV1(entity, plural, false, false)

	// Set Location header
	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, entity.ID))

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
	entity, found := a.g.GetNode(entityID)
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
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	entity, found := a.g.GetNode(entityID)
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

	oldEntity := entity.Clone()

	if req.Properties != nil {
		for k, v := range req.Properties {
			entity.Properties[k] = v
		}
	}

	if req.Content != nil {
		entity.Content = *req.Content
	}

	if _, err := a.ws.UpdateEntity(entity, oldEntity); err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}

	result := a.entityToV1(entity, plural, true, false)
	newETag := a.computeEntityETag(entity)
	w.Header().Set("ETag", newETag)

	writeV1JSON(w, http.StatusOK, result)
}

func (a *App) handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, _, entityID string) {
	// Need write lock
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Check for incoming relations
	incoming := a.g.IncomingEdges(entityID)
	if len(incoming) > 0 {
		writeV1Error(w, r, http.StatusConflict, "has_relations",
			"Cannot delete entity with incoming relations",
			fmt.Sprintf("Entity has %d incoming relations", len(incoming)))
		return
	}

	if _, err := a.ws.DeleteEntity(typeName, entityID, true); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "delete_failed", "Failed to delete entity", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Relation Handlers ---

func (a *App) handleV1EntityRelations(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	outgoing := a.g.OutgoingEdges(entityID)
	incoming := a.g.IncomingEdges(entityID)

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
		relDef, ok := a.meta.Relations[edge.Type]
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

func (a *App) handleV1GetRelationType(w http.ResponseWriter, r *http.Request, typeName, entityID, relType string) {
	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	edges := a.g.OutgoingEdges(entityID)
	relations := make([]map[string]interface{}, 0, len(edges))

	for _, edge := range edges {
		if edge.Type != relType {
			continue
		}
		rel := map[string]interface{}{
			"id": edge.To,
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
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	var req struct {
		ID         string                 `json:"id"`
		Properties map[string]interface{} `json:"meta,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	if req.ID == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_id", "Target ID is required", "")
		return
	}

	var opts []workspace.CreateRelationOptions
	if len(req.Properties) > 0 {
		opts = append(opts, workspace.CreateRelationOptions{Properties: req.Properties})
	}

	_, err := a.ws.CreateRelation(entity.ID, relType, req.ID, opts...)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "relation_failed", "Failed to create relation", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *App) handleV1RelationTarget(w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string) {
	if r.Method != http.MethodDelete {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Need write lock
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	if err := a.ws.DeleteRelation(entity.ID, relType, targetID); err != nil {
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
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Clone properties
	props := make(map[string]interface{})
	for k, v := range entity.Properties {
		props[k] = v
	}

	newEntity, _, err := a.ws.CreateEntity(typeName, workspace.CreateOptions{
		Properties: props,
		Content:    entity.Content,
	})
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "Failed to clone entity", err.Error())
		return
	}

	entityDef := a.meta.Entities[typeName]
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

	a.mu.RLock()
	defer a.mu.RUnlock()

	schema := V1Schema{
		Entities:  make(map[string]V1EntityType),
		Relations: make(map[string]V1RelationType),
		Types:     make(map[string]V1CustomType),
	}

	for name, def := range a.meta.Entities {
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
			pd := V1PropertyDef{
				Type:        propDef.Type,
				Required:    propDef.Required,
				Default:     propDef.Default,
				Description: propDef.Description,
				List:        propDef.List,
			}
			if ct, ok := a.meta.Types[propDef.Type]; ok {
				pd.Values = ct.Values
			} else if len(propDef.Values) > 0 {
				pd.Values = propDef.Values
			}
			et.Properties[propName] = pd
		}
		schema.Entities[name] = et
	}

	for name, def := range a.meta.Relations {
		schema.Relations[name] = V1RelationType{
			Label:       def.Label,
			Description: def.Description,
			From:        def.From,
			To:          def.To,
			MinOutgoing: def.MinOutgoing,
			MaxOutgoing: def.MaxOutgoing,
			MinIncoming: def.MinIncoming,
			MaxIncoming: def.MaxIncoming,
		}
	}

	for name, def := range a.meta.Types {
		schema.Types[name] = V1CustomType{
			Values:  def.Values,
			Default: def.Default,
		}
	}

	writeV1JSON(w, http.StatusOK, schema)
}

func (a *App) handleV1SchemaRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_schema/")

	a.mu.RLock()
	defer a.mu.RUnlock()

	switch {
	case path == "types":
		// List entity type names
		names := make([]string, 0, len(a.meta.Entities))
		for name := range a.meta.Entities {
			names = append(names, name)
		}
		sort.Strings(names)
		writeV1JSON(w, http.StatusOK, names)

	case strings.HasPrefix(path, "types/"):
		// Get specific entity type
		typeName := strings.TrimPrefix(path, "types/")
		def, ok := a.meta.Entities[typeName]
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
			if ct, ok := a.meta.Types[propDef.Type]; ok {
				pd.Values = ct.Values
			}
			et.Properties[propName] = pd
		}
		writeV1JSON(w, http.StatusOK, et)

	case path == "relations":
		writeV1JSON(w, http.StatusOK, a.meta.Relations)

	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

func (a *App) handleV1Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	config := V1Config{
		App: V1AppConfig{
			Name:        a.Cfg.App.Name,
			Description: a.Cfg.App.Description,
		},
		Forms:      a.Cfg.Forms,
		Lists:      a.Cfg.Lists,
		Views:      a.Cfg.Views,
		Kanbans:    a.Cfg.Kanbans,
		Dashboard:  a.Cfg.Dashboard,
		Navigation: a.Cfg.Navigation,
	}

	writeV1JSON(w, http.StatusOK, config)
}

func (a *App) handleV1Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	query := r.URL.Query().Get("q")
	if query == "" {
		writeV1JSON(w, http.StatusOK, V1ListResponse{Data: []V1Entity{}, Meta: V1ListMeta{}})
		return
	}

	entities := a.executeQuery(query)

	// Apply type filter if provided
	if typeFilter := r.URL.Query().Get("type"); typeFilter != "" {
		filtered := make([]*model.Entity, 0)
		for _, e := range entities {
			if e.Type == typeFilter {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	data := make([]V1Entity, 0, len(entities))
	for _, e := range entities {
		entityDef := a.meta.Entities[e.Type]
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

	a.mu.RLock()
	defer a.mu.RUnlock()

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

func (a *App) entityToV1(e *model.Entity, plural string, includeRelations, includeActions bool) V1Entity {
	v1 := V1Entity{
		ID:         e.ID,
		Type:       e.Type,
		Properties: make(map[string]interface{}),
		Content:    e.Content,
		Self:       fmt.Sprintf("/api/v1/%s/%s", plural, e.ID),
	}

	for k, v := range e.Properties {
		v1.Properties[k] = v
	}

	if includeRelations {
		v1.Relations = make(map[string][]string)
		for _, edge := range a.g.OutgoingEdges(e.ID) {
			v1.Relations[edge.Type] = append(v1.Relations[edge.Type], edge.To)
		}
	}

	if includeActions {
		v1.Actions = a.computeEntityActions(e)
	}

	return v1
}

func (a *App) computeEntityActions(e *model.Entity) *V1Actions {
	actions := &V1Actions{}

	// Check if entity can be deleted
	incoming := a.g.IncomingEdges(e.ID)
	if len(incoming) > 0 {
		actions.Delete = &V1ActionStatus{
			Allowed: false,
			Reason:  fmt.Sprintf("Has %d incoming relations", len(incoming)),
		}
	} else {
		actions.Delete = &V1ActionStatus{Allowed: true}
	}

	// Get valid status transitions
	if status, ok := e.Properties["status"].(string); ok {
		entityDef := a.meta.Entities[e.Type]
		if statusProp, ok := entityDef.Properties["status"]; ok {
			if ct, ok := a.meta.Types[statusProp.Type]; ok {
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

func (a *App) resolveV1Includes(entity *model.Entity, includes string) map[string]V1Entity {
	included := make(map[string]V1Entity)

	parts := strings.Split(includes, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "_actions" {
			continue
		}

		// Handle nested includes like "implements.requires"
		relParts := strings.SplitN(part, ".", 2)
		relType := relParts[0]

		for _, edge := range a.g.OutgoingEdges(entity.ID) {
			if edge.Type != relType {
				continue
			}
			target, found := a.g.GetNode(edge.To)
			if !found {
				continue
			}
			entityDef := a.meta.Entities[target.Type]
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

func (a *App) applyV1Filters(entities []*model.Entity, query map[string][]string, _ string) []*model.Entity {
	filtered := entities

	for key, values := range query {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}

		// Parse filter[property] or filter[property][operator]
		filterKey := strings.TrimPrefix(key, "filter[")
		filterKey = strings.TrimSuffix(filterKey, "]")
		parts := strings.Split(filterKey, "][")

		property := parts[0]
		operator := "eq"
		if len(parts) > 1 {
			operator = parts[1]
		}
		value := values[0]

		var newFiltered []*model.Entity
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
				if propStr != value {
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
			default:
				// Unknown operator, include entity
				newFiltered = append(newFiltered, e)
			}
		}
		filtered = newFiltered
	}

	return filtered
}

func (a *App) applyV1Sorting(entities []*model.Entity, query map[string][]string) []*model.Entity {
	sortParam := ""
	if vals, ok := query["sort"]; ok && len(vals) > 0 {
		sortParam = vals[0]
	}
	if sortParam == "" {
		return entities
	}

	// Parse sort param: "-created,title" means descending created, ascending title
	sortSpecs := make([]model.SortSpec, 0)
	for _, part := range strings.Split(sortParam, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		spec := model.SortSpec{Direction: "asc"}
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

	sorted := make([]*model.Entity, len(entities))
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

func (a *App) computeEntityETag(e *model.Entity) string {
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
	entityID := parts[1]

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Get form config
	form, ok := a.Cfg.Forms[formID]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "form_not_found", "Form not found", "")
		return
	}

	// Check if form has side panel
	if form.SidePanel == nil {
		writeV1JSON(w, http.StatusOK, []V1SidePanelSection{})
		return
	}

	// Execute side panel traversal
	sections := a.executeSidePanel(form.SidePanel, entityID, form.EntityType)
	if sections == nil {
		writeV1JSON(w, http.StatusOK, []V1SidePanelSection{})
		return
	}

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
			apiSec.Fields = append(apiSec.Fields, V1SectionField{
				Label:    f.Label,
				Value:    f.Value,
				PropType: f.PropType,
			})
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
				apiEnt.Fields = append(apiEnt.Fields, V1SectionField{
					Label:    f.Label,
					Value:    f.Value,
					PropType: f.PropType,
				})
			}
			apiSec.Entities = append(apiSec.Entities, apiEnt)
		}

		result = append(result, apiSec)
	}

	writeV1JSON(w, http.StatusOK, result)
}
