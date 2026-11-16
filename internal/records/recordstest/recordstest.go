// Package recordstest is the conformance suite every [records.Store]
// passes. A backend's test calls [RunAll] with a factory for an empty
// store.
package recordstest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/records"
)

// T0 is the suite's reference time.
var T0 = time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)

// Factory returns an empty store.
type Factory func(t *testing.T) records.Store

// Row returns a record of typ for owner at T0 plus d, with the given id
// and fields.
func Row(id, typ, owner string, d time.Duration, fields map[string]any) records.Record {
	if fields == nil {
		fields = map[string]any{}
	}
	return records.Record{ID: id, Type: typ, Owner: owner, TS: T0.Add(d), RecordedAt: T0,
		RecordedBy: "alice", Fields: fields}
}

// RunAll runs every conformance test.
func RunAll(t *testing.T, newStore Factory) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s records.Store)
	}{
		{"RoundTrip", testRoundTrip},
		{"OrderAndPaging", testOrderAndPaging},
		{"Filters", testFilters},
		{"Owners", testOwners},
		{"Types", testTypes},
		{"GroupClaim", testGroupClaim},
		{"GroupClaimIsAtomic", testGroupClaimAtomic},
		{"ConcurrentClaim", testConcurrentClaim},
		{"Rekey", testRekey},
		{"Drop", testDrop},
		{"Expire", testExpire},
		{"BadCursor", testBadCursor},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, newStore(t)) })
	}
}

func appendRows(t *testing.T, s records.Store, rows ...records.Record) {
	t.Helper()
	require.NoError(t, s.Append(context.Background(), records.AppendRequest{Records: rows}))
}

func listAll(t *testing.T, s records.Store, q records.ListQuery) []records.Record {
	t.Helper()
	var out []records.Record
	for {
		p, err := s.List(context.Background(), q)
		require.NoError(t, err)
		out = append(out, p.Records...)
		if p.Next == "" {
			return out
		}
		q.After = p.Next
	}
}

func ids(rs []records.Record) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

// fineOffset checks that a backend keeps time below a millisecond.
const fineOffset = 1500 * time.Microsecond

func testRoundTrip(t *testing.T, s records.Store) {
	t.Helper()
	in := records.Record{
		ID: "r1", Type: "meter", Owner: "SITE-1", TS: T0.Add(fineOffset), RecordedAt: T0,
		RecordedBy: "alice", Group: "g", Reverses: "h",
		Fields: map[string]any{"kwh": "12.50", "count": int64(3), "ok": true, "note": "a \"b\" é"},
	}
	appendRows(t, s, in)
	got := listAll(t, s, records.ListQuery{Type: "meter", Owner: "SITE-1"})
	require.Len(t, got, 1)
	require.Equal(t, in, got[0])
	require.Equal(t, time.UTC, got[0].TS.Location())
}

