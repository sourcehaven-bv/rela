package workspace

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
	mm, _, _ := metamodel.LoadWithoutMigrationCheck(ctx.MetamodelPath, fs)

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

	// Load metamodel for context-aware migrations (ignore errors - may need migration itself)
	mm, _, _ := metamodel.LoadWithoutMigrationCheck(ctx.MetamodelPath, fs)

	files := getMigrateFiles(ctx)
	result := &MigrateResult{}

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

func getMigrateFiles(ctx *project.Context) []MigrateFile {
	return []MigrateFile{
		{
			Path:     ctx.MetamodelPath,
			Name:     "metamodel.yaml",
			FileType: migration.FileTypeMetamodel,
		},
		{
			Path:     filepath.Join(ctx.Root, dataentryconfig.ConfigFile),
			Name:     dataentryconfig.ConfigFile,
			FileType: migration.FileTypeDataEntry,
		},
	}
}
