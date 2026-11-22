package records_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const mutableSchema = schema + `
  hours:
    owner: site
    mutable: true
    fields:
      hours: {type: decimal, scale: 2, required: true}
      note: {type: string}
`

func newMutableService(t *testing.T) (*records.Service, *owners, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(mutableSchema))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	o := &owners{
		types:    map[string]string{"SITE-1": "site", "SITE-2": "site", "SITE-3": "site"},
		props:    map[string]map[string]any{},
		hidden:   map[string]bool{},
		readOnly: map[string]bool{},
	}
	sink := audit.NewMemory()
	now := t0
	s, err := records.NewService(records.Deps{Store: st, Owners: o, Meta: meta, Audit: sink,
		Now: func() time.Time { now = now.Add(time.Minute); return now }})
	require.NoError(t, err)
	return s, o, sink
}

func withUser(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user})
}

func TestUpdate_ChangesRowAndAudits(t *testing.T) {
	s, _, sink := newMutableService(t)
	ctx := withUser("alice")
	rows, err := s.Append(ctx, "hours", []records.Input{
		{Owner: "SITE-1", TS: t0, Fields: map[string]any{"hours": "7.5", "note": "typo"}},
	})
	require.NoError(t, err)
	id := rows[0].ID
	require.Len(t, sink.Records(), 1, "the append writes one line")
	base := 1

	got, err := s.Update(withUser("bob"), "hours", id, records.Change{
		Revision: 0, TS: t0.Add(24 * time.Hour), Fields: map[string]any{"hours": "8", "note": nil},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"hours": "8"}, got.Fields, "a nil clears note")
	require.Equal(t, int64(1), got.Revision)
	require.Equal(t, "bob", got.UpdatedBy)
	require.Equal(t, "alice", got.RecordedBy, "the append's attribution stays")
	require.True(t, got.TS.Equal(t0.Add(24*time.Hour)))

	page, err := s.List(ctx, "hours", "SITE-1", records.ListQuery{})
	require.NoError(t, err)
	require.Equal(t, []records.Record{got}, page.Records)

	lines := sink.Records()[base:]
	require.Len(t, lines, 1)
	require.Equal(t, audit.OpUpdateRecord, lines[0].Op)
	require.Equal(t, &audit.Subject{Kind: "record", Type: "hours", ID: id}, lines[0].Subject)
	require.Equal(t, "bob", lines[0].Principal.User)
	require.Contains(t, lines[0].Summary, "fields: hours, note")

	_, err = s.Update(ctx, "hours", id, records.Change{Revision: 0, Fields: map[string]any{"hours": "9"}})
	require.ErrorIs(t, err, records.ErrStale)
	_, err = s.Update(ctx, "hours", id, records.Change{Revision: 1, Fields: map[string]any{"hours": nil}})
	require.ErrorIs(t, err, records.ErrInvalid, "a required field cannot be cleared")
	_, err = s.Update(ctx, "hours", id, records.Change{Revision: 1, Fields: map[string]any{"nope": "x"}})
	require.ErrorIs(t, err, records.ErrInvalid)
	require.Len(t, sink.Records(), base+1, "a refused change is not audited")
}

func TestDelete_RemovesRowAndAudits(t *testing.T) {
	s, _, sink := newMutableService(t)
	ctx := withUser("alice")
	rows, err := s.Append(ctx, "hours", []records.Input{{Owner: "SITE-1", Fields: map[string]any{"hours": "1"}}})
	require.NoError(t, err)
	id := rows[0].ID
	require.ErrorIs(t, s.Delete(ctx, "hours", id, 3), records.ErrStale)
	require.NoError(t, s.Delete(ctx, "hours", id, 0))
	page, err := s.List(ctx, "hours", "SITE-1", records.ListQuery{})
	require.NoError(t, err)
	require.Empty(t, page.Records)
	require.ErrorIs(t, s.Delete(ctx, "hours", id, 0), records.ErrNotFound)
	lines := sink.Records()[1:]
	require.Len(t, lines, 1)
	require.Equal(t, audit.OpDeleteRecord, lines[0].Op)
}

func TestChange_Refuses(t *testing.T) {
	s, o, _ := newMutableService(t)
	ctx := withUser("alice")
	rows, err := s.Append(ctx, "hours", []records.Input{
		{Owner: "SITE-2", Fields: map[string]any{"hours": "1"}},
		{Owner: "SITE-3", Fields: map[string]any{"hours": "1"}},
	})
	require.NoError(t, err)
	reading, err := s.Append(ctx, "reading", []records.Input{{Owner: "SITE-1", Fields: map[string]any{"kwh": "1"}}})
	require.NoError(t, err)
	o.hidden["SITE-2"] = true
	o.readOnly["SITE-3"] = true

	cases := map[string]struct {
		typ, id string
		want    error
	}{
		"hidden owner":        {"hours", rows[0].ID, records.ErrNotFound},
		"read-only owner":     {"hours", rows[1].ID, records.ErrForbidden},
		"no such row":         {"hours", "nope", records.ErrNotFound},
		"no id":               {"hours", "", records.ErrInvalid},
		"append-only type":    {"reading", reading[0].ID, records.ErrInvalid},
		"unknown type":        {"nope", rows[1].ID, records.ErrNotFound},
		"row of another type": {"hours", reading[0].ID, records.ErrNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, uerr := s.Update(ctx, tc.typ, tc.id, records.Change{Fields: map[string]any{"hours": "2"}})
			require.ErrorIs(t, uerr, tc.want)
			require.ErrorIs(t, s.Delete(ctx, tc.typ, tc.id, 0), tc.want)
		})
	}
	// A stale revision on a hidden row is still not found: the revision is
	// compared only after access.
	_, err = s.Update(ctx, "hours", rows[0].ID, records.Change{Revision: 9, Fields: map[string]any{"hours": "2"}})
	require.ErrorIs(t, err, records.ErrNotFound)
}

