package dataentry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// --- REST API v1 Handlers ---
// RESTful API with entity type in the URL path.
// Base path: /api/v1/{entity-type-plural}

// RESTEntity is the JSON representation of an entity in the REST API.
type RESTEntity struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content,omitempty"`
	Relations  map[string][]string    `json:"relations,omitempty"`
	Self       string                 `json:"_self"`
	Actions    *RESTActions           `json:"_actions,omitempty"`
}

// RESTActions contains available actions for an entity.
type RESTActions struct {
	Delete      *RESTDeleteAction `json:"delete,omitempty"`
	Transitions []string          `json:"transitions,omitempty"`
}

// RESTDeleteAction describes delete availability.
type RESTDeleteAction struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// RESTListResponse wraps a list of entities with metadata.
type RESTListResponse struct {
	Data []RESTEntity `json:"data"`
	Meta RESTListMeta `json:"meta"`
}

// RESTListMeta contains pagination metadata.
type RESTListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"perPage"`
	HasMore bool `json:"hasMore"`
}

// RESTCreateRequest is the request body for creating an entity.
type RESTCreateRequest struct {
	ID         string                 `json:"id,omitempty"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content,omitempty"`
}

// RESTUpdateRequest is the request body for updating an entity.
type RESTUpdateRequest struct {
	Properties map[string]interface{} `json:"properties,omitempty"`
	Content    *string                `json:"content,omitempty"`
}

// RESTRelation represents a relation in the REST API.
type RESTRelation struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// RESTCreateRelationRequest is the request body for creating a relation.
type RESTCreateRelationRequest struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// RESTError follows RFC 7807 Problem Details.
type RESTError struct {
	Type     string           `json:"type"`
	Title    string           `json:"title"`
	Status   int              `json:"status"`
	Detail   string           `json:"detail,omitempty"`
	Instance string           `json:"instance,omitempty"`
	Errors   []RESTFieldError `json:"errors,omitempty"`
}

// RESTFieldError represents a validation error for a specific field.
type RESTFieldError struct {
	Source RESTErrorSource `json:"source"`
	Code   string          `json:"code"`
	Detail string          `json:"detail"`
}

// RESTErrorSource identifies the source of a validation error.
type RESTErrorSource struct {
	Pointer string `json:"pointer"`
}

// handleRESTEntities handles /api/v1/{type} for listing and creating entities.
func (a *App) handleRESTEntities(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/v1/{type-plural}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		return
	}

	typePlural := parts[0]
	entityType, entDef := a.findEntityTypeByPlural(typePlural)
	if entityType == "" {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Unknown entity type: %s", typePlural), "")
		return
	}

	// Route based on path structure
	switch {
	case len(parts) == 1:
		// /api/v1/{type}
		switch r.Method {
		case http.MethodGet:
			a.handleRESTListEntities(w, r, entityType, entDef)
		case http.MethodPost:
			a.handleRESTCreateEntity(w, r, entityType)
		default:
			a.writeRESTError(w, r, http.StatusMethodNotAllowed, "method_not_allowed",
				"Method not allowed", "")
		}

	case len(parts) == 2:
		// /api/v1/{type}/{id}
		entityID := parts[1]
		switch r.Method {
		case http.MethodGet:
			a.handleRESTGetEntity(w, r, entityType, entityID)
		case http.MethodPatch:
			a.handleRESTUpdateEntity(w, r, entityType, entityID)
		case http.MethodDelete:
			a.handleRESTDeleteEntity(w, r, entityType, entityID)
		default:
			a.writeRESTError(w, r, http.StatusMethodNotAllowed, "method_not_allowed",
				"Method not allowed", "")
		}

	case len(parts) == 3 && parts[2] == "relations":
		// /api/v1/{type}/{id}/relations
		entityID := parts[1]
		if r.Method == http.MethodGet {
			a.handleRESTGetRelations(w, r, entityType, entityID)
		} else {
			a.writeRESTError(w, r, http.StatusMethodNotAllowed, "method_not_allowed",
				"Method not allowed", "")
		}

	case len(parts) == 4 && parts[2] == "relations":
		// /api/v1/{type}/{id}/relations/{rel-type}
		entityID := parts[1]
		relType := parts[3]
		switch r.Method {
		case http.MethodGet:
			a.handleRESTGetRelationType(w, r, entityType, entityID, relType)
		case http.MethodPost:
			a.handleRESTCreateRelation(w, r, entityType, entityID, relType)
		default:
			a.writeRESTError(w, r, http.StatusMethodNotAllowed, "method_not_allowed",
				"Method not allowed", "")
		}

	case len(parts) == 5 && parts[2] == "relations":
		// /api/v1/{type}/{id}/relations/{rel-type}/{target-id}
		entityID := parts[1]
		relType := parts[3]
		targetID := parts[4]
		if r.Method == http.MethodDelete {
			a.handleRESTDeleteRelation(w, r, entityType, entityID, relType, targetID)
		} else {
			a.writeRESTError(w, r, http.StatusMethodNotAllowed, "method_not_allowed",
				"Method not allowed", "")
		}

	default:
		a.writeRESTError(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

// handleRESTListEntities returns a paginated list of entities.
func (a *App) handleRESTListEntities(w http.ResponseWriter, r *http.Request, entityType string, entDef metamodel.EntityDef) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Pagination params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	// Get all entities of type
	allEntities := a.g.NodesByType(entityType)

	// Sort by ID for deterministic output
	sort.Slice(allEntities, func(i, j int) bool {
		return allEntities[i].ID < allEntities[j].ID
	})

	total := len(allEntities)

	// Apply pagination
	start := (page - 1) * perPage
	end := start + perPage
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	entities := allEntities[start:end]
	hasMore := end < total

	// Build response
	data := make([]RESTEntity, 0, len(entities))
	for _, e := range entities {
		data = append(data, a.entityToREST(e, &entDef, false))
	}

	resp := RESTListResponse{
		Data: data,
		Meta: RESTListMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: hasMore,
		},
	}

	// Add pagination headers
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Per-Page", strconv.Itoa(perPage))

	a.writeRESTJSON(w, http.StatusOK, resp)
}

