package dataentry

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// --- JSON API Handlers ---
// These endpoints return JSON for mobile/programmatic access.
// They complement the HTML handlers used by the web UI.

// APIEntity is the JSON representation of an entity for the API.
type APIEntity struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content,omitempty"`
	Relations  []APIRelation          `json:"relations,omitempty"`
}

// APIRelation is the JSON representation of a relation for the API.
type APIRelation struct {
	Type        string                 `json:"type"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Direction   Direction              `json:"direction"` // "outgoing" or "incoming"
	TargetID    string                 `json:"targetId"`
	TargetTitle string                 `json:"targetTitle"`
	TargetType  string                 `json:"targetType"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// APIEntityType is the JSON representation of an entity type definition.
type APIEntityType struct {
	Name       string                 `json:"name"`
	Plural     string                 `json:"plural"`
	Primary    string                 `json:"primary,omitempty"`
	Properties map[string]APIProperty `json:"properties"`
}

// APIProperty is the JSON representation of a property definition.
type APIProperty struct {
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  string   `json:"default,omitempty"`
	Values   []string `json:"values,omitempty"` // for enum types
}

// APIRelationType is the JSON representation of a relation type definition.
type APIRelationType struct {
	Name   string   `json:"name"`
	From   []string `json:"from"`
	To     []string `json:"to"`
	Plural string   `json:"plural,omitempty"`
}

// APIMetamodel is the JSON representation of the project metamodel.
type APIMetamodel struct {
	EntityTypes   []APIEntityType   `json:"entityTypes"`
	RelationTypes []APIRelationType `json:"relationTypes"`
}

// APIAnalysisResult is the JSON representation of analysis results.
type APIAnalysisResult struct {
	Errors   int            `json:"errors"`
	Warnings int            `json:"warnings"`
	Issues   []APIIssue     `json:"issues"`
	ByCheck  map[string]int `json:"byCheck"`
}

// APIIssue is the JSON representation of a single analysis issue.
type APIIssue struct {
	EntityID   string `json:"entityId"`
	EntityType string `json:"entityType"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // "error" or "warning"
	CheckType  string `json:"checkType"`
}

// handleAPIEntitiesCRUD routes /api/entities requests based on HTTP method.
func (a *App) handleAPIEntitiesCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIEntities(w, r)
	case http.MethodPost:
		a.handleAPICreateEntity(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIEntityCRUD routes /api/entities/{id} requests based on HTTP method.
func (a *App) handleAPIEntityCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIEntity(w, r)
	case http.MethodPut, http.MethodPatch:
		a.handleAPIUpdateEntity(w, r)
	case http.MethodDelete:
		a.handleAPIDeleteEntity(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIRelationsCRUD routes /api/relations requests based on HTTP method.
func (a *App) handleAPIRelationsCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIListRelations(w, r)
	case http.MethodPost:
		a.handleAPICreateRelation(w, r)
	case http.MethodDelete:
		a.handleAPIDeleteRelation(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIEntityTypes returns a list of entity type definitions.
func (a *App) handleAPIEntityTypes(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	types := make([]APIEntityType, 0, len(a.meta.Entities))
	for name, def := range a.meta.Entities {
		apiType := APIEntityType{
			Name:       name,
			Plural:     def.GetPlural(),
			Primary:    def.GetPrimaryProperty(),
			Properties: make(map[string]APIProperty),
		}
		for propName, propDef := range def.Properties {
			apiProp := APIProperty{
				Type:     propDef.Type,
				Required: propDef.Required,
				Default:  propDef.Default,
			}
			// Include enum values if this is a custom type
			if ct, ok := a.meta.Types[propDef.Type]; ok {
				apiProp.Values = ct.Values
			}
			apiType.Properties[propName] = apiProp
		}
		types = append(types, apiType)
	}

	writeJSON(w, types)
}

// handleAPIEntities returns entities, optionally filtered by type.
func (a *App) handleAPIEntities(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entityType := r.URL.Query().Get("type")

	var entities []*model.Entity
	if entityType != "" {
		entities = a.g.NodesByType(entityType)
	} else {
		entities = a.g.AllNodes()
	}

	result := make([]APIEntity, 0, len(entities))
	for _, e := range entities {
		result = append(result, a.entityToAPI(e, false))
	}

	writeJSON(w, result)
}

// handleAPIEntity returns a single entity by ID.
func (a *App) handleAPIEntity(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Extract entity ID from path: /api/entities/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/entities/")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "missing entity ID")
		return
	}

	entity, found := a.g.GetNode(path)
	if !found {
		writeJSONError(w, http.StatusNotFound, "entity not found")
		return
	}

	result := a.entityToAPI(entity, true)
	writeJSON(w, result)
}

// handleAPIMetamodel returns the project metamodel.
func (a *App) handleAPIMetamodel(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := APIMetamodel{
		EntityTypes:   make([]APIEntityType, 0, len(a.meta.Entities)),
		RelationTypes: make([]APIRelationType, 0, len(a.meta.Relations)),
	}

	// Entity types
	for name, def := range a.meta.Entities {
		apiType := APIEntityType{
			Name:       name,
			Plural:     def.Plural,
			Primary:    def.GetPrimaryProperty(),
			Properties: make(map[string]APIProperty),
		}
		for propName, propDef := range def.Properties {
			apiProp := APIProperty{
				Type:     propDef.Type,
				Required: propDef.Required,
				Default:  propDef.Default,
			}
			if ct, ok := a.meta.Types[propDef.Type]; ok {
				apiProp.Values = ct.Values
			}
			apiType.Properties[propName] = apiProp
		}
		result.EntityTypes = append(result.EntityTypes, apiType)
	}

	// Relation types
	for name, def := range a.meta.Relations {
		result.RelationTypes = append(result.RelationTypes, APIRelationType{
			Name: name,
			From: def.From,
			To:   def.To,
		})
	}

	writeJSON(w, result)
}

// handleAPIAnalyze returns analysis/validation results.
func (a *App) handleAPIAnalyze(w http.ResponseWriter, _ *http.Request) {
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

	writeJSON(w, result)
}

// handleAPISearch performs a search and returns matching entities.
func (a *App) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, []APIEntity{})
		return
	}

	// Use the existing executeQuery method
	entities := a.executeQuery(query)

	result := make([]APIEntity, 0, len(entities))
	for _, e := range entities {
		result = append(result, a.entityToAPI(e, false))
	}

	writeJSON(w, result)
}

// entityToAPI converts a model.Entity to APIEntity.
func (a *App) entityToAPI(e *model.Entity, includeRelations bool) APIEntity {
	api := APIEntity{
		ID:         e.ID,
		Type:       e.Type,
		Properties: make(map[string]interface{}),
		Content:    e.Content,
	}

	for k, v := range e.Properties {
		api.Properties[k] = v
	}

	if includeRelations {
		api.Relations = make([]APIRelation, 0)

		// Outgoing relations
		for _, edge := range a.g.OutgoingEdges(e.ID) {
			target, found := a.g.GetNode(edge.To)
			if !found {
				continue
			}
			rel := APIRelation{
				Type:        edge.Type,
				From:        edge.From,
				To:          edge.To,
				Direction:   DirectionOutgoing,
				TargetID:    edge.To,
				TargetTitle: a.entityDisplayTitle(target),
				TargetType:  target.Type,
			}
			if edge.Properties != nil {
				rel.Properties = make(map[string]interface{})
				for k, v := range edge.Properties {
					rel.Properties[k] = v
				}
			}
			api.Relations = append(api.Relations, rel)
		}

		// Incoming relations
		for _, edge := range a.g.IncomingEdges(e.ID) {
			source, found := a.g.GetNode(edge.From)
			if !found {
				continue
			}
			rel := APIRelation{
				Type:        edge.Type,
				From:        edge.From,
				To:          edge.To,
				Direction:   DirectionIncoming,
				TargetID:    edge.From,
				TargetTitle: a.entityDisplayTitle(source),
				TargetType:  source.Type,
			}
			if edge.Properties != nil {
				rel.Properties = make(map[string]interface{})
				for k, v := range edge.Properties {
					rel.Properties[k] = v
				}
			}
			api.Relations = append(api.Relations, rel)
		}
	}

	return api
}

// writeJSON writes a JSON response with 200 OK status.
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// --- JSON API CRUD Handlers ---
// These endpoints support POST/PUT/DELETE for mobile clients.

// APICreateEntityRequest is the request body for creating an entity.
type APICreateEntityRequest struct {
	ID         string                 `json:"id,omitempty"` // Optional, auto-generated if empty
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Content    string                 `json:"content,omitempty"`
}

// APIUpdateEntityRequest is the request body for updating an entity.
type APIUpdateEntityRequest struct {
	Properties map[string]interface{} `json:"properties,omitempty"`
	Content    *string                `json:"content,omitempty"` // Pointer to distinguish empty from not provided
}

// APICreateRelationRequest is the request body for creating a relation.
type APICreateRelationRequest struct {
	From       string                 `json:"from"`
	Type       string                 `json:"type"`
	To         string                 `json:"to"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// handleAPICreateEntity handles POST /api/entities to create a new entity.
func (a *App) handleAPICreateEntity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req APICreateEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Type == "" {
		writeJSONError(w, http.StatusBadRequest, "type is required")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entity, _, err := a.ws.CreateEntity(req.Type, workspace.CreateOptions{
		ID:         req.ID,
		Properties: req.Properties,
		Content:    req.Content,
	})
	if err != nil {
		var valErr *workspace.ValidationError
		if errors.As(err, &valErr) {
			writeJSONError(w, http.StatusBadRequest, "validation error: "+valErr.Errors[0].Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to create entity: "+err.Error())
		return
	}

	// Return created entity
	result := a.entityToAPI(entity, false)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

// handleAPIUpdateEntity handles PUT /api/entities/{id} to update an entity.
func (a *App) handleAPIUpdateEntity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		writeJSONError(w, http.StatusMethodNotAllowed, "PUT or PATCH required")
		return
	}

	// Extract entity ID from path: /api/entities/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/entities/")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "missing entity ID")
		return
	}

	var req APIUpdateEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entity, found := a.g.GetNode(path)
	if !found {
		writeJSONError(w, http.StatusNotFound, "entity not found")
		return
	}

	oldEntity := entity.Clone()

	// Update properties
	if req.Properties != nil {
		for k, v := range req.Properties {
			entity.Properties[k] = v
		}
	}

	// Update content if provided
	if req.Content != nil {
		entity.Content = *req.Content
	}

	if _, err := a.ws.UpdateEntity(entity, oldEntity); err != nil {
		var valErr *workspace.ValidationError
		if errors.As(err, &valErr) {
			writeJSONError(w, http.StatusBadRequest, "validation error: "+valErr.Errors[0].Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update entity: "+err.Error())
		return
	}

	// Return updated entity
	result := a.entityToAPI(entity, false)
	writeJSON(w, result)
}

// handleAPIDeleteEntity handles DELETE /api/entities/{id} to delete an entity.
func (a *App) handleAPIDeleteEntity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "DELETE required")
		return
	}

	// Extract entity ID from path: /api/entities/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/entities/")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "missing entity ID")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entity, found := a.g.GetNode(path)
	if !found {
		writeJSONError(w, http.StatusNotFound, "entity not found")
		return
	}

	if _, err := a.ws.DeleteEntity(entity.Type, entity.ID, true); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete entity: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleAPICreateRelation handles POST /api/relations to create a new relation.
func (a *App) handleAPICreateRelation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req APICreateRelationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.From == "" || req.Type == "" || req.To == "" {
		writeJSONError(w, http.StatusBadRequest, "from, type, and to are required")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	var opts []workspace.CreateRelationOptions
	if len(req.Properties) > 0 {
		opts = append(opts, workspace.CreateRelationOptions{Properties: req.Properties})
	}

	relation, err := a.ws.CreateRelation(req.From, req.Type, req.To, opts...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create relation: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"from": relation.From,
		"type": relation.Type,
		"to":   relation.To,
	})
}

// handleAPIDeleteRelation handles DELETE /api/relations to delete a relation.
func (a *App) handleAPIDeleteRelation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "DELETE required")
		return
	}

	from := r.URL.Query().Get("from")
	relType := r.URL.Query().Get("type")
	to := r.URL.Query().Get("to")

	if from == "" || relType == "" || to == "" {
		writeJSONError(w, http.StatusBadRequest, "from, type, and to query params required")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Find the relation
	var targetEdge *model.Relation
	for _, edge := range a.g.OutgoingEdges(from) {
		if edge.Type == relType && edge.To == to {
			targetEdge = edge
			break
		}
	}

	if targetEdge == nil {
		writeJSONError(w, http.StatusNotFound, "relation not found")
		return
	}

	if err := a.ws.DeleteRelation(from, relType, to); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete relation: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleAPIListRelations handles GET /api/relations to list relations.
func (a *App) handleAPIListRelations(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	var relations []APIRelation

	switch {
	case from != "":
		relations = a.listOutgoingRelations(from)
	case to != "":
		relations = a.listIncomingRelations(to)
	default:
		relations = a.listAllRelations()
	}

	if relations == nil {
		relations = []APIRelation{}
	}

	writeJSON(w, relations)
}

// listOutgoingRelations returns relations where the given entity is the source.
func (a *App) listOutgoingRelations(from string) []APIRelation {
	edges := a.g.OutgoingEdges(from)
	relations := make([]APIRelation, 0, len(edges))
	for _, edge := range edges {
		target, found := a.g.GetNode(edge.To)
		if !found {
			continue
		}
		rel := a.edgeToAPIRelation(edge, target, DirectionOutgoing, edge.To)
		relations = append(relations, rel)
	}
	return relations
}

// listIncomingRelations returns relations where the given entity is the target.
func (a *App) listIncomingRelations(to string) []APIRelation {
	edges := a.g.IncomingEdges(to)
	relations := make([]APIRelation, 0, len(edges))
	for _, edge := range edges {
		source, found := a.g.GetNode(edge.From)
		if !found {
			continue
		}
		rel := a.edgeToAPIRelation(edge, source, DirectionIncoming, edge.From)
		relations = append(relations, rel)
	}
	return relations
}

// listAllRelations returns all relations in the graph.
func (a *App) listAllRelations() []APIRelation {
	edges := a.g.AllEdges()
	relations := make([]APIRelation, 0, len(edges))
	for _, edge := range edges {
		target, found := a.g.GetNode(edge.To)
		if !found {
			continue
		}
		rel := a.edgeToAPIRelation(edge, target, DirectionOutgoing, edge.To)
		relations = append(relations, rel)
	}
	return relations
}

// edgeToAPIRelation converts a graph edge to an APIRelation.
func (a *App) edgeToAPIRelation(edge *model.Relation, relatedEntity *model.Entity, direction Direction, targetID string) APIRelation {
	rel := APIRelation{
		Type:        edge.Type,
		From:        edge.From,
		To:          edge.To,
		Direction:   direction,
		TargetID:    targetID,
		TargetTitle: a.entityDisplayTitle(relatedEntity),
		TargetType:  relatedEntity.Type,
	}
	if edge.Properties != nil {
		rel.Properties = make(map[string]interface{})
		for k, v := range edge.Properties {
			rel.Properties[k] = v
		}
	}
	return rel
}

// --- Settings API ---

// APISettingsData contains all data needed for the settings page.
type APISettingsData struct {
	UserDefaults  APIUserDefaults  `json:"userDefaults"`
	AllProperties []APIPropertyDef `json:"allProperties"`
	AllRelations  []APIRelationDef `json:"allRelations"`
	EntityTypes   []string         `json:"entityTypes"`
}

// APIUserDefaults is the JSON representation of user defaults.
type APIUserDefaults struct {
	Defaults         map[string]string    `json:"defaults"`
	RelationDefaults map[string]string    `json:"relationDefaults"`
	Overrides        []APIDefaultOverride `json:"overrides"`
}

// APIDefaultOverride is the JSON representation of a default override.
type APIDefaultOverride struct {
	Types            []string          `json:"types"`
	Defaults         map[string]string `json:"defaults"`
	RelationDefaults map[string]string `json:"relationDefaults"`
}

// APIPropertyDef describes a property for the settings page.
type APIPropertyDef struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Values []string `json:"values"`
}

