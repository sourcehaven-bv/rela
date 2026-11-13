package records_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const schema = `
version: "1"
entities:
  account:
    label: Account
    id_prefix: "ACC-"
    properties:
      locked_until: {type: date}
  site:
    label: Site
    id_prefix: "SITE-"
    properties:
      name: {type: string}
records:
  posting:
    owner: account
    ledger:
      amount: amount
      group_by: txn
      commodity: currency
      closed_before: locked_until
    fields:
      amount: {type: decimal, scale: 2, required: true}
      txn: {type: string, required: true}
      currency: {type: string, required: true}
      memo: {type: string}
  reading:
    owner: site
    retention: 30d
    fields:
      kwh: {type: decimal}
      count: {type: integer}
      ok: {type: boolean}
`

var t0 = time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)

// owners is a fake gate: hidden ids are not readable, readOnly ids are
// readable but not writable.
type owners struct {
	types    map[string]string
	props    map[string]map[string]any
	hidden   map[string]bool
	readOnly map[string]bool
}

func (o *owners) Readable(_ context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		if _, ok := o.types[id]; ok && !o.hidden[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (o *owners) Writable(_ context.Context, id string) (records.Owner, error) {
	t, ok := o.types[id]
	if !ok || o.hidden[id] {
		return records.Owner{}, records.ErrNotFound
	}
	if o.readOnly[id] {
		return records.Owner{}, records.ErrForbidden
	}
	return records.Owner{ID: id, Type: t, Faces: []map[string]any{o.props[id]}}, nil
}

func newService(t *testing.T) (*records.Service, *owners) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	o := &owners{
		types: map[string]string{"ACC-1": "account", "ACC-2": "account", "ACC-3": "account",
			"SITE-1": "site", "SITE-2": "site", "SITE-3": "site"},
		props:    map[string]map[string]any{},
		hidden:   map[string]bool{},
		readOnly: map[string]bool{},
	}
	s, err := records.NewService(records.Deps{Store: st, Owners: o, Meta: meta, Now: func() time.Time { return t0 }})
	require.NoError(t, err)
	return s, o
}

func TestNewServiceRejectsNil(t *testing.T) {
	_, err := records.NewService(records.Deps{})
	require.Error(t, err)
}

func TestAppend_NormalizesAndLists(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	got, err := s.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-1", TS: t0.Add(-time.Hour), Fields: map[string]any{"kwh": 1.5, "count": float64(3), "ok": "true"}},
		{Owner: "SITE-1", Fields: map[string]any{"kwh": "2"}},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, t0, got[1].TS, "a zero TS is now")
	page, err := s.List(ctx, "reading", "SITE-1", records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 2)
	require.Equal(t, map[string]any{"kwh": "1.5", "count": int64(3), "ok": true}, page.Records[0].Fields)
}

func TestAppend_Refuses(t *testing.T) {
	s, o := newService(t)
	o.hidden["SITE-2"] = true
	o.readOnly["SITE-3"] = true
	ctx := context.Background()
	cases := map[string]struct {
		typ  string
		in   records.Input
		want error
	}{
		"unknown type":   {"nope", records.Input{Owner: "SITE-1"}, records.ErrNotFound},
		"unknown field":  {"reading", records.Input{Owner: "SITE-1", Fields: map[string]any{"x": 1}}, records.ErrInvalid},
		"bad integer":    {"reading", records.Input{Owner: "SITE-1", Fields: map[string]any{"count": 1.5}}, records.ErrInvalid},
		"hidden owner":   {"reading", records.Input{Owner: "SITE-2"}, records.ErrNotFound},
		"read-only":      {"reading", records.Input{Owner: "SITE-3"}, records.ErrForbidden},
		"wrong type":     {"reading", records.Input{Owner: "ACC-1"}, records.ErrInvalid},
		"missing owner":  {"reading", records.Input{}, records.ErrInvalid},
		"missing amount": {"posting", records.Input{Owner: "ACC-1", Fields: map[string]any{"txn": "t"}}, records.ErrInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := s.Append(ctx, tc.typ, []records.Input{tc.in})
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func posting(owner, txn, amount string) records.Input {
	return records.Input{Owner: owner, TS: t0, Fields: map[string]any{"txn": txn, "amount": amount, "currency": "EUR"}}
}

func TestLedger_Balance(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "10.00"), posting("ACC-2", "T1", "-9.99")})
	require.ErrorIs(t, err, records.ErrUnbalanced)
	usd := posting("ACC-2", "T1", "-10.00")
	usd.Fields["currency"] = "USD"
	_, err = s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "10.00"), usd})
	require.ErrorIs(t, err, records.ErrUnbalanced, "each commodity balances on its own")

	got, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "10"), posting("ACC-2", "T1", "-10")})
	require.NoError(t, err)
	require.Equal(t, "T1", got[0].Group)
	require.Equal(t, "10", got[0].Fields["amount"], "a decimal keeps the form it was written in")

	_, err = s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "1"), posting("ACC-2", "T1", "-1")})
	require.ErrorIs(t, err, records.ErrGroupExists, "a transaction id is used once")
	_, err = s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T2/reversal", "1"), posting("ACC-2", "T2/reversal", "-1")})
	require.ErrorIs(t, err, records.ErrInvalid, "the reversal suffix is reserved")
}

