//go:build sqlite

package appbuild_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/records/sqliterecords"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

const recordsSchemaYAML = dataSchemaYAML + `records:
  reading:
    owner: doc
    fields:
      value: {type: integer}
`

var recordTypes = []string{"reading"}

// writeRecordsProject is writeDataProject with a record type and three
// rows of it in the file record backend, two of them in one group, plus a
// row of a type the schema does not declare. The rows are in the order
// records.Each reads them: by owner, then by (TS, ID).
func writeRecordsProject(t *testing.T) (string, []records.Record) {
	t.Helper()
	root := writeDataProject(t)
	writeFile(t, root, "schema.yaml", recordsSchemaYAML)
	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	rows := []records.Record{
		{ID: "R1", Type: "reading", Owner: "DOC-1", TS: ts, RecordedAt: ts, RecordedBy: "ann",
			Group: "tx-1", Fields: map[string]any{"value": int64(5)}},
		{ID: "R3", Type: "reading", Owner: "DOC-1", TS: ts.Add(time.Hour), RecordedAt: ts,
			Fields: map[string]any{"value": int64(7)}},
		{ID: "R2", Type: "reading", Owner: "DOC-2", TS: ts, RecordedAt: ts, RecordedBy: "ann",
			Group: "tx-1", Fields: map[string]any{"value": int64(-5)}},
	}
	fsrec, err := filerecords.New(osFS(), filepath.Join(root, "records"), filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	gone := records.Record{ID: "G1", Type: "gone", Owner: "DOC-1", TS: ts, RecordedAt: ts, Fields: map[string]any{}}
	if err := fsrec.Append(context.Background(), records.AppendRequest{Records: append(rows, gone)}); err != nil {
		t.Fatal(err)
	}
	return root, rows
}

func allRows(t *testing.T, src records.Reader) []records.Record {
	t.Helper()
	var out []records.Record
	if err := records.Each(context.Background(), src, recordTypes, func(r records.Record) error {
		r.TS, r.RecordedAt = r.TS.UTC(), r.RecordedAt.UTC()
		out = append(out, r)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func withRecordDB(t *testing.T, root string, fn func(records.Store)) {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(root, ".rela", "rela.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rs, err := sqliterecords.New(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	fn(rs)
}

// `rela db load --data` copies the record rows of declared types into the
// database with their group claims, and `rela db dump --data` writes them
// back in the file layout, so the round trip keeps every row.
func TestMarkdownData_RecordsRoundTrip(t *testing.T) {
	root, want := writeRecordsProject(t)
	rep, err := importData(t, root, false, audit.NewMemory())
	if err != nil {
		t.Fatalf("import: %v (errors %v)", err, rep.Errors)
	}
	if rep.Records != len(want) {
		t.Fatalf("imported %d record rows, want %d", rep.Records, len(want))
	}
	skippedGone := false
	for _, s := range rep.Skipped {
		skippedGone = skippedGone || strings.HasPrefix(s.Path, "records/gone/")
	}
	if !skippedGone {
		t.Fatalf("the undeclared record type is not listed as skipped: %+v", rep.Skipped)
	}
	withRecordDB(t, root, func(rs records.Store) {
		if got := allRows(t, rs); !reflect.DeepEqual(got, want) {
			t.Fatalf("database rows:\n got  %+v\n want %+v", got, want)
		}
		// The import claimed the group, so its transaction id is taken.
		claimErr := rs.Append(context.Background(), records.AppendRequest{
			Groups: []records.GroupKey{{Type: "reading", Group: "tx-1"}},
		})
		if !errors.Is(claimErr, records.ErrGroupExists) {
			t.Fatalf("claiming tx-1 again: err = %v, want ErrGroupExists", claimErr)
		}
	})

	out := filepath.Join(t.TempDir(), "out")
	sum, err := appbuild.ExportMarkdownData(context.Background(), osFS(), projectAt(t, root), out)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if sum.Records != len(want) {
		t.Fatalf("exported %d record rows, want %d", sum.Records, len(want))
	}
	exported, err := filerecords.New(osFS(), filepath.Join(out, "records"), filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	if got := allRows(t, exported); !reflect.DeepEqual(got, want) {
		t.Fatalf("exported rows:\n got  %+v\n want %+v", got, want)
	}

	writeFile(t, out, "schema.yaml", recordsSchemaYAML)
	if _, err := importData(t, out, false, audit.NewMemory()); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	withRecordDB(t, out, func(rs records.Store) {
		if got := allRows(t, rs); !reflect.DeepEqual(got, want) {
			t.Fatalf("re-imported rows:\n got  %+v\n want %+v", got, want)
		}
	})
}
