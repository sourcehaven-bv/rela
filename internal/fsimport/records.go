package fsimport

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
)

// recordsDir is where the file tier keeps its record rows, below the
// project root.
const recordsDir = "records"

// RecordTarget is the record store an import writes to. It reads the rows
// back to verify them.
type RecordTarget interface {
	records.Reader
	records.Appender
}

// errNoRecordTarget fails an import of record rows into a target that has
// no record store to take them.
var errNoRecordTarget = errors.New("this import cannot copy record rows")

// sourceRecords opens the source's file record store. The source is
// read-only and its fingerprint guards the import, so no lock is taken.
func sourceRecords(src *source) (records.Reader, error) {
	return filerecords.New(src.fs, filepath.Join(src.root, recordsDir), noLock{})
}

// copyRecords copies the rows of every record type the source schema
// declares. Ids, times, attribution and ledger groups are kept, and each
// group is claimed in the target so its transaction id cannot be used
// again.
func (c *copier) copyRecords(ctx context.Context) error {
	types := c.src.meta.RecordKeys()
	if len(types) == 0 {
		return nil
	}
	if c.dst.Records == nil {
		n := 0
		if err := records.Each(ctx, c.src.records, types, func(records.Record) error {
			n++
			return nil
		}); err != nil {
			c.rep.fail("read record rows: %v", err)
			return nil
		}
		if n > 0 {
			c.rep.fail("%v: the source has %d record row(s)", errNoRecordTarget, n)
		}
		return nil
	}
	n, err := records.Copy(ctx, c.src.records, c.dst.Records, types, records.CopyOptions{ClaimGroups: true})
	c.rep.Records += n
	if err != nil {
		c.rep.fail("copy record rows: %v", err)
	}
	return nil
}

// records checks that every source row of a declared record type is in the
// target, unchanged. The target may hold rows of its own (a forced import).
func (v verifier) records(ctx context.Context) {
	types := v.src.meta.RecordKeys()
	if len(types) == 0 || v.dst.Records == nil {
		return
	}
	got := map[string]string{}
	if err := records.Each(ctx, v.dst.Records, types, func(rec records.Record) error {
		line, err := recordForm(rec)
		got[rec.ID] = line
		return err
	}); err != nil {
		v.rep.fail("verify: read target record rows: %v", err)
		return
	}
	if err := records.Each(ctx, v.src.records, types, func(rec records.Record) error {
		want, err := recordForm(rec)
		if err != nil {
			return err
		}
		switch line, ok := got[rec.ID]; {
		case !ok:
			v.rep.fail("verify: record %s (%s of %s) is missing from the target", rec.ID, rec.Type, rec.Owner)
		case line != want:
			v.rep.fail("verify: record %s (%s of %s) differs in the target", rec.ID, rec.Type, rec.Owner)
		}
		return nil
	}); err != nil {
		v.rep.fail("verify: read source record rows: %v", err)
	}
}

// recordForm is the JSON line of rec with its times in UTC at the
// microsecond precision the database backends keep.
func recordForm(rec records.Record) (string, error) {
	rec.TS = rec.TS.UTC().Truncate(time.Microsecond)
	rec.RecordedAt = rec.RecordedAt.UTC().Truncate(time.Microsecond)
	b, err := records.EncodeRecord(rec)
	return string(b), err
}

// records accounts for the files below records/. A file of a declared
// record type was read by the copy, which fails on one it cannot parse; a
// file of an undeclared type is listed as not copied.
func (w *fileWalk) records() error {
	declared := w.src.meta.Records
	return w.walk(recordsDir, func(p string, _ fs.DirEntry) {
		parts := strings.Split(p, "/")
		if len(parts) != 4 || !strings.HasSuffix(parts[3], ".jsonl") {
			w.rep.skip(p, "not a record file (records/<type>/<owner>/<month>.jsonl)")
			return
		}
		// A record type key is lower-case letters, digits, - and _, which
		// the file backend keeps unescaped, so the folder is the key.
		if _, ok := declared[parts[1]]; !ok {
			w.rep.skip(p, "record type %q is not declared in the schema", parts[1])
		}
	})
}