func TestLedger_ReverseOnce(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "10.50"), posting("ACC-2", "T1", "-10.50")})
	require.NoError(t, err)
	rev, err := s.Reverse(ctx, "posting", "T1")
	require.NoError(t, err)
	require.Len(t, rev, 2)
	for _, r := range rev {
		require.Equal(t, "T1/reversal", r.Group)
		require.Equal(t, "T1", r.Reverses)
	}
	amounts := map[string]any{}
	for _, r := range rev {
		amounts[r.Owner] = r.Fields["amount"]
	}
	require.Equal(t, map[string]any{"ACC-1": "-10.50", "ACC-2": "10.50"}, amounts)
	pts, err := s.Aggregate(ctx, records.AggregateQuery{Type: "posting", Field: "amount", PerOwner: true})
	require.NoError(t, err)
	for _, p := range pts {
		require.Equal(t, "0.00", p.Sum, p.Owner)
	}
	_, err = s.Reverse(ctx, "posting", "T1")
	require.ErrorIs(t, err, records.ErrGroupExists, "a transaction is reversed at most once")
	_, err = s.Reverse(ctx, "posting", "NOPE")
	require.ErrorIs(t, err, records.ErrNotFound)
}

func TestLedger_ClosedPeriod(t *testing.T) {
	s, o := newService(t)
	ctx := context.Background()
	o.props["ACC-1"] = map[string]any{"locked_until": "2026-03-30"}
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "1"), posting("ACC-2", "T1", "-1")})
	require.ErrorIs(t, err, records.ErrClosed, "a row dated on the lock date is refused")
	in := []records.Input{posting("ACC-1", "T2", "1"), posting("ACC-2", "T2", "-1")}
	in[0].TS, in[1].TS = t0.AddDate(0, 0, 1), t0.AddDate(0, 0, 1)
	_, err = s.Append(ctx, "posting", in)
	require.NoError(t, err)
	o.props["ACC-1"] = map[string]any{"locked_until": "garbage"}
	_, err = s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T3", "1"), posting("ACC-2", "T3", "-1")})
	require.ErrorIs(t, err, records.ErrClosed, "an unreadable lock fails closed")
}

// A transaction with one hidden leg is not found: the visible legs would
// otherwise reveal the hidden leg's amount as what they are off by.
func TestTransaction_HiddenLegHidesTransaction(t *testing.T) {
	s, o := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "5"), posting("ACC-2", "T1", "-5")})
	require.NoError(t, err)
	rows, err := s.Transaction(ctx, "posting", "T1")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	o.hidden["ACC-2"] = true
	_, err = s.Transaction(ctx, "posting", "T1")
	require.ErrorIs(t, err, records.ErrNotFound)
	_, err = s.Reverse(ctx, "posting", "T1")
	require.ErrorIs(t, err, records.ErrNotFound)
}

func TestLedger_EmptyGroupRefused(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	_, err := s.Transaction(ctx, "posting", "")
	require.ErrorIs(t, err, records.ErrInvalid)
	_, err = s.Reverse(ctx, "posting", "")
	require.ErrorIs(t, err, records.ErrInvalid)
}

func TestLedger_LegsShareTS(t *testing.T) {
	s, _ := newService(t)
	a, b := posting("ACC-1", "T1", "5"), posting("ACC-2", "T1", "-5")
	b.TS = a.TS.Add(time.Hour)
	_, err := s.Append(context.Background(), "posting", []records.Input{a, b})
	require.ErrorIs(t, err, records.ErrInvalid)
}

// A batch lists in the order it was sent, even when every row has one ts.
func TestAppend_BatchKeepsOrder(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	in := make([]records.Input, 20)
	for i := range in {
		in[i] = records.Input{Owner: "SITE-1", TS: t0, Fields: map[string]any{"kwh": strconv.Itoa(i)}}
	}
	_, err := s.Append(ctx, "reading", in)
	require.NoError(t, err)
	page, err := s.List(ctx, "reading", "SITE-1", records.ListQuery{})
	require.NoError(t, err)
	for i, r := range page.Records {
		require.Equal(t, strconv.Itoa(i), r.Fields["kwh"])
	}
}

