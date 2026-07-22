package cli

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/renametype"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// readServices is the read-only capability bundle CLI commands bind to.
// Plain fields, no methods — the same shape as lua.ReadDeps. A command
// that only queries the graph takes *readServices in its Run signature;
// kong injects the bound instance.
type readServices struct {
	Store store.Store
	// Versions is the content-versioning service (history, restore, purge), a
	// pgstore-only injected concern — nil on the fs/mem builds. History/purge
	// commands bind the narrow sub-interface they need and print an "unsupported
	// backend" message when it is nil, instead of type-asserting the store.
	Versions  store.VersionService
	Meta      *metamodel.Metamodel
	Paths     *project.Context
	Tracer    tracer.Tracer
	Searcher  search.Searcher
	Config    config.Loader
	Templater templating.Templater
	FS        storage.FS
}

// writeServices is the read-write capability bundle. It embeds
// readServices (a write command almost always reads too), mirroring
// lua.WriteDeps embedding lua.ReadDeps.
//
// Audit is here for the version-purge commands (TKT-BW6UUL): purge is a
// store-level destructive op with no entity write, so it does NOT route
// through entitymanager.Manager (the write-path audit hook) — it emits
// the OpPurgeVersion record directly through the same sink the Manager
// writes to.
type writeServices struct {
	readServices
	EntityManager entitymanager.EntityManager
	Validator     validator.Validator
	Audit         audit.Audit
	LuaCache      *lua.Cache
	LuaWriteDeps  lua.WriteDeps
	State         state.KV
}

// cliBundles is everything the kong wiring binds for command Run methods:
// the two capability bundles plus the focused services the CLI owns
// directly (attachment / renametype / analysis). Commands never see this
// type — each Run binds only the pieces it uses.
type cliBundles struct {
	read       *readServices
	write      *writeServices
	attachment *attachment.Service
	renametype *renametype.Service
	analysis   *analysis.Service
}

// newCLIBundles wires the focused services around an already-constructed
// appbuild.Services. Used by the kong wiring in production and by CLI
// test fixtures.
func newCLIBundles(svc *appbuild.Services) (*cliBundles, error) {
	att, err := attachment.New(attachment.Deps{
		Store:         svc.Store(),
		Meta:          svc.Meta(),
		EntityManager: svc.EntityManager(),
		// Native MIME allowlist on the CLI attach path too (runner nil →
		// no external scan/transform until the cmd: harness is wired).
		Processor: attachment.NewPolicyProcessor(svc.Meta(), nil),
	})
	if err != nil {
		return nil, fmt.Errorf("attachment service: %w", err)
	}
	rt, err := renametype.New(renametype.Deps{
		FS:    svc.FS(),
		Meta:  svc.Meta(),
		Paths: svc.Paths(),
	})
	if err != nil {
		return nil, fmt.Errorf("renametype service: %w", err)
	}
	an, err := analysis.New(analysis.Deps{
		Store:       svc.Store(),
		Meta:        svc.Meta(),
		Tracer:      svc.Tracer(),
		LuaReadDeps: svc.LuaReadDeps(),
		LuaCache:    svc.ScriptEngine().LuaCache(),
		FS:          svc.FS(),
		Paths:       svc.Paths(),
	})
	if err != nil {
		return nil, fmt.Errorf("analysis service: %w", err)
	}
	read := readServices{
		Store:     svc.Store(),
		Versions:  svc.Versions(),
		Meta:      svc.Meta(),
		Paths:     svc.Paths(),
		Tracer:    svc.Tracer(),
		Searcher:  svc.Searcher(),
		Config:    svc.Config(),
		Templater: svc.Templater(),
		FS:        svc.FS(),
	}
	write := writeServices{
		readServices:  read,
		EntityManager: svc.EntityManager(),
		Validator:     svc.Validator(),
		Audit:         svc.Audit(),
		LuaCache:      svc.ScriptEngine().LuaCache(),
		LuaWriteDeps:  svc.LuaWriteDeps(),
		State:         svc.State(),
	}
	return &cliBundles{
		read:       &read,
		write:      &write,
		attachment: att,
		renametype: rt,
		analysis:   an,
	}, nil
}
