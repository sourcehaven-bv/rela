// Package appbuildtest provides a test fixture for assembling an
// [appbuild.Services] bundle. It lives in its own package so that
// the bleve dependency it requires (in-memory search index for tests)
// is not compiled into the production [appbuild] package — keeping
// the bleve import out of the production binary when an alternative
// search backend is built.
//
// Mirrors the [internal/store/storetest] pattern: a sibling package
// next to the production code that supplies test-only constructors
// without polluting the production package's import graph.
package appbuildtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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

// Option configures a [*appbuild.Services] built via [New].
type Option func(*testConfig)

type testConfig struct {
	fs    storage.FS
	paths *project.Context
	store store.Store
	audit audit.Audit
	acl   acl.ACL
}

// WithStore replaces the default empty memstore with a caller-supplied
// store. The fixture's search index is populated from the store's
// current contents at construction time.
//
// Caveat: a caller-supplied store is NOT auto-wired with the search
// backend as an observer — observer setup must happen at store
// construction, which the fixture cannot retrofit. Initial-state
// backfill still runs, so any entities already in the store appear in
// search results; subsequent writes will not reach the index. If you
// need incremental sync, build the memstore with [memstore.WithObserver]
// yourself and pass that store here, or use the default memstore
// (omit WithStore) which wires the observer automatically.
func WithStore(s store.Store) Option {
	return func(c *testConfig) { c.store = s }
}

// WithFS attaches a filesystem and project paths to the test fixture,
// enabling paths-aware behavior (Paths(), Config(), templating against
// project files). Without this, those accessors return zero values.
func WithFS(fs storage.FS, paths *project.Context) Option {
	return func(c *testConfig) {
		c.fs = fs
		c.paths = paths
	}
}

// WithAudit replaces the default [audit.Nop] sink with a
// caller-supplied audit backend. Tests that assert on audit records
// pass [audit.NewMemory]; tests that don't care can omit this option
// and rely on the default Nop.
func WithAudit(a audit.Audit) Option {
	return func(c *testConfig) { c.audit = a }
}

// WithACL replaces the default [acl.NopACL] with a caller-supplied
// ACL backend. Tests that assert on the deny path pass
// [acl.ReadOnlyACL]; tests that don't care can omit this option and
// rely on the allow-all default.
func WithACL(a acl.ACL) Option {
	return func(c *testConfig) { c.acl = a }
}

// New constructs a *appbuild.Services bundle suitable for tests. By
// default the fixture has no filesystem, an empty memstore, and a real
// script engine (cheap to construct; only exercised when automations
// fire, which require WithFS-backed automation config). Use
// [WithFS] / [WithStore] to customize.
//
// New takes *Metamodel directly and bypasses the loader, so test
// metamodels that use pre-migration syntax work without running
// migrations first.
//
// Panics on construction failure: tests have no recovery path, and a
// loud panic surfaces fixture-setup bugs at their source.
func New(meta *metamodel.Metamodel, opts ...Option) *appbuild.Services {
	if meta == nil {
		panic("appbuildtest.New: meta is required")
	}
	cfg := &testConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	searchBackend := newSearchBackend()
	st := resolveStore(cfg.store, searchBackend)
	tr := tracer.New(st)
	searcher := resolveSearcher(st, searchBackend)
	readDeps := buildReadDeps(st, tr, searcher, meta, cfg.paths)

	autoEngine, cascadeRunner := buildAutomation(meta)
	templater := templating.NewFSTemplater(cfg.fs, cfg.paths)
	cfgLoader := buildConfigLoader(cfg.fs, cfg.paths)
	stateKV := mustBuildStateKV(cfg.fs, cfg.paths)
	scriptEngine := script.NewEngine()
	auditSink := cfg.audit
	if auditSink == nil {
		auditSink = audit.Nop{}
	}
	aclImpl := cfg.acl
	if aclImpl == nil {
		aclImpl = acl.NopACL{}
	}

	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         meta,
		Templater:    templater,
		Audit:        auditSink,
		ACL:          aclImpl,
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: script.NewLuaScriptRunner(scriptEngine, readDeps),
	})
	if err != nil {
		panic(fmt.Sprintf("appbuildtest.New: build entitymanager: %v", err))
	}

	// Backfill the search index when a caller-supplied store is used:
	// observers are NOT invoked for entities already present at
	// construction time. (The default memstore wires the observer at
	// build time so backfill is unnecessary there.)
	if cfg.store != nil && searchBackend != nil {
		if backfillErr := backfill(context.Background(), searchBackend, cfg.store); backfillErr != nil {
			panic(fmt.Sprintf("appbuildtest.New: index entities: %v", backfillErr))
		}
	}

	svc, err := appbuild.NewFromCollaborators(appbuild.Collaborators{
		FS:            cfg.fs,
		Paths:         cfg.paths,
		Meta:          meta,
		Store:         st,
		Searcher:      searcher,
		EntityManager: mgr,
		Tracer:        tr,
		Validator:     validator.New(st, meta, readDeps),
		Templater:     templater,
		CfgLoader:     cfgLoader,
		StateKV:       stateKV,
		ScriptEngine:  scriptEngine,
		SearchCloser:  searchCloser(searchBackend),
		ACL:           aclImpl,
	})
	if err != nil {
		panic(fmt.Sprintf("appbuildtest.New: assemble services: %v", err))
	}
	return svc
}