// APIRelationDef describes a relation for the settings page.
type APIRelationDef struct {
	Name       string              `json:"name"`
	Label      string              `json:"label"`
	TargetType string              `json:"targetType"`
	Targets    []APIRelationTarget `json:"targets"`
}

// APIRelationTarget is a possible target for a relation.
type APIRelationTarget struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// handleAPISettingsCRUD routes /api/v1/settings requests based on HTTP method.
func (a *App) handleAPISettingsCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIGetSettings(w, r)
	case http.MethodPut, http.MethodPost:
		a.handleAPISaveSettings(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetSettings returns the settings data for the settings page.
func (a *App) handleAPIGetSettings(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ud := a.userDefaults
	if ud == nil {
		ud = &UserDefaults{}
	}

	// Convert user defaults to API format
	apiDefaults := APIUserDefaults{
		Defaults:         ud.Defaults,
		RelationDefaults: ud.RelationDefaults,
	}
	if apiDefaults.Defaults == nil {
		apiDefaults.Defaults = make(map[string]string)
	}
	if apiDefaults.RelationDefaults == nil {
		apiDefaults.RelationDefaults = make(map[string]string)
	}
	for _, o := range ud.Overrides {
		apiOverride := APIDefaultOverride{
			Types:            o.Types,
			Defaults:         o.Defaults,
			RelationDefaults: o.RelationDefaults,
		}
		if apiOverride.Defaults == nil {
			apiOverride.Defaults = make(map[string]string)
		}
		if apiOverride.RelationDefaults == nil {
			apiOverride.RelationDefaults = make(map[string]string)
		}
		apiDefaults.Overrides = append(apiDefaults.Overrides, apiOverride)
	}

	// Collect all properties across entity types
	propMap := make(map[string]APIPropertyDef)
	for _, entTypeName := range a.meta.EntityTypes() {
		entDef, ok := a.meta.GetEntityDef(entTypeName)
		if !ok {
			continue
		}
		for propName, propDef := range entDef.Properties {
			if _, exists := propMap[propName]; !exists {
				propMap[propName] = APIPropertyDef{
					Name:   propName,
					Type:   propDef.Type,
					Values: resolvePropertyValues(propDef, a.meta),
				}
			} else {
				// Merge values for properties that appear on multiple types
				existing := propMap[propName]
				seen := make(map[string]bool)
				for _, v := range existing.Values {
					seen[v] = true
				}
				for _, v := range resolvePropertyValues(propDef, a.meta) {
					if !seen[v] {
						existing.Values = append(existing.Values, v)
						seen[v] = true
					}
				}
				propMap[propName] = existing
			}
		}
	}
	allProperties := make([]APIPropertyDef, 0, len(propMap))
	for _, p := range propMap {
		allProperties = append(allProperties, p)
	}

	// Collect all relation types with their targets
	allRelations := make([]APIRelationDef, 0)
	for _, relName := range a.meta.RelationTypes() {
		relDef, ok := a.meta.GetRelationDef(relName)
		if !ok {
			continue
		}
		rd := APIRelationDef{
			Name:  relName,
			Label: relDef.Label,
		}
		if len(relDef.To) > 0 {
			rd.TargetType = relDef.To[0]
			for _, targetType := range relDef.To {
				for _, e := range a.g.NodesByType(targetType) {
					rd.Targets = append(rd.Targets, APIRelationTarget{
						ID:    e.ID,
						Title: a.entityDisplayTitle(e),
					})
				}
			}
		}
		allRelations = append(allRelations, rd)
	}

	data := APISettingsData{
		UserDefaults:  apiDefaults,
		AllProperties: allProperties,
		AllRelations:  allRelations,
		EntityTypes:   a.meta.EntityTypes(),
	}

	writeJSON(w, data)
}

// handleAPISaveSettings saves the user defaults from JSON input.
func (a *App) handleAPISaveSettings(w http.ResponseWriter, r *http.Request) {
	var input APIUserDefaults
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Convert API format to internal UserDefaults
	ud := UserDefaults{
		Defaults:         input.Defaults,
		RelationDefaults: input.RelationDefaults,
	}
	for _, o := range input.Overrides {
		ud.Overrides = append(ud.Overrides, DefaultOverride{
			Types:            o.Types,
			Defaults:         o.Defaults,
			RelationDefaults: o.RelationDefaults,
		})
	}

	// Save to file
	if err := a.saveUserDefaults(&ud); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save settings: "+err.Error())
		return
	}

	a.mu.Lock()
	a.userDefaults = &ud
	a.mu.Unlock()

	writeJSON(w, map[string]bool{"ok": true})
}
