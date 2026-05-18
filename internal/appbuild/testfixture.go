package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// TestOption configures a [Services] built via [NewForTest].
type TestOption func(*testConfig)

type testConfig struct {
	fs    storage.FS
	paths *project.Context
	store store.Store
	audit audit.Audit
}

// WithTestStore replaces the default empty memstore with a
// caller-supplied store. The fixture's search index is populated from
// the store's current contents at construction time.
//
// Caveat: a caller-supplied store is NOT auto-wired with the search
// backend as an observer — observer setup must happen at store
// construction, which the fixture cannot retrofit. Initial-state
// backfill still runs, so any entities already in the store appear in
// search results; subsequent writes will not reach the index. If you
// need incremental sync, build the memstore with [memstore.WithObserver]
// yourself and pass that store here, or use the default memstore
// (omit WithTestStore) which wires the observer automatically.
func WithTestStore(s store.Store) TestOption {
	return func(c *testConfig) { c.store = s }
}

// WithFS attaches a filesystem and project paths to the test fixture,
// enabling paths-aware behavior (Paths(), Config(), templating against
// project files). Without this, those accessors return zero values.
func WithFS(fs storage.FS, paths *project.Context) TestOption {
	return func(c *testConfig) {
		c.fs = fs
		c.paths = paths
	}
}

// WithTestAudit replaces the default [audit.Nop] sink with a
// caller-supplied audit backend. Tests that assert on audit records
// pass [audit.NewMemory]; tests that don't care can omit this option
// and rely on the default Nop.
func WithTestAudit(a audit.Audit) TestOption {
	return func(c *testConfig) { c.audit = a }
}

// NewForTest constructs a *Services bundle suitable for tests. By
// default the fixture has no filesystem, an empty memstore, and a real
// script engine (cheap to construct; only exercised when automations
// fire, which require WithFS-backed automation config). Use
// [WithFS] / [WithTestStore] to customize.
//
// NewForTest takes *Metamodel directly and bypasses the loader, so test
// metamodels that use pre-migration syntax work without running
// migrations first. Mirrors workspace.NewForTest semantics for callers
// migrating off the legacy fixture.
//
// Panics on construction failure: tests have no recovery path, and a
// loud panic surfaces fixture-setup bugs at their source.
func NewForTest(meta *metamodel.Metamodel, opts ...TestOption) *Services {
	if meta == nil {
		panic("appbuild.NewForTest: meta is required")
	}
	cfg := &testConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	searchBackend := newTestSearchBackend()
	st := resolveTestStore(cfg.store, searchBackend)
	tr := tracer.New(st)
	searcher := resolveTestSearcher(st, searchBackend)
	readDeps := buildTestReadDeps(st, tr, searcher, meta, cfg.paths)

	autoEngine, cascadeRunner := buildTestAutomation(meta)
	templater := templating.NewFSTemplater(cfg.fs, cfg.paths)
	cfgLoader := buildTestConfigLoader(cfg.fs, cfg.paths)
	stateKV := mustBuildTestStateKV(cfg.fs, cfg.paths)
	scriptEngine := script.NewEngine()
	auditSink := cfg.audit
	if auditSink == nil {
		auditSink = audit.Nop{}
	}

	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         meta,
		Templater:    templater,
		Audit:        auditSink,
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: script.NewLuaScriptRunner(scriptEngine, readDeps),
	})
	if err != nil {
		panic(fmt.Sprintf("appbuild.NewForTest: build entitymanager: %v", err))
	}

	// Backfill the search index when a caller-supplied store is used:
	// observers are NOT invoked for entities already present at
	// construction time. (The default memstore wires the observer at
	// build time so backfill is unnecessary there.)
	if cfg.store != nil && searchBackend != nil {
		if err := backfillSearchBackend(context.Background(), searchBackend, cfg.store); err != nil {
			panic(fmt.Sprintf("appbuild.NewForTest: index entities: %v", err))
		}
	}

	return &Services{
		fs:            cfg.fs,
		paths:         cfg.paths,
		meta:          meta,
		store:         st,
		searcher:      searcher,
		entityManager: mgr,
		tracer:        tr,
		validator:     validator.New(st, meta, readDeps),
		templater:     templater,
		cfgLoader:     cfgLoader,
		stateKV:       stateKV,
		scriptEngine:  scriptEngine,
		searchBackend: searchBackend,
	}
}

func newTestSearchBackend() *bleveindex.Index {
	idx, err := bleveindex.NewMem()
	if err != nil {
		slog.Warn("appbuild.NewForTest: failed to create search index", "error", err)
		return nil
	}
	return idx
}

func resolveTestStore(custom store.Store, backend *bleveindex.Index) store.Store {
	if custom != nil {
		return custom
	}
	if backend != nil {
		return memstore.New(memstore.WithObserver(backend))
	}
	return memstore.New()
}

func resolveTestSearcher(st store.Store, backend *bleveindex.Index) search.Searcher {
	if backend != nil {
		return search.New(st, backend)
	}
	return search.ErrSearcher(errors.New("search index not available"))
}

func buildTestReadDeps(st store.Store, tr tracer.Tracer, searcher search.Searcher,
	meta *metamodel.Metamodel, paths *project.Context) lua.ReadDeps {
	root := ""
	if paths != nil {
		root = paths.Root
	}
	return lua.ReadDeps{
		Store:       st,
		Tracer:      tr,
		Searcher:    searcher,
		Meta:        meta,
		ProjectRoot: root,
	}
}

func buildTestAutomation(meta *metamodel.Metamodel) (*automation.Engine, *autocascade.Runner) {
	if len(meta.Automations) == 0 {
		return nil, nil
	}
	autoEngine := automation.NewEngineFromMetamodel(meta.Automations)
	r, err := autocascade.New(autocascade.Deps{Engine: autoEngine})
	if err != nil {
		panic(fmt.Sprintf("appbuild.NewForTest: build autocascade runner: %v", err))
	}
	return autoEngine, r
}

func buildTestConfigLoader(fs storage.FS, paths *project.Context) config.Loader {
	if fs == nil || paths == nil {
		return nil
	}
	return config.NewFSLoader(fs, paths.Root)
}

// mustBuildTestStateKV mirrors workspace.NewForTest semantics so
// dataentry tests that exercise per-user state (UIState, UserDefaults)
// work. Panics on invalid cache root — fixture setup bug.
func mustBuildTestStateKV(fs storage.FS, paths *project.Context) state.KV {
	kv, err := buildStateKV(fs, paths)
	if err != nil {
		panic(fmt.Sprintf("appbuild.NewForTest: build state KV: %v", err))
	}
	return kv
}
