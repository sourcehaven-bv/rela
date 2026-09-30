package cli

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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
	// World is the world `list` reads in when the user names none, from
	// the compiled worlds' default-world seam.
	World store.WorldScope
	// Families selects one row per entity whichever face it stores, for
	// per-type counts (worlds.Compiled.Families). Never a read world.
	Families store.WorldScope
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
	EntityManager entityWriter

	// Recreator brings a deleted face back at its own id on `rela restore`,
	// create-only: it never falls through to a whole-record update when the
	// face was recreated in the meantime.
	Recreator    entityRecreator
	Validator    validator.Validator
	Audit        audit.Audit
	LuaCache     *lua.Cache
	LuaWriteDeps lua.WriteDeps
	State        state.KV
	// MigState is the per-store migration record. Distinct from State: that
	// is node-local cache, this describes what the CONTENT conforms to.
	MigState datamigration.StateStore
}

// entityWriter is the write surface the CLI's mutating subcommands call. See
// the internal/entitymanager package doc for why each consumer declares its own
// interface rather than sharing one (TKT-IVSJV6).
//
// Eight of the manager's nine write methods. ValidateCreate is absent because
// no subcommand dry-runs a create — the CLI either writes or it doesn't, and
// the advisory path exists for the data-entry form.
type entityWriter interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	DeleteEntityFace(ctx context.Context, id string, face entity.Face, cascade bool) (*entity.DeleteResult, error)
	RenameEntity(
		ctx context.Context, oldID, newID string, opts entity.RenameOptions,
	) (*entity.RenameResult, error)
	CreateRelation(
		ctx context.Context, from, relType, to string, opts entity.RelationOptions,
	) (*entity.Relation, error)
	UpdateRelation(
		ctx context.Context, from, relType, to string, opts entity.RelationOptions,
	) (*entity.Relation, error)
	DeleteRelationState(ctx context.Context, from string, face entity.Face, relType, to string) error
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
	owner, err := entitymanager.AttachmentsOf(svc.EntityManager())
	if err != nil { // coverage-ignore: defensive: appbuild always wires the manager's attachment lock
		return nil, fmt.Errorf("attachment service: %w", err)
	}
	att, err := attachment.New(attachment.Deps{
		Store:         svc.Store(),
		Meta:          svc.Meta(),
		EntityManager: owner,
		Locker:        owner,
		Authorizer:    attachment.AllowAllWrites{}, // operator shell: no ACL
		// Native MIME allowlist on the CLI attach path too (runner nil →
		// no external scan/transform until the cmd: harness is wired).
		Processor: attachment.NewPolicyProcessor(svc.Meta(), nil),
	})
	if err != nil { // coverage-ignore: defensive: attachment.New only fails on nil deps; newCLIBundles always passes a
		// valid appbuild.Services
		return nil, fmt.Errorf("attachment service: %w", err)
	}
	rt, err := renametype.New(renametype.Deps{
		FS:    svc.FS(),
		Meta:  svc.Meta(),
		Paths: svc.Paths(),
	})
	if err != nil { // coverage-ignore: defensive: renametype.New only fails on nil deps; newCLIBundles always passes a
		// valid appbuild.Services
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
	if err != nil { // coverage-ignore: defensive: analysis.New only fails on nil deps; newCLIBundles always passes a
		// valid appbuild.Services
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
		World:     appbuild.CompiledWorlds(svc).Default(),
		Families:  appbuild.CompiledWorlds(svc).Families(),
	}
	write := writeServices{
		readServices:  read,
		EntityManager: svc.EntityManager(),
		Recreator:     entitymanager.Recreator{M: svc.EntityManager()},
		Validator:     svc.Validator(),
		Audit:         svc.Audit(),
		LuaCache:      svc.ScriptEngine().LuaCache(),
		LuaWriteDeps:  svc.LuaWriteDeps(),
		State:         svc.State(),
		MigState:      svc.MigState(),
	}
	return &cliBundles{
		read:       &read,
		write:      &write,
		attachment: att,
		renametype: rt,
		analysis:   an,
	}, nil
}
