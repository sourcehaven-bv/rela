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
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config/configsql"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// configRootFiles are the project-root config files a database carries,
// besides the schema file and its includes.
var configRootFiles = []string{
	"data-entry.yaml",
	"desktop.yaml",
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
		if nameErr := checkConfigName(rel); nameErr != nil {
			return nameErr
		}
		if regErr := requireRegularFile(dir, rel); regErr != nil {
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

// requireRegularFile refuses a stored name that is not a regular file
// under dir, or that reaches one through a symlink at any level. Every
// component is checked with Lstat, not just the leaf: a symlinked acl.yaml,
// or a symlinked directory holding a schema include, could point at
// .rela/secrets.yaml or outside the project, and following it would bake
// that file into a database meant to be shipped. Refused rather than
// skipped, because silently dropping a config file the project relies on
// would ship a database that boots differently from the directory it came
// from.
func requireRegularFile(dir, rel string) error {
	current := dir
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("refusing to store %s: %s is a symlink (symlinks are not followed)",
				filepath.Join(dir, filepath.FromSlash(rel)), current)
		case i == len(parts)-1 && !info.Mode().IsRegular():
			return fmt.Errorf("refusing to store %s: not a regular file", current)
		}
	}
	return nil
}

// checkConfigName refuses a name that is not one a project's config can
// have: the schema file, a root config file, a file under a config
// directory, or a schema include (a .yaml file outside the data
// directories). No segment may be hidden, which keeps .rela/ (secrets,
// mail settings), .git/ and the like out in both directions.
//
// It runs when config is collected AND when it is dumped. A database is a
// file someone can hand over, so its names are checked again before any
// is written: a crafted row named .git/config would otherwise run code on
// the next git command in the target directory.
func checkConfigName(name string) error {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return fmt.Errorf("refusing config path %q: hidden files and directories are not config", name)
		}
	}
	switch {
	case name == project.SchemaFile, slices.Contains(configRootFiles, name):
		return nil
	case len(parts) > 1 && slices.Contains(configDirs, parts[0]):
		return nil
	case slices.Contains(markdownDataDirs, parts[0]):
		return fmt.Errorf("refusing config path %q: %s/ holds data, not config", name, parts[0])
	case strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml"):
		return nil // a schema include
	default:
		return fmt.Errorf("refusing config path %q: not a config file a project can have", name)
	}
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

// ConfigImportOptions tunes [StoreProjectConfig] and [LoadProjectConfig].
type ConfigImportOptions struct {
	// Source names where the files came from. It is used for the audit
	// record only.
	Source string
	// Audit receives the run's single audit record.
	//
	// Nil: rejected. Storing config bypasses the entitymanager, so this
	// record is the only trace of it in the audit log.
	Audit audit.Audit
}

// LoadProjectConfig replaces the config the project's database carries
// with the config collected from dir, and returns the stored paths. An
// empty opts.Source defaults to dir.
//
// It opens the database, so it fails while another process (a server or
// the desktop app) has the project open. A running instance would not see
// the new config until it reloads anyway.
func LoadProjectConfig(
	ctx context.Context, fsys storage.FS, paths *project.Context, dir string, opts ConfigImportOptions,
) ([]string, error) {
	if opts.Audit == nil {
		return nil, errors.New("appbuild: LoadProjectConfig requires an audit sink")
	}
	files, err := CollectProjectConfig(fsys, dir)
	if err != nil {
		return nil, err
	}
	if opts.Source == "" {
		opts.Source = dir
	}
	return StoreProjectConfig(ctx, paths, files, opts)
}

// StoreProjectConfig replaces the config the project's database carries
// with files, as collected by [CollectProjectConfig], and returns the stored
// paths. Collecting first lets a caller validate the config before it
// changes anything else.
//
// It refuses a project that keeps its data in markdown files: creating its
// database would switch it to an empty one. Import the data first.
//
// Like [ImportMarkdownData] it is an operator-shell raw write, so every run
// that reaches the database leaves one audit record. The record carries the
// source and the file count, never file names or contents.
func StoreProjectConfig(
	ctx context.Context, paths *project.Context, files map[string][]byte, opts ConfigImportOptions,
) ([]string, error) {
	if opts.Audit == nil {
		return nil, errors.New("appbuild: StoreProjectConfig requires an audit sink")
	}
	if KeepsMarkdownData(paths) {
		return nil, errors.New("the project keeps its data in markdown files and has no database; " +
			"import the data first (rela db load --data) so the database does not open empty")
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
	err = loader.Replace(ctx, files)
	recordConfigImport(ctx, opts, len(files), err)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// recordConfigImport writes the audit record of one config import. The
// replace is one transaction, so a failed run wrote nothing. The principal
// is the caller's, as for [ImportMarkdownData]; an unstamped ctx shows up as
// unknown rather than as an invented identity.
func recordConfigImport(ctx context.Context, opts ConfigImportOptions, count int, err error) {
	source := opts.Source
	if source == "" {
		source = "an unnamed source"
	}
	summary := fmt.Sprintf("config import from %s: %d files", source, count)
	if err != nil {
		summary = fmt.Sprintf("config import from %s FAILED, nothing written: %s", source, err)
	}
	opts.Audit.Record(audit.Record{
		Time:      time.Now().UTC(),
		Op:        audit.OpConfigImport,
		Principal: principal.From(ctx),
		Summary:   summary,
	})
}

// PutProjectConfigFile stores one config file in the project's database,
// keeping the others. The database must already exist.
func PutProjectConfigFile(ctx context.Context, paths *project.Context, name string, content []byte) error {
	db, err := openExistingDatabase(ctx, paths)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	loader, err := configsql.New(db.DB())
	if err != nil {
		return err
	}
	return loader.Put(ctx, name, content)
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
	db, err := openExistingDatabase(ctx, paths)
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
	if err := checkConfigName(name); err != nil {
		return "", err
	}
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
