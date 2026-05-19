package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
	Title      string `json:"title,omitempty"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // "error" or "warning"
	CheckType  string `json:"checkType"`

	// ScriptError carries the structured Lua-failure envelope for
	// validation script-error rows. Absent (omitempty) on every
	// other row. The frontend uses presence as the discriminator:
	// rows with scriptError open the ScriptErrorDialog instead of
	// navigating to an entity. Same loopback gating as the
	// action-surface envelope (security.AllowFullScriptDetail).
	ScriptError *ScriptErrorEnvelope `json:"scriptError,omitempty"`
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
	s := a.State()
	types := make([]APIEntityType, 0, len(s.Meta.Entities))
	for name, def := range s.Meta.Entities {
		apiType := APIEntityType{
			Name:       name,
			Plural:     def.GetLabelPlural(),
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
			if ct, ok := s.Meta.Types[propDef.Type]; ok {
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
	ctx := r.Context()
	st := a.store
	entityType := r.URL.Query().Get("type")

	q := store.EntityQuery{}
	if entityType != "" {
		q.Type = entityType
	}

	result := make([]APIEntity, 0)
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, a.entityToAPI(e, false))
	}

	writeJSON(w, result)
}

// handleAPIEntity returns a single entity by ID.
func (a *App) handleAPIEntity(w http.ResponseWriter, r *http.Request) { // Extract entity ID from path: /api/entities/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/entities/")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "missing entity ID")
		return
	}

	e, err := a.store.GetEntity(r.Context(), path)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "entity not found")
		return
	}

	result := a.entityToAPI(e, true)
	writeJSON(w, result)
}

// handleAPIMetamodel returns the project metamodel.
func (a *App) handleAPIMetamodel(w http.ResponseWriter, _ *http.Request) {
	s := a.State()
	result := APIMetamodel{
		EntityTypes:   make([]APIEntityType, 0, len(s.Meta.Entities)),
		RelationTypes: make([]APIRelationType, 0, len(s.Meta.Relations)),
	}

	// Entity types
	for name, def := range s.Meta.Entities {
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
			if ct, ok := s.Meta.Types[propDef.Type]; ok {
				apiProp.Values = ct.Values
			}
			apiType.Properties[propName] = apiProp
		}
		result.EntityTypes = append(result.EntityTypes, apiType)
	}

	// Relation types
	for name, def := range s.Meta.Relations {
		result.RelationTypes = append(result.RelationTypes, APIRelationType{
			Name: name,
			From: def.From,
			To:   def.To,
		})
	}

	writeJSON(w, result)
}

// entityToAPI converts an entity.Entity to APIEntity.
func (a *App) entityToAPI(e *entity.Entity, includeRelations bool) APIEntity {
	s := a.State()
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
		ctx := context.Background()
		st := a.store
		api.Relations = make([]APIRelation, 0)

		// Outgoing relations
		outQ := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionOutgoing}
		for edge, err := range st.ListRelations(ctx, outQ) {
			if err != nil {
				break
			}
			target, err := st.GetEntity(ctx, edge.To)
			if err != nil {
				continue
			}
			rel := APIRelation{
				Type:        edge.Type,
				From:        edge.From,
				To:          edge.To,
				Direction:   DirectionOutgoing,
				TargetID:    edge.To,
				TargetTitle: s.Meta.DisplayTitle(target.ID, target.Type, target.Properties),
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
		inQ := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionIncoming}
		for edge, err := range st.ListRelations(ctx, inQ) {
			if err != nil {
				break
			}
			source, err := st.GetEntity(ctx, edge.From)
			if err != nil {
				continue
			}
			rel := APIRelation{
				Type:        edge.Type,
				From:        edge.From,
				To:          edge.To,
				Direction:   DirectionIncoming,
				TargetID:    edge.From,
				TargetTitle: s.Meta.DisplayTitle(source.ID, source.Type, source.Properties),
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

// writeForbiddenIfACLDenied checks whether err is an ACL deny and, if
// so, emits the structured 403 body documented in TKT-GN5LN. Returns
// true when the response has been written and the caller should
// return. The structured body — `{error, rule_kind, rule_id, reason}`
// — lets the SPA surface the specific rule that fired (the AWS IAM
// lesson: opaque denials are unsupportable).
//
// Every handler that calls a write entry point on
// [entitymanager.EntityManager] must invoke this *before* falling
// back to the generic 500 path. The check is cheap (an errors.As
// type assertion) and centralizing the 403 body shape here keeps the
// wire contract identical across all handlers.
func writeForbiddenIfACLDenied(w http.ResponseWriter, err error) bool {
	var fe *acl.ForbiddenError
	if !errors.As(err, &fe) {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":     "forbidden",
		"rule_kind": fe.Decision.RuleKind,
		"rule_id":   fe.Decision.RuleID,
		"reason":    fe.Decision.Reason,
	})
	return true
}

// --- JSON API CRUD Handlers ---
// These endpoints support POST/PUT/DELETE for mobile clients.

// APICreateEntityRequest is the request body for creating an entity.
type APICreateEntityRequest struct {
	ID         string                 `json:"id,omitempty"`     // Optional, auto-generated unless type uses manual IDs
	Prefix     string                 `json:"prefix,omitempty"` // Optional prefix override for multi-prefix types
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

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	entityDef, defOK := a.State().Meta.Entities[req.Type]
	if !defOK {
		writeJSONError(w, http.StatusBadRequest, "unknown entity type: "+req.Type)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Prefix = strings.TrimSpace(req.Prefix)
	// Use 422 — body parsed cleanly but failed a semantic rule. Matches the
	// v1 handler's choice and the new validateCreateIDOpts contract.
	if msg := validateCreateIDOpts(&entityDef, req.ID, req.Prefix); msg != "" {
		writeJSONError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	newEntity := &entity.Entity{
		ID:         req.ID,
		Type:       req.Type,
		Properties: req.Properties,
		Content:    req.Content,
	}
	result, err := a.entityManager.CreateEntity(r.Context(), newEntity, entity.CreateOptions{ID: req.ID, Prefix: req.Prefix})
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		var valErr *entitymanager.ValidationError
		if errors.As(err, &valErr) {
			writeJSONError(w, http.StatusBadRequest, "validation error: "+valErr.Errors[0].Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to create entity: "+err.Error())
		return
	}

	apiResult := a.entityToAPI(result.Entity, false)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(apiResult)
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

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	e, err := a.store.GetEntity(r.Context(), path)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "entity not found")
		return
	}

	// Apply updates
	if req.Properties != nil {
		for k, v := range req.Properties {
			e.Properties[k] = v
		}
	}
	if req.Content != nil {
		e.Content = *req.Content
	}

	result, err := a.entityManager.UpdateEntity(r.Context(), e)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		var valErr *entitymanager.ValidationError
		if errors.As(err, &valErr) {
			writeJSONError(w, http.StatusBadRequest, "validation error: "+valErr.Errors[0].Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update entity: "+err.Error())
		return
	}

	apiResult := a.entityToAPI(result.Entity, false)
	writeJSON(w, apiResult)
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

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	if _, err := a.entityManager.DeleteEntity(r.Context(), path, true); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
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

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	relation, err := a.entityManager.CreateRelation(r.Context(), req.From, req.Type, req.To, entity.RelationOptions{
		Properties: req.Properties,
	})
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
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

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	// Verify the relation exists
	if _, err := a.store.GetRelation(r.Context(), from, relType, to); err != nil {
		writeJSONError(w, http.StatusNotFound, "relation not found")
		return
	}

	if err := a.entityManager.DeleteRelation(r.Context(), from, relType, to); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to delete relation: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleAPIListRelations handles GET /api/relations to list relations.
func (a *App) handleAPIListRelations(w http.ResponseWriter, r *http.Request) {
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
	ctx := context.Background()
	st := a.store
	relations := make([]APIRelation, 0)
	q := store.RelationQuery{EntityID: from, Direction: store.DirectionOutgoing}
	for edge, err := range st.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		target, err := st.GetEntity(ctx, edge.To)
		if err != nil {
			continue
		}
		relations = append(relations, a.edgeToAPIRelation(edge, target, DirectionOutgoing, edge.To))
	}
	return relations
}

// listIncomingRelations returns relations where the given entity is the target.
func (a *App) listIncomingRelations(to string) []APIRelation {
	ctx := context.Background()
	st := a.store
	relations := make([]APIRelation, 0)
	q := store.RelationQuery{EntityID: to, Direction: store.DirectionIncoming}
	for edge, err := range st.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		source, err := st.GetEntity(ctx, edge.From)
		if err != nil {
			continue
		}
		relations = append(relations, a.edgeToAPIRelation(edge, source, DirectionIncoming, edge.From))
	}
	return relations
}

// listAllRelations returns all relations in the graph.
func (a *App) listAllRelations() []APIRelation {
	ctx := context.Background()
	st := a.store
	relations := make([]APIRelation, 0)
	for edge, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			break
		}
		target, err := st.GetEntity(ctx, edge.To)
		if err != nil {
			continue
		}
		relations = append(relations, a.edgeToAPIRelation(edge, target, DirectionOutgoing, edge.To))
	}
	return relations
}

// edgeToAPIRelation converts a store relation to an APIRelation.
func (a *App) edgeToAPIRelation(edge *entity.Relation, relatedEntity *entity.Entity, direction Direction, targetID string) APIRelation {
	rel := APIRelation{
		Type:        edge.Type,
		From:        edge.From,
		To:          edge.To,
		Direction:   direction,
		TargetID:    targetID,
		TargetTitle: a.State().Meta.DisplayTitle(relatedEntity.ID, relatedEntity.Type, relatedEntity.Properties),
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
	UserDefaults  APIUserDefaults                `json:"userDefaults"`
	UserPalette   *dataentryconfig.PaletteConfig `json:"userPalette,omitempty"`
	AllProperties []APIPropertyDef               `json:"allProperties"`
	AllRelations  []APIRelationDef               `json:"allRelations"`
	EntityTypes   []string                       `json:"entityTypes"`
	// LogoURL is the cache-busted URL of the user-uploaded sidebar logo,
	// or nil when no logo is set. The SPA reads this on boot to render
	// the sidebar branding.
	LogoURL *string `json:"logoUrl,omitempty"`
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
	s := a.State()
	ud := s.UserDefaults
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
	for _, entTypeName := range s.Meta.EntityTypes() {
		entDef, ok := s.Meta.GetEntityDef(entTypeName)
		if !ok {
			continue
		}
		for propName, propDef := range entDef.Properties {
			if _, exists := propMap[propName]; !exists {
				propMap[propName] = APIPropertyDef{
					Name:   propName,
					Type:   propDef.Type,
					Values: resolvePropertyValues(propDef, s.Meta),
				}
			} else {
				// Merge values for properties that appear on multiple types
				existing := propMap[propName]
				seen := make(map[string]bool)
				for _, v := range existing.Values {
					seen[v] = true
				}
				for _, v := range resolvePropertyValues(propDef, s.Meta) {
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
	for _, relName := range s.Meta.RelationTypes() {
		relDef, ok := s.Meta.GetRelationDef(relName)
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
				for _, e := range listFromStoreByTypes(a.Services(), []string{targetType}) {
					rd.Targets = append(rd.Targets, APIRelationTarget{
						ID:    e.ID,
						Title: s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
					})
				}
			}
		}
		allRelations = append(allRelations, rd)
	}

	data := APISettingsData{
		UserDefaults:  apiDefaults,
		UserPalette:   s.UserPalette,
		AllProperties: allProperties,
		AllRelations:  allRelations,
		EntityTypes:   s.Meta.EntityTypes(),
	}
	data.LogoURL = s.LogoURL()

	writeJSON(w, data)
}

// handleAPISaveSettings saves the user defaults from JSON input. The save
// to disk and the publication of the new AppState happen atomically via
// mutateState so concurrent readers see a coherent snapshot.
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

	// Save to file under writeMu, then publish a new AppState with the
	// updated UserDefaults via mutateState. The save and the publish
	// are wrapped together so a concurrent reader cannot observe a
	// State whose UserDefaults disagrees with what's on disk.
	var saveErr error
	a.mutateState(func(s *AppState) {
		if err := a.saveUserDefaults(&ud); err != nil {
			saveErr = err
			return
		}
		s.UserDefaults = &ud
	})
	if saveErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save settings: "+saveErr.Error())
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}

// coverage-ignore: HTTP handlers tested via e2e tests

// handleAPIPaletteCRUD routes /api/v1/_palette requests based on HTTP method.
func (a *App) handleAPIPaletteCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIGetPalette(w, r)
	case http.MethodPut, http.MethodPost:
		a.handleAPISavePalette(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetPalette returns the current user palette.
func (a *App) handleAPIGetPalette(w http.ResponseWriter, _ *http.Request) {
	p := a.State().UserPalette
	if p == nil {
		p = &dataentryconfig.PaletteConfig{}
	}
	writeJSON(w, p)
}

// handleAPISavePalette validates and saves the user palette.
func (a *App) handleAPISavePalette(w http.ResponseWriter, r *http.Request) {
	var input dataentryconfig.PaletteConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := dataentryconfig.ValidatePalette(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid palette: "+err.Error())
		return
	}

	// Save to file and publish a new AppState atomically so concurrent
	// readers see the new palette via state.Load() rather than
	// observing torn writes through a shared snapshot pointer.
	var saveErr error
	a.mutateState(func(s *AppState) {
		if err := a.saveUserPalette(&input); err != nil {
			saveErr = err
			return
		}
		s.UserPalette = &input
		s.Palette = dataentryconfig.ResolvePalette(s.Cfg.Palette, &input)
	})
	if saveErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save palette: "+saveErr.Error())
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}
