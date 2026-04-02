package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// InitResult contains information about what was created during initialization.
type InitResult struct {
	Root            string
	MetamodelPath   string
	GitignoreUpdate bool
}

// Initialize creates a new rela project in the given directory.
// If targetDir is empty, it uses the current working directory.
// It creates the directory structure, writes a default metamodel, and
// optionally updates .gitignore.
func Initialize(targetDir string) (*InitResult, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	return InitializeWithFS(targetDir, fs)
}

// InitializeWithFS creates a new rela project using the provided filesystem.
// This is useful for testing.
func InitializeWithFS(targetDir string, fs storage.FS) (*InitResult, error) {
	// Resolve target directory
	if targetDir == "" {
		cwd, err := fs.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		targetDir = cwd
	}

	metamodelPath := filepath.Join(targetDir, project.MetamodelFile)

	// Check if already initialized
	if _, err := fs.Stat(metamodelPath); err == nil {
		return nil, fmt.Errorf("project already initialized (metamodel.yaml exists)")
	}

	// Create project context with all paths
	ctx := &project.Context{
		Root:                 targetDir,
		MetamodelPath:        metamodelPath,
		CacheDir:             filepath.Join(targetDir, project.CacheDir),
		CachePath:            filepath.Join(targetDir, project.CacheDir, project.CacheFile),
		EntitiesDir:          filepath.Join(targetDir, project.EntitiesDir),
		RelationsDir:         filepath.Join(targetDir, project.RelationsDir),
		TemplatesDir:         filepath.Join(targetDir, project.TemplatesDir),
		EntityTemplatesDir:   filepath.Join(targetDir, project.TemplatesDir, project.EntityTemplatesDir),
		RelationTemplatesDir: filepath.Join(targetDir, project.TemplatesDir, project.RelationTemplatesDir),
	}

	// Create directories
	if err := ctx.Initialize(fs); err != nil {
		return nil, fmt.Errorf("create directories: %w", err)
	}

	// Write default metamodel
	if err := fs.WriteFile(metamodelPath, []byte(metamodel.DefaultMetamodelYAML()), 0644); err != nil {
		return nil, fmt.Errorf("write metamodel: %w", err)
	}

	result := &InitResult{
		Root:          targetDir,
		MetamodelPath: metamodelPath,
	}

	// Add .rela to .gitignore if it exists
	gitignorePath := filepath.Join(targetDir, ".gitignore")
	if _, err := fs.Stat(gitignorePath); err == nil {
		content, err := fs.ReadFile(gitignorePath)
		if err == nil && !strings.Contains(string(content), ".rela") {
			content = append(content, []byte("\n# rela cache\n.rela/\n")...)
			if writeErr := fs.WriteFile(gitignorePath, content, 0644); writeErr == nil {
				result.GitignoreUpdate = true
			}
		}
	}

	return result, nil
}

// NewAfterInit creates a workspace for a newly initialized project.
// Use this after Initialize() when you need a workspace immediately.
// Uses NopScriptExecutor since newly initialized projects don't have
// Lua automations configured.
func NewAfterInit(targetDir string) (*Workspace, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	ctx, err := project.Discover(targetDir, fs)
	if err != nil {
		return nil, err
	}
	repo := repository.New(fs, ctx)
	return New(repo, NopScriptExecutor)
}
