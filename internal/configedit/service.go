package configedit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Errors the HTTP layer maps to status codes.
var (
	// ErrConflict: the files changed since the draft was read (409).
	ErrConflict = errors.New("configedit: the configuration changed since it was read")
	// ErrBusy: a migration or cleanup run holds the migration lock (409).
	ErrBusy = errors.New("configedit: a data migration is already running")
	// ErrUnsupported: the project's files use something the Configure space
	// cannot edit safely (includes, anchors).
	ErrUnsupported = errors.New("configedit: this configuration cannot be edited in the app")
	// ErrBadDraft: the draft is malformed (400).
	ErrBadDraft = errors.New("configedit: malformed draft")
	// ErrMigrationNotStarted is wrapped by a Candidate's Migrate when it
	// failed before changing any record, so the save can still be undone.
	ErrMigrationNotStarted = errors.New("configedit: the data migration did not start")
)

// Files is the project directory. WriteFile must replace a file atomically.
type Files interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Remove(path string) error
	MkdirAll(path string, perm os.FileMode) error
}

// Validator checks a pair of files the way the server checks them at start,
// returning the parsed schema.
type Validator interface {
	Validate(schema, dataEntry []byte) (*metamodel.Metamodel, error)
}

// ValidationError lists what a Validator found wrong. Any other error from
// Validate is treated as a failure of the check itself.
type ValidationError struct{ Problems []string }

func (e *ValidationError) Error() string { return strings.Join(e.Problems, "; ") }

// MigrationState is the store's migration record. A new migration may only
// start from the current schema with nothing pending.
type MigrationState interface {
	// Ready reports whether the record allows a new migration. It writes
	// nothing, so a preview can ask.
	Ready(ctx context.Context, current *metamodel.Metamodel) error
	// Adopt is Ready that also records a compatible change to current, the
	// way `rela migrate` does. A save calls it before writing a migration
	// file, because that file starts from current and the runner resolves
	// it against the record.
	Adopt(ctx context.Context, current *metamodel.Metamodel) error
}

// Activator turns the files on disk into the running server.
type Activator interface {
	// Pause holds the running server still until resume is called: it stops
	// reacting to the configuration files and refuses writes to records.
	// A save writes schema.yaml and data-entry.yaml one after the other, and
	// a reload in between would check one against the other half-written.
	// A write to a record while the candidate migrates the same records
	// would put back a value the migration moved. A save that activates a
	// candidate never calls resume: the paused server is retired.
	//
	// An error means writes already running did not finish in time; the
	// save is refused as busy and nothing was paused.
	Pause(ctx context.Context) (resume func(), err error)
	// Prepare builds a complete server from the files on disk without
	// serving it. A *ValidationError means the files are not acceptable.
	Prepare(ctx context.Context) (Candidate, error)
}

// Candidate is a server built from new files, not yet serving.
type Candidate interface {
	// Migrate runs every pending data migration and returns how many records
	// changed. An error wrapping datamigration.ErrLockHeld or
	// [ErrMigrationNotStarted] means it did not start.
	Migrate(ctx context.Context) (int, error)
	// Activate starts serving the candidate and retires the old server.
	Activate()
	// Discard releases a candidate that will not serve.
	Discard()
}

// Deps are the Service's collaborators, all required.
type Deps struct {
	Files      Files
	Validator  Validator
	Counter    Counter
	Migrations MigrationState
	Activator  Activator
	Audit      audit.Audit
}

// Service edits schema.yaml and data-entry.yaml for the Configure space.
//
// One Service lives for the whole process, across rebuilds of the server, so
// its mutex serializes every save however many times the app is rebuilt.
type Service struct {
	deps Deps
	mu   sync.Mutex
	now  func() time.Time
}

// NewService builds a Service.
//
// Nil: every Deps field is rejected when nil; a save with any of them missing
// would write files it can neither check nor serve.
func NewService(deps Deps) (*Service, error) {
	switch {
	case deps.Files == nil:
		return nil, errors.New("configedit: NewService: Files is required")
	case deps.Validator == nil:
		return nil, errors.New("configedit: NewService: Validator is required")
	case deps.Counter == nil:
		return nil, errors.New("configedit: NewService: Counter is required")
	case deps.Migrations == nil:
		return nil, errors.New("configedit: NewService: Migrations is required")
	case deps.Activator == nil:
		return nil, errors.New("configedit: NewService: Activator is required")
	case deps.Audit == nil:
		return nil, errors.New("configedit: NewService: Audit is required")
	}
	return &Service{deps: deps, now: time.Now}, nil
}

