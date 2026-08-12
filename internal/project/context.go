package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const (
	// SchemaFile is the canonical name of the project schema file.
	SchemaFile = "schema.yaml"
	// LegacySchemaFile is the pre-rename name, still accepted by Discover so
	// existing projects keep working. Deprecated: create SchemaFile instead;
	// `rela migrate` renames it. Support stays until a major version bump.
	LegacySchemaFile     = "metamodel.yaml"
	CacheDir             = ".rela"
	EntitiesDir          = "entities"
	RelationsDir         = "relations"
	TemplatesDir         = "templates"
	EntityTemplatesDir   = "entities"
	RelationTemplatesDir = "relations"
	// AppsDir holds custom data-entry apps (apps/<id>/index.html). Like
	// templates/, it lives on the filesystem in every storage backend and is
	// not modeled into Context (loaded traversal-resistant where needed).
	AppsDir = "apps"
)

// Context holds the paths and state for a rela project
type Context struct {
	Root string // Project root directory
	// SchemaPath is the resolved path to the schema file that was actually
	// found — SchemaFile or LegacySchemaFile. In-place writers (rename-type,
	// migrate) must use this rather than rejoining SchemaFile, so that a
	// legacy project is rewritten under the name it already has instead of
	// sprouting a second file.
	//
	// When a project is discovered via the CacheDir marker with neither file
	// present, this points at the preferred (new) name so "missing schema"
	// errors name the file the user should create.
	SchemaPath string
	// SchemaIsLegacy reports whether SchemaPath resolved to LegacySchemaFile.
	// Entry points read this once at startup to emit the deprecation notice;
	// see the doc on Discover for why it is not warned about here.
	SchemaIsLegacy       bool
	CacheDir             string // Path to .rela directory
	EntitiesDir          string // Path to entities directory
	RelationsDir         string // Path to relations directory
	TemplatesDir         string // Path to templates directory
	EntityTemplatesDir   string // Path to templates/entities directory
	RelationTemplatesDir string // Path to templates/relations directory
}

// Discover finds the project root by searching for the schema file using the
// given filesystem. It starts from the given directory and walks up the tree.
//
// Both SchemaFile and LegacySchemaFile are accepted, checked in that order at
// each level before ascending — per-level rather than as two separate walks, so
// the nearest project root still wins when a legacy-named child sits inside a
// new-named parent. When both names exist in one directory the new name is
// preferred: a project with both is mid-migration, and SchemaFile is the file
// the user just wrote.
//
// Discover deliberately does NOT warn about a legacy name. It runs per-request
// in the data-entry and MCP servers, so a warning here would spam logs; entry
// points read Context.SchemaIsLegacy and warn once at startup instead.
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

	// Walk up the directory tree looking for a schema file
	dir := startDir
	for {
		// New name wins over the legacy one in the same directory.
		if path, isLegacy, found := SchemaFileAt(dir, fs); found {
			ctx := newContext(dir)
			ctx.SchemaPath = path
			ctx.SchemaIsLegacy = isLegacy
			return ctx, nil
		}

		// Also check for .rela directory (legacy/alternative marker).
		//
		// Note this matches BEFORE ascending, so a stray .rela/ in a
		// subdirectory shadows a real project further up: the walk stops here
		// and SchemaPath points at a file that does not exist. Preserved as-is
		// (changing discovery semantics is out of scope for the rename), but
		// the resulting "missing schema" error names the selected root so the
		// situation is diagnosable.
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
		SchemaPath:           filepath.Join(root, SchemaFile),
		CacheDir:             filepath.Join(root, CacheDir),
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
//
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
//
// Deprecated: Use EntityFilePathWithPlural when metamodel is available
func (c *Context) EntityFilePath(entityType, id string) string {
	return filepath.Join(c.EntityTypeDir(entityType), id+".md")
}

// EntityFilePathWithPlural returns the file path for an entity using the provided plural form
func (c *Context) EntityFilePathWithPlural(plural, id string) string {
	return filepath.Join(c.EntityTypeDirWithPlural(plural), id+".md")
}

// RelationFilePath returns the file path for a relation.
//
// Callers are responsible for validating from / relationType / to upstream:
//
//   - entity IDs go through entity.ValidateID at creation time
//   - relation types come from the metamodel and are checked by
//     metamodel.ValidateRelation before any write reaches this code path
//
// This function deliberately does NOT defensively reject malformed inputs:
// turning a programming-error panic into a returned error here would force
// every caller (sync, write, rename) to grow an error path for a condition
// that the validation gates above already make impossible. If those gates
// ever change, the test in IsSafeRelationComponent should be reused to
// reject untrusted input at the new entry point instead.
func (c *Context) RelationFilePath(from, relationType, to string) string {
	filename := from + "--" + relationType + "--" + to + ".md"
	return filepath.Join(c.RelationsDir, filename)
}

// Exists reports whether the project has been initialized, accepting EITHER
// schema file name. Checking only the new name would let a caller conclude a
// legacy project is uninitialized and overwrite it — see SchemaFileAt.
func (c *Context) Exists(fs storage.FS) bool {
	_, _, found := SchemaFileAt(c.Root, fs)
	return found
}

// legacySchemaWarning ensures the deprecation notice is printed at most once
// per process, however many times a project is opened.
var legacySchemaWarning sync.Once

// WarnIfLegacySchema writes a one-time deprecation notice to stderr when ctx
// resolved to the legacy schema name. It is a no-op otherwise.
//
// Call this from an entry point's startup path — every binary that opens a
// project, not just the CLI, or server-only and desktop-only operators would
// never learn the name is going away. It is deliberately NOT called from
// [Discover], which runs per-request in the data-entry and MCP servers.
func WarnIfLegacySchema(ctx *Context) {
	if ctx == nil || !ctx.SchemaIsLegacy {
		return
	}
	legacySchemaWarning.Do(func() {
		fmt.Fprintf(os.Stderr,
			"warning: %s is deprecated and will stop being read in a future major version; "+
				"run `rela migrate` to rename it to %s\n",
			LegacySchemaFile, SchemaFile)
	})
}

// SchemaFileAt reports which schema file exists directly in dir, without
// walking up the tree. It returns the resolved path, whether that path is the
// legacy name, and whether either was found. SchemaFile takes precedence.
//
// This is the shared guard for code that must not mistake a legacy project for
// an empty directory — notably project creation, which would otherwise write a
// default schema.yaml alongside a real metamodel.yaml. Because discovery
// prefers the new name, the project would then silently come up on the empty
// default with the operator's real schema ignored.
func SchemaFileAt(dir string, fs storage.FS) (path string, isLegacy, found bool) {
	for _, name := range []string{SchemaFile, LegacySchemaFile} {
		p := filepath.Join(dir, name)
		if info, err := fs.Stat(p); err == nil && !info.IsDir() {
			return p, name == LegacySchemaFile, true
		}
	}
	return "", false, false
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
