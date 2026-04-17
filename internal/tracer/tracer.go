// Package tracer provides graph traversal operations (trace, path,
// orphan detection, cycle detection, clustering) as a service separate
// from the store.
//
// The generic Tracer reads from a store.EntityReader + store.RelationReader.
// Smart backends (e.g. Postgres) can provide native implementations using
// recursive CTEs without going through the store abstraction.
package tracer

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TraceResult represents a tree of entities reachable from a starting point.
type TraceResult struct {
	ID       string
	Type     string
	Title    string
	Depth    int
	Relation string // relation that led to this node
	Incoming bool   // reached via an incoming relation
	Children []*TraceResult
}

// PathStep represents one step in a path between two entities.
type PathStep struct {
	ID       string
	Type     string
	Title    string
	Relation string // relation that led to this step
}

// Tracer provides graph traversal operations.
type Tracer interface {
	// TraceFrom follows outgoing and incoming edges from the given entity.
	TraceFrom(ctx context.Context, id string, maxDepth int) *TraceResult

	// TraceTo follows incoming edges only (upstream dependencies).
	TraceTo(ctx context.Context, id string, maxDepth int) *TraceResult

	// FindPath finds the shortest path between two entities (BFS, undirected).
	FindPath(ctx context.Context, fromID, toID string) []PathStep

	// FindOrphans returns IDs of entities with no relations.
	FindOrphans(ctx context.Context) ([]string, error)

	// HasCycle returns true if there is a cycle reachable from the given entity.
	HasCycle(ctx context.Context, startID string) bool
}

// reader combines EntityReader and RelationReader for the generic tracer.
type reader interface {
	store.EntityReader
	store.RelationReader
}

// New creates a generic Tracer backed by a store's entity and relation readers.
func New(r reader) *GenericTracer {
	return &GenericTracer{r: r}
}

// GenericTracer implements Tracer by reading from the store.
type GenericTracer struct {
	r reader
}

var _ Tracer = (*GenericTracer)(nil)

func (t *GenericTracer) TraceFrom(ctx context.Context, id string, maxDepth int) *TraceResult {
	visited := make(map[string]bool)
	return t.traceBidirectional(ctx, id, 0, maxDepth, "", false, visited)
}

func (t *GenericTracer) TraceTo(ctx context.Context, id string, maxDepth int) *TraceResult {
	visited := make(map[string]bool)
	return t.traceTo(ctx, id, 0, maxDepth, "", visited)
}

func (t *GenericTracer) traceBidirectional(
	ctx context.Context,
	id string, depth, maxDepth int, relation string, incoming bool,
	visited map[string]bool,
) *TraceResult {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	e, err := t.r.GetEntity(ctx, id)
	if err != nil {
		return nil
	}

	result := &TraceResult{
		ID:       id,
		Type:     e.Type,
		Title:    e.Title(),
		Depth:    depth,
		Relation: relation,
		Incoming: incoming,
	}

	if visited[id] {
		return result
	}
	visited[id] = true

	// Outgoing edges
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
	for r, err := range t.r.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		child := t.traceBidirectional(ctx, r.To, depth+1, maxDepth, r.Type, false, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	// Incoming edges
	q = store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming}
	for r, err := range t.r.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		child := t.traceBidirectional(ctx, r.From, depth+1, maxDepth, r.Type, true, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	return result
}

func (t *GenericTracer) traceTo(
	ctx context.Context,
	id string, depth, maxDepth int, relation string,
	visited map[string]bool,
) *TraceResult {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	e, err := t.r.GetEntity(ctx, id)
	if err != nil {
		return nil
	}

	result := &TraceResult{
		ID:       id,
		Type:     e.Type,
		Title:    e.Title(),
		Depth:    depth,
		Relation: relation,
	}

	if visited[id] {
		return result
	}
	visited[id] = true

	q := store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming}
	for r, err := range t.r.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		child := t.traceTo(ctx, r.From, depth+1, maxDepth, r.Type, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	return result
}

type pathNeighbor struct {
	id, relation string
}

type pathQueueItem struct {
	id   string
	path []PathStep
}

func (t *GenericTracer) FindPath(ctx context.Context, fromID, toID string) []PathStep {
	if fromID == toID {
		e, err := t.r.GetEntity(ctx, fromID)
		if err != nil {
			return nil
		}
		return []PathStep{{ID: fromID, Type: e.Type, Title: e.Title()}}
	}

	queue := t.initPathQueue(ctx, fromID)
	visited := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		for _, nb := range t.collectNeighbors(ctx, current.id) {
			if nb.id == toID {
				if step, ok := t.step(ctx, toID, nb.relation); ok {
					return append(clonePath(current.path), step)
				}
			}
			if visited[nb.id] {
				continue
			}
			if step, ok := t.step(ctx, nb.id, nb.relation); ok {
				newPath := append(clonePath(current.path), step)
				queue = append(queue, pathQueueItem{id: nb.id, path: newPath})
			}
		}
	}

	return nil
}

func (t *GenericTracer) initPathQueue(ctx context.Context, fromID string) []pathQueueItem {
	e, err := t.r.GetEntity(ctx, fromID)
	if err != nil {
		return nil
	}
	return []pathQueueItem{{
		id:   fromID,
		path: []PathStep{{ID: fromID, Type: e.Type, Title: e.Title()}},
	}}
}

func (t *GenericTracer) collectNeighbors(ctx context.Context, id string) []pathNeighbor {
	var out []pathNeighbor
	for r, err := range t.r.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}) {
		if err != nil {
			break
		}
		out = append(out, pathNeighbor{r.To, r.Type})
	}
	for r, err := range t.r.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming}) {
		if err != nil {
			break
		}
		out = append(out, pathNeighbor{r.From, r.Type})
	}
	return out
}

func (t *GenericTracer) step(ctx context.Context, id, relation string) (PathStep, bool) {
	e, err := t.r.GetEntity(ctx, id)
	if err != nil {
		return PathStep{}, false
	}
	return PathStep{ID: id, Type: e.Type, Title: e.Title(), Relation: relation}, true
}

func clonePath(p []PathStep) []PathStep {
	out := make([]PathStep, len(p), len(p)+1)
	copy(out, p)
	return out
}

func (t *GenericTracer) FindOrphans(ctx context.Context) ([]string, error) {
	var orphans []string
	for e, err := range t.r.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			return nil, err
		}
		n, err := t.r.CountRelations(ctx, store.RelationQuery{EntityID: e.ID, Direction: store.DirectionBoth})
		if err != nil {
			return nil, err
		}
		if n == 0 {
			orphans = append(orphans, e.ID)
		}
	}
	return orphans, nil
}

func (t *GenericTracer) HasCycle(ctx context.Context, startID string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	return t.hasCycle(ctx, startID, visited, recStack)
}

func (t *GenericTracer) hasCycle(ctx context.Context, id string, visited, recStack map[string]bool) bool {
	visited[id] = true
	recStack[id] = true

	q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
	for r, err := range t.r.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		if !visited[r.To] {
			if t.hasCycle(ctx, r.To, visited, recStack) {
				return true
			}
		} else if recStack[r.To] {
			return true
		}
	}

	recStack[id] = false
	return false
}
