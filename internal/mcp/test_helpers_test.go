package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// testServices is the in-test Services bundle used by MCP tool tests.
// Mirrors the production cli.mcpServices wiring but without project
// discovery, fsstore, or the Lua engine — the surface MCP tools
// actually exercise.
type testServices struct {
	store    store.Store
	meta     *metamodel.Metamodel
	tracer   tracer.Tracer
	searcher search.Searcher
	valid    validator.Validator
	manager  *entitymanager.Manager
	watcher  Watcher
}

var _ Services = (*testServices)(nil)

func (s *testServices) Store() store.Store                         { return s.store }
func (s *testServices) Meta() *metamodel.Metamodel                 { return s.meta }
func (s *testServices) Tracer() tracer.Tracer                      { return s.tracer }
func (s *testServices) Searcher() search.Searcher                  { return s.searcher }
func (s *testServices) Validator() validator.Validator             { return s.valid }
func (s *testServices) EntityManager() entitymanager.EntityManager { return s.manager }
func (s *testServices) Config() config.Loader                      { return nopConfigLoader{} }
func (s *testServices) Paths() *project.Context                    { return nil }
func (s *testServices) LuaCache() *lua.Cache                       { return nil }
func (s *testServices) Watcher() Watcher                           { return nopWatcher{} }

func (s *testServices) LuaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			Store:    s.store,
			Tracer:   s.tracer,
			Searcher: s.searcher,
			Meta:     s.meta,
		},
		EntityManager: s.manager,
	}
}

// newTestServices wires the focused services around a caller-supplied
// store. The store is hooked to a fresh bleve search backend via
// observer wiring so writes through the store reach the index
// synchronously.
//
// Callers that want to seed entities before the bleve observer is
// installed should call backfill manually after seeding.
func newTestServices(t *testing.T, meta *metamodel.Metamodel, st store.Store) *testServices {
	t.Helper()

	backend, err := bleveindex.NewMem()
	if err != nil {
		t.Fatalf("bleveindex.NewMem: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	// Backfill any entities already in the store (the test fixture
	// seeds before constructing services).
	ctx := context.Background()
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			continue
		}
		_ = backend.EntityPut(e)
	}

	tr := tracer.New(st)
	srch := search.New(st, backend)
	val := validator.New(st, meta, lua.ReadDeps{
		Store:    st,
		Tracer:   tr,
		Searcher: srch,
		Meta:     meta,
	})

	// Wire autocascade if the metamodel declares automations; mirrors
	// the production wiring in cli.newMCPServices so tests that add
	// automations to their metamodel still exercise the full cascade
	// pipeline.
	var autoEngine *automation.Engine
	var cascadeRunner *autocascade.Runner
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
		r, rerr := autocascade.New(autocascade.Deps{Engine: autoEngine})
		if rerr != nil {
			t.Fatalf("autocascade.New: %v", rerr)
		}
		cascadeRunner = r
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   templating.NewFSTemplater(nil, nil),
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Automations: autoEngine,
		Cascade:     cascadeRunner,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	return &testServices{
		store:    st,
		meta:     meta,
		tracer:   tr,
		searcher: srch,
		valid:    val,
		manager:  mgr,
		watcher:  nopWatcher{},
	}
}

// --- stub helpers ---

type nopWatcher struct{}

func (nopWatcher) Start(func()) error { return nil }
func (nopWatcher) Stop()              {}
func (nopWatcher) Pause()             {}
func (nopWatcher) Resume()            {}

type nopConfigLoader struct{}

func (nopConfigLoader) Load(context.Context, string) ([]byte, error) {
	return nil, errors.New("test: no config loader")
}
