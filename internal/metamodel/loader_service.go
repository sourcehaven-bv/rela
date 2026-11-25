package metamodel

import (
	"context"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Loader is the top-level service for loading a project's metamodel.
// Implementations cover different sources (filesystem, remote API,
// database) behind a uniform contract.
//
// The returned []string lists the source files that were read when the
// loader has a filesystem-like layout. Sources that don't (remote APIs)
// return nil — the value is informational for consumers like watchers,
// not part of the load contract.
type Loader interface {
	Load(ctx context.Context) (*Metamodel, []string, error)
}

// Subscriber is the optional change-notification interface on a Loader.
// Implementations that can detect external changes to the metamodel data
// satisfy Subscriber; consumers type-assert to subscribe. Backends with no
// change-detection capability (e.g. an embedded byte-slice loader) simply
// don't implement it.
type Subscriber interface {
	// Subscribe starts a watcher that invokes onChange whenever the
	// underlying metamodel data changes. The returned stop function
	// releases all watcher resources.
	Subscribe(ctx context.Context, onChange func()) (stop func(), err error)
}

// FSLoader loads a metamodel from a file on the given filesystem. Deprecated
// syntax in the root file is read through [migration.MigrateBytes] with a
// warning, or refused with a *migration.Error by a [NewStrictFSLoader].
// A migration that spans files is refused in both modes.
type FSLoader struct {
	fs     storage.FS
	path   string
	strict bool
}

// NewFSLoader constructs a filesystem-backed metamodel loader that reads
// deprecated syntax through in-memory migrations and logs a warning.
func NewFSLoader(fs storage.FS, path string) *FSLoader {
	return &FSLoader{fs: fs, path: path}
}

// NewStrictFSLoader constructs a loader that refuses deprecated syntax with
// a *migration.Error, as `rela validate` must.
func NewStrictFSLoader(fs storage.FS, path string) *FSLoader {
	return &FSLoader{fs: fs, path: path, strict: true}
}

var (
	_ Loader     = (*FSLoader)(nil)
	_ Subscriber = (*FSLoader)(nil)
)

// Load migrates or refuses deprecated syntax in the root file, then parses
// the metamodel. Includes are resolved recursively; the returned file list
// covers the main file plus every included file.
func (l *FSLoader) Load(_ context.Context) (*Metamodel, []string, error) {
	data, err := l.fs.ReadFile(l.path)
	if err != nil {
		return nil, nil, err
	}
	migrated, err := l.migrate(data)
	if err != nil {
		return nil, nil, err
	}
	m, files, err := loadData(l.path, migrated.Data, l.fs)
	if err != nil {
		return nil, nil, migrated.Explain(err)
	}
	if err := l.checkIncludesMoved(files); err != nil {
		return nil, nil, err
	}
	return m, files, nil
}

// migrate returns the root content to parse: data itself in strict mode,
// which refuses any blocking detection, and otherwise the in-memory
// migration of data, with its warning logged.
func (l *FSLoader) migrate(data []byte) (migration.Migrated, error) {
	if l.strict {
		if blocking := migration.Blocking(migration.DetectBytes(data, migration.FileTypeMetamodel)); len(blocking) > 0 {
			return migration.Migrated{}, &migration.Error{FilePath: l.path, Detections: blocking}
		}
		return migration.Migrated{Data: data}, nil
	}
	migrated, err := migration.MigrateBytes(l.path, data, migration.FileTypeMetamodel, nil)
	if err != nil {
		return migration.Migrated{}, err
	}
	migration.WarnApplied(l.path, migrated.Applied)
	return migrated, nil
}

// checkIncludesMoved refuses an included file that still declares
// `automations:` (TKT-GWOBH2U). The include parser ignores the key, so
// without this check a project whose automations live in an include would
// load with them silently gone. Only this detection runs on includes: the
// other migrations have never been checked there.
func (l *FSLoader) checkIncludesMoved(files []string) error {
	for _, f := range files { // the root is clean already; re-checking it is harmless
		detections, err := migration.Detect(f, migration.FileTypeMetamodel, l.fs)
		if err != nil {
			return err
		}
		for _, d := range migration.Blocking(detections) {
			if d.Migration != nil && d.Migration.Name() == migration.AutomationsToFlowsName {
				return &migration.Error{FilePath: f, Detections: []migration.DetectionResult{d}}
			}
		}
	}
	return nil
}

// refuseAutomations refuses schema content that still declares
// `automations:` (TKT-GWOBH2U), with the migration's message. [Parse] and
// [Load] call it, so a caller that skips [FSLoader] cannot load a schema
// with its automations silently gone. [LoadWithoutMigrationCheck] does not:
// 'rela migrate' reads the schema with it before the move. The callers run
// it after parseRaw, so data is valid YAML.
func refuseAutomations(path string, data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil { // coverage-ignore: defensive: parseRaw parsed data already
		return humanizeYAMLError(err)
	}
	m := &migration.AutomationsToFlowsMigration{}
	if !m.Detect(&doc) {
		return nil
	}
	return &migration.Error{FilePath: path, Detections: []migration.DetectionResult{
		{Migration: m, Description: m.Description()},
	}}
}

// Subscribe watches the metamodel file and its includes. Because includes
// can change between reloads, the file list is refreshed on each event —
// new includes are added to the watcher so they fire subsequent events.
func (l *FSLoader) Subscribe(_ context.Context, onChange func()) (func(), error) {
	files := l.resolveFiles()

	var watcher *storage.Watcher
	w, err := storage.NewWatcher(storage.WatchConfig{
		Files:      files,
		Extensions: []string{".yaml", ".yml"},
		Debounce:   200 * time.Millisecond,
		SkipHidden: true,
		// coverage-ignore-start: os-fs-event: OnChange fires only on a real debounced file-watcher event, not
		// reachable deterministically in
		// a unit test
		OnChange: func(_ []storage.ChangeEvent) {
			onChange()
			// Include set may have changed — add any new files we
			// aren't already watching. Removal is tolerated silently;
			// the watcher will ignore events for files that no longer
			// exist.
			for _, f := range l.resolveFiles() {
				_ = watcher.AddFile(f)
			}
		},
		// coverage-ignore-end
	})
	if err != nil { // coverage-ignore: defensive: storage.NewWatcher does not fail for a valid config over existing
		// files; the failure path is not reachable in tests
		return nil, err
	}
	watcher = w

	go watcher.Start()
	return watcher.Stop, nil
}

// resolveFiles returns the main metamodel file plus all includes. Uses
// LoadWithoutMigrationCheck so the watch list stays current even when the
// file is pending a migration (Load() would return an error in that case,
// leaving the watcher blind to subsequent fixes). Returns just the main
// file on any error — the watcher will still trigger on changes to it.
func (l *FSLoader) resolveFiles() []string {
	_, files, err := LoadWithoutMigrationCheck(l.path, l.fs)
	if err != nil || len(files) == 0 {
		return []string{l.path}
	}
	return files
}
