package cli

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/renametype"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// Services bundles are scoped by purpose (read / write / analyze) so
// each subcommand declares the narrowest dependency it needs.
// Implementations are constructed once per CLI invocation and
// attached to the cobra command context.
//
// Pattern: subcommands retrieve their bundle from cmd.Context() via
// cliReadFromContext / cliWriteFromContext / cliAnalyzeFromContext.
// Tests use attachServicesForTest to populate the context directly.

// cliRead exposes read-only project services. Used by show / list /
// trace / graph / export / template / fmt and embedded by the
// write/analyze bundles.
type cliRead interface {
	Store() store.Store
	Meta() *metamodel.Metamodel
	Paths() *project.Context
	Tracer() tracer.Tracer
	Searcher() search.Searcher
	Config() config.Loader
	Templater() templating.Templater
	FS() storage.FS
}

// cliWrite is cliRead + the write-path services that mutating
// subcommands need (create / delete / update / link / unlink /
// detach / import / normalize / script / scheduler). State() lives
// here (not on cliRead) because state.KV.Put/Delete mutate
// persistent state — same write-bundle invariant as EntityManager.
type cliWrite interface {
	cliRead
	EntityManager() entitymanager.EntityManager
	Validator() validator.Validator
	LuaCache() *lua.Cache
	LuaWriteDeps() lua.WriteDeps
	State() state.KV
}

// cliAnalyze bundles the read-side analysis surface plus the CLI's
// maintenance facades (rename-entity-type, attach-file). Drift
// warning: after the TKT-2W0X / TKT-04YA / TKT-B01S lifts the bundle
// spans three subsystems (analysis, attachment, renametype). The
// attach/attachments/rename subcommands piggyback this bundle today
// because they ran on workspace originally; a follow-up should split
// attachment + renametype into their own bundles so cliAnalyze stays
// narrow per CLAUDE.md "scoped consumer-side Services interface."
type cliAnalyze interface {
	cliRead
	AnalyzeAll(ctx context.Context, opts analysis.Options) *analysis.Summary
	CheckCardinality(opts analysis.Options) []analysis.CardinalityViolation
	FindDuplicates(opts analysis.Options) []analysis.DuplicateGroup
	FindGaps(opts analysis.Options) []analysis.GapResult
	FindOrphansWithScope(opts analysis.Options) []*entity.Entity
	FindOrphanedTempFiles() ([]string, error)
	CleanupOrphanedTempFiles() (int, error)
	RunValidations(ctx context.Context, opts analysis.Options) analysis.ValidationResult
	RunValidationsFiltered(
		ctx context.Context,
		opts analysis.Options,
		filters []analysis.ValidationFilter,
	) analysis.ValidationResult
	RenameEntityType(oldType, newType, newPlural string) (int, error)
	AttachFile(ctx context.Context, entityID, filePath, property string) (*attachment.Result, error)
	ListAttachments(ctx context.Context, entityID string) ([]attachment.Info, error)
}

// cliServices is the single concrete implementation that satisfies
// all three bundle interfaces. It holds an *appbuild.Services for the
// per-project collaborators plus dedicated services that have been
// lifted out (attachment, renametype, analysis). The interfaces — not
// the struct — are the consumer-facing contracts: subcommands see
// only the bundle they pulled from context.
type cliServices struct {
	svc        *appbuild.Services
	attachment *attachment.Service
	renametype *renametype.Service
	analysis   *analysis.Service
}

// Compile-time assertions: cliServices must satisfy every bundle
// interface. A method-signature drift surfaces at this type rather
// than at every subcommand call site.
var (
	_ cliRead    = (*cliServices)(nil)
	_ cliWrite   = (*cliServices)(nil)
	_ cliAnalyze = (*cliServices)(nil)
)

// --- cliRead ---

func (s *cliServices) Store() store.Store              { return s.svc.Store() }
func (s *cliServices) Meta() *metamodel.Metamodel      { return s.svc.Meta() }
func (s *cliServices) Paths() *project.Context         { return s.svc.Paths() }
func (s *cliServices) Tracer() tracer.Tracer           { return s.svc.Tracer() }
func (s *cliServices) Searcher() search.Searcher       { return s.svc.Searcher() }
func (s *cliServices) Config() config.Loader           { return s.svc.Config() }
func (s *cliServices) Templater() templating.Templater { return s.svc.Templater() }
func (s *cliServices) FS() storage.FS                  { return s.svc.FS() }

// --- cliWrite ---

func (s *cliServices) EntityManager() entitymanager.EntityManager { return s.svc.EntityManager() }
func (s *cliServices) Validator() validator.Validator             { return s.svc.Validator() }
func (s *cliServices) LuaCache() *lua.Cache                       { return s.svc.ScriptEngine().LuaCache() }
func (s *cliServices) LuaWriteDeps() lua.WriteDeps                { return s.svc.LuaWriteDeps() }
func (s *cliServices) State() state.KV                            { return s.svc.State() }

// --- cliAnalyze ---

func (s *cliServices) AnalyzeAll(ctx context.Context, opts analysis.Options) *analysis.Summary {
	return s.analysis.AnalyzeAll(ctx, opts)
}

func (s *cliServices) CheckCardinality(opts analysis.Options) []analysis.CardinalityViolation {
	return s.analysis.CheckCardinality(opts)
}

func (s *cliServices) FindDuplicates(opts analysis.Options) []analysis.DuplicateGroup {
	return s.analysis.FindDuplicates(opts)
}