const (
	aclFile       = "acl.yaml"
	migrationsDir = "migrations"
	defaultTitle  = "Configuration change"
	maxTitleLen   = 120
)

// Snapshot is what the Configure space starts from.
type Snapshot struct {
	Version   string `json:"version"`
	Schema    any    `json:"schema"`
	DataEntry any    `json:"data_entry"`
	// Editable lists the key patterns the Configure space may change, so the
	// browser can show what it can edit (see the allowlist).
	Editable map[File][]string `json:"editable"`
}

// Draft is an edited configuration the browser sends back.
type Draft struct {
	BaseVersion string          `json:"base_version"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	DataEntry   json.RawMessage `json:"data_entry,omitempty"`
	Renames     []Rename        `json:"renames,omitempty"`
	Values      []ValueMapping  `json:"value_mappings,omitempty"`
	// MigrationTitle names the migration in the history, when one is needed.
	MigrationTitle string `json:"migration_title,omitempty"`
}

// Result answers a preview or a save.
type Result struct {
	// Version is the files' version after a save, or before a preview.
	Version   string     `json:"version"`
	Problems  []Problem  `json:"problems"`
	Migration *Migration `json:"migration,omitempty"`
	Saved     bool       `json:"saved"`
	// Incomplete reports a save whose migration failed after it started.
	// The new configuration is serving; Retry finishes the migration.
	Incomplete bool `json:"incomplete,omitempty"`
}

// Migration is the data migration a save writes.
type Migration struct {
	File  string `json:"file"`
	Title string `json:"title"`
	Steps []Step `json:"steps"`
}

// Snapshot reads the two files.
func (s *Service) Snapshot() (*Snapshot, error) {
	cur, err := s.read()
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Version: cur.version(), Schema: cur.schemaTree, DataEntry: cur.dataEntryTree, Editable: editable,
	}, nil
}

// Preview checks a draft and says what saving it would do, without writing.
func (s *Service) Preview(ctx context.Context, d *Draft) (*Result, error) {
	cur, err := s.read()
	if err != nil {
		return nil, err
	}
	pl, err := s.prepare(ctx, cur, d)
	if err != nil {
		return nil, err
	}
	return pl.result(cur.version()), nil
}

// Save writes a draft, migrates the records it affects and switches the
// server to it.
//
// Until the migration starts every failure puts the files back as they were,
// and the old server keeps serving. Once it has started there is no going
// back: on the filesystem store a half-run migration cannot be undone, and
// putting the old schema back would leave records that match neither. The
// new configuration then serves, and Retry runs the rest (every step is safe
// to repeat).
func (s *Service) Save(ctx context.Context, d *Draft) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, err := s.read()
	if err != nil {
		return nil, err
	}
	pl, err := s.prepare(ctx, cur, d)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			s.audit(ctx, cur, nil, nil, "refused: the configuration changed since it was read")
		} else {
			s.audit(ctx, cur, nil, nil, "refused: the draft could not be read")
		}
		return nil, err
	}
	res := pl.result(cur.version())
	if len(res.Problems) > 0 {
		s.audit(ctx, cur, nil, pl, fmt.Sprintf("refused: %d problems", len(res.Problems)))
		return res, nil
	}
	if bytes.Equal(pl.schema, cur.schema) && bytes.Equal(pl.dataEntry, cur.dataEntry) {
		res.Saved = true
		return res, nil
	}

	if pl.migration != nil {
		if prob := stateProblem(s.deps.Migrations.Adopt(ctx, pl.before)); prob != nil {
			res.Problems = append(res.Problems, *prob)
			res.Migration = nil
			s.audit(ctx, cur, nil, pl, "refused: the data migration record is not ready")
			return res, nil
		}
	}

	resume, err := s.deps.Activator.Pause(ctx)
	if err != nil {
		s.audit(ctx, cur, nil, pl, "refused: records were being written")
		return nil, fmt.Errorf("%w: %v", ErrBusy, err) //nolint:errorlint // one %w is the category
	}
	w := &writer{files: s.deps.Files}
	restore := func() {
		w.restore()
		resume()
	}
	if err = s.write(w, cur, pl); err != nil {
		restore()
		s.audit(ctx, cur, nil, pl, "refused: the files could not be written")
		return nil, err
	}

	// From here the request no longer decides: a client that disconnects
	// must not stop a migration halfway.
	ctx = context.WithoutCancel(ctx)
	cand, err := s.deps.Activator.Prepare(ctx)
	if err != nil {
		restore()
		return s.prepareFailed(ctx, cur, pl, res, err)
	}

	if pl.migration != nil {
		if err = migrate(ctx, cand, pl, res); err != nil {
			cand.Discard()
			restore()
			s.audit(ctx, cur, nil, pl, "refused: the data migration could not start")
			return nil, err
		}
	}
	cand.Activate()

	res.Saved = true
	res.Version = versionOf(pl.schema, pl.dataEntry)
	summary := "saved"
	if res.Incomplete {
		summary = "saved; migration incomplete"
	}
	s.audit(ctx, cur, pl, pl, summary)
	return res, nil
}

// prepareFailed reports a candidate that could not be built. A
// *ValidationError is the files' fault, so it comes back as problems.
func (s *Service) prepareFailed(
	ctx context.Context, cur *current, pl *prepared, res *Result, err error,
) (*Result, error) {
	var verr *ValidationError
	if errors.As(err, &verr) {
		for _, msg := range verr.Problems {
			res.Problems = append(res.Problems, Problem{Code: "invalid", Message: msg})
		}
		s.audit(ctx, cur, nil, pl, "refused: the server would not start with it")
		return res, nil
	}
	s.audit(ctx, cur, nil, pl, "refused: the server could not be built")
	return nil, err
}

// migrate runs the save's migration on the candidate server. An error means
// the migration did not start, so the caller can still put the old files
// back. A failure after it started only marks res incomplete, because from
// then on the new configuration has to serve.
func migrate(ctx context.Context, cand Candidate, pl *prepared, res *Result) error {
	changed, err := cand.Migrate(ctx)
	switch {
	case errors.Is(err, datamigration.ErrLockHeld):
		return ErrBusy
	case errors.Is(err, ErrMigrationNotStarted):
		return err
	case err != nil:
		slog.Error("configedit: migration failed after it started; serving the new configuration",
			"migration", pl.migration.FileName, "error", err)
		res.Incomplete = true
	default:
		slog.Info("configedit: migration applied", "migration", pl.migration.FileName, "records", changed)
	}
	return nil
}

// stateProblem turns a refusal from the migration record into a problem for
// the user. The record not being ready means "not now", not a failed request.
func stateProblem(err error) *Problem {
	if err == nil {
		return nil
	}
	return &Problem{Code: "migration_state", Message: err.Error()}
}

// Retry finishes a migration a save left incomplete. With nothing left to
// finish it does nothing, so the server is not rebuilt for no reason.
func (s *Service) Retry(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.read()
	if err != nil {
		return err
	}
	if meta, vErr := s.deps.Validator.Validate(cur.schema, cur.dataEntry); vErr == nil &&
		s.deps.Migrations.Ready(ctx, meta) == nil {

		return nil
	}
	ctx = context.WithoutCancel(ctx)
	cand, err := s.deps.Activator.Prepare(ctx)
	if err != nil {
		return err
	}
	if _, err := cand.Migrate(ctx); err != nil {
		cand.Discard()
		s.audit(ctx, cur, nil, nil, "migration retry failed")
		if errors.Is(err, datamigration.ErrLockHeld) {
			return ErrBusy
		}
		return err
	}
	cand.Activate()
	s.audit(ctx, cur, nil, nil, "migration finished by a retry")
	return nil
}

// current is the two files as read, and their trees.
type current struct {
	schema, dataEntry, acl    []byte
	schemaTree, dataEntryTree any
}

func (c *current) version() string { return versionOf(c.schema, c.dataEntry) }

// versionOf identifies a pair of file contents.
func versionOf(schema, dataEntry []byte) string {
	h := sha256.New()
	h.Write(schema)
	h.Write([]byte{0})
	h.Write(dataEntry)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (s *Service) read() (*current, error) {
	c := &current{}
	var err error
	if c.schema, err = s.deps.Files.ReadFile(string(SchemaFile)); err != nil {
		return nil, err
	}
	if c.dataEntry, err = s.deps.Files.ReadFile(string(DataEntryFile)); err != nil {
		return nil, err
	}
	if c.acl, err = s.deps.Files.ReadFile(aclFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if c.schemaTree, err = treeOfFile(SchemaFile, c.schema); err != nil {
		return nil, err
	}
	if c.dataEntryTree, err = treeOfFile(DataEntryFile, c.dataEntry); err != nil {
		return nil, err
	}
	if m, ok := c.schemaTree.(*Map); ok {
		if _, has := m.Get("includes"); has {
			return nil, fmt.Errorf("%w: schema.yaml uses includes", ErrUnsupported)
		}
	}
	return c, nil
}

func treeOfFile(file File, src []byte) (any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrUnsupported, file, err) //nolint:errorlint // one %w is the category
	}
	tree, err := FromYAML(&doc)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrUnsupported, file, err) //nolint:errorlint // one %w is the category
	}
	return tree, nil
}

// prepared is a checked draft, ready to write.
type prepared struct {
	schema, dataEntry []byte
	problems          []Problem
	steps             []Step
	migration         *datamigration.Draft
	before            *metamodel.Metamodel // the schema the migration starts from
	title             string
	refused           []string // locked paths the draft touched, "file:path"
	changed           []string // paths the draft changed, "file:path"
}

func (p *prepared) result(version string) *Result {
	r := &Result{Version: version, Problems: p.problems}
	if r.Problems == nil {
		r.Problems = []Problem{}
	}
	if p.migration != nil {
		r.Migration = &Migration{File: p.migration.FileName, Title: p.title, Steps: p.steps}
	}
	return r
}

// prepare runs every check a save needs, in the order a user can act on:
// what may not be edited, then what the server would reject, then what the
// records need.
func (s *Service) prepare(ctx context.Context, cur *current, d *Draft) (*prepared, error) {
	if d.BaseVersion != cur.version() {
		return nil, ErrConflict
	}
	p := &prepared{schema: cur.schema, dataEntry: cur.dataEntry}

	for _, f := range []struct {
		file   File
		raw    json.RawMessage
		before any
		src    []byte
		out    *[]byte
	}{
		{SchemaFile, d.Schema, cur.schemaTree, cur.schema, &p.schema},
		{DataEntryFile, d.DataEntry, cur.dataEntryTree, cur.dataEntry, &p.dataEntry},
	} {
		after, err := UnmarshalTree(f.raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrBadDraft, f.file, err) //nolint:errorlint // category
		}
		if after == nil {
			continue // not edited
		}
		if _, ok := after.(*Map); !ok {
			return nil, fmt.Errorf("%w: %s must be a mapping", ErrBadDraft, f.file)
		}
		for _, path := range changedPaths(f.before, after, nil) {
			p.changed = append(p.changed, string(f.file)+":"+path)
		}
		for _, v := range CheckEdit(f.file, f.before, after) {
			p.problems = append(p.problems, Problem{
				Code: "locked", File: v.File, Path: v.Path,
				Message: fmt.Sprintf("%s cannot be %s in the app; edit %s directly.", v.Path, v.Change, v.File),
			})
			p.refused = append(p.refused, string(v.File)+":"+v.Path)
		}
		if *f.out, err = Apply(f.src, after); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrBadDraft, f.file, err) //nolint:errorlint // category
		}
	}
	if len(p.problems) > 0 {
		return p, nil
	}

	before, err := s.deps.Validator.Validate(cur.schema, cur.dataEntry)
	if err != nil {
		return nil, fmt.Errorf("%w: the current configuration does not validate: %v", ErrUnsupported, err) //nolint:errorlint // category
	}
	after, err := s.deps.Validator.Validate(p.schema, p.dataEntry)
	if err != nil {
		var verr *ValidationError
		if !errors.As(err, &verr) {
			return nil, err
		}
		for _, msg := range verr.Problems {
			p.problems = append(p.problems, Problem{Code: "invalid", Message: msg})
		}
		return p, nil
	}
	if bytes.Equal(p.schema, cur.schema) {
		return p, nil
	}

	plan, err := planMigration(ctx, before, after, d.Renames, d.Values, s.deps.Counter)
	if err != nil {
		return nil, err
	}
	p.problems = append(p.problems, plan.problems...)
	p.problems = append(p.problems, aclReferences(cur.acl, before, after)...)
	if len(p.problems) > 0 {
		return p, nil
	}

	p.title = strings.TrimSpace(d.MigrationTitle)
	if p.title == "" {
		p.title = defaultTitle
	}
	if len(p.title) > maxTitleLen {
		p.problems = append(p.problems, Problem{
			Code: "invalid", Message: fmt.Sprintf("the migration name is longer than %d characters", maxTitleLen),
		})
		return p, nil
	}
	if p.migration, err = buildMigration(plan, before, after, p.title, s.now()); err != nil {
		return nil, err
	}
	if p.migration == nil {
		return p, nil
	}
	if prob := stateProblem(s.deps.Migrations.Ready(ctx, before)); prob != nil {
		p.problems = append(p.problems, *prob)
		p.migration = nil
		return p, nil
	}
	p.steps = plan.steps
	p.before = before
	return p, nil
}

// write puts the migration file and the changed configuration files on disk.
// Each file is checked against what was read just before it is replaced, so
// an edit made on disk in the meantime is never overwritten.
func (s *Service) write(w *writer, cur *current, p *prepared) error {
	if p.migration != nil {
		if err := s.deps.Files.MkdirAll(migrationsDir, 0o755); err != nil {
			return err
		}
		if err := w.write(path.Join(migrationsDir, p.migration.FileName), nil, p.migration.Content); err != nil {
			return err
		}
	}
	if !bytes.Equal(p.schema, cur.schema) {
		if err := w.write(string(SchemaFile), cur.schema, p.schema); err != nil {
			return err
		}
	}
	if !bytes.Equal(p.dataEntry, cur.dataEntry) {
		if err := w.write(string(DataEntryFile), cur.dataEntry, p.dataEntry); err != nil {
			return err
		}
	}
	return nil
}

// writer remembers what it wrote so it can put it back.
type writer struct {
	files   Files
	written []written
}

type written struct {
	name          string
	before, wrote []byte // before is nil for a new file
}

// write replaces name, whose content must still be expected (nil: must not
// exist).
func (w *writer) write(name string, expected, data []byte) error {
	now, err := w.files.ReadFile(name)
	switch {
	case expected == nil && err == nil:
		return fmt.Errorf("configedit: %s already exists", name)
	case expected == nil && !errors.Is(err, fs.ErrNotExist):
		return err
	case expected != nil && err != nil:
		return err
	case expected != nil && !bytes.Equal(now, expected):
		return ErrConflict
	}
	if err := w.files.WriteFile(name, data, 0o644); err != nil {
		return err
	}
	w.written = append(w.written, written{name: name, before: expected, wrote: data})
	return nil
}

// restore puts back every file written, newest first, as long as it still
// holds what was written: a file someone has edited since is left alone.
func (w *writer) restore() {
	for i := len(w.written) - 1; i >= 0; i-- {
		f := w.written[i]
		now, err := w.files.ReadFile(f.name)
		if err != nil || !bytes.Equal(now, f.wrote) {
			slog.Warn("configedit: not restoring a file that changed after the save wrote it", "file", f.name)
			continue
		}
		if f.before == nil {
			err = w.files.Remove(f.name)
		} else {
			err = w.files.WriteFile(f.name, f.before, 0o644)
		}
		if err != nil {
			slog.Error("configedit: failed to restore a file", "file", f.name, "error", err)
		}
	}
	w.written = nil
}

// audit records a save or a refusal. It names files by hash and keys by
// path, never content.
func (s *Service) audit(ctx context.Context, cur *current, saved, p *prepared, outcome string) {
	var b strings.Builder
	fmt.Fprintf(&b, "configuration %s; schema.yaml %s, data-entry.yaml %s", outcome, short(cur.schema), short(cur.dataEntry))
	if saved != nil {
		fmt.Fprintf(&b, " → schema.yaml %s, data-entry.yaml %s", short(saved.schema), short(saved.dataEntry))
		if saved.migration != nil {
			fmt.Fprintf(&b, "; migration %s", saved.migration.FileName)
		}
	}
	if saved != nil && len(saved.changed) > 0 {
		fmt.Fprintf(&b, "; changed paths: %s", strings.Join(saved.changed, ", "))
	}
	if p != nil && len(p.refused) > 0 {
		fmt.Fprintf(&b, "; refused paths: %s", strings.Join(p.refused, ", "))
	}
	s.deps.Audit.Record(audit.Record{
		Time:      s.now().UTC(),
		Op:        audit.OpConfigEdit,
		Principal: principal.From(ctx),
		Summary:   b.String(),
	})
}

func short(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}
