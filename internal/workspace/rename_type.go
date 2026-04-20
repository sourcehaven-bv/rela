package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// ErrRenameTypeNotSupportedOnEncryptedRepo is returned by
// RenameEntityType when the project has at-rest encryption enabled.
// The current implementation rewrites entity files through the raw
// workspace FS; on an encrypted repo that reads ciphertext and fails
// to match the YAML frontmatter delimiters, silently no-opping while
// reporting success (see encryption-security-review finding C3). A
// proper fix requires routing the operation through a schemaop
// migrator per backend — deferred pending a package-layout refactor.
//
// Workaround for users: `rela keys decrypt` → `rela rename-type …` →
// `rela keys init`.
var ErrRenameTypeNotSupportedOnEncryptedRepo = errors.New(
	"rename-type is not supported on encrypted repos; " +
		"run `rela keys decrypt`, rename, then `rela keys init`")

// RenameEntityType renames an entity type across the project:
// metamodel, entity directory, entity files, and templates.
// Returns the number of entity files updated.
//
// Refuses to run on encryption-enabled projects — the current
// implementation cannot unseal entity files, and a silent no-op
// would violate user trust more than an upfront error does.
func (w *Workspace) RenameEntityType(oldType, newType, newPlural string) (int, error) {
	fs := w.FS()
	meta := w.Meta()
	paths := w.Paths()

	if enabled, err := encryption.IsEnabled(paths.Root); err != nil {
		return 0, fmt.Errorf("check encryption status: %w", err)
	} else if enabled {
		return 0, ErrRenameTypeNotSupportedOnEncryptedRepo
	}

	oldDef, ok := meta.GetEntityDef(oldType)
	if !ok {
		return 0, fmt.Errorf("unknown entity type: %s", oldType)
	}
	oldPlural := oldDef.GetDirPlural(oldType)

	// 1. Update metamodel.yaml
	if err := metamodel.RenameEntityType(paths.MetamodelPath, oldType, newType, fs); err != nil {
		return 0, fmt.Errorf("failed to update metamodel: %w", err)
	}

	oldDir := paths.EntityTypeDirWithPlural(oldPlural)
	newDir := paths.EntityTypeDirWithPlural(newPlural)

	// 2. Rename entity directory
	if _, err := fs.Stat(oldDir); err == nil {
		if err := fs.Rename(oldDir, newDir); err != nil {
			return 0, fmt.Errorf("failed to rename directory: %w", err)
		}
	}

	// 3. Rewrite the type field in every entity file under the new directory.
	count, err := rewriteEntityTypeInDir(fs, newDir, newType)
	if err != nil {
		return count, fmt.Errorf("failed to update entity files: %w", err)
	}

	// 4. Rename template if it exists
	oldTemplatePath := paths.EntityTemplatePath(oldType)
	if _, err := fs.Stat(oldTemplatePath); err == nil {
		newTemplatePath := paths.EntityTemplatePath(newType)
		_ = fs.MkdirAll(paths.EntityTemplatesDir, 0755)
		_ = fs.Rename(oldTemplatePath, newTemplatePath)
	}

	return count, nil
}

// rewriteEntityTypeInDir rewrites the YAML `type:` field in every .md
// file under dir to newType, leaving everything else untouched. Returns
// the number of files rewritten. Missing dirs are treated as no-ops.
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

// rewriteEntityTypeInFile reads a single entity file and replaces its
// YAML frontmatter `type:` field with newType. The surrounding bytes
// (other frontmatter keys, body content, blank lines) are preserved
// verbatim — only the `type:` value changes.
func rewriteEntityTypeInFile(fs storage.FS, path, newType string) error {
	content, err := fs.ReadFile(path)
	if err != nil {
		return err
	}
	updated, ok := replaceYAMLType(string(content), newType)
	if !ok {
		// No type: line found — leave the file untouched.
		return nil
	}
	return fs.WriteFile(path, []byte(updated), 0644)
}

// replaceYAMLType replaces the first occurrence of a top-level
// `type: <something>` line in the YAML frontmatter block (delimited by
// leading and trailing `---` lines). Returns the updated content and
// whether a replacement happened.
func replaceYAMLType(content, newType string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "---" {
			// end of frontmatter
			return content, false
		}
		trimmed := strings.TrimLeft(line, " \t")
		// Only match top-level keys (no indentation).
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
