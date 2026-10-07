package fsimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// CopyOptions configures [Copy].
type CopyOptions struct {
	// Source is the filesystem project to read.
	Source string
	// Meta, when set, is the schema to read the source's data by. Nil reads
	// the source's own schema file.
	Meta *metamodel.Metamodel
	// Target receives the data. It is already open, and may already hold
	// data of its own.
	Target Target
	// Exclude lists slash-separated paths below Source, matched as
	// prefixes, that the caller writes during the copy: the database of a
	// project that imports its own markdown files. The check that the
	// source did not change skips them.
	Exclude []string
	// Principal is the operator running the import. Its user, or
	// "system:fs-import" when empty, is stamped on every row.
	Principal principal.Principal
	// Progress, when set, receives one line per phase.
	Progress io.Writer
}

// Copy copies the data of the filesystem project at opts.Source into
// opts.Target: entities, relations, attachments, comment threads, the
// applied-migration record and the runtime state keys. It is the data half
// of [Run], for a caller that already has its database open, such as
// `rela db load --data`.
//
// The checks are Run's: the source is read through a read-only filesystem,
// symlinks and encrypted content are refused, every row error is collected,
// every source file is accounted for in the report, and the result is
// verified against the source before Copy returns. A state key or migration
// record the target already holds is kept, and the report says so.
//
// Copy writes no project files and no audit record, and does not
// transaction the write: a caller that needs the copy to be all-or-nothing
// runs it inside its store's transaction, with every Target store bound to
// that transaction. On error the report lists every problem found.
func Copy(ctx context.Context, opts CopyOptions) (*Report, error) {
	t := opts.Target
	if t.Store == nil || t.State == nil || t.Comments == nil || t.Migrations == nil {
		return nil, errors.New("fsimport: CopyOptions.Target is incomplete")
	}
	root, err := filepath.Abs(opts.Source)
	if err != nil {
		return nil, err
	}
	rep := newReport(root, "")
	progress := func(msg string) {
		if opts.Progress != nil {
			_, _ = fmt.Fprintln(opts.Progress, msg)
		}
	}

	before, err := treeFingerprint(root, opts.Exclude...)
	if err != nil {
		return rep, err
	}
	if symErr := checkSymlinks(root, rep); symErr != nil {
		return rep, symErr
	}
	src, err := openSource(ctx, root, opts.Meta)
	if err != nil {
		return rep, err
	}
	defer src.close()
	if lockErr := src.checkLocked(ctx, rep); lockErr != nil {
		return rep, lockErr
	}

	progress("Writing data")
	ctx = store.WithAttribution(ctx, store.Attribution{User: importUser(opts.Principal), Tool: Tool})
	c := &copier{src: src, dst: t, rep: rep, inPlace: true}
	if copyErr := c.copyAll(ctx); copyErr != nil {
		return rep, copyErr
	}

	progress("Verifying")
	if verifyErr := verifyTarget(ctx, src, t, c.imported, rep); verifyErr != nil {
		return rep, verifyErr
	}
	after, err := treeFingerprint(root, opts.Exclude...)
	if err != nil {
		return rep, err
	}
	if after != before {
		return rep, errSourceChanged
	}
	return rep, nil
}

var errSourceChanged = errors.New("the source project changed during the import; run it again " +
	"while nothing else writes to the source")

// importUser is the user stamped on imported rows: the operator's, or a
// system user named after the tool when the import runs without one.
func importUser(p principal.Principal) string {
	if p.User != "" {
		return p.User
	}
	return principal.ReservedPrefix + Tool
}
