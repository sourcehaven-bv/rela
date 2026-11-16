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

// FSLoader loads a metamodel from a file on the given filesystem. It
// performs a migration-detection pre-check: if the file uses deprecated
// syntax the current code can't safely interpret, Load returns a
// *migration.Error telling the user to run `rela migrate`.
type FSLoader struct {
	fs   storage.FS
	path string
}

// NewFSLoader constructs a filesystem-backed metamodel loader.
func NewFSLoader(fs storage.FS, path string) *FSLoader {
	return &FSLoader{fs: fs, path: path}
}

var (
	_ Loader     = (*FSLoader)(nil)
	_ Subscriber = (*FSLoader)(nil)
)

// Load runs migration detection and then parses the metamodel. Includes
// are resolved recursively; the returned file list covers the main file
// plus every included file.
func (l *FSLoader) Load(_ context.Context) (*Metamodel, []string, error) {
	detections, err := migration.Detect(l.path, migration.FileTypeMetamodel, l.fs)
	if err != nil {
		return nil, nil, err
	}
	if blocking := migration.Blocking(detections); len(blocking) > 0 {
		return nil, nil, &migration.Error{
			FilePath:   l.path,
			Detections: blocking,
		}
	}
	m, files, err := Load(l.path, l.fs)
	if err != nil {
		return nil, nil, err
	}
	if err := l.checkIncludesMoved(files); err != nil {
		return nil, nil, err
	}
	return m, files, nil
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