func testOrderAndPaging(t *testing.T, s records.Store) {
	t.Helper()
	// Same TS for b and a: ID breaks the tie. Out of order on input.
	appendRows(t, s,
		Row("c", "m", "O", 2*time.Hour, nil),
		Row("b", "m", "O", time.Hour, nil),
		Row("a", "m", "O", time.Hour, nil),
		Row("d", "m", "O", 24*40*time.Hour, nil), // another month
	)
	p, err := s.List(context.Background(), records.ListQuery{Type: "m", Owner: "O", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, ids(p.Records))
	require.NotEmpty(t, p.Next)
	p, err = s.List(context.Background(), records.ListQuery{Type: "m", Owner: "O", Limit: 2, After: p.Next})
	require.NoError(t, err)
	require.Equal(t, []string{"c", "d"}, ids(p.Records))
	require.Empty(t, p.Next, "a full last page has no next cursor")
	require.Equal(t, []string{"a", "b", "c", "d"}, ids(listAll(t, s, records.ListQuery{Type: "m", Limit: 1})))
}

func testFilters(t *testing.T, s records.Store) {
	t.Helper()
	appendRows(t, s,
		Row("a", "m", "O1", 0, nil),
		Row("b", "m", "O2", time.Hour, nil),
		Row("c", "m", "O1", 2*time.Hour, nil),
		Row("x", "other", "O1", time.Hour, nil),
	)
	g := Row("g1", "m", "O2", 3*time.Hour, nil)
	g.Group = "tx-1"
	appendRows(t, s, g)
	require.Equal(t, []string{"a", "c"}, ids(listAll(t, s, records.ListQuery{Type: "m", Owner: "O1"})))
	require.Equal(t, []string{"b", "c"}, ids(listAll(t, s, records.ListQuery{
		Type: "m", From: T0.Add(time.Hour), To: T0.Add(3 * time.Hour)})), "From is inclusive, To exclusive")
	require.Equal(t, []string{"g1"}, ids(listAll(t, s, records.ListQuery{Type: "m", Group: "tx-1"})))
	require.Empty(t, listAll(t, s, records.ListQuery{Type: "none"}))
}

func testOwners(t *testing.T, s records.Store) {
	t.Helper()
	appendRows(t, s, Row("a", "m", "B", 0, nil), Row("b", "m", "A", 0, nil), Row("c", "m", "A", time.Hour, nil),
		Row("d", "n", "C", 0, nil))
	got, err := s.Owners(context.Background(), "m")
	require.NoError(t, err)
	require.Equal(t, []string{"A", "B"}, got)
	got, err = s.Owners(context.Background(), "none")
	require.NoError(t, err)
	require.Empty(t, got)
}

// testTypes: Types lists every type with rows, sorted, and drops a type
// once its last row is gone.
func testTypes(t *testing.T, s records.Store) {
	t.Helper()
	ctx := context.Background()
	got, err := s.Types(ctx)
	require.NoError(t, err)
	require.Empty(t, got)
	appendRows(t, s, Row("a", "n", "O", 0, nil), Row("b", "m", "O", 0, nil), Row("c", "m", "P", 0, nil),
		Row("d", "x", "Q", 0, nil))
	got, err = s.Types(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"m", "n", "x"}, got)
	require.NoError(t, s.DropOwner(ctx, "Q"))
	_, err = s.Expire(ctx, "n", T0.Add(time.Hour))
	require.NoError(t, err)
	got, err = s.Types(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"m"}, got)
}

func grouped(id, owner, group string) records.Record {
	r := Row(id, "l", owner, 0, map[string]any{"amount": "1"})
	r.Group = group
	return r
}

func testGroupClaim(t *testing.T, s records.Store) {
	t.Helper()
	ctx := context.Background()
	req := records.AppendRequest{
		Records: []records.Record{grouped("a", "O", "tx"), grouped("b", "P", "tx")},
		Groups:  []records.GroupKey{{Type: "l", Group: "tx"}},
	}
	require.NoError(t, s.Append(ctx, req))
	err := s.Append(ctx, records.AppendRequest{
		Records: []records.Record{grouped("c", "O", "tx")},
		Groups:  []records.GroupKey{{Type: "l", Group: "tx"}},
	})
	require.ErrorIs(t, err, records.ErrGroupExists)
	// The same group id under another type is free.
	other := grouped("d", "O", "tx")
	other.Type = "l2"
	require.NoError(t, s.Append(ctx, records.AppendRequest{
		Records: []records.Record{other}, Groups: []records.GroupKey{{Type: "l2", Group: "tx"}},
	}))
	require.Equal(t, []string{"a", "b"}, ids(listAll(t, s, records.ListQuery{Type: "l", Group: "tx"})))
}

// A refused claim writes none of the request's rows, and claims none of
// its other groups.
func testGroupClaimAtomic(t *testing.T, s records.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.Append(ctx, records.AppendRequest{
		Records: []records.Record{grouped("a", "O", "taken")},
		Groups:  []records.GroupKey{{Type: "l", Group: "taken"}},
	}))
	err := s.Append(ctx, records.AppendRequest{
		Records: []records.Record{grouped("b", "O", "fresh"), grouped("c", "O", "taken")},
		Groups:  []records.GroupKey{{Type: "l", Group: "fresh"}, {Type: "l", Group: "taken"}},
	})
	require.ErrorIs(t, err, records.ErrGroupExists)
	require.Equal(t, []string{"a"}, ids(listAll(t, s, records.ListQuery{Type: "l"})))
	require.NoError(t, s.Append(ctx, records.AppendRequest{
		Records: []records.Record{grouped("d", "O", "fresh")},
		Groups:  []records.GroupKey{{Type: "l", Group: "fresh"}},
	}), "the refused append must not have claimed fresh")
}

