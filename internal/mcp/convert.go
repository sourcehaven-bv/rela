package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// entityJSON represents an entity for JSON output in MCP responses.
type entityJSON struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Face       string         `json:"face,omitempty"`
	Title      string         `json:"title,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
	Relations  *relationsJSON `json:"relations,omitempty"`
}

// relationsJSON groups outgoing and incoming relations.
type relationsJSON struct {
	Outgoing map[string][]relationTargetJSON `json:"outgoing,omitempty"`
	Incoming map[string][]relationTargetJSON `json:"incoming,omitempty"`
}

// relationTargetJSON represents a related entity. ID is the neighbor's
// address: the tail of a content-scoped incoming edge is one face of its
// source, so it is named `ID@face`.
type relationTargetJSON struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

// relationJSON represents a relation for JSON output.
type relationJSON struct {
	From string `json:"from"`
	// FromFace is the source face a content-scoped edge attaches to; empty
	// for an identity-scoped edge.
	FromFace   string         `json:"from_face,omitempty"`
	Type       string         `json:"relation"`
	To         string         `json:"to"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// traceNodeJSON represents a trace result node for JSON output. Depth is
// omitted: it equals the nesting level, so repeating it on every node only
// spends context.
type traceNodeJSON struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Title    string           `json:"title,omitempty"`
	Faces    []entity.Face    `json:"faces,omitempty"`
	Relation string           `json:"relation,omitempty"`
	Incoming bool             `json:"incoming,omitempty"`
	Children []*traceNodeJSON `json:"children,omitempty"`
}

// pathStepJSON represents a path step for JSON output.
type pathStepJSON struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Title    string        `json:"title,omitempty"`
	Faces    []entity.Face `json:"faces,omitempty"`
	Relation string        `json:"relation,omitempty"`
}

// entityView selects the optional parts of an entity rendered by
// [convertStoreEntity].
type entityView struct {
	relations bool
	content   bool
}

// convertStoreEntity converts an entity.Entity to a JSON string, with the
// parts view selects.
func convertStoreEntity(
	ctx context.Context, e *entity.Entity, st GraphReader, meta *metamodel.Metamodel, view entityView,
) (string, error) {
	ej := entityJSON{
		ID:         e.ID,
		Type:       e.Type,
		Face:       e.Face.String(),
		Title:      derivedTitle(meta, e),
		Properties: e.Properties,
	}
	if view.content {
		ej.Content = e.Content
	}
	if view.relations {
		ej.Relations = buildStoreRelations(ctx, e, st, meta)
	}
	return marshalJSON(ej)
}

// displayTitle returns the entity's display name as the metamodel defines it
// (display_property, a template, or the autoderived primary property), or ""
// when that resolves to nothing but the ID. Returning "" rather than the ID
// lets callers omit the field instead of repeating the ID.
//
// Every MCP summary goes through this rather than [entity.Entity.Title]:
// Title reads only a property literally named `title`, which a schema that
// names its entities with `name` or `naam` does not have, and a list of bare
// IDs forces the agent to fetch each entity to tell them apart.
func displayTitle(meta *metamodel.Metamodel, e *entity.Entity) string {
	return titleOrEmpty(e.ID, meta.DisplayTitle(e.ID, e.Type, e.Properties))
}

// derivedTitle is [displayTitle] for a view that already carries every
// property: it returns "" when the title is the value of the type's single
// display property, which the caller can read from the properties, and the
// title otherwise (a display_property template, or the property is absent).
func derivedTitle(meta *metamodel.Metamodel, e *entity.Entity) string {
	if def, ok := meta.GetEntityDef(e.Type); ok {
		if primary := def.GetPrimaryProperty(); primary != "" {
			if _, present := e.Properties[primary]; present {
				return ""
			}
		}
	}
	return displayTitle(meta, e)
}

// titleOrEmpty maps a display title equal to the ID onto "".
func titleOrEmpty(id, title string) string {
	if title == id {
		return ""
	}
	return title
}

// entitySummary is the one-line form of an entity used by every list-shaped
// result: list, search, orphans.
type entitySummary struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Face   string `json:"face,omitempty"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"`
}

// convertStoreEntitySummary returns the summary form of e.
func convertStoreEntitySummary(meta *metamodel.Metamodel, e *entity.Entity) entitySummary {
	summary := entitySummary{
		ID:     e.ID,
		Type:   e.Type,
		Title:  displayTitle(meta, e),
		Status: e.Status(),
	}
	if !e.Face.IsImplicit() {
		summary.Face = e.Face.String()
	}
	return summary
}

