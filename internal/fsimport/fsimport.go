// Package fsimport copies a filesystem project into a new project directory
// whose data lives in a database backend. It backs `rela db import-fs`.
//
// # Contract
//
// The source project is never modified: it is read through a read-only
// filesystem, fsstore ignores its index cache, and no database is opened in
// it. Everything is built in a staging directory beside the target and
// renamed into place only after the data has been written. Any failure removes
// the staging directory, so a run either produces a complete target or
// nothing.
//
// # What is copied
//
// Entities (every face), relations, attachments, comment threads of imported
// entities, the applied-migration record, an allowlist of runtime state keys
// and the operator-authored project files. The id of an entity is its file
// name, as fsstore reads it. What is not copied is listed in the [Report] with
// a reason; nothing is skipped silently.
//
// # The target judges
//
// Rows are written without a pre-scan that re-implements the target's rules
// (id syntax, case-insensitive uniqueness, attachment size): every row is
// written, every row error is collected, and any error fails the run with all
// of them listed. A pre-scan would drift from the store it imitates.
//
// # Trust boundary
//
// Operator shell, like `rela db migrate` and `rela dev seed`: no ACL check, no
// automations, no validation gate. Rows are attributed to the tool
// [Tool], and the target's audit log gets one record per successful run.
package fsimport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Tool is the attribution tool name stamped on every imported row and on the
// audit record.
const Tool = "fs-import"

// Target is the set of stores one database file holds.
type Target struct {
	Store      store.Store
	State      state.KV
	Comments   comments.Store
	Migrations datamigration.StateStore
}

// Opened is a target database opened by a [Backend].
type Opened struct {
	Target

	// Close flushes and releases the database. After it returns, the
	// database must be complete in its own file: the project directory is
	// renamed next.
	Close func(ctx context.Context) error
}

// Backend opens the target database. It is supplied by the wiring site,
// which is the only place that knows the backend; this package stays
// untagged and is tested against an in-memory backend.
type Backend struct {
	// Open opens the database of the project rooted at root, creating it
	// when it does not exist.
	Open func(ctx context.Context, root string) (*Opened, error)

	// Finish runs once the data is written and the database is closed,
	// before the staging directory is renamed: derived-index
	// reconciliation and any check that the database left no side files.
	// Optional.
	Finish func(ctx context.Context, root string) error

	// Owned lists project-relative paths the backend keeps under the
	// target's .rela directory, such as the database file. A source that
	// already has one is refused: it is not a filesystem project.
	Owned []string
}

// Options configures one run.
type Options struct {
	// Source is the filesystem project to read.
	Source string
	// Target is the directory to create. It must not exist; its parent must.
	Target string
	// Backend opens the target database.
	Backend Backend
	// Principal is the operator running the import. Its user, or
	// "system:fs-import" when empty, is stamped on every row.
	Principal principal.Principal
	// Progress, when set, receives one line per phase.
	Progress io.Writer
}

// Run performs the import. On success the target exists and the returned
// report describes it. On failure nothing is left at the target or in a
// staging directory, and the error lists every problem found; the report is
// returned as far as it got, for the caller to print.
func Run(ctx context.Context, opts Options) (*Report, error) {
	if opts.Backend.Open == nil {
		return nil, errors.New("fsimport: Options.Backend.Open is required")
	}
	paths, err := resolvePaths(opts.Source, opts.Target, opts.Backend.Owned)
	if err != nil {
		return nil, err
	}

	rep := newReport(paths.source, paths.target)
	r := &run{opts: opts, paths: paths, rep: rep}

	if err := r.execute(ctx); err != nil {
		r.cleanup()
		return rep, err
	}
	return rep, nil
}

// run holds the state of one import.
type run struct {
	opts    Options
	paths   resolvedPaths
	rep     *Report
	staging string // set once the staging directory exists
	renamed bool   // set once staging became the target
}

func (r *run) progress(format string, args ...any) {
	if r.opts.Progress != nil {
		_, _ = fmt.Fprintf(r.opts.Progress, format+"\n", args...)
	}
}

