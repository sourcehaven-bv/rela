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
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// cliServices is the concrete bundle every CLI subcommand binds to.
// It composes the appbuild service collection with the focused
// service objects (attachment / renametype / analysis) that the CLI
// owns directly.
//
// Methods are grouped by their "purpose" (read / write / analyze) so
// reviewing them documents the same separation the previous bundle
// interfaces enforced; the consumer of *cliServices is each kong
// command's Run method.
//
// TODO(TKT-N0IKN9): 28 exported methods, over the 20 exported-method line.
// This is the CLI service bundle each command binds to; the count tracks the
// breadth of the CLI surface. Ratchet candidate — purpose-grouped sub-bundles
// (read / write / analyze) would let each command bind only what it uses.
//
//plimsoll:max-exported-methods=28
type cliServices struct {
	svc        *appbuild.Services
	attachment *attachment.Service
	renametype *renametype.Service
	analysis   *analysis.Service
}

// --- read-side ---

func (s *cliServices) Store() store.Store              { return s.svc.Store() }
func (s *cliServices) Meta() *metamodel.Metamodel      { return s.svc.Meta() }
func (s *cliServices) Paths() *project.Context         { return s.svc.Paths() }
func (s *cliServices) Tracer() tracer.Tracer           { return s.svc.Tracer() }
func (s *cliServices) Searcher() search.Searcher       { return s.svc.Searcher() }
func (s *cliServices) Config() config.Loader           { return s.svc.Config() }
func (s *cliServices) Templater() templating.Templater { return s.svc.Templater() }
func (s *cliServices) FS() storage.FS                  { return s.svc.FS() }

// --- write-side ---

func (s *cliServices) EntityManager() entitymanager.EntityManager { return s.svc.EntityManager() }
func (s *cliServices) Validator() validator.Validator             { return s.svc.Validator() }
func (s *cliServices) LuaCache() *lua.Cache                       { return s.svc.ScriptEngine().LuaCache() }
func (s *cliServices) LuaWriteDeps() lua.WriteDeps                { return s.svc.LuaWriteDeps() }
func (s *cliServices) State() state.KV                            { return s.svc.State() }

// LuaReadDeps surfaces the read-only Lua capability bundle —
// scheduler.WorkspaceProvider requires it.
func (s *cliServices) LuaReadDeps() lua.ReadDeps { return s.svc.LuaReadDeps() }

// --- analyze-side ---

func (s *cliServices) AnalyzeAll(ctx context.Context, opts analysis.Options) *analysis.Summary {
	return s.analysis.AnalyzeAll(ctx, opts)
}

func (s *cliServices) CheckCardinality(ctx context.Context, opts analysis.Options) []analysis.CardinalityViolation {
	return s.analysis.CheckCardinality(ctx, opts)
}

func (s *cliServices) CheckRelationOrder(ctx context.Context, opts analysis.Options) []analysis.RelationOrderIssue {
	return s.analysis.CheckRelationOrder(ctx, opts)
}

func (s *cliServices) FindDuplicates(ctx context.Context, opts analysis.Options) []analysis.DuplicateGroup {
	return s.analysis.FindDuplicates(ctx, opts)
}

func (s *cliServices) FindGaps(ctx context.Context, opts analysis.Options) []analysis.GapResult {
	return s.analysis.FindGaps(ctx, opts)
}

func (s *cliServices) FindOrphansWithScope(ctx context.Context, opts analysis.Options) []*entity.Entity {
	return s.analysis.FindOrphansWithScope(ctx, opts)
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
	return s.renametype.Rename(oldType, newType, newPlural)
}

func (s *cliServices) AttachFile(ctx context.Context, entityID, filePath, property string) (*attachment.Result, error) {
	return s.attachment.Attach(ctx, entityID, filePath, property)
}

func (s *cliServices) DetachFile(ctx context.Context, entityID, property, fileName string) error {
	return s.attachment.Detach(ctx, entityID, property, fileName)
}

func (s *cliServices) ListAttachments(ctx context.Context, entityID string) ([]attachment.Info, error) {
	return s.attachment.List(ctx, entityID)
}

// newCLIServicesFromAppbuild wires the focused services around an
// already-constructed appbuild.Services. Used by [newCLIServices] in
// production and by CLI test fixtures.
func newCLIServicesFromAppbuild(svc *appbuild.Services) (*cliServices, error) {
	att, err := attachment.New(attachment.Deps{
		Store:         svc.Store(),
		Meta:          svc.Meta(),
		EntityManager: svc.EntityManager(),
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
	return &cliServices{svc: svc, attachment: att, renametype: rt, analysis: an}, nil
}
