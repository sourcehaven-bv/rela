package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
	cfg := &testConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	if meta == nil {
		panic("appbuild.NewForTest: meta is required")
	}

	var searchBackend *bleveindex.Index
	if idx, err := bleveindex.NewMem(); err == nil {
		searchBackend = idx
	} else {
		slog.Warn("appbuild.NewForTest: failed to create search index", "error", err)
	}

	st := cfg.store
	if st == nil {
		if searchBackend != nil {
			st = memstore.New(memstore.WithObserver(searchBackend))
		} else {
			st = memstore.New()
		}
	}

	tr := tracer.New(st)
	var searcher search.Searcher
	if searchBackend != nil {
		searcher = search.New(st, searchBackend)
	} else {
		searcher = search.ErrSearcher(errors.New("search index not available"))
	}

	root := ""
	if cfg.paths != nil {
		root = cfg.paths.Root
	}
	readDeps := lua.ReadDeps{
		Store:       st,
		Tracer:      tr,
		Searcher:    searcher,
		Meta:        meta,
		ProjectRoot: root,
	}

	scriptEngine := script.NewEngine()

	var autoEngine *automation.Engine
	var cascadeRunner *autocascade.Runner
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
		r, rerr := autocascade.New(autocascade.Deps{Engine: autoEngine})
		if rerr != nil {
			panic(fmt.Sprintf("appbuild.NewForTest: build autocascade runner: %v", rerr))
		}
		cascadeRunner = r
	}

	templater := templating.NewFSTemplater(cfg.fs, cfg.paths)
	var cfgLoader config.Loader
	if cfg.fs != nil && cfg.paths != nil {
		cfgLoader = config.NewFSLoader(cfg.fs, cfg.paths.Root)
	}

	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         meta,
		Templater:    templater,
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: script.NewLuaScriptRunner(scriptEngine, readDeps),
	})
	if err != nil {
		panic(fmt.Sprintf("appbuild.NewForTest: build entitymanager: %v", err))
	}

	val := validator.New(st, meta, readDeps)

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
		validator:     val,
		templater:     templater,
		cfgLoader:     cfgLoader,
		stateKV:       nopKV{},
		scriptEngine:  scriptEngine,
		searchBackend: searchBackend,
	}
}
