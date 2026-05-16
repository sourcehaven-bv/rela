package cli

import (
	"context"
	"fmt"

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
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
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
// detach / import / normalize / script).
type cliWrite interface {
	cliRead
	EntityManager() entitymanager.EntityManager
	Validator() validator.Validator
	LuaCache() *lua.Cache
	LuaWriteDeps() lua.WriteDeps
}

// cliAnalyze is cliRead + the CLI-specific facade methods that
// today live on *workspace.Workspace. The arc that started with
// TKT-2W0X (attachment) lifts these methods into dedicated packages;
// internal/renametype (TKT-04YA) and internal/analysis (TKT-B01S)
// follow. Each lift swaps signatures here without touching the
// subcommand handlers.
type cliAnalyze interface {
	cliRead
	AnalyzeAll(ctx context.Context, opts workspace.AnalyzeOptions) *workspace.AnalysisSummary
	CheckCardinality(opts workspace.AnalyzeOptions) []workspace.CardinalityViolation
	FindDuplicates(opts workspace.AnalyzeOptions) []workspace.DuplicateGroup
	FindGaps(opts workspace.AnalyzeOptions) []workspace.GapResult
	FindOrphansWithScope(opts workspace.AnalyzeOptions) []*entity.Entity
	FindOrphanedTempFiles() ([]string, error)
	CleanupOrphanedTempFiles() (int, error)
	RunValidations(ctx context.Context, opts workspace.AnalyzeOptions) workspace.ValidationResult
	RunValidationsFiltered(
		ctx context.Context,
		opts workspace.AnalyzeOptions,
		filters []workspace.ValidationFilter,
	) workspace.ValidationResult
	RenameEntityType(oldType, newType, newPlural string) (int, error)
	AttachFile(ctx context.Context, entityID, filePath, property string) (*attachment.Result, error)
	ListAttachments(ctx context.Context, entityID string) ([]attachment.Info, error)
}

// cliServices is the single concrete implementation that satisfies
// all three bundle interfaces. It holds a *workspace.Workspace for
// the methods still rooted there + dedicated services that have
// been lifted out (currently: attachment; more to follow as
// TKT-04YA / TKT-B01S land). The interfaces — not the struct — are
// the consumer-facing contracts: subcommands see only the bundle
// they pulled from context.
type cliServices struct {
	ws         *workspace.Workspace
	attachment *attachment.Service
	renametype *renametype.Service
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

func (s *cliServices) Store() store.Store              { return s.ws.Store() }
func (s *cliServices) Meta() *metamodel.Metamodel      { return s.ws.Meta() }
func (s *cliServices) Paths() *project.Context         { return s.ws.Paths() }
func (s *cliServices) Tracer() tracer.Tracer           { return s.ws.Tracer() }
func (s *cliServices) Searcher() search.Searcher       { return s.ws.Searcher() }
func (s *cliServices) Config() config.Loader           { return s.ws.Config() }
func (s *cliServices) Templater() templating.Templater { return s.ws.Templater() }
func (s *cliServices) FS() storage.FS                  { return s.ws.FS() }

// --- cliWrite ---

func (s *cliServices) EntityManager() entitymanager.EntityManager { return s.ws.EntityManager() }
func (s *cliServices) Validator() validator.Validator             { return s.ws.Validator() }
func (s *cliServices) LuaCache() *lua.Cache                       { return s.ws.LuaCache() }
func (s *cliServices) LuaWriteDeps() lua.WriteDeps                { return s.ws.LuaWriteDeps() }

// --- cliAnalyze (transitional — TKT-2W0X) ---

func (s *cliServices) AnalyzeAll(ctx context.Context, opts workspace.AnalyzeOptions) *workspace.AnalysisSummary {
	return s.ws.AnalyzeAll(ctx, opts)
}

func (s *cliServices) CheckCardinality(opts workspace.AnalyzeOptions) []workspace.CardinalityViolation {
	return s.ws.CheckCardinality(opts)
}

func (s *cliServices) FindDuplicates(opts workspace.AnalyzeOptions) []workspace.DuplicateGroup {
	return s.ws.FindDuplicates(opts)
}

func (s *cliServices) FindGaps(opts workspace.AnalyzeOptions) []workspace.GapResult {
	return s.ws.FindGaps(opts)
}

func (s *cliServices) FindOrphansWithScope(opts workspace.AnalyzeOptions) []*entity.Entity {
	return s.ws.FindOrphansWithScope(opts)
}

func (s *cliServices) FindOrphanedTempFiles() ([]string, error) {
	return s.ws.FindOrphanedTempFiles()
}

func (s *cliServices) CleanupOrphanedTempFiles() (int, error) {
	return s.ws.CleanupOrphanedTempFiles()
}

func (s *cliServices) RunValidations(ctx context.Context, opts workspace.AnalyzeOptions) workspace.ValidationResult {
	return s.ws.RunValidations(ctx, opts)
}

func (s *cliServices) RunValidationsFiltered(
	ctx context.Context,
	opts workspace.AnalyzeOptions,
	filters []workspace.ValidationFilter,
) workspace.ValidationResult {
	return s.ws.RunValidationsFiltered(ctx, opts, filters)
}

func (s *cliServices) RenameEntityType(oldType, newType, newPlural string) (int, error) {
	if s.renametype == nil {
		// Reached only when a test built cliServices from an
		// FS-less workspace.NewForTest fixture and then drove a
		// rename. Production wiring always populates renametype
		// (workspace.Discover guarantees FS + Paths). Panic loudly
		// so the test setup gap is unmistakable.
		panic("cli: renametype service not wired — test fixture must use workspace.WithFS")
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
// the focused-services bundle. Mirrors workspace.Discover's
// behavior; the returned *cliServices satisfies cliRead / cliWrite /
// cliAnalyze.
func newCLIServices(startDir string) (*cliServices, error) {
	ws, err := workspace.Discover(startDir, script.NewEngine())
	if err != nil {
		return nil, err
	}
	return newCLIServicesFromWorkspace(ws)
}

// newCLIServicesFromWorkspace wires the focused services around an
// already-constructed workspace. Used by [newCLIServices] in
// production and by the test fixture (which constructs workspace
// via [workspace.NewForTest]).
func newCLIServicesFromWorkspace(ws *workspace.Workspace) (*cliServices, error) {
	att, err := attachment.New(attachment.Deps{
		Store:         ws.Store(),
		Meta:          ws.Meta(),
		EntityManager: ws.EntityManager(),
	})
	if err != nil {
		return nil, fmt.Errorf("attachment service: %w", err)
	}
	// renametype needs FS + Paths; the test fixture's
	// workspace.NewForTest skips both. Skip wiring renametype when
	// either is absent — handlers panic clearly via the unset-service
	// check rather than nil-deref when an FS-less workspace is given
	// to a rename test by accident.
	var rt *renametype.Service
	if ws.FS() != nil && ws.Paths() != nil {
		rt, err = renametype.New(renametype.Deps{
			FS:    ws.FS(),
			Meta:  ws.Meta(),
			Paths: ws.Paths(),
		})
		if err != nil {
			return nil, fmt.Errorf("renametype service: %w", err)
		}
	}
	return &cliServices{ws: ws, attachment: att, renametype: rt}, nil
}