// handleRESTGetEntity returns a single entity.
func (a *App) handleRESTGetEntity(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Entity not found: %s", entityID), "")
		return
	}

	entDef, _ := a.meta.GetEntityDef(entityType)
	includeRelations := r.URL.Query().Get("include") != ""

	resp := a.entityToREST(entity, entDef, includeRelations)
	a.writeRESTJSON(w, http.StatusOK, resp)
}

// handleRESTCreateEntity creates a new entity.
func (a *App) handleRESTCreateEntity(w http.ResponseWriter, r *http.Request, entityType string) {
	var req RESTCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeRESTError(w, r, http.StatusBadRequest, "invalid_json",
			"Invalid JSON in request body", err.Error())
		return
	}

	if req.Properties == nil {
		req.Properties = make(map[string]interface{})
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entity, _, err := a.ws.CreateEntity(entityType, workspace.CreateOptions{
		ID:         req.ID,
		Properties: req.Properties,
		Content:    req.Content,
	})
	if err != nil {
		var valErr *workspace.ValidationError
		if errors.As(err, &valErr) {
			a.writeRESTValidationError(w, r, valErr)
			return
		}
		// Log the error for debugging
		log.Printf("REST API: failed to create %s entity %s: %v", entityType, req.ID, err)
		a.writeRESTError(w, r, http.StatusInternalServerError, "create_failed",
			fmt.Sprintf("Failed to create %s entity", entityType), err.Error())
		return
	}

	entDef, _ := a.meta.GetEntityDef(entityType)
	resp := a.entityToREST(entity, entDef, false)

	// Set Location header
	w.Header().Set("Location", resp.Self)
	a.writeRESTJSON(w, http.StatusCreated, resp)
}

// handleRESTUpdateEntity updates an existing entity.
func (a *App) handleRESTUpdateEntity(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	var req RESTUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeRESTError(w, r, http.StatusBadRequest, "invalid_json",
			"Invalid JSON in request body", err.Error())
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Entity not found: %s", entityID), "")
		return
	}

	oldEntity := entity.Clone()

	// Merge properties (PATCH semantics)
	if req.Properties != nil {
		for k, v := range req.Properties {
			if v == nil {
				delete(entity.Properties, k)
			} else {
				entity.Properties[k] = v
			}
		}
	}

	if req.Content != nil {
		entity.Content = *req.Content
	}

	if _, err := a.ws.UpdateEntity(entity, oldEntity); err != nil {
		var valErr *workspace.ValidationError
		if errors.As(err, &valErr) {
			a.writeRESTValidationError(w, r, valErr)
			return
		}
		// Log the error for debugging
		log.Printf("REST API: failed to update %s entity %s: %v", entityType, entityID, err)
		a.writeRESTError(w, r, http.StatusInternalServerError, "update_failed",
			fmt.Sprintf("Failed to update %s entity", entityType), err.Error())
		return
	}

	entDef, _ := a.meta.GetEntityDef(entityType)
	resp := a.entityToREST(entity, entDef, false)
	a.writeRESTJSON(w, http.StatusOK, resp)
}