// buildStoreRelations builds relation JSON for the served face e.
//
// Face ownership (BUG-ISJHML): a content-scoped edge belongs to one face of
// its source. An OUTGOING edge is therefore listed only when e's own face is
// its tail; identity-scoped edges belong to the entity and are listed with
// every face. An INCOMING edge's head is e, which is entity level, so every
// incoming edge is e's; its tail is named by address, `From@face` for a
// content-scoped one.
//
// Neighbor visibility (RR-CFFL52): an edge is only reported when the entity
// at its far end is readable through st. Listing the edge while dropping
// only the unreadable neighbor's title would still disclose that neighbor's
// ID, and an ID is exactly what the row-level rule protects, since whether
// an entity EXISTS is a genuine secret. So the whole edge is withheld. See
// [neighbor] for how each end is checked.
//
// Every neighbor is answered by ONE [GraphReader.ResolveHeaders] batch, so
// the cost does not grow with the number of edges (RR-XD7YN9).
func buildStoreRelations(
	ctx context.Context, e *entity.Entity, st GraphReader, meta *metamodel.Metamodel,
) *relationsJSON {
	type edge struct {
		typ string
		ref entity.Ref
	}
	var out, in []edge
	var refs []entity.Ref

	outQ := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionOutgoing}
	for r, err := range st.ListRelations(ctx, outQ) {
		if err != nil {
			break
		}
		if metamodel.IsContentScoped(meta, r.Type) && r.FromFace != e.Face {
			continue // another face's content edge
		}
		ref := entity.Ref{ID: r.To}
		out = append(out, edge{typ: r.Type, ref: ref})
		refs = append(refs, ref)
	}

	inQ := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionIncoming}
	for r, err := range st.ListRelations(ctx, inQ) {
		if err != nil {
			break
		}
		ref := entity.Ref{ID: r.From, Face: r.FromFace}
		in = append(in, edge{typ: r.Type, ref: ref})
		refs = append(refs, ref)
	}

	if len(refs) == 0 {
		return nil
	}
	resolved := st.ResolveHeaders(ctx, refs)
	rels := &relationsJSON{
		Outgoing: make(map[string][]relationTargetJSON),
		Incoming: make(map[string][]relationTargetJSON),
	}
	for _, ed := range out {
		if t, ok := neighbor(meta, ed.ref, resolved[ed.ref]); ok {
			rels.Outgoing[ed.typ] = append(rels.Outgoing[ed.typ], t)
		}
	}
	for _, ed := range in {
		if t, ok := neighbor(meta, ed.ref, resolved[ed.ref]); ok {
			rels.Incoming[ed.typ] = append(rels.Incoming[ed.typ], t)
		}
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

// neighbor returns the far end of an edge as the caller may see it, or false
// when the caller may not read it (hidden and absent alike). res is the
// batch answer for ref.
//
// An end with a face (a content-scoped tail) is that face, and must be
// served. An end without one is entity level: it is readable when some face
// of it is (res.Family). Its title belongs to a face, so it comes from the
// face the world resolves the bare id to; a faced neighbor has none in the
// default world until TKT-7IZHP0, and is listed by id alone.
func neighbor(
	meta *metamodel.Metamodel, ref entity.Ref, res visibility.ResolvedHeader,
) (relationTargetJSON, bool) {
	addr := ref.String()
	if res.Served() {
		h := res.Header
		title := titleOrEmpty(h.ID, meta.DisplayTitle(h.ID, h.Type, h.Properties))
		return relationTargetJSON{ID: addr, Title: title}, true
	}
	if !ref.Face.IsImplicit() || !res.Family {
		return relationTargetJSON{}, false
	}
	return relationTargetJSON{ID: ref.ID}, true
}

// convertStoreRelation converts an entity.Relation to JSON string.
func convertStoreRelation(r *entity.Relation) (string, error) {
	rj := relationJSON{
		From:       r.From,
		FromFace:   r.FromFace.String(),
		Type:       r.Type,
		To:         r.To,
		Properties: r.Properties,
		Content:    r.Content,
	}
	return marshalJSON(rj)
}

// convertTraceResult converts a tracer.TraceResult to JSON string.
func convertTraceResult(tr *tracer.TraceResult, meta *metamodel.Metamodel) (string, error) {
	node := convertTraceNode(tr, meta)
	return marshalJSON(node)
}

// convertTraceNode resolves each node's display title from the properties
// the tracer carries. Under a networked wiring those properties have already
// been redacted by the visibility tracer decorator, so the title cannot
// reveal a hidden field.
func convertTraceNode(tr *tracer.TraceResult, meta *metamodel.Metamodel) *traceNodeJSON {
	if tr == nil {
		return nil
	}
	node := &traceNodeJSON{
		ID:       tr.ID,
		Type:     tr.Type,
		Title:    titleOrEmpty(tr.ID, meta.DisplayTitle(tr.ID, tr.Type, tr.Properties)),
		Faces:    tr.Faces,
		Relation: tr.Relation,
		Incoming: tr.Incoming,
	}
	for _, child := range tr.Children {
		node.Children = append(node.Children, convertTraceNode(child, meta))
	}
	return node
}

// convertPathSteps converts a tracer.PathStep slice to a JSON string. title
// resolves a step's display title; a path carries no properties, so the
// caller looks each step up through its gated reader.
func convertPathSteps(steps []tracer.PathStep, title func(tracer.PathStep) string) (string, error) {
	result := make([]pathStepJSON, len(steps))
	for i, s := range steps {
		result[i] = pathStepJSON{
			ID:       s.ID,
			Type:     s.Type,
			Title:    title(s),
			Faces:    s.Faces,
			Relation: s.Relation,
		}
	}
	return marshalJSON(result)
}

// convertStoreRelationsList converts an entity.Relation slice to its JSON
// DTOs. Bodies are left out; list results are summaries.
func convertStoreRelationsList(relations []*entity.Relation) []relationJSON {
	result := make([]relationJSON, len(relations))
	for i, r := range relations {
		result[i] = relationJSON{
			From:       r.From,
			FromFace:   r.FromFace.String(),
			Type:       r.Type,
			To:         r.To,
			Properties: r.Properties,
		}
	}
	return result
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

// marshalJSON renders v as compact JSON. Tool results are read by a model,
// not a person, so indentation only costs context (roughly a third of a
// typical result).
func marshalJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil { // coverage-ignore: defensive: every caller passes entity/relation DTOs and YAML-derived property
		// maps (no chan/func/cycle); json.Marshal cannot fail.
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}
