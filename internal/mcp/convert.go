package mcp

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// entityJSON represents an entity for JSON output in MCP responses.
type entityJSON struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Relations  *relationsJSON         `json:"relations,omitempty"`
}

// relationsJSON groups outgoing and incoming relations.
type relationsJSON struct {
	Outgoing map[string][]relationTargetJSON `json:"outgoing,omitempty"`
	Incoming map[string][]relationTargetJSON `json:"incoming,omitempty"`
}

// relationTargetJSON represents a related entity.
type relationTargetJSON struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

// relationJSON represents a relation for JSON output.
type relationJSON struct {
	From       string                 `json:"from"`
	Type       string                 `json:"relation"`
	To         string                 `json:"to"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Content    string                 `json:"content,omitempty"`
}

// traceNodeJSON represents a trace result node for JSON output.
type traceNodeJSON struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Title    string           `json:"title"`
	Depth    int              `json:"depth"`
	Relation string           `json:"relation,omitempty"`
	Incoming bool             `json:"incoming,omitempty"`
	Children []*traceNodeJSON `json:"children,omitempty"`
}

// pathStepJSON represents a path step for JSON output.
type pathStepJSON struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Relation string `json:"relation,omitempty"`
}

// convertEntity converts a model.Entity to JSON string with optional relations.
func convertEntity(e *model.Entity, g *graph.Graph, includeRelations bool) (string, error) {
	ej := entityJSON{
		ID:         e.ID,
		Type:       e.Type,
		Properties: e.Properties,
		Content:    e.Content,
	}

	if includeRelations {
		ej.Relations = buildRelations(e.ID, g)
	}

	return marshalJSON(ej)
}

// convertEntitySummary converts an entity to a brief JSON summary (no content, no relations).
func convertEntitySummary(e *model.Entity) map[string]interface{} {
	result := map[string]interface{}{
		"id":   e.ID,
		"type": e.Type,
	}
	if title := e.Title(); title != "" {
		result["title"] = title
	}
	if status := e.GetString("status"); status != "" {
		result["status"] = status
	}
	return result
}

// convertRelation converts a model.Relation to JSON string.
func convertRelation(r *model.Relation) (string, error) {
	rj := relationJSON{
		From:       r.From,
		Type:       r.Type,
		To:         r.To,
		Properties: r.Properties,
		Content:    r.Content,
	}
	return marshalJSON(rj)
}

// convertTraceResult converts a model.TraceResult to JSON string.
func convertTraceResult(tr *model.TraceResult) (string, error) {
	node := convertTraceNode(tr)
	return marshalJSON(node)
}

func convertTraceNode(tr *model.TraceResult) *traceNodeJSON {
	if tr == nil {
		return nil
	}
	node := &traceNodeJSON{
		ID:       tr.ID,
		Type:     tr.Type,
		Title:    tr.Title,
		Depth:    tr.Depth,
		Relation: tr.Relation,
		Incoming: tr.Incoming,
	}
	for _, child := range tr.Children {
		node.Children = append(node.Children, convertTraceNode(child))
	}
	return node
}

// convertPathSteps converts model.PathStep slice to JSON string.
func convertPathSteps(steps []model.PathStep) (string, error) {
	result := make([]pathStepJSON, len(steps))
	for i, s := range steps {
		result[i] = pathStepJSON{
			ID:       s.ID,
			Type:     s.Type,
			Title:    s.Title,
			Relation: s.Relation,
		}
	}
	return marshalJSON(result)
}

// buildRelations builds the relations JSON for an entity.
func buildRelations(entityID string, g *graph.Graph) *relationsJSON {
	outgoing := g.OutgoingEdges(entityID)
	incoming := g.IncomingEdges(entityID)

	if len(outgoing) == 0 && len(incoming) == 0 {
		return nil
	}

	rels := &relationsJSON{
		Outgoing: make(map[string][]relationTargetJSON),
		Incoming: make(map[string][]relationTargetJSON),
	}

	for _, rel := range outgoing {
		target := relationTargetJSON{ID: rel.To}
		if node, ok := g.GetNode(rel.To); ok {
			target.Title = node.Title()
		}
		rels.Outgoing[rel.Type] = append(rels.Outgoing[rel.Type], target)
	}

	for _, rel := range incoming {
		source := relationTargetJSON{ID: rel.From}
		if node, ok := g.GetNode(rel.From); ok {
			source.Title = node.Title()
		}
		rels.Incoming[rel.Type] = append(rels.Incoming[rel.Type], source)
	}

	if len(rels.Outgoing) == 0 {
		rels.Outgoing = nil
	}
	if len(rels.Incoming) == 0 {
		rels.Incoming = nil
	}

	return rels
}

// convertEntitiesList converts a slice of entities to JSON string.
func convertEntitiesList(entities []*model.Entity) (string, error) {
	summaries := make([]map[string]interface{}, len(entities))
	for i, e := range entities {
		summaries[i] = convertEntitySummary(e)
	}
	return marshalJSON(summaries)
}

// convertRelationsList converts a slice of relations to JSON string.
func convertRelationsList(relations []*model.Relation) (string, error) {
	result := make([]relationJSON, len(relations))
	for i, r := range relations {
		result[i] = relationJSON{
			From:       r.From,
			Type:       r.Type,
			To:         r.To,
			Properties: r.Properties,
		}
	}
	return marshalJSON(result)
}

// sortEntitiesByID sorts entities by ID using natural ordering for consistent output.
func sortEntitiesByID(entities []*model.Entity) {
	sort.Slice(entities, func(i, j int) bool {
		return natsort.Less(entities[i].ID, entities[j].ID)
	})
}

// sortRelations sorts relations using natural ordering for consistent output.
func sortRelations(relations []*model.Relation) {
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].From != relations[j].From {
			return natsort.Less(relations[i].From, relations[j].From)
		}
		if relations[i].Type != relations[j].Type {
			return natsort.Less(relations[i].Type, relations[j].Type)
		}
		return natsort.Less(relations[i].To, relations[j].To)
	})
}

func marshalJSON(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}