// handleRESTDeleteEntity deletes an entity.
func (a *App) handleRESTDeleteEntity(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Entity not found: %s", entityID), "")
		return
	}

	// Check for incoming relations
	incoming := a.g.IncomingEdges(entityID)
	if len(incoming) > 0 {
		a.writeRESTError(w, r, http.StatusConflict, "has_relations",
			"Cannot delete entity with incoming relations",
			fmt.Sprintf("Entity has %d incoming relation(s)", len(incoming)))
		return
	}

	if _, err := a.ws.DeleteEntity(entityType, entityID, true); err != nil {
		a.writeRESTError(w, r, http.StatusInternalServerError, "delete_failed",
			"Failed to delete entity", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRESTGetRelations returns all relations for an entity.
func (a *App) handleRESTGetRelations(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Entity not found: %s", entityID), "")
		return
	}

	// Group outgoing relations by type
	relations := make(map[string][]RESTRelation)
	for _, edge := range a.g.OutgoingEdges(entityID) {
		rel := RESTRelation{
			ID: edge.To,
		}
		if len(edge.Properties) > 0 {
			rel.Properties = edge.Properties
		}
		relations[edge.Type] = append(relations[edge.Type], rel)
	}

	a.writeRESTJSON(w, http.StatusOK, relations)
}

// handleRESTGetRelationType returns relations of a specific type.
func (a *App) handleRESTGetRelationType(w http.ResponseWriter, r *http.Request, entityType, entityID, relType string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Entity not found: %s", entityID), "")
		return
	}

	// Validate relation type exists and source is valid
	relDef, ok := a.meta.Relations[relType]
	if !ok {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Unknown relation type: %s", relType), "")
		return
	}

	validSource := false
	for _, from := range relDef.From {
		if from == entityType {
			validSource = true
			break
		}
	}
	if !validSource {
		a.writeRESTError(w, r, http.StatusBadRequest, "invalid_source",
			fmt.Sprintf("Entity type %s cannot have %s relations", entityType, relType), "")
		return
	}

	// Get relations of this type
	relations := make([]RESTRelation, 0)
	for _, edge := range a.g.OutgoingEdges(entityID) {
		if edge.Type == relType {
			rel := RESTRelation{
				ID: edge.To,
			}
			if len(edge.Properties) > 0 {
				rel.Properties = edge.Properties
			}
			relations = append(relations, rel)
		}
	}

	a.writeRESTJSON(w, http.StatusOK, relations)
}

