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

// DetectMigrationsWithFS checks for pending migrations using the provided filesystem.
func DetectMigrationsWithFS(startDir string, fs storage.FS) ([]MigrateDetection, error) {
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, errors.New("no project found: run 'rela init' to create one")
	}

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
	result := &MigrateResult{SchemaRenamedFrom: renamed}

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

// LegacySchemaPending reports whether the project rooted at (or above)
// startDir still uses the pre-rename schema filename, so `--check` callers can
// fail CI on it without mutating anything. A discovery failure is reported as
// "not pending" — the caller's own error path handles a missing project.
func LegacySchemaPending(startDir string, fs storage.FS) bool {
	ctx, err := project.Discover(startDir, fs)
	return err == nil && ctx.SchemaIsLegacy
}

// renameLegacySchema renames metamodel.yaml to schema.yaml and updates
// ctx.SchemaPath in place. It returns the old basename when a rename happened,
// or "" when there was nothing to do.
//
// It REFUSES when both files exist rather than renaming: storage.FS.Rename
// wraps os.Rename, which on POSIX replaces an existing target silently, so a
// stat-then-rename guard cannot make overwriting safe — it can only decide not
// to try. Destroying an operator's schema.yaml here would be unrecoverable, and
// a project holding both files is ambiguous enough to be worth a human look.
func renameLegacySchema(ctx *project.Context, fs storage.FS) (string, error) {
	if !ctx.SchemaIsLegacy {
		return "", nil
	}

	target := filepath.Join(ctx.Root, project.SchemaFile)
	if _, err := fs.Stat(target); err == nil {
		return "", fmt.Errorf(
			"both %s and %s exist in %s: remove or merge %s, then re-run",
			project.SchemaFile, project.LegacySchemaFile, ctx.Root, project.LegacySchemaFile)
	}

	if err := fs.Rename(ctx.SchemaPath, target); err != nil {
		return "", fmt.Errorf("rename %s to %s: %w",
			project.LegacySchemaFile, project.SchemaFile, err)
	}

	ctx.SchemaPath = target
	ctx.SchemaIsLegacy = false
	return project.LegacySchemaFile, nil
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
