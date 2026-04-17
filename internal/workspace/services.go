package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetemplate"
	"github.com/Sourcehaven-BV/rela/internal/store/storetrace"
	"github.com/Sourcehaven-BV/rela/internal/store/storevalidate"
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

// Validator returns a Validator service backed by the workspace's store and
// metamodel. The service uses workspace as the Lua execution context.
func (w *Workspace) Validator() storevalidate.Validator {
	return storevalidate.New(w.Store(), w.meta())
}

// Templater returns the entity template service backed by the workspace's
// repository.
func (w *Workspace) Templater() storetemplate.Templater {
	return &repoTemplater{w: w}
}

// repoTemplater adapts the legacy repo.DiscoverEntityTemplates to the
// storetemplate.Templater interface.
type repoTemplater struct {
	w *Workspace
}

var _ storetemplate.Templater = (*repoTemplater)(nil)

func (t *repoTemplater) EntityTemplates(_ context.Context, entityType string) ([]*storetemplate.Template, error) {
	if t.w.repo == nil {
		return nil, nil
	}
	models, err := t.w.repo.DiscoverEntityTemplates(entityType)
	if err != nil {
		return nil, err
	}
	out := make([]*storetemplate.Template, 0, len(models))
	for _, m := range models {
		out = append(out, modelTemplateToService(m))
	}
	return out, nil
}

func modelTemplateToService(m *model.EntityTemplate) *storetemplate.Template {
	if m == nil {
		return nil
	}
	rels := make([]storetemplate.Relation, len(m.Relations))
	for i, r := range m.Relations {
		rels[i] = storetemplate.Relation{Type: r.Relation, Target: r.Target}
	}
	return &storetemplate.Template{
		Name:       m.Name,
		EntityType: m.EntityType,
		Properties: m.Properties,
		Content:    m.Content,
		Relations:  rels,
	}
}

func (t *repoTemplater) EntityTemplate(ctx context.Context, entityType, variant string) (*storetemplate.Template, error) {
	all, err := t.EntityTemplates(ctx, entityType)
	if err != nil {
		return nil, err
	}
	for _, tpl := range all {
		if tpl.Name == variant {
			return tpl, nil
		}
	}
	return nil, nil
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
