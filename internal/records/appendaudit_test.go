package records_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
)

func TestAppend_AuditsOneLinePerBatch(t *testing.T) {
	s, _, sink := newAuditedService(t)
	ctx := principal.With(context.Background(), principal.Principal{User: "alice"})
	ctx = audit.WithTriggeredBy(ctx, "schedule:nightly")
	ctx = audit.WithAssertedBy(ctx, "ops")
	in := []records.Input{
		{Owner: "SITE-1", Fields: map[string]any{"kwh": "1"}},
		{Owner: "SITE-2", Fields: map[string]any{"kwh": "2"}},
		{Owner: "SITE-1", Fields: map[string]any{"kwh": "3"}},
	}
	rows, err := s.Append(ctx, "reading", in)
	require.NoError(t, err)

	lines := sink.Records()
	require.Len(t, lines, 1)
	l := lines[0]
	require.Equal(t, audit.OpAppendRecords, l.Op)
	require.Equal(t, &audit.Subject{Kind: "record", Type: "reading", ID: rows[0].ID}, l.Subject)
	require.Equal(t, "alice", l.Principal.User)
	require.Equal(t, "schedule:nightly", l.TriggeredBy)
	require.Equal(t, "ops", l.AssertedBy)
	require.Equal(t, fmt.Sprintf("appended 3 rows of reading (%s..%s); owners: SITE-1, SITE-2",
		rows[0].ID, rows[2].ID), l.Summary)
}

func TestAppend_AuditsLedgerTransactionsAndReversal(t *testing.T) {
	s, _, sink := newAuditedService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "5"), posting("ACC-2", "T1", "-5")})
	require.NoError(t, err)
	rev, err := s.Reverse(ctx, "posting", "T1")
	require.NoError(t, err)

	lines := sink.Records()
	require.Len(t, lines, 2)
	require.Contains(t, lines[0].Summary, "; transactions: T1")
	require.Equal(t, audit.OpAppendRecords, lines[1].Op)
	require.Equal(t, rev[0].ID, lines[1].Subject.ID)
	require.Contains(t, lines[1].Summary, "reversed transaction T1: appended 2 rows of posting")
	require.Contains(t, lines[1].Summary, "transactions: T1/reversal")
}

func TestAppend_AuditListsManyOwnersAsCount(t *testing.T) {
	s, o, sink := newAuditedService(t)
	var in []records.Input
	for i := range 25 {
		id := fmt.Sprintf("SITE-%02d", i)
		o.types[id] = "site"
		in = append(in, records.Input{Owner: id, Fields: map[string]any{"kwh": "1"}})
	}
	_, err := s.Append(context.Background(), "reading", in)
	require.NoError(t, err)
	require.Contains(t, sink.Records()[0].Summary, "SITE-19 and 5 more")
}

// A refused append is audited as denied when the caller may read the owner,
// and not at all when the owner is hidden or the request is invalid, so the
// log never shows that a hidden owner exists.
func TestAppend_AuditsDenial(t *testing.T) {
	s, o, sink := newAuditedService(t)
	ctx := principal.With(context.Background(), principal.Principal{User: "alice"})
	o.hidden["SITE-2"] = true
	o.readOnly["SITE-3"] = true

	_, err := s.Append(ctx, "reading", []records.Input{{Owner: "SITE-2", Fields: map[string]any{"kwh": "1"}}})
	require.ErrorIs(t, err, records.ErrNotFound)
	_, err = s.Append(ctx, "reading", []records.Input{{Owner: "SITE-1", Fields: map[string]any{"kwh": "x"}}})
	require.ErrorIs(t, err, records.ErrInvalid)
	require.Empty(t, sink.Records())

	_, err = s.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-1", Fields: map[string]any{"kwh": "1"}},
		{Owner: "SITE-3", Fields: map[string]any{"kwh": "1"}},
	})
	require.ErrorIs(t, err, records.ErrForbidden)
	lines := sink.Records()
	require.Len(t, lines, 1)
	require.Equal(t, audit.OpDeniedWrite, lines[0].Op)
	require.Equal(t, &audit.Subject{Kind: "record", Type: "reading"}, lines[0].Subject)
	require.Equal(t, "denied: may not update owner SITE-3 (op=append-records)", lines[0].Summary)
	require.Equal(t, "alice", lines[0].Principal.User)
}

func TestReverse_AuditsDenial(t *testing.T) {
	s, o, sink := newAuditedService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "5"), posting("ACC-2", "T1", "-5")})
	require.NoError(t, err)
	o.readOnly["ACC-2"] = true
	_, err = s.Reverse(ctx, "posting", "T1")
	require.ErrorIs(t, err, records.ErrForbidden)
	lines := sink.Records()
	require.Len(t, lines, 2)
	require.Equal(t, audit.OpDeniedWrite, lines[1].Op)
	require.Equal(t, "denied: may not update owner ACC-2 (reversal of transaction T1) (op=append-records)",
		lines[1].Summary)
}

// Refusals other than a readable owner the caller may not update write no
// line: a hidden owner anywhere in the batch, a hidden leg of a reversal, a
// second reversal and a reused ledger key.
func TestAppend_NoLineForOtherRefusals(t *testing.T) {
	s, o, sink := newAuditedService(t)
	ctx := context.Background()
	_, err := s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "5"), posting("ACC-2", "T1", "-5")})
	require.NoError(t, err)
	_, err = s.Reverse(ctx, "posting", "T1")
	require.NoError(t, err)
	before := len(sink.Records())

	_, err = s.Reverse(ctx, "posting", "T1")
	require.ErrorIs(t, err, records.ErrGroupExists)
	_, err = s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T1", "1"), posting("ACC-2", "T1", "-1")})
	require.ErrorIs(t, err, records.ErrGroupExists)

	o.hidden["SITE-2"] = true
	o.readOnly["SITE-3"] = true
	_, err = s.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-2", Fields: map[string]any{"kwh": "1"}},
		{Owner: "SITE-3", Fields: map[string]any{"kwh": "1"}},
	})
	require.ErrorIs(t, err, records.ErrNotFound)

	_, err = s.Append(ctx, "posting", []records.Input{posting("ACC-1", "T2", "5"), posting("ACC-3", "T2", "-5")})
	require.NoError(t, err)
	before++
	o.hidden["ACC-3"] = true
	_, err = s.Reverse(ctx, "posting", "T2")
	require.ErrorIs(t, err, records.ErrNotFound)

	require.Len(t, sink.Records(), before)
}

func TestAppend_AuditSingularRow(t *testing.T) {
	s, _, sink := newAuditedService(t)
	rows, err := s.Append(context.Background(), "reading", []records.Input{{Owner: "SITE-1"}})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("appended 1 row of reading (%s..%s); owners: SITE-1", rows[0].ID, rows[0].ID),
		sink.Records()[0].Summary)
}