// searchCloser returns the bleve index as an io.Closer, or nil when
// the index is itself nil. Avoids the typed-nil-into-interface trap
// where Services.Close would otherwise invoke Close on a nil
// *bleveindex.Index.
func searchCloser(backend *bleveindex.Index) io.Closer {
	if backend == nil {
		return nil
	}
	return backend
}

// backfill re-indexes every entity currently in the caller-supplied
// store. Lives in the test fixture so the production search seam can
// stay build-tag-specific while tests always use bleve.
func backfill(ctx context.Context, backend *bleveindex.Index, s store.Store) error {
	if backend == nil || s == nil {
		return nil
	}
	entities := make([]*entity.Entity, 0)
	var listErrs []error
	for e, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			listErrs = append(listErrs, err)
			continue
		}
		entities = append(entities, e)
	}
	indexed, indexErr := backend.IndexBatch(entities)
	if len(listErrs) == 0 && indexErr == nil {
		return nil
	}
	skipped := len(entities) - indexed
	return fmt.Errorf("backfill indexed %d entities, skipped %d, list errors: %v, index error: %w",
		indexed, skipped, listErrs, indexErr)
}

func newSearchBackend() *bleveindex.Index {
	idx, err := bleveindex.NewMem()
	if err != nil {
		slog.Warn("appbuildtest.New: failed to create search index", "error", err)
		return nil
	}
	return idx
}

func resolveStore(custom store.Store, backend *bleveindex.Index) store.Store {
	if custom != nil {
		return custom
	}
	if backend != nil {
		return memstore.New(memstore.WithObserver(backend))
	}
	return memstore.New()
}

func resolveSearcher(st store.Store, backend *bleveindex.Index) search.Searcher {
	if backend != nil {
		return search.New(st, backend)
	}
	return search.ErrSearcher(errors.New("search index not available"))
}

func buildReadDeps(st store.Store, tr tracer.Tracer, searcher search.Searcher,
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

func buildAutomation(meta *metamodel.Metamodel) (*automation.Engine, *autocascade.Runner) {
	if len(meta.Automations) == 0 {
		return nil, nil
	}
	autoEngine := automation.NewEngineFromMetamodel(meta.Automations)
	r, err := autocascade.New(autocascade.Deps{Engine: autoEngine})
	if err != nil {
		panic(fmt.Sprintf("appbuildtest.New: build autocascade runner: %v", err))
	}
	return autoEngine, r
}

func buildConfigLoader(fs storage.FS, paths *project.Context) config.Loader {
	if fs == nil || paths == nil {
		return nil
	}
	return config.NewFSLoader(fs, paths.Root)
}

// mustBuildStateKV mirrors the production buildStateKV semantics so
// dataentry tests that exercise per-user state (UIState, UserDefaults)
// work. Panics on invalid cache root — fixture setup bug.
func mustBuildStateKV(fs storage.FS, paths *project.Context) state.KV {
	if fs == nil || paths == nil || paths.CacheDir == "" {
		return nopKV{}
	}
	rfs, err := storage.NewRootedFS(fs, paths.CacheDir)
	if err != nil {
		panic(fmt.Sprintf("appbuildtest.New: build state KV: invalid root %q: %v", paths.CacheDir, err))
	}
	return state.NewFSKV(rfs)
}

// nopKV is the fallback state.KV used when no cache directory is
// available. Mirrors appbuild.nopKV — scheduler-style state callers
// expect missing-key to be silent, not a hard error.
type nopKV struct{}

func (nopKV) Get(context.Context, string) ([]byte, error) { return nil, os.ErrNotExist }
func (nopKV) Put(context.Context, string, []byte) error   { return nil }
func (nopKV) Delete(context.Context, string) error        { return nil }
