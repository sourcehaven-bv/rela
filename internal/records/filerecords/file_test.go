package filerecords_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/records/recordstest"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestConformance(t *testing.T) {
	recordstest.RunAll(t, func(t *testing.T) records.Store {
		t.Helper()
		s, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
		require.NoError(t, err)
		return s
	})
}

func TestConformanceOnDisk(t *testing.T) {
	recordstest.RunAll(t, func(t *testing.T) records.Store {
		t.Helper()
		dir := t.TempDir()
		l, err := filelock.New(filepath.Join(dir, "records.lock"))
		require.NoError(t, err)
		s, err := filerecords.New(storage.NewOsFS(), filepath.Join(dir, "records"), l)
		require.NoError(t, err)
		return s
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := filerecords.New(nil, "/r", filelock.ProcessPrivate{})
	require.Error(t, err)
	_, err = filerecords.New(storage.NewMemFS(), "/r", nil)
	require.Error(t, err)
}

// Owner ids with separators or dots stay inside the type directory.
func TestOwnerIDIsEscaped(t *testing.T) {
	fs := storage.NewMemFS()
	s, err := filerecords.New(fs, "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(t, s.Append(ctx, records.AppendRequest{Records: []records.Record{
		recordstest.Row("a", "m", "../x/y", 0, nil),
	}}))
	_, err = fs.Stat("/records/m/..%2Fx%2Fy")
	require.Error(t, err, "the segment is percent-escaped")
	_, err = fs.Stat("/records/m/%2E%2E%2Fx%2Fy/2026-03.jsonl")
	require.NoError(t, err)
	owners, err := s.Owners(ctx, "m")
	require.NoError(t, err)
	require.Equal(t, []string{"../x/y"}, owners)
}

// Owner ids that differ only in case keep separate rows, also on a
// case-folding filesystem such as the macOS default.
func TestOwnerIDCaseIsKept(t *testing.T) {
	dir := t.TempDir()
	l, err := filelock.New(filepath.Join(dir, "records.lock"))
	require.NoError(t, err)
	s, err := filerecords.New(storage.NewOsFS(), filepath.Join(dir, "records"), l)
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(t, s.Append(ctx, records.AppendRequest{Records: []records.Record{
		recordstest.Row("a", "m", "mtr-1", 0, nil),
		recordstest.Row("b", "m", "MTR-1", 0, nil),
	}}))
	for _, owner := range []string{"mtr-1", "MTR-1"} {
		page, err := s.List(ctx, records.ListQuery{Type: "m", Owner: owner, Limit: 10})
		require.NoError(t, err)
		require.Len(t, page.Records, 1, owner)
		require.Equal(t, owner, page.Records[0].Owner)
	}
}
