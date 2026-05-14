package workspace

import (
	"context"
	"errors"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// LuaReadDeps materializes the read-only capability bundle required by the
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

// LuaWriteDeps materializes the read-write capability bundle required by the
// lua runtime from this workspace's backend services. Consumers pass the
// result to lua.NewWriter or script.NewWriterRuntime.
func (w *Workspace) LuaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps:      w.LuaReadDeps(),
		EntityManager: w.EntityManager(),
	}
}

// LuaCache returns the process-wide Lua cache shared by every runtime
// built from this workspace's script executor. Callers wrap the result
// in lua.WithCache when building runtimes directly (validation rules,
// lua_eval in MCP, flow commands, etc.). Returns nil when the workspace
// was constructed with a no-op script executor (tests without Lua).
func (w *Workspace) LuaCache() *lua.Cache {
	if w.scriptExec == nil {
		return nil
	}
	return w.scriptExec.LuaCache()
}

// Tracer returns the store-backed graph traversal service. The wrapper is
// created on first access and reused for the lifetime of the workspace.
func (w *Workspace) Tracer() tracer.Tracer {
	w.tracerOnce.Do(func() {
		w.tracer = tracer.New(w.Store())
	})
	return w.tracer
}

// errSearcher is the Searcher returned when the workspace failed to
// construct a search backend at startup. Every call yields a single
// error so callers see an explicit failure rather than silently empty
// results.
type errSearcher struct{ err error }

var _ search.Searcher = errSearcher{}

func (s errSearcher) Search(_ context.Context, _ search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		yield(search.Hit{}, s.err)
	}
}

// Searcher returns a search.Searcher backed by the workspace's search index.
// The wrapper is created on first access and reused for the lifetime of the
// workspace.
func (w *Workspace) Searcher() search.Searcher {
	w.searcherOnce.Do(func() {
		if w.searchBackend == nil {
			w.searcher = errSearcher{err: errors.New("search index not available")}
			return
		}
		w.searcher = search.New(w.store, w.searchBackend)
	})
	return w.searcher
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
