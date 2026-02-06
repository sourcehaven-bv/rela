package project

import (
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const (
	MetamodelFile        = "metamodel.yaml"
	CacheDir             = ".rela"
	CacheFile            = "cache.json"
	EntitiesDir          = "entities"
	RelationsDir         = "relations"
	TemplatesDir         = "templates"
	EntityTemplatesDir   = "entities"
	RelationTemplatesDir = "relations"
)

// Context holds the paths and state for a rela project
type Context struct {
	Root                 string // Project root directory
	MetamodelPath        string // Path to metamodel.yaml
	CacheDir             string // Path to .rela directory
	CachePath            string // Path to .rela/cache.json
	EntitiesDir          string // Path to entities directory
	RelationsDir         string // Path to relations directory
	TemplatesDir         string // Path to templates directory
	EntityTemplatesDir   string // Path to templates/entities directory
	RelationTemplatesDir string // Path to templates/relations directory
}

// Discover finds the project root by searching for metamodel.yaml
// using the given filesystem.
// It starts from the given directory and walks up the tree.
func Discover(startDir string, fs storage.FS) (*Context, error) {
	if startDir == "" {
		var err error
		startDir, err = fs.Getwd()
		if err != nil {
			return nil, err
		}
	}

	// Convert to absolute path
	startDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	// Walk up the directory tree looking for metamodel.yaml
	dir := startDir
	for {
		metamodelPath := filepath.Join(dir, MetamodelFile)
		if _, err := fs.Stat(metamodelPath); err == nil {
			return newContext(dir), nil
		}

		// Also check for .rela directory (legacy/alternative marker)
		relaDir := filepath.Join(dir, CacheDir)
		if info, err := fs.Stat(relaDir); err == nil && info.IsDir() {
			return newContext(dir), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return nil, errors.ErrNoProject
		}
		dir = parent
	}
}

// newContext creates a Context for the given project root
func newContext(root string) *Context {
	templatesDir := filepath.Join(root, TemplatesDir)
	return &Context{
		Root:                 root,
		MetamodelPath:        filepath.Join(root, MetamodelFile),
		CacheDir:             filepath.Join(root, CacheDir),
		CachePath:            filepath.Join(root, CacheDir, CacheFile),
		EntitiesDir:          filepath.Join(root, EntitiesDir),
		RelationsDir:         filepath.Join(root, RelationsDir),
		TemplatesDir:         templatesDir,
		EntityTemplatesDir:   filepath.Join(templatesDir, EntityTemplatesDir),
		RelationTemplatesDir: filepath.Join(templatesDir, RelationTemplatesDir),
	}
}

// Initialize creates the project structure using the given filesystem.
func (c *Context) Initialize(fs storage.FS) error {
	// Create .rela directory
	if err := fs.MkdirAll(c.CacheDir, 0755); err != nil {
		return err
	}

	// Create entities directory
	if err := fs.MkdirAll(c.EntitiesDir, 0755); err != nil {
		return err
	}

	// Create relations directory
	return fs.MkdirAll(c.RelationsDir, 0755)
}

// EntityTypeDir returns the directory for a given entity type (pluralized)
// Deprecated: Use EntityTypeDirWithPlural when metamodel is available
func (c *Context) EntityTypeDir(entityType string) string {
	// Simple pluralization: just add 's'
	// The metamodel can provide proper plural names
	plural := entityType + "s"
	return filepath.Join(c.EntitiesDir, plural)
}

// EntityTypeDirWithPlural returns the directory for a given entity type using the provided plural form
func (c *Context) EntityTypeDirWithPlural(plural string) string {
	return filepath.Join(c.EntitiesDir, plural)
}

// EntityFilePath returns the file path for an entity
// Deprecated: Use EntityFilePathWithPlural when metamodel is available
func (c *Context) EntityFilePath(entityType, id string) string {
	return filepath.Join(c.EntityTypeDir(entityType), id+".md")
}

// EntityFilePathWithPlural returns the file path for an entity using the provided plural form
func (c *Context) EntityFilePathWithPlural(plural, id string) string {
	return filepath.Join(c.EntityTypeDirWithPlural(plural), id+".md")
}

// RelationFilePath returns the file path for a relation
func (c *Context) RelationFilePath(from, relationType, to string) string {
	filename := from + "--" + relationType + "--" + to + ".md"
	return filepath.Join(c.RelationsDir, filename)
}

// Exists checks if the project has been initialized using the given filesystem.
func (c *Context) Exists(fs storage.FS) bool {
	_, err := fs.Stat(c.MetamodelPath)
	return err == nil
}

// EntityTemplatePath returns the file path for an entity type template.
// If variant is non-empty, returns the path for that variant (e.g., type--variant.md).
func (c *Context) EntityTemplatePath(entityType string) string {
	return filepath.Join(c.EntityTemplatesDir, entityType+".md")
}

// EntityTemplateVariantPath returns the file path for an entity template variant.
// Variant templates use the naming convention: <type>--<variant>.md
func (c *Context) EntityTemplateVariantPath(entityType, variant string) string {
	if variant == "" {
		return c.EntityTemplatePath(entityType)
	}
	return filepath.Join(c.EntityTemplatesDir, entityType+"--"+variant+".md")
}

// RelationTemplatePath returns the file path for a relation type template
func (c *Context) RelationTemplatePath(relationType string) string {
	return filepath.Join(c.RelationTemplatesDir, relationType+".md")
}
