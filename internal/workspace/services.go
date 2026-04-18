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

// LuaReadDeps materialises the read-only capability bundle required by the
// lua runtime from this workspace's backend services. Consumers pass the
// result to lua.NewReader or script.NewReaderRuntime.
func (w *Workspace) LuaReadDeps() lua.ReadDeps {
	var root string
	if w.paths != nil {
		root = w.paths.Root
	}
	return lua.ReadDeps{
		Store:       w.Store(),
		Tracer:      w.Tracer(),
		Searcher:    w.Searcher(),
		Meta:        w.Meta(),
		ProjectRoot: root,
	}
}

// LuaWriteDeps materialises the read-write capability bundle required by the
// lua runtime from this workspace's backend services. Consumers pass the
// result to lua.NewWriter or script.NewWriterRuntime.
func (w *Workspace) LuaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps:      w.LuaReadDeps(),
		EntityManager: w.EntityManager(),
	}
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
		typeSet := typeSetFromQuery(q)
		emit := makeHitEmitter(typeSet, q.Filters, yield)

		if q.Text == "" {
			s.streamAll(ctx, emit, yield)
			return
		}
		s.streamText(q, emit, yield)
	}
}

func typeSetFromQuery(q search.Query) map[string]bool {
	typeSet := make(map[string]bool, len(q.Types))
	for _, t := range q.Types {
		typeSet[t] = true
	}
	return typeSet
}

func makeHitEmitter(
	typeSet map[string]bool,
	filters []search.PropertyFilter,
	yield func(search.Hit, error) bool,
) func(*entity.Entity) bool {
	return func(e *entity.Entity) bool {
		if len(typeSet) > 0 && !typeSet[e.Type] {
			return true
		}
		if !search.MatchFilters(e, filters) {
			return true
		}
		return yield(search.Hit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil)
	}
}

func (s *wsSearcher) streamAll(
	ctx context.Context,
	emit func(*entity.Entity) bool,
	yield func(search.Hit, error) bool,
) {
	for e, err := range s.w.Store().ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		if !emit(e) {
			return
		}
	}
}

func (s *wsSearcher) streamText(
	q search.Query,
	emit func(*entity.Entity) bool,
	yield func(search.Hit, error) bool,
) {
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
// metamodel, using read-only lua deps to execute Lua validation rules.
func (w *Workspace) Validator() validator.Validator {
	return validator.New(w.Store(), w.Meta(), w.LuaReadDeps())
}

// Templater returns the entity-and-relation template service.
func (w *Workspace) Templater() templating.Templater {
	return templating.NewFSTemplater(w.FS(), w.Paths())
}