func (s *cliServices) FindGaps(opts analysis.Options) []analysis.GapResult {
	return s.analysis.FindGaps(opts)
}

func (s *cliServices) FindOrphansWithScope(opts analysis.Options) []*entity.Entity {
	return s.analysis.FindOrphansWithScope(opts)
}

func (s *cliServices) FindOrphanedTempFiles() ([]string, error) {
	return s.analysis.FindOrphanedTempFiles()
}

func (s *cliServices) CleanupOrphanedTempFiles() (int, error) {
	return s.analysis.CleanupOrphanedTempFiles()
}

func (s *cliServices) RunValidations(ctx context.Context, opts analysis.Options) analysis.ValidationResult {
	return s.analysis.RunValidations(ctx, opts)
}

func (s *cliServices) RunValidationsFiltered(
	ctx context.Context,
	opts analysis.Options,
	filters []analysis.ValidationFilter,
) analysis.ValidationResult {
	return s.analysis.RunValidationsFiltered(ctx, opts, filters)
}

func (s *cliServices) RenameEntityType(oldType, newType, newPlural string) (int, error) {
	if s.renametype == nil {
		// Reached only when a test built cliServices from an
		// FS-less appbuild.NewForTest fixture and then drove a
		// rename. Production wiring always populates renametype
		// (appbuild.Discover guarantees FS + Paths). Panic loudly
		// so the test setup gap is unmistakable.
		panic("cli: renametype service not wired — test fixture must use appbuild.WithFS")
	}
	return s.renametype.Rename(oldType, newType, newPlural)
}

func (s *cliServices) AttachFile(ctx context.Context, entityID, filePath, property string) (*attachment.Result, error) {
	return s.attachment.Attach(ctx, entityID, filePath, property)
}

func (s *cliServices) ListAttachments(ctx context.Context, entityID string) ([]attachment.Info, error) {
	return s.attachment.List(ctx, entityID)
}

// --- context plumbing ---

type ctxKey int

const (
	keyRead ctxKey = iota
	keyWrite
	keyAnalyze
)

// attachServices stores the bundle implementations on ctx so
// subcommand RunE handlers can retrieve them via the typed
// accessors.
func attachServices(ctx context.Context, svc *cliServices) context.Context {
	ctx = context.WithValue(ctx, keyRead, cliRead(svc))
	ctx = context.WithValue(ctx, keyWrite, cliWrite(svc))
	ctx = context.WithValue(ctx, keyAnalyze, cliAnalyze(svc))
	return ctx
}

// cliReadFromContext retrieves the read bundle. Panics with a
// targeted message when services were not attached — that almost
// always means the subcommand is missing the
// PersistentPreRunE wiring (annotated with skipProjectDiscovery
// but reaches for a bundle), and a clear panic surfaces the
// configuration error at its source instead of as an opaque nil
// dereference three frames deep.
func cliReadFromContext(ctx context.Context) cliRead {
	v, ok := ctx.Value(keyRead).(cliRead)
	if !ok {
		panic("cli: read services not attached on context — subcommand may be annotated skipProjectDiscovery or invoked without PersistentPreRunE")
	}
	return v
}

// cliWriteFromContext retrieves the write bundle.
func cliWriteFromContext(ctx context.Context) cliWrite {
	v, ok := ctx.Value(keyWrite).(cliWrite)
	if !ok {
		panic("cli: write services not attached on context — subcommand may be annotated skipProjectDiscovery or invoked without PersistentPreRunE")
	}
	return v
}

// cliAnalyzeFromContext retrieves the analyze bundle.
func cliAnalyzeFromContext(ctx context.Context) cliAnalyze {
	v, ok := ctx.Value(keyAnalyze).(cliAnalyze)
	if !ok {
		panic("cli: analyze services not attached on context — subcommand may be annotated skipProjectDiscovery or invoked without PersistentPreRunE")
	}
	return v
}

// newCLIServices discovers the project at startDir and constructs
// the focused-services bundle. Mirrors appbuild.Discover's behavior;
// the returned *cliServices satisfies cliRead / cliWrite / cliAnalyze.
func newCLIServices(startDir string) (*cliServices, error) {
	svc, err := appbuild.Discover(startDir, script.NewEngine())
	if err != nil {
		return nil, err
	}
	return newCLIServicesFromAppbuild(svc)
}

// newCLIServicesFromAppbuild wires the focused services around an
// already-constructed appbuild.Services. Used by [newCLIServices] in
// production and by the CLI test fixtures.
func newCLIServicesFromAppbuild(svc *appbuild.Services) (*cliServices, error) {
	att, err := attachment.New(attachment.Deps{
		Store:         svc.Store(),
		Meta:          svc.Meta(),
		EntityManager: svc.EntityManager(),
	})
	if err != nil {
		return nil, fmt.Errorf("attachment service: %w", err)
	}
	// renametype needs FS + Paths; the FS-less appbuild.NewForTest
	// fixture skips both. Skip wiring renametype when either is
	// absent — handlers panic clearly via the unset-service check
	// rather than nil-deref when an FS-less fixture is given to a
	// rename test by accident.
	var rt *renametype.Service
	if svc.FS() != nil && svc.Paths() != nil {
		rt, err = renametype.New(renametype.Deps{
			FS:    svc.FS(),
			Meta:  svc.Meta(),
			Paths: svc.Paths(),
		})
		if err != nil {
			return nil, fmt.Errorf("renametype service: %w", err)
		}
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
	return &cliServices{svc: svc, attachment: att, renametype: rt, analysis: an}, nil
}
