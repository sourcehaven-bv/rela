// Package renametype lifts the entity-type rename operation
// (metamodel + directory + per-file YAML frontmatter + template)
// out of the legacy [internal/workspace] package. The service
// depends only on the focused primitives it needs (FS, Meta,
// Paths) so it can be constructed at any wiring site.
package renametype

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Deps is the dependency bundle [New] requires. Every field is
// mandatory; [New] returns an error if any is nil.
type Deps struct {
	FS    storage.FS
	Meta  *metamodel.Metamodel
	Paths *project.Context
}

// Service performs an entity-type rename across the project.
// Constructed once at the wiring site and shared across subcommands.
type Service struct {
	deps Deps
}

// New constructs a Service. Returns an error if any required
// dependency is nil — CLAUDE.md "constructors reject nil required
// fields".
func New(d Deps) (*Service, error) {
	switch {
	case d.FS == nil:
		return nil, errors.New("renametype: FS is required")
	case d.Meta == nil:
		return nil, errors.New("renametype: Meta is required")
	case d.Paths == nil:
		return nil, errors.New("renametype: Paths is required")
	}
	return &Service{deps: d}, nil
}

// Rename renames an entity type across the project: metamodel,
// entity directory, entity files, and templates. Returns the number
// of entity files updated.
//
// Steps, in order:
//  1. Rewrite the type entry in metamodel.yaml.
//  2. Rename the entity directory (old plural → new plural). Skipped
//     if the directory does not exist.
//  3. Rewrite the `type:` field of every entity file under the new
//     directory.
//  4. Rename the entity template, if it exists.
//
// Step 3 is the only step whose count is reported (the per-file
// dimension). On a step-2 skip (missing dir) and a step-4 skip
// (missing template), no error is returned.
//
// NOT ATOMIC. If a step fails after earlier steps succeeded, the
// project is left in a partially-renamed state — metamodel may
// advertise the new type while the directory still uses the old
// plural, or files 1..k may carry the new type while k+1..n still
// carry the old. The error names the failing step so the operator
// can re-run after addressing the cause.
func (s *Service) Rename(oldType, newType, newPlural string) (int, error) {
	fs := s.deps.FS
	meta := s.deps.Meta
	paths := s.deps.Paths

	oldDef, ok := meta.GetEntityDef(oldType)
	if !ok {
		return 0, fmt.Errorf("unknown entity type: %s", oldType)
	}
	oldPlural := oldDef.GetPlural(oldType)

	if err := metamodel.RenameEntityType(paths.MetamodelPath, oldType, newType, fs); err != nil {
		return 0, fmt.Errorf("update metamodel: %w", err)
	}

	oldDir := paths.EntityTypeDirWithPlural(oldPlural)
	newDir := paths.EntityTypeDirWithPlural(newPlural)

	if _, err := fs.Stat(oldDir); err == nil {
		if err := fs.Rename(oldDir, newDir); err != nil {
			return 0, fmt.Errorf("rename directory %s → %s (metamodel already updated; rerun after fixing): %w",
				oldDir, newDir, err)
		}
	}

	count, err := rewriteEntityTypeInDir(fs, newDir, newType)
	if err != nil {
		return count, fmt.Errorf("update entity files (metamodel + directory already renamed; %d/N files rewritten): %w",
			count, err)
	}

	oldTemplatePath := paths.EntityTemplatePath(oldType)
	if _, err := fs.Stat(oldTemplatePath); err == nil {
		newTemplatePath := paths.EntityTemplatePath(newType)
		if err := fs.MkdirAll(paths.EntityTemplatesDir, 0o755); err != nil {
			return count, fmt.Errorf("create template dir %s (entity files already renamed): %w",
				paths.EntityTemplatesDir, err)
		}
		if err := fs.Rename(oldTemplatePath, newTemplatePath); err != nil {
			return count, fmt.Errorf("rename template %s → %s (entity files already renamed): %w",
				oldTemplatePath, newTemplatePath, err)
		}
	}

	return count, nil
}

// rewriteEntityTypeInDir rewrites the YAML `type:` field in every
// .md file under dir to newType, leaving everything else untouched.
// Returns the number of files rewritten. Missing dirs are no-ops.
func rewriteEntityTypeInDir(fs storage.FS, dir, newType string) (int, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := rewriteEntityTypeInFile(fs, path, newType); err != nil {
			return count, fmt.Errorf("update %s: %w", entry.Name(), err)
		}
		count++
	}
	return count, nil
}

// rewriteEntityTypeInFile reads a single entity file and replaces
// its YAML frontmatter `type:` field with newType. Surrounding bytes
// (other frontmatter keys, body content, blank lines) are preserved
// verbatim — only the `type:` value changes.
func rewriteEntityTypeInFile(fs storage.FS, path, newType string) error {
	content, err := fs.ReadFile(path)
	if err != nil {
		return err
	}
	updated, ok := replaceYAMLType(string(content), newType)
	if !ok {
		return nil
	}
	return fs.WriteFile(path, []byte(updated), 0o644)
}

// replaceYAMLType replaces the first occurrence of a top-level
// `type: <something>` line in the YAML frontmatter block (delimited
// by leading and trailing `---` lines). Returns the updated content
// and whether a replacement happened.
func replaceYAMLType(content, newType string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "---" {
			return content, false
		}
		trimmed := strings.TrimLeft(line, " \t")
		if line != trimmed {
			continue
		}
		if strings.HasPrefix(trimmed, "type:") {
			lines[i] = "type: " + newType
			return strings.Join(lines, "\n"), true
		}
	}
	return content, false
}
