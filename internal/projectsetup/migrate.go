package projectsetup

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// MigrateFile represents a file that can be migrated.
type MigrateFile struct {
	Path     string
	Name     string
	FileType migration.FileType
}

// MigrateDetection represents a detected migration for a file.
type MigrateDetection struct {
	File       MigrateFile
	Migrations []migration.DetectionResult
}

// MigrateResult contains the outcome of applying migrations.
type MigrateResult struct {
	FilesUpdated    int
	TotalMigrations int
	FileResults     []MigrateFileResult
	// SchemaRenamedFrom is the old basename when the legacy schema file was
	// renamed during this run, or "" when no rename happened.
	SchemaRenamedFrom string
	// OrphanedLegacySchema is the path to a metamodel.yaml that is being
	// ignored because a schema.yaml exists alongside it, or "" when there is
	// none. Reported rather than deleted: removing an operator's file is not
	// migrate's call, but leaving it unmentioned makes it invisible.
	OrphanedLegacySchema string
}

// MigrateFileResult contains the result for a single file.
type MigrateFileResult struct {
	File    MigrateFile
	Results []migration.Result
	Error   error
}

// DetectMigrations checks for pending migrations in project files.
// If startDir is empty, it uses the current working directory.
func DetectMigrations(startDir string) ([]MigrateDetection, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	return DetectMigrationsWithFS(startDir, fs)
}

// CheckPending reports everything `rela migrate --check` needs: content
// migrations plus schema-filename work. It discovers the project ONCE and
// derives both answers from that single Context, so the two cannot disagree
// because the disk changed between two separate walks.
func CheckPending(startDir string) ([]MigrateDetection, SchemaNameStatus, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, SchemaNameStatus{}, errors.New("no project found: run 'rela init' to create one")
	}
	detections, err := detectMigrationsIn(ctx, fs)
	if err != nil {
		return nil, SchemaNameStatus{}, err
	}
	return detections, SchemaName(ctx, fs), nil
}

// DetectMigrationsWithFS checks for pending migrations using the provided filesystem.
func DetectMigrationsWithFS(startDir string, fs storage.FS) ([]MigrateDetection, error) {
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, errors.New("no project found: run 'rela init' to create one")
	}
	return detectMigrationsIn(ctx, fs)
}

// detectMigrationsIn is the shared body, taking an already-discovered project.
func detectMigrationsIn(ctx *project.Context, fs storage.FS) ([]MigrateDetection, error) {
	// Load metamodel for context-aware migrations (ignore errors - may need migration itself)
	mm, _, _ := metamodel.LoadWithoutMigrationCheck(ctx.SchemaPath, fs)

	files := getMigrateFiles(ctx)
	var detections []MigrateDetection

	for _, f := range files {
		// Skip files that don't exist
		if _, statErr := fs.Stat(f.Path); statErr != nil {
			continue
		}

		detected, err := migration.DetectWithMetamodel(f.Path, f.FileType, fs, mm)
		if err != nil {
			return nil, fmt.Errorf("checking %s: %w", f.Name, err)
		}

		if len(detected) > 0 {
			detections = append(detections, MigrateDetection{
				File:       f,
				Migrations: detected,
			})
		}
	}

	return detections, nil
}

// Migrate applies pending migrations to project files.
// If startDir is empty, it uses the current working directory.
func Migrate(startDir string) (*MigrateResult, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	return MigrateWithFS(startDir, fs)
}

// MigrateWithFS applies migrations using the provided filesystem.
func MigrateWithFS(startDir string, fs storage.FS) (*MigrateResult, error) {
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, errors.New("no project found: run 'rela init' to create one")
	}

	// Rename the schema file BEFORE anything reads ctx.SchemaPath. Ordering is
	// load-bearing: getMigrateFiles snapshots the path, and the loop below
	// silently skips entries whose file is missing, so renaming afterwards
	// would leave the project renamed but never content-migrated.
	renamed, err := renameLegacySchema(ctx, fs)
	if err != nil {
		return nil, err
	}

	// Load metamodel for context-aware migrations (ignore errors - may need migration itself)
	mm, _, _ := metamodel.LoadWithoutMigrationCheck(ctx.SchemaPath, fs)

	files := getMigrateFiles(ctx)
	result := &MigrateResult{
		SchemaRenamedFrom:    renamed,
		OrphanedLegacySchema: OrphanedLegacySchema(ctx, fs),
	}

	for _, f := range files {
		// Skip files that don't exist
		if _, statErr := fs.Stat(f.Path); statErr != nil {
			continue
		}

		fileResult, err := migration.ApplyWithMetamodel(f.Path, f.FileType, fs, mm)
		if err != nil {
			return result, fmt.Errorf("migrating %s: %w", f.Name, err)
		}

		if fileResult.HasErrors() {
			return result, fmt.Errorf("migrating %s: %w", f.Name, fileResult.Error)
		}

		migrationCount := 0
		var migrationResults []migration.Result
		for _, mr := range fileResult.Results {
			if mr.Applied {
				migrationCount++
			}
			migrationResults = append(migrationResults, mr)
		}

		if migrationCount > 0 {
			result.FilesUpdated++
			result.TotalMigrations += migrationCount
		}

		result.FileResults = append(result.FileResults, MigrateFileResult{
			File:    f,
			Results: migrationResults,
		})
	}

	return result, nil
}

