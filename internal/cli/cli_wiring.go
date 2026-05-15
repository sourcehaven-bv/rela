package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
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
// today live on *workspace.Workspace. TKT-2W0X lifts these methods
// into dedicated packages (internal/analysis, internal/attachment,
// internal/renametype); when that lands the implementation behind
// this interface swaps without touching subcommands.
//
// Deprecated workspace.* types in the signatures (AnalyzeOptions,
// AnalysisSummary, etc.) move with their methods in TKT-2W0X.
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
	AttachFile(entityID, filePath, property string) (*workspace.AttachResult, error)
	ListAttachments(entityID string) ([]workspace.AttachmentInfo, error)
}

// cliServices is the single concrete implementation that satisfies
// all three bundle interfaces. It holds a *workspace.Workspace and
// forwards every accessor. The interfaces — not the struct — are
// the consumer-facing contracts: subcommands see only the bundle
// they pulled from context.
type cliServices struct {
	ws *workspace.Workspace
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
	return s.ws.RenameEntityType(oldType, newType, newPlural)
}

func (s *cliServices) AttachFile(entityID, filePath, property string) (*workspace.AttachResult, error) {
	return s.ws.AttachFile(entityID, filePath, property)
}

func (s *cliServices) ListAttachments(entityID string) ([]workspace.AttachmentInfo, error) {
	return s.ws.ListAttachments(entityID)
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
	return &cliServices{ws: ws}, nil
}
