package workspace

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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
	if w.paths != nil {
		root = w.paths.Root
	}
	return lua.Services{
		Store:       w.Store(),
		Manager:     w.EntityManager(),
		Tracer:      w.Tracer(),
		Searcher:    w.Searcher(),
		Meta:        w.Meta(),
		ProjectRoot: root,
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

// MetaLoader returns the metamodel loader for this workspace. Callers
// invoke Load(ctx) to read a fresh metamodel from the configured source.
func (w *Workspace) MetaLoader() metamodel.Loader {
	return metamodel.NewFSLoader(w.FS(), w.Paths().MetamodelPath)
}

// Validator returns a Validator service backed by the workspace's store and
// metamodel. The service uses workspace as the Lua execution context.
func (w *Workspace) Validator() validator.Validator {
	var root string
	if w.paths != nil {
		root = w.paths.Root
	}
	return validator.New(w.Store(), w.Meta(), w.luaServices(), root)
}

// Templater returns the entity-and-relation template service.
func (w *Workspace) Templater() templating.Templater {
	return templating.NewFSTemplater(w.FS(), w.Paths())
}


