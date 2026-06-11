package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
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

// convertStoreEntity converts an entity.Entity to JSON string with optional relations from store.
func convertStoreEntity(ctx context.Context, e *entity.Entity, st store.Store, includeRelations bool) (string, error) {
	ej := entityJSON{
		ID:         e.ID,
		Type:       e.Type,
		Properties: e.Properties,
		Content:    e.Content,
	}
	if includeRelations {
		ej.Relations = buildStoreRelations(ctx, e.ID, st)
	}
	return marshalJSON(ej)
}

// convertStoreEntitySummary returns a brief summary map from an entity.Entity.
func convertStoreEntitySummary(e *entity.Entity) map[string]interface{} {
	result := map[string]interface{}{
		"id":   e.ID,
		"type": e.Type,
	}
	if title := e.Title(); title != "" {
		result["title"] = title
	}
	if status := e.Status(); status != "" {
		result["status"] = status
	}
	return result
}

// buildStoreRelations builds relation JSON for an entity using the store.
func buildStoreRelations(ctx context.Context, entityID string, st store.Store) *relationsJSON {
	rels := &relationsJSON{
		Outgoing: make(map[string][]relationTargetJSON),
		Incoming: make(map[string][]relationTargetJSON),
	}

	outQ := store.RelationQuery{EntityID: entityID, Direction: store.DirectionOutgoing}
	for r, err := range st.ListRelations(ctx, outQ) {
		if err != nil {
			break
		}
		target := relationTargetJSON{ID: r.To}
		if e, getErr := st.GetEntity(ctx, r.To); getErr == nil {
			target.Title = e.Title()
		}
		rels.Outgoing[r.Type] = append(rels.Outgoing[r.Type], target)
	}

	inQ := store.RelationQuery{EntityID: entityID, Direction: store.DirectionIncoming}
	for r, err := range st.ListRelations(ctx, inQ) {
		if err != nil {
			break
		}
		source := relationTargetJSON{ID: r.From}
		if e, getErr := st.GetEntity(ctx, r.From); getErr == nil {
			source.Title = e.Title()
		}
		rels.Incoming[r.Type] = append(rels.Incoming[r.Type], source)
	}

	if len(rels.Outgoing) == 0 {
		rels.Outgoing = nil
	}
	if len(rels.Incoming) == 0 {
		rels.Incoming = nil
	}
	if rels.Outgoing == nil && rels.Incoming == nil {
		return nil
	}
	return rels
}

// convertStoreRelation converts an entity.Relation to JSON string.
func convertStoreRelation(r *entity.Relation) (string, error) {
	rj := relationJSON{
		From:       r.From,
		Type:       r.Type,
		To:         r.To,
		Properties: r.Properties,
		Content:    r.Content,
	}
	return marshalJSON(rj)
}

// convertTraceResult converts a tracer.TraceResult to JSON string.
func convertTraceResult(tr *tracer.TraceResult) (string, error) {
	node := convertTraceNode(tr)
	return marshalJSON(node)
}

func convertTraceNode(tr *tracer.TraceResult) *traceNodeJSON {
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

// convertPathSteps converts tracer.PathStep slice to JSON string.
func convertPathSteps(steps []tracer.PathStep) (string, error) {
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

// convertStoreRelationsList converts entity.Relation slice to JSON string.
func convertStoreRelationsList(relations []*entity.Relation) (string, error) {
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

// sortStoreRelations sorts entity.Relation slice using natural ordering.
func sortStoreRelations(relations []*entity.Relation) {
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