// handleRESTCreateRelation creates a new relation.
func (a *App) handleRESTCreateRelation(w http.ResponseWriter, r *http.Request, entityType, entityID, relType string) {
	var req RESTCreateRelationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeRESTError(w, r, http.StatusBadRequest, "invalid_json",
			"Invalid JSON in request body", err.Error())
		return
	}

	if req.ID == "" {
		a.writeRESTError(w, r, http.StatusBadRequest, "missing_id",
			"Target entity ID is required", "")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Validate source entity exists
	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Source entity not found: %s", entityID), "")
		return
	}

	// Validate relation type and source
	relDef, ok := a.meta.Relations[relType]
	if !ok {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Unknown relation type: %s", relType), "")
		return
	}

	validSource := false
	for _, from := range relDef.From {
		if from == entityType {
			validSource = true
			break
		}
	}
	if !validSource {
		a.writeRESTError(w, r, http.StatusBadRequest, "invalid_source",
			fmt.Sprintf("Entity type %s cannot have %s relations", entityType, relType), "")
		return
	}

	// Validate target exists and is valid type
	target, found := a.g.GetNode(req.ID)
	if !found {
		a.writeRESTError(w, r, http.StatusNotFound, "target_not_found",
			fmt.Sprintf("Target entity not found: %s", req.ID), "")
		return
	}

	validTarget := false
	for _, to := range relDef.To {
		if to == target.Type {
			validTarget = true
			break
		}
	}
	if !validTarget {
		a.writeRESTError(w, r, http.StatusBadRequest, "invalid_target",
			fmt.Sprintf("Target entity type %s is not valid for %s relation (expected: %v)",
				target.Type, relType, relDef.To), "")
		return
	}

	// Check for duplicate
	for _, edge := range a.g.OutgoingEdges(entityID) {
		if edge.Type == relType && edge.To == req.ID {
			a.writeRESTError(w, r, http.StatusConflict, "duplicate",
				"Relation already exists", "")
			return
		}
	}

	// Create relation
	var opts []workspace.CreateRelationOptions
	if len(req.Properties) > 0 {
		opts = append(opts, workspace.CreateRelationOptions{Properties: req.Properties})
	}

	if _, err := a.ws.CreateRelation(entityID, relType, req.ID, opts...); err != nil {
		a.writeRESTError(w, r, http.StatusInternalServerError, "create_failed",
			"Failed to create relation", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleRESTDeleteRelation deletes a specific relation.
func (a *App) handleRESTDeleteRelation(w http.ResponseWriter, r *http.Request, entityType, entityID, relType, targetID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entity, found := a.g.GetNode(entityID)
	if !found || entity.Type != entityType {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found",
			fmt.Sprintf("Source entity not found: %s", entityID), "")
		return
	}

	// Find and delete the relation
	relationFound := false
	for _, edge := range a.g.OutgoingEdges(entityID) {
		if edge.Type == relType && edge.To == targetID {
			relationFound = true
			break
		}
	}

	if !relationFound {
		a.writeRESTError(w, r, http.StatusNotFound, "not_found", "Relation not found", "")
		return
	}

	if err := a.ws.DeleteRelation(entityID, relType, targetID); err != nil {
		a.writeRESTError(w, r, http.StatusInternalServerError, "delete_failed",
			"Failed to delete relation", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Helper methods ---

// findEntityTypeByPlural finds an entity type by its plural directory name.
func (a *App) findEntityTypeByPlural(plural string) (string, metamodel.EntityDef) {
	for name, def := range a.meta.Entities {
		if def.GetDirPlural(name) == plural {
			return name, def
		}
	}
	return "", metamodel.EntityDef{}
}

// entityToREST converts a model.Entity to RESTEntity.
func (a *App) entityToREST(e *model.Entity, entDef *metamodel.EntityDef, includeRelations bool) RESTEntity {
	rest := RESTEntity{
		ID:         e.ID,
		Type:       e.Type,
		Properties: make(map[string]interface{}),
		Content:    e.Content,
		Self:       fmt.Sprintf("/api/v1/%s/%s", entDef.GetDirPlural(e.Type), e.ID),
	}

	for k, v := range e.Properties {
		rest.Properties[k] = v
	}

	if includeRelations {
		rest.Relations = make(map[string][]string)
		for _, edge := range a.g.OutgoingEdges(e.ID) {
			rest.Relations[edge.Type] = append(rest.Relations[edge.Type], edge.To)
		}
	}

	// Add actions
	incoming := a.g.IncomingEdges(e.ID)
	rest.Actions = &RESTActions{
		Delete: &RESTDeleteAction{
			Allowed: len(incoming) == 0,
		},
	}
	if len(incoming) > 0 {
		rest.Actions.Delete.Reason = fmt.Sprintf("Has %d incoming relation(s)", len(incoming))
	}

	return rest
}

// writeRESTJSON writes a JSON response.
func (a *App) writeRESTJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeRESTError writes an RFC 7807 error response.
func (a *App) writeRESTError(w http.ResponseWriter, r *http.Request, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	resp := RESTError{
		Type:     "https://rela.dev/errors/" + errType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// writeRESTValidationError writes a validation error response.
func (a *App) writeRESTValidationError(w http.ResponseWriter, r *http.Request, valErr *workspace.ValidationError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)

	resp := RESTError{
		Type:     "https://rela.dev/errors/validation_failed",
		Title:    "Validation failed",
		Status:   http.StatusUnprocessableEntity,
		Instance: r.URL.Path,
		Errors:   make([]RESTFieldError, 0, len(valErr.Errors)),
	}

	for _, e := range valErr.Errors {
		resp.Errors = append(resp.Errors, RESTFieldError{
			Source: RESTErrorSource{Pointer: "/properties"},
			Code:   "invalid",
			Detail: e.Error(),
		})
	}

	_ = json.NewEncoder(w).Encode(resp)
}
