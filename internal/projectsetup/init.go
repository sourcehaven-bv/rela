package projectsetup

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// InitResult contains information about what was created during initialization.
type InitResult struct {
	Root            string
	SchemaPath      string
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

	// Refuse if EITHER schema name is present. Checking only the new name
	// would treat a legacy project as an empty directory and write a default
	// schema.yaml next to the operator's real metamodel.yaml; since discovery
	// prefers the new name, the project would then come up on the empty
	// default with the real schema silently ignored.
	if existing, isLegacy, found := project.SchemaFileAt(targetDir, fs); found {
		if isLegacy {
			return nil, fmt.Errorf(
				"project already initialized (%s exists) — run `rela migrate` to rename it to %s",
				filepath.Base(existing), project.SchemaFile)
		}
		return nil, fmt.Errorf("project already initialized (%s exists)", project.SchemaFile)
	}

	schemaPath := filepath.Join(targetDir, project.SchemaFile)

	// Create project context with all paths
	ctx := &project.Context{
		Root:                 targetDir,
		SchemaPath:           schemaPath,
		CacheDir:             filepath.Join(targetDir, project.CacheDir),
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
	if err := fs.WriteFile(schemaPath, []byte(metamodel.DefaultMetamodelYAML()), 0644); err != nil {
		return nil, fmt.Errorf("write metamodel: %w", err)
	}

	result := &InitResult{
		Root:       targetDir,
		SchemaPath: schemaPath,
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
