package workspace

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
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
		Searcher:    w.Searcher(),
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

// Tracer returns the store-backed graph traversal service.
func (w *Workspace) Tracer() tracer.Tracer {
	return tracer.New(w.Store())
}

// wsSearcher adapts the workspace's Bleve-backed Search to search.Searcher.
type wsSearcher struct {
	w *Workspace
}

var _ search.Searcher = (*wsSearcher)(nil)

func (s *wsSearcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		typeSet := make(map[string]bool, len(q.Types))
		for _, t := range q.Types {
			typeSet[t] = true
		}

		emit := func(e *entity.Entity) bool {
			if len(typeSet) > 0 && !typeSet[e.Type] {
				return true
			}
			if !search.MatchFilters(e, q.Filters) {
				return true
			}
			return yield(search.Hit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil)
		}

		if q.Text == "" {
			// No text: iterate all via the store, applying filters.
			for e, err := range s.w.Store().ListEntities(ctx, store.EntityQuery{}) {
				if err != nil {
					yield(search.Hit{}, err)
					return
				}
				if !emit(e) {
					return
				}
			}
			return
		}

		words, phrases := searchparser.SplitFreeText(q.Text)
		entities, _, err := s.w.search(words, phrases, q.Limit)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		emitted := 0
		for _, e := range entities {
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}
			if !emit(e) {
				return
			}
			emitted++
		}
	}
}

// Searcher returns a search.Searcher backed by the workspace's search index.
func (w *Workspace) Searcher() search.Searcher {
	return &wsSearcher{w: w}
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

