package appbuild_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const lockDateMetamodel = `version: "1.0"
entities:
  account:
    label: Account
    plural: accounts
    id_type: manual
    properties:
      title: {type: string}
      locked_until: {type: date}
records:
  posting:
    owner: account
    ledger:
      amount: amount
      group_by: txn
      closed_before: locked_until
    fields:
      amount: {type: decimal, scale: 2, required: true}
      txn: {type: string, required: true}
`

// poster may post on accounts but cannot see locked_until.
const lockDatePolicy = `roles:
  poster:
    read: [account]
    update: [account]
    visible:
      account:
        - field: title
assignments:
  pat: poster
`

// TestRecords_ReusedKeyHidesLockDate pins issue #2367: a poster who cannot
// see the owner's lock date gets the same answer for a reused group key
// whatever the posting's date, so the answer does not reveal the date.
func TestRecords_ReusedKeyHidesLockDate(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, lockDateMetamodel)
	writePolicy(t, root, lockDatePolicy)
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	for _, id := range []string{"ACC-1", "ACC-2"} {
		require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{ID: id, Type: "account",
			Properties: map[string]any{"title": id, "locked_until": "2026-01-31"}}))
	}
	rs := appbuild.Records(svc)
	pat := boundAs(t, svc, principal.Principal{User: "pat", Tool: principal.ToolMCP})

	open := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	in := func(txn string, ts time.Time) []records.Input {
		return []records.Input{
			{Owner: "ACC-1", TS: ts, Fields: map[string]any{"amount": "5", "txn": txn}},
			{Owner: "ACC-2", TS: ts, Fields: map[string]any{"amount": "-5", "txn": txn}},
		}
	}
	_, err = rs.Append(pat, "posting", in("T1", open))
	require.NoError(t, err)

	_, err = rs.Append(pat, "posting", in("T1", open))
	require.ErrorIs(t, err, records.ErrGroupExists)
	_, err = rs.Append(pat, "posting", in("T1", closed))
	require.ErrorIs(t, err, records.ErrGroupExists, "a reused key answers 409 in a closed period too")
	require.NotErrorIs(t, err, records.ErrClosed)

	_, err = rs.Append(pat, "posting", in("T2", closed))
	require.ErrorIs(t, err, records.ErrClosed, "the hidden lock date still guards a new key")
}