func TestNewService_NeedsAuditAndMutableNeedsMutator(t *testing.T) {
	meta, err := metamodel.Parse([]byte(mutableSchema))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	_, err = records.NewService(records.Deps{Store: st, Owners: &owners{}, Meta: meta})
	require.ErrorContains(t, err, "Audit")
	_, err = records.NewService(records.Deps{Store: appendOnly{st}, Owners: &owners{}, Meta: meta, Audit: audit.Nop{}})
	require.ErrorContains(t, err, "cannot change rows")
}

// appendOnly hides the Mutator capability of the store it wraps.
type appendOnly struct{ records.Store }

func TestMetamodel_LedgerCannotBeMutable(t *testing.T) {
	_, err := metamodel.Parse([]byte(`
version: "1"
entities:
  account: {label: Account, id_prefix: "ACC-", properties: {name: {type: string}}}
records:
  posting:
    owner: account
    mutable: true
    ledger: {amount: amount, group_by: txn}
    fields:
      amount: {type: decimal, required: true}
      txn: {type: string, required: true}
`))
	require.ErrorContains(t, err, "a ledger cannot be mutable")
}

// A field the schema no longer declares does not lock old rows: a change
// drops it.
func TestUpdate_DropsUndeclaredStoredField(t *testing.T) {
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	o := &owners{types: map[string]string{"SITE-1": "site"}, props: map[string]map[string]any{},
		hidden: map[string]bool{}, readOnly: map[string]bool{}}
	service := func(schemaText string) *records.Service {
		meta, perr := metamodel.Parse([]byte(schemaText))
		require.NoError(t, perr)
		s, serr := records.NewService(records.Deps{Store: st, Owners: o, Meta: meta, Audit: audit.NewMemory()})
		require.NoError(t, serr)
		return s
	}
	ctx := withUser("alice")
	rows, err := service(mutableSchema).Append(ctx, "hours", []records.Input{
		{Owner: "SITE-1", Fields: map[string]any{"hours": "1", "note": "old"}},
	})
	require.NoError(t, err)

	withoutNote := strings.Replace(mutableSchema, "      note: {type: string}\n", "", 1)
	got, err := service(withoutNote).Update(ctx, "hours", rows[0].ID,
		records.Change{Revision: rows[0].Revision, Fields: map[string]any{"hours": "2"}})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"hours": "2"}, got.Fields)
}

func TestUpdate_RefusesEmptyChange(t *testing.T) {
	s, _, sink := newMutableService(t)
	ctx := withUser("alice")
	rows, err := s.Append(ctx, "hours", []records.Input{{Owner: "SITE-1", Fields: map[string]any{"hours": "1"}}})
	require.NoError(t, err)
	_, err = s.Update(ctx, "hours", rows[0].ID, records.Change{Revision: rows[0].Revision})
	require.ErrorIs(t, err, records.ErrInvalid)
	require.Len(t, sink.Records(), 1, "only the append is audited")
}

// A change refused on a readable owner is audited as denied; one on a hidden
// owner is not, so the log never shows that a hidden row exists.
func TestChange_AuditsDenial(t *testing.T) {
	s, o, sink := newMutableService(t)
	ctx := withUser("alice")
	rows, err := s.Append(ctx, "hours", []records.Input{
		{Owner: "SITE-2", Fields: map[string]any{"hours": "1"}},
		{Owner: "SITE-3", Fields: map[string]any{"hours": "1"}},
	})
	require.NoError(t, err)
	o.hidden["SITE-2"] = true
	o.readOnly["SITE-3"] = true

	require.ErrorIs(t, s.Delete(ctx, "hours", rows[0].ID, 0), records.ErrNotFound)
	require.Len(t, sink.Records(), 1, "only the append is audited")

	require.ErrorIs(t, s.Delete(ctx, "hours", rows[1].ID, 0), records.ErrForbidden)
	lines := sink.Records()[1:]
	require.Len(t, lines, 1)
	require.Equal(t, audit.OpDeniedWrite, lines[0].Op)
	require.Equal(t, &audit.Subject{Kind: "record", Type: "hours", ID: rows[1].ID}, lines[0].Subject)
	require.Equal(t, "alice", lines[0].Principal.User)
}