// Of many appends racing for one group, exactly one wins.
func testConcurrentClaim(t *testing.T, s records.Store) {
	t.Helper()
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			errs[i] = s.Append(context.Background(), records.AppendRequest{
				Records: []records.Record{grouped(fmt.Sprintf("r%d", i), "O", "tx")},
				Groups:  []records.GroupKey{{Type: "l", Group: "tx"}},
			})
		})
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
			continue
		}
		require.ErrorIs(t, err, records.ErrGroupExists)
	}
	require.Equal(t, 1, won)
	require.Len(t, listAll(t, s, records.ListQuery{Type: "l"}), 1)
}

func testRekey(t *testing.T, s records.Store) {
	t.Helper()
	ctx := context.Background()
	appendRows(t, s, Row("a", "m", "OLD", 0, nil), Row("b", "n", "OLD", 24*40*time.Hour, nil),
		Row("c", "m", "NEW", time.Hour, nil), Row("d", "m", "X", 0, nil))
	require.NoError(t, s.RekeyOwner(ctx, "OLD", "NEW"))
	got := listAll(t, s, records.ListQuery{Type: "m", Owner: "NEW"})
	require.Equal(t, []string{"a", "c"}, ids(got))
	require.Equal(t, "NEW", got[0].Owner)
	require.Equal(t, []string{"b"}, ids(listAll(t, s, records.ListQuery{Type: "n", Owner: "NEW"})))
	require.Empty(t, listAll(t, s, records.ListQuery{Type: "m", Owner: "OLD"}))
	require.NoError(t, s.RekeyOwner(ctx, "NONE", "OTHER"))
}

func testDrop(t *testing.T, s records.Store) {
	t.Helper()
	ctx := context.Background()
	appendRows(t, s, Row("a", "m", "O", 0, nil), Row("b", "n", "O", 0, nil), Row("c", "m", "P", 0, nil))
	require.NoError(t, s.Append(ctx, records.AppendRequest{
		Records: []records.Record{grouped("g", "O", "tx")}, Groups: []records.GroupKey{{Type: "l", Group: "tx"}},
	}))
	require.NoError(t, s.DropOwner(ctx, "O"))
	require.Equal(t, []string{"c"}, ids(listAll(t, s, records.ListQuery{Type: "m"})))
	require.Empty(t, listAll(t, s, records.ListQuery{Type: "n"}))
	owners, err := s.Owners(ctx, "m")
	require.NoError(t, err)
	require.Equal(t, []string{"P"}, owners)
	require.NoError(t, s.Append(ctx, records.AppendRequest{
		Records: []records.Record{grouped("g2", "P", "tx")}, Groups: []records.GroupKey{{Type: "l", Group: "tx"}},
	}), "a group no row holds any more is free again")
	require.NoError(t, s.DropOwner(ctx, "NONE"))
}

func testExpire(t *testing.T, s records.Store) {
	t.Helper()
	ctx := context.Background()
	appendRows(t, s, Row("a", "m", "O", 0, nil), Row("b", "m", "P", time.Hour, nil),
		Row("c", "m", "O", 2*time.Hour, nil), Row("d", "n", "O", 0, nil))
	n, err := s.Expire(ctx, "m", T0.Add(2*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
	require.Equal(t, []string{"c"}, ids(listAll(t, s, records.ListQuery{Type: "m"})))
	require.Equal(t, []string{"d"}, ids(listAll(t, s, records.ListQuery{Type: "n"})), "other types stay")
	owners, err := s.Owners(ctx, "m")
	require.NoError(t, err)
	require.Equal(t, []string{"O"}, owners, "an owner with no rows left is not listed")
}

func testBadCursor(t *testing.T, s records.Store) {
	t.Helper()
	_, err := s.List(context.Background(), records.ListQuery{Type: "m", After: "nonsense"})
	require.ErrorIs(t, err, records.ErrInvalid)
}
