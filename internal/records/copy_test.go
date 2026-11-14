package records_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func fileStore(t *testing.T) *filerecords.Store {
	t.Helper()
	s, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// appendLog keeps every append request it is handed.
type appendLog struct{ reqs []records.AppendRequest }

func (a *appendLog) Append(_ context.Context, r records.AppendRequest) error {
	a.reqs = append(a.reqs, r)
	return nil
}

// seed stores n rows of type "reading" for owner A, every other row in
// ledger group g<i/2>, plus one row for owner B and one of type "other".
func seed(t *testing.T, s records.Store, n int) {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var rows []records.Record
	for i := range n {
		ts := base.Add(time.Duration(i) * time.Minute)
		rows = append(rows, records.Record{
			ID: fmt.Sprintf("A%05d", i), Type: "reading", Owner: "A", TS: ts, RecordedAt: ts,
			Group: fmt.Sprintf("g%d", i/2), Fields: map[string]any{"v": int64(i)},
		})
	}
	rows = append(rows,
		records.Record{ID: "B1", Type: "reading", Owner: "B", TS: base, RecordedAt: base, Fields: map[string]any{}},
		records.Record{ID: "O1", Type: "other", Owner: "A", TS: base, RecordedAt: base, Fields: map[string]any{}},
	)
	for start := 0; start < len(rows); start += records.MaxBatch {
		batch := rows[start:min(start+records.MaxBatch, len(rows))]
		if err := s.Append(context.Background(), records.AppendRequest{Records: batch}); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEach pages through every owner of the named types only.
func TestEach(t *testing.T) {
	src := fileStore(t)
	n := records.MaxPage + 5 // more than one page for owner A
	seed(t, src, n)
	var ids []string
	if err := records.Each(context.Background(), src, []string{"reading"}, func(r records.Record) error {
		ids = append(ids, r.ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(ids) != n+1 {
		t.Fatalf("visited %d rows, want %d", len(ids), n+1)
	}
	if ids[0] != "A00000" || ids[n-1] != fmt.Sprintf("A%05d", n-1) || ids[n] != "B1" {
		t.Fatalf("order: first %s, last of A %s, then %s", ids[0], ids[n-1], ids[n])
	}
}

// TestCopy copies rows unchanged in batches, and claims each group once
// only when asked.
func TestCopy(t *testing.T) {
	src := fileStore(t)
	n := records.MaxBatch + 3
	seed(t, src, n)
	types := []string{"reading", "other"}

	t.Run("claims", func(t *testing.T) {
		dst := &appendLog{}
		got, err := records.Copy(context.Background(), src, dst, types, records.CopyOptions{ClaimGroups: true})
		if err != nil {
			t.Fatal(err)
		}
		if got != n+2 {
			t.Fatalf("copied %d, want %d", got, n+2)
		}
		if len(dst.reqs) != 2 {
			t.Fatalf("%d appends, want 2", len(dst.reqs))
		}
		claims := map[records.GroupKey]int{}
		for _, r := range dst.reqs {
			if len(r.Records) > records.MaxBatch {
				t.Fatalf("batch of %d rows", len(r.Records))
			}
			for _, g := range r.Groups {
				claims[g]++
			}
		}
		if len(claims) != (n+1)/2 {
			t.Fatalf("%d groups claimed, want %d", len(claims), (n+1)/2)
		}
		for g, c := range claims {
			if c != 1 {
				t.Fatalf("group %v claimed %d times", g, c)
			}
		}
	})

	t.Run("no claims, into a file store", func(t *testing.T) {
		dst := fileStore(t)
		if _, err := records.Copy(context.Background(), src, dst, types, records.CopyOptions{}); err != nil {
			t.Fatal(err)
		}
		var want, got []string
		collect := func(out *[]string) func(records.Record) error {
			return func(r records.Record) error {
				line, err := records.EncodeRecord(r)
				*out = append(*out, string(line))
				return err
			}
		}
		if err := records.Each(context.Background(), src, types, collect(&want)); err != nil {
			t.Fatal(err)
		}
		if err := records.Each(context.Background(), dst, types, collect(&got)); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatal("the copy differs from the source")
		}
	})
}