func TestList_HiddenOwnerIsNotFound(t *testing.T) {
	s, o := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "reading", []records.Input{{Owner: "SITE-2", Fields: map[string]any{"kwh": "1"}}})
	require.NoError(t, err)
	o.hidden["SITE-2"] = true
	_, err = s.List(ctx, "reading", "SITE-2", records.ListQuery{})
	require.ErrorIs(t, err, records.ErrNotFound)
	_, err = s.List(ctx, "reading", "SITE-9", records.ListQuery{})
	require.ErrorIs(t, err, records.ErrNotFound, "a hidden owner reads as one that does not exist")
}

// A hidden owner's values never reach an aggregate, in the sum, the count,
// the min/max or the latest value.
func TestRecords_AggregateExcludesHiddenOwner(t *testing.T) {
	s, o := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-1", TS: t0, Fields: map[string]any{"kwh": "1.0"}},
		{Owner: "SITE-2", TS: t0.Add(time.Hour), Fields: map[string]any{"kwh": "100.0"}},
	})
	require.NoError(t, err)
	o.hidden["SITE-2"] = true
	for _, q := range []records.AggregateQuery{
		{Type: "reading", Field: "kwh"},
		{Type: "reading", Field: "kwh", Owners: []string{"SITE-1", "SITE-2"}},
	} {
		pts, aerr := s.Aggregate(ctx, q)
		require.NoError(t, aerr)
		require.Len(t, pts, 1)
		require.Equal(t, records.Point{Count: 1, Sum: "1.0", Min: "1.0", Max: "1.0", Avg: "1.000", Latest: "1.0"}, pts[0])
	}
	_, err = s.Aggregate(ctx, records.AggregateQuery{Type: "reading", Owners: []string{"SITE-2"}})
	require.ErrorIs(t, err, records.ErrNotFound)
}

func TestAggregate_Buckets(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-1", TS: t0, Fields: map[string]any{"kwh": "1.25"}},
		{Owner: "SITE-1", TS: t0.Add(time.Hour), Fields: map[string]any{"kwh": "2"}},
		{Owner: "SITE-1", TS: t0.AddDate(0, 0, 1), Fields: map[string]any{}},
		{Owner: "SITE-3", TS: t0, Fields: map[string]any{"kwh": "3"}},
	})
	require.NoError(t, err)
	pts, err := s.Aggregate(ctx, records.AggregateQuery{Type: "reading", Field: "kwh", Bucket: records.BucketDay,
		Owners: []string{"SITE-1"}})
	require.NoError(t, err)
	require.Len(t, pts, 2)
	require.Equal(t, time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC), pts[0].Start)
	require.Equal(t, records.Point{Start: pts[0].Start, Count: 2, Sum: "3.25", Min: "1.25", Max: "2.00",
		Avg: "1.6250", Latest: "2.00"}, pts[0])
	require.Equal(t, int64(1), pts[1].Count)
	require.Empty(t, pts[1].Sum, "a row with no value counts but adds nothing")

	pts, err = s.Aggregate(ctx, records.AggregateQuery{Type: "reading", PerOwner: true})
	require.NoError(t, err)
	require.Equal(t, []records.Point{{Owner: "SITE-1", Count: 3}, {Owner: "SITE-3", Count: 1}}, pts)

	_, err = s.Aggregate(ctx, records.AggregateQuery{Type: "reading", Field: "ok"})
	require.ErrorIs(t, err, records.ErrInvalid)
}

func TestBucketWeekStartsMonday(t *testing.T) {
	sunday := time.Date(2026, 4, 5, 23, 0, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC), records.BucketWeek.Start(sunday))
	_, err := records.ParseBucket("year")
	require.ErrorIs(t, err, records.ErrInvalid)
}

func TestExpire(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-1", TS: t0.AddDate(0, 0, -31)},
		{Owner: "SITE-1", TS: t0},
	})
	require.NoError(t, err)
	n, err := s.Expire(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	page, err := s.List(ctx, "reading", "SITE-1", records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)
}

func TestTypes(t *testing.T) {
	s, _ := newService(t)
	require.Len(t, s.Types(), 2)
	ts := s.TypesFor("site")
	require.Len(t, ts, 1)
	require.Equal(t, "reading", ts[0].Key)
	require.Equal(t, "reading", ts[0].Label)
}
