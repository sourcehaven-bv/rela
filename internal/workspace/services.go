package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetrace"
)

// graphTracer adapts *graph.Graph to the storetrace.Tracer interface.
type graphTracer struct {
	w *Workspace
}

var _ storetrace.Tracer = (*graphTracer)(nil)

func (t *graphTracer) TraceFrom(_ context.Context, id string, maxDepth int) *storetrace.TraceResult {
	return convertTraceResult(t.w.graph().TraceFrom(id, maxDepth))
}

func (t *graphTracer) TraceTo(_ context.Context, id string, maxDepth int) *storetrace.TraceResult {
	return convertTraceResult(t.w.graph().TraceTo(id, maxDepth))
}

func (t *graphTracer) FindPath(_ context.Context, from, to string) []storetrace.PathStep {
	steps := t.w.graph().FindPath(from, to)
	if steps == nil {
		return nil
	}
	out := make([]storetrace.PathStep, len(steps))
	for i, s := range steps {
		out[i] = storetrace.PathStep{ID: s.ID, Type: s.Type, Title: s.Title, Relation: s.Relation}
	}
	return out
}

func (t *graphTracer) FindOrphans(_ context.Context) ([]string, error) {
	orphans := t.w.graph().FindOrphans()
	ids := make([]string, len(orphans))
	for i, e := range orphans {
		ids[i] = e.ID
	}
	return ids, nil
}

func (t *graphTracer) HasCycle(_ context.Context, startID string) bool {
	return t.w.graph().HasCycle(startID)
}

// Tracer returns the graph traversal service.
func (w *Workspace) Tracer() storetrace.Tracer {
	return &graphTracer{w: w}
}

// legacyFormatter adapts the workspace's FormatEntity/FormatRelation methods
// to the store.Formatter interface. Used when no backend-specific formatter
// is wired via WithFormatter.
type legacyFormatter struct {
	w *Workspace
}

var _ store.Formatter = (*legacyFormatter)(nil)

func (f *legacyFormatter) FormatEntity(_ context.Context, id string, dryRun bool) (bool, error) {
	e, ok := f.w.GetEntity(id)
	if !ok {
		return false, nil
	}
	return f.w.FormatEntity(e, dryRun)
}

func (f *legacyFormatter) FormatRelation(_ context.Context, from, relType, to string, dryRun bool) (bool, error) {
	r, ok := f.w.GetRelation(from, relType, to)
	if !ok {
		return false, nil
	}
	return f.w.FormatRelation(r, dryRun)
}

// convertTraceResult converts model.TraceResult → storetrace.TraceResult.
func convertTraceResult(r *TraceResult) *storetrace.TraceResult {
	if r == nil {
		return nil
	}
	out := &storetrace.TraceResult{
		ID:       r.ID,
		Type:     r.Type,
		Title:    r.Title,
		Depth:    r.Depth,
		Relation: r.Relation,
		Incoming: r.Incoming,
	}
	for _, c := range r.Children {
		if child := convertTraceResult(c); child != nil {
			out.Children = append(out.Children, child)
		}
	}
	return out
}
