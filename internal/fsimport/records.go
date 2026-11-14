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

// copyRecords copies every source row: the rows of the types the source
// schema declares and of any other type the source holds rows of. A row of
// an undeclared type is copied as stored; there is no schema to check its
// fields against. Ids, times, attribution and ledger groups are kept, and
// each group is claimed in the target so its transaction id cannot be used
// again.
func (c *copier) copyRecords(ctx context.Context) error {
	types, err := records.AllTypes(ctx, c.src.records, c.src.meta.RecordKeys())
	if err != nil {
		c.rep.fail("read record types: %v", err)
		return nil
	}
	if len(types) == 0 {
		return nil
	}
	if c.dst.Records == nil {
		n := 0
		if readErr := records.Each(ctx, c.src.records, types, func(records.Record) error {
			n++
			return nil
		}); readErr != nil {
			c.rep.fail("read record rows: %v", readErr)
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

// records checks that every source row, of a declared type or not, is in
// the target, unchanged. The target may hold rows of its own (a forced
// import).
func (v verifier) records(ctx context.Context) {
	if v.dst.Records == nil {
		return
	}
	types, err := records.AllTypes(ctx, v.src.records, v.src.meta.RecordKeys())
	if err != nil {
		v.rep.fail("verify: read source record types: %v", err)
		return
	}
	if len(types) == 0 {
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

// records accounts for the files below records/. Every record file was
// read by the copy, whether its type is declared or not, and the copy fails
// on one it cannot parse.
func (w *fileWalk) records() error {
	return w.walk(recordsDir, func(p string, _ fs.DirEntry) {
		parts := strings.Split(p, "/")
		if len(parts) != 4 || !strings.HasSuffix(parts[3], ".jsonl") {
			w.rep.skip(p, "not a record file (records/<type>/<owner>/<month>.jsonl)")
		}
	})
}