// SchemaNameStatus describes schema-filename work a project needs, without
// mutating anything, so `--check` callers can fail CI on it.
type SchemaNameStatus struct {
	// RenamePending is true when the project still uses the legacy filename.
	RenamePending bool
	// Orphaned is the path to an ignored metamodel.yaml sitting beside a live
	// schema.yaml, or "" when there is none.
	Orphaned string
}

// NeedsAttention reports whether there is anything to tell the operator.
func (s SchemaNameStatus) NeedsAttention() bool {
	return s.RenamePending || s.Orphaned != ""
}

// SchemaName inspects an already-discovered project. It takes a Context rather
// than a start directory so callers that have discovered the project do not
// walk the tree a second time — two independent walks can disagree if the disk
// changes between them.
func SchemaName(ctx *project.Context, fs storage.FS) SchemaNameStatus {
	return SchemaNameStatus{
		RenamePending: ctx.SchemaIsLegacy,
		Orphaned:      OrphanedLegacySchema(ctx, fs),
	}
}

// renameLegacySchema renames metamodel.yaml to schema.yaml and updates
// ctx.SchemaPath in place. It returns the old basename when a rename happened,
// or "" when there was nothing to do.
//
// Note the both-files case never reaches here: ctx.SchemaIsLegacy is only set
// when discovery fell through to the legacy name, which means schema.yaml is
// absent. An orphaned legacy file alongside a live schema.yaml is reported by
// OrphanedLegacySchema instead — this function is only reached when there is
// exactly one schema file and it has the old name.
//
// The rename is therefore never an overwrite. That matters because
// storage.FS.Rename wraps os.Rename, which replaces an existing target
// silently on POSIX: a stat-then-rename guard could not have made overwriting
// safe, only declined to try.
func renameLegacySchema(ctx *project.Context, fs storage.FS) (string, error) {
	if !ctx.SchemaIsLegacy {
		return "", nil
	}

	target := filepath.Join(ctx.Root, project.SchemaFile)
	if err := fs.Rename(ctx.SchemaPath, target); err != nil {
		return "", fmt.Errorf("rename %s to %s: %w",
			project.LegacySchemaFile, project.SchemaFile, err)
	}

	ctx.SchemaPath = target
	ctx.SchemaIsLegacy = false
	return project.LegacySchemaFile, nil
}

// OrphanedLegacySchema reports whether a metamodel.yaml is sitting next to a
// live schema.yaml. Discovery prefers the new name and ignores the old one, so
// without this the stale file would be silently invisible forever: it is not
// "pending a rename" (there is nothing to rename it to), and every command
// reads past it without comment.
//
// Returned as a path so callers can name it. Empty when there is no orphan.
func OrphanedLegacySchema(ctx *project.Context, fs storage.FS) string {
	if ctx.SchemaIsLegacy {
		return "" // the legacy file IS the live one; that's a rename, not an orphan
	}
	legacy := filepath.Join(ctx.Root, project.LegacySchemaFile)
	if info, err := fs.Stat(legacy); err == nil && !info.IsDir() {
		return legacy
	}
	return ""
}

func getMigrateFiles(ctx *project.Context) []MigrateFile {
	return []MigrateFile{
		{
			Path:     ctx.SchemaPath,
			Name:     filepath.Base(ctx.SchemaPath),
			FileType: migration.FileTypeMetamodel,
		},
		{
			Path:     filepath.Join(ctx.Root, dataentryconfig.ConfigFile),
			Name:     dataentryconfig.ConfigFile,
			FileType: migration.FileTypeDataEntry,
		},
		{
			// Only migrated when it already exists — the callers' skip-on-
			// missing guard is load-bearing here. A project with no acl.yaml
			// runs on NopACL and must stay that way; creating one would flip
			// every principal to deny-by-default (RR-SVQ5HE).
			Path:     filepath.Join(ctx.Root, aclConfigFile),
			Name:     aclConfigFile,
			FileType: migration.FileTypeACL,
		},
	}
}

// aclConfigFile is the policy filename. Declared here because the migrate
// file list is the only place in projectsetup that needs it; appbuild and
// the acl CLI commands each join the same literal against the project root.
const aclConfigFile = "acl.yaml"