func (r *run) execute(ctx context.Context) error {
	// The fingerprint comes first, so opening the source store and every
	// later read fall inside the window it guards.
	before, err := treeFingerprint(r.paths.source)
	if err != nil {
		return err
	}
	err = checkSymlinks(r.paths.source, r.rep)
	if err != nil {
		return err
	}
	src, err := openSource(ctx, r.paths.source)
	if err != nil {
		return err
	}
	defer src.close()

	err = src.checkLocked(ctx, r.rep)
	if err != nil {
		return err
	}

	staging, err := makeStaging(r.paths.target)
	if err != nil {
		return err
	}
	r.staging = staging

	r.progress("Copying project files")
	err = copyProjectFiles(r.paths.source, staging, r.rep)
	if err != nil {
		return err
	}

	r.progress("Writing data")
	err = r.writeData(ctx, src)
	if err != nil {
		return err
	}
	if r.opts.Backend.Finish != nil {
		err = r.opts.Backend.Finish(ctx, staging)
		if err != nil {
			return err
		}
	}

	after, err := treeFingerprint(r.paths.source)
	if err != nil {
		return err
	}
	if after != before {
		return errors.New("the source project changed during the import; run it again " +
			"while nothing else writes to the source")
	}

	// rename(2) replaces an empty directory, so a target created since
	// resolvePaths would vanish silently. Checking again narrows that to the
	// instant between this check and the rename; Go has no portable
	// no-replace rename.
	if _, err := os.Lstat(r.paths.target); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("target %s appeared during the import; it was left alone", r.paths.target)
	}
	if err := os.Rename(staging, r.paths.target); err != nil {
		return fmt.Errorf("move the staged project to %s: %w", r.paths.target, err)
	}
	r.renamed = true

	r.progress("Verifying")
	if err := r.verify(ctx, src); err != nil {
		return err
	}

	return r.recordAudit()
}

// writeData opens the target database in the staging directory, copies every
// row into it and closes it again.
func (r *run) writeData(ctx context.Context, src *source) (err error) {
	db, err := r.open(ctx, r.staging)
	if err != nil {
		return fmt.Errorf("open the new database: %w", err)
	}
	defer func() {
		err = errors.Join(err, db.Close(ctx))
	}()

	ctx = store.WithAttribution(ctx, store.Attribution{User: r.user(), Tool: Tool})
	c := &copier{src: src, dst: db.Target, rep: r.rep}
	return c.copyAll(ctx)
}

// verify reopens the renamed target and compares it with the source.
func (r *run) verify(ctx context.Context, src *source) (err error) {
	db, err := r.open(ctx, r.paths.target)
	if err != nil {
		return fmt.Errorf("reopen the new database: %w", err)
	}
	defer func() {
		err = errors.Join(err, db.Close(ctx))
	}()
	return verifyTarget(ctx, src, db.Target, r.rep)
}

// open calls Backend.Open and rejects an incomplete result, which would
// otherwise panic halfway through a copy.
func (r *run) open(ctx context.Context, root string) (*Opened, error) {
	db, err := r.opts.Backend.Open(ctx, root)
	if err != nil {
		return nil, err
	}
	t := db.Target
	if db.Close == nil || t.Store == nil || t.State == nil || t.Comments == nil || t.Migrations == nil {
		if db.Close != nil {
			_ = db.Close(ctx)
		}
		return nil, errors.New("fsimport: Backend.Open returned an incomplete target")
	}
	return db, nil
}

func (r *run) user() string {
	if u := r.opts.Principal.User; u != "" {
		return u
	}
	return principal.ReservedPrefix + Tool
}

// recordAudit writes the run's single audit record into the target.
//
// Only a successful run has one: a failed run leaves no target, so there is
// no log to write it into, and the source is never written. The audit sink
// logs a failed write and drops it, so the record is read back: an import
// without its record would be an unaudited raw-store write, and that fails
// the run.
func (r *run) recordAudit() error {
	dir := filepath.Join(r.paths.target, ".rela", "audit")
	sink, err := audit.NewFilesystem(dir)
	if err != nil {
		return fmt.Errorf("open the audit log: %w", err)
	}
	p := r.opts.Principal
	if p.User == "" {
		p.User = r.user()
	}
	if p.Tool == "" {
		p.Tool = Tool
	}
	now := time.Now().UTC()
	sink.Record(audit.Record{
		Time:      now,
		Op:        audit.OpFSImport,
		Principal: p,
		Summary:   r.rep.summary(),
	})
	if !auditHasImport(dir, now) {
		return errors.New("the audit record of the import could not be written")
	}
	return nil
}

// auditHasImport reports whether the audit log in dir holds an fs-import
// record stamped at.
func auditHasImport(dir string, at time.Time) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	stamp := []byte(at.Format(time.RFC3339Nano))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err == nil && bytes.Contains(data, stamp) && bytes.Contains(data, []byte(audit.OpFSImport)) {
			return true
		}
	}
	return false
}

// cleanup removes whatever this run created.
func (r *run) cleanup() {
	dir := r.staging
	if r.renamed {
		dir = r.paths.target
	}
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		r.rep.warn("could not remove %s: %v; remove it by hand", dir, err)
	}
}
