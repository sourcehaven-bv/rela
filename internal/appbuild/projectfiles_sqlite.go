//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/config/configsql"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// configRootFiles are the project-root config files a database carries,
// besides the schema file and its includes.
var configRootFiles = []string{
	"data-entry.yaml",
	"acl.yaml",
	"schedules.yaml",
	"mail-templates.yaml",
	"classification.yaml",
}

// configDirs are the project directories a database carries, recursively.
//
// migrations/applied.json is the one file under them that is not config:
// it is the filesystem tier's migration record, and the database keeps its
// own (TKT-XCJ0Y2), so it is skipped.
var configDirs = []string{
	"scripts", "actions", "validations", "migrations", "templates", "custom", "apps",
}

// skippedConfigFiles are paths under configDirs that are state, not config.
var skippedConfigFiles = map[string]bool{
	"migrations/applied.json": true,
}

// CollectProjectConfig reads the operator-authored config of the project
// rooted at dir: the schema file and its includes, the root config files
// that exist, and every regular file under the config directories. Keys
// are slash-separated paths relative to dir.
//
// It reads the directory, not the project's database, so the result is
// what `rela db load` bakes in. Hidden files are skipped; a symlink fails the
// collection (see [requireRegularFile]).
func CollectProjectConfig(fsys storage.FS, dir string) (map[string][]byte, error) {
	schemaPath, _, found := project.SchemaFileAt(dir, fsys)
	if !found {
		return nil, fmt.Errorf("no schema.yaml in %s", dir)
	}
	// LoadWithoutMigrationCheck: carrying a schema that still needs `rela
	// migrate` is the operator's call, and the boot that reads it will say so.
	_, sources, err := metamodel.LoadWithoutMigrationCheck(schemaPath, fsys)
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}

	files := map[string][]byte{}
	add := func(abs string) error {
		rel, err := relativeConfigPath(dir, abs)
		if err != nil {
			return err
		}
		if regErr := requireRegularFile(abs); regErr != nil {
			return regErr
		}
		data, err := fsys.ReadFile(abs)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	}

	for _, src := range sources {
		if err := add(src); err != nil {
			return nil, fmt.Errorf("schema include: %w", err)
		}
	}
	// The main schema file is stored under the canonical name even when the
	// project still uses the legacy metamodel.yaml: once the file is gone
	// from disk, discovery resolves the schema to schema.yaml, and that is
	// the name the database must answer to.
	if name := filepath.Base(schemaPath); name != project.SchemaFile {
		files[project.SchemaFile] = files[name]
		delete(files, name)
	}
	for _, name := range configRootFiles {
		abs := filepath.Join(dir, name)
		if _, err := os.Lstat(abs); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := add(abs); err != nil {
			return nil, err
		}
	}
	for _, sub := range configDirs {
		if err := collectConfigDir(fsys, dir, sub, add); err != nil {
			return nil, err
		}
	}
	for name := range skippedConfigFiles {
		delete(files, name)
	}
	return files, nil
}

// collectConfigDir adds every regular, non-hidden file under dir/sub.
func collectConfigDir(fsys storage.FS, dir, sub string, add func(string) error) error {
	root := filepath.Join(dir, sub)
	if info, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("refusing to store %s: not a directory (symlinks are not followed)", root)
	}
	return fsys.Walk(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && path != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		return add(path) // refuses anything but a regular file
	})
}

// requireRegularFile refuses a path that is not a regular file, a symlink
// above all. Lstat, not Stat: a symlinked acl.yaml or schema include could
// point at .rela/secrets.yaml, and following it would bake a credential
// into a file meant to be shipped. Refused rather than skipped, because
// silently dropping a config file the project relies on would ship a
// database that boots differently from the directory it came from.
func requireRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to store %s: not a regular file (symlinks are not followed)", path)
	}
	return nil
}

// relativeConfigPath turns abs into a slash path under dir, refusing one
// outside it: a schema include that escapes the project cannot be carried.
func relativeConfigPath(dir, abs string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the project directory", abs)
	}
	return filepath.ToSlash(rel), nil
}

// LoadProjectConfig replaces the config the project's database carries
// with the config collected from dir, and returns the stored paths.
//
// It opens the database, so it fails while another process — a server or
// the desktop app — has the project open. A running instance would not see
// the new config until it reloads anyway.
func LoadProjectConfig(ctx context.Context, fsys storage.FS, paths *project.Context, dir string) ([]string, error) {
	files, err := CollectProjectConfig(fsys, dir)
	if err != nil {
		return nil, err
	}
	db, err := openDatabase(ctx, Config{Paths: paths})
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	loader, err := configsql.New(db.DB())
	if err != nil {
		return nil, err
	}
	if err := loader.Replace(ctx, files); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// DumpProjectConfig writes the config the project's database carries into
// dir, and returns the written paths. Without overwrite it refuses before
// writing anything if any target file already exists.
//
// It reads the database only, not the layered view: the point is to get
// back what was baked in, not the disk copy that shadows it.
func DumpProjectConfig(
	ctx context.Context, fsys storage.FS, paths *project.Context, dir string, overwrite bool,
) ([]string, error) {
	db, err := openDatabase(ctx, Config{Paths: paths})
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	loader, err := configsql.New(db.DB())
	if err != nil {
		return nil, err
	}
	names, err := loader.Paths(ctx)
	if err != nil {
		return nil, err
	}

	targets := make([]string, len(names))
	for i, name := range names {
		target, err := dumpTarget(dir, name, overwrite)
		if err != nil {
			return nil, err
		}
		targets[i] = target
	}
	for i, name := range names {
		data, err := loader.Load(ctx, name)
		if err != nil {
			return nil, err
		}
		if err := fsys.MkdirAll(filepath.Dir(targets[i]), 0o755); err != nil {
			return nil, err
		}
		if err := fsys.WriteFile(targets[i], data, 0o644); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// dumpTarget resolves where name is written under dir, refusing a name that
// would leave dir and a path that passes through a symlink.
//
// Names are re-checked here although [configsql.Loader.Put] validated them:
// a database is a file someone can hand over, and one written by anything but
// rela need not have gone through Put. Every component below dir is checked
// with Lstat, the leaf and its parent directories alike, because writing
// through a symlink at any of them would land the file outside dir. That
// refusal holds even with overwrite.
//
// The checks run against the real filesystem rather than a storage.FS: the
// interface has no Lstat, and following symlinks is exactly what must not
// happen here.
func dumpTarget(dir, name string, overwrite bool) (string, error) {
	local := filepath.FromSlash(name)
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("refusing stored config path %q: not a relative path inside the target directory", name)
	}
	target := filepath.Join(dir, local)
	current := dir
	for part := range strings.SplitSeq(local, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return target, nil // nothing below a missing component can be a symlink
		case err != nil:
			return "", err
		case info.Mode()&os.ModeSymlink != 0:
			return "", fmt.Errorf("refusing to write %s: %s is a symlink", target, current)
		}
	}
	if !overwrite {
		return "", fmt.Errorf("%s already exists (use --force to overwrite)", target)
	}
	return target, nil
}
