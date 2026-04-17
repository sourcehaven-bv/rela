package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/validator"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// LuaServices builds a lua.Services struct wired to this workspace's
// backend services. Consumers use it to run Lua scripts via lua.New.
func (w *Workspace) LuaServices() lua.Services {
	var root string
	if w.repo != nil {
		root = w.Paths().Root
	}
	return lua.Services{
		Store:       w.Store(),
		Manager:     w.EntityManager(),
		Tracer:      w.Tracer(),
		Meta:        w.Meta(),
		ProjectRoot: root,
		Sync: func() error {
			_, err := w.Sync()
			return err
		},
	}
}

// luaServices is the internal alias for LuaServices, used by scriptContextImpl.
func (w *Workspace) luaServices() lua.Services {
	return w.LuaServices()
}

// graphTracer adapts *graph.Graph to the tracer.Tracer interface.
type graphTracer struct {
	w *Workspace
}

var _ tracer.Tracer = (*graphTracer)(nil)

func (t *graphTracer) TraceFrom(_ context.Context, id string, maxDepth int) *tracer.TraceResult {
	return convertTraceResult(t.w.Graph().TraceFrom(id, maxDepth))
}

func (t *graphTracer) TraceTo(_ context.Context, id string, maxDepth int) *tracer.TraceResult {
	return convertTraceResult(t.w.Graph().TraceTo(id, maxDepth))
}

func (t *graphTracer) FindPath(_ context.Context, from, to string) []tracer.PathStep {
	steps := t.w.Graph().FindPath(from, to)
	if steps == nil {
		return nil
	}
	out := make([]tracer.PathStep, len(steps))
	for i, s := range steps {
		out[i] = tracer.PathStep{ID: s.ID, Type: s.Type, Title: s.Title, Relation: s.Relation}
	}
	return out
}

func (t *graphTracer) FindOrphans(_ context.Context) ([]string, error) {
	orphans := t.w.Graph().FindOrphans()
	ids := make([]string, len(orphans))
	for i, e := range orphans {
		ids[i] = e.ID
	}
	return ids, nil
}

func (t *graphTracer) HasCycle(_ context.Context, startID string) bool {
	return t.w.Graph().HasCycle(startID)
}

// Tracer returns the graph traversal service.
func (w *Workspace) Tracer() tracer.Tracer {
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
	return f.w.FormatEntity(model.EntityFromDomain(e), dryRun)
}

func (f *legacyFormatter) FormatRelation(_ context.Context, from, relType, to string, dryRun bool) (bool, error) {
	r, ok := f.w.GetRelation(from, relType, to)
	if !ok {
		return false, nil
	}
	return f.w.FormatRelation(model.RelationFromDomain(r), dryRun)
}

// Validator returns a Validator service backed by the workspace's store and
// metamodel. The service uses workspace as the Lua execution context.
func (w *Workspace) Validator() validator.Validator {
	var root string
	if w.repo != nil {
		root = w.repo.Paths().Root
	}
	return validator.New(w.Store(), w.Meta(), w.luaServices(), root)
}

// Templater returns the entity template service backed by the workspace's
// repository.
func (w *Workspace) Templater() templating.Templater {
	return &repoTemplater{w: w}
}

// repoTemplater adapts the legacy repo.DiscoverEntityTemplates to the
// templating.Templater interface.
type repoTemplater struct {
	w *Workspace
}

var _ templating.Templater = (*repoTemplater)(nil)

func (t *repoTemplater) EntityTemplates(_ context.Context, entityType string) ([]*templating.Template, error) {
	if t.w.repo == nil {
		return nil, nil
	}
	models, err := t.w.repo.DiscoverEntityTemplates(entityType)
	if err != nil {
		return nil, err
	}
	out := make([]*templating.Template, 0, len(models))
	for _, m := range models {
		out = append(out, modelTemplateToService(m))
	}
	return out, nil
}

func modelTemplateToService(m *model.EntityTemplate) *templating.Template {
	if m == nil {
		return nil
	}
	rels := make([]templating.Relation, len(m.Relations))
	for i, r := range m.Relations {
		rels[i] = templating.Relation{Type: r.Relation, Target: r.Target}
	}
	return &templating.Template{
		Name:       m.Name,
		EntityType: m.EntityType,
		Properties: m.Properties,
		Content:    m.Content,
		Relations:  rels,
	}
}

func (t *repoTemplater) EntityTemplate(ctx context.Context, entityType, variant string) (*templating.Template, error) {
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

// convertTraceResult converts model.TraceResult → tracer.TraceResult.
func convertTraceResult(r *TraceResult) *tracer.TraceResult {
	if r == nil {
		return nil
	}
	out := &tracer.TraceResult{
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
