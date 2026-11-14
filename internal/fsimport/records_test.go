package fsimport_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// recordSource adds a record type to the base project's schema and two
// rows of it in the file record backend.
func recordSource(t *testing.T) (string, []records.Record) {
	t.Helper()
	source := baseProject(t)
	schema, err := os.ReadFile(filepath.Join(source, "schema.yaml"))
	require.NoError(t, err)
	writeTree(t, source, map[string]string{
		"schema.yaml": string(schema) + "records:\n  reading:\n    owner: item\n    fields:\n      v: {type: integer}\n",
	})
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []records.Record{
		{ID: "R1", Type: "reading", Owner: "ITEM-1", TS: ts, RecordedAt: ts, Group: "g", Fields: map[string]any{"v": int64(1)}},
		{ID: "R2", Type: "reading", Owner: "ITEM-1", TS: ts, RecordedAt: ts, Group: "g", Fields: map[string]any{"v": int64(2)}},
	}
	src, err := filerecords.New(storage.NewOsFS(), filepath.Join(source, "records"), filelock.ProcessPrivate{})
	require.NoError(t, err)
	require.NoError(t, src.Append(context.Background(), records.AppendRequest{Records: rows}))
	return source, rows
}

// TestRun_CopiesRecords: every row of a declared record type reaches the
// target unchanged.
func TestRun_CopiesRecords(t *testing.T) {
	source, want := recordSource(t)
	b := newMemBackend(t)
	dst, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	b.target.Records = dst

	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.NoError(t, err, "errors: %v", rep.Errors)
	assert.Equal(t, 2, rep.Records)

	var got []records.Record
	require.NoError(t, records.Each(context.Background(), dst, []string{"reading"}, func(r records.Record) error {
		got = append(got, r)
		return nil
	}))
	assert.Equal(t, want, got)
}

// TestRun_RefusesRecordsWithoutRecordStore: a target that cannot take
// record rows refuses a source that has some, rather than dropping them.
func TestRun_RefusesRecordsWithoutRecordStore(t *testing.T) {
	source, _ := recordSource(t)
	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), newMemBackend(t))
	require.Error(t, err)
	assert.Contains(t, strings.Join(rep.Errors, "\n"), "the source has 2 record row(s)")
}

// TestRun_CopiesUndeclaredRecords: a row of a type the schema no longer
// declares is copied as stored and passes the verify step, and its file is
// not listed as skipped.
func TestRun_CopiesUndeclaredRecords(t *testing.T) {
	source, declared := recordSource(t)
	ts := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	gone := records.Record{ID: "G1", Type: "gone", Owner: "ITEM-1", TS: ts, RecordedAt: ts,
		Fields: map[string]any{"note": "kept as stored"}}
	src, err := filerecords.New(storage.NewOsFS(), filepath.Join(source, "records"), filelock.ProcessPrivate{})
	require.NoError(t, err)
	require.NoError(t, src.Append(context.Background(), records.AppendRequest{Records: []records.Record{gone}}))

	b := newMemBackend(t)
	dst, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	b.target.Records = dst

	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.NoError(t, err, "errors: %v", rep.Errors)
	assert.Equal(t, 3, rep.Records)
	for _, s := range rep.Skipped {
		assert.False(t, strings.HasPrefix(s.Path, "records/"), "record file listed as skipped: %+v", s)
	}

	types, err := records.AllTypes(context.Background(), dst, nil)
	require.NoError(t, err)
	var got []records.Record
	require.NoError(t, records.Each(context.Background(), dst, types, func(r records.Record) error {
		got = append(got, r)
		return nil
	}))
	assert.Equal(t, append([]records.Record{gone}, declared...), got)
}
