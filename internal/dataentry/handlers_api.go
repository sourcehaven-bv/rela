package dataentry

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/model"
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
	Direction   string                 `json:"direction"` // "outgoing" or "incoming"
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

// handleAPIEntityTypes returns a list of entity type definitions.
func (a *App) handleAPIEntityTypes(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	types := make([]APIEntityType, 0, len(a.meta.Entities))
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
				Direction:   "outgoing",
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
				Direction:   "incoming",
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
