package appbuild_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ledgerMetamodel has a ledger whose owner type has two faces. An account
// created at one face only lets a policy grant per account: a face grant
// then covers some accounts and not others.
const ledgerMetamodel = `version: "1.0"
entities:
  account:
    label: Account
    plural: accounts
    id_type: manual
    faces:
      public: { label: Public }
      private: { label: Private }
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

// alice keeps the books. erin reads public accounts only. frank reads every
// account but updates public ones only.
const ledgerPolicy = `roles:
  books:
    read: ["account@public", "account@private"]
    update: ["account@public", "account@private"]
  publicreader:
    read: ["account@public"]
  publicclerk:
    read: ["account@public", "account@private"]
    update: ["account@public"]
assignments:
  alice: books
  erin: publicreader
  frank: publicclerk
`

func ledgerServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, ledgerMetamodel)
	writePolicy(t, root, ledgerPolicy)
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.NotNil(t, appbuild.Records(svc))
	return svc
}

func createAccount(t *testing.T, svc *appbuild.Services, id, face string) {
	t.Helper()
	require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
		ID: id, Type: "account", Face: entity.Face(face), Properties: map[string]any{"title": id},
	}))
}

// leg is one posting of transaction txn on owner.
func leg(owner, txn, amount string, ts time.Time) records.Input {
	return records.Input{Owner: owner, TS: ts, Fields: map[string]any{"amount": amount, "txn": txn}}
}

var ledgerDay = time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

// seedLedger creates the public account ACC-A and the private account
// ACC-B, and books T1 across both and T2 on ACC-A alone.
func seedLedger(t *testing.T, svc *appbuild.Services) {
	t.Helper()
	createAccount(t, svc, "ACC-A", "public")
	createAccount(t, svc, "ACC-B", "private")
	_, err := appbuild.Records(svc).Append(asAlice(t, svc), "posting", []records.Input{
		leg("ACC-A", "T1", "10.00", ledgerDay), leg("ACC-B", "T1", "-10.00", ledgerDay),
		leg("ACC-A", "T2", "5.00", ledgerDay), leg("ACC-A", "T2", "-5.00", ledgerDay),
	})
	require.NoError(t, err)
}

// TestRecords_TransactionNeedsEveryLegReadable runs Transaction against a
// real acl.yaml: a caller who cannot read the owner of one leg gets not
// found for the whole transaction, so the legs it can see never reveal the
// hidden one's amount.
func TestRecords_TransactionNeedsEveryLegReadable(t *testing.T) {
	svc := ledgerServices(t)
	seedLedger(t, svc)
	rs := appbuild.Records(svc)
	erin := boundAs(t, svc, principal.Principal{User: "erin", Tool: principal.ToolMCP})

	rows, err := rs.Transaction(asAlice(t, svc), "posting", "T1")
	require.NoError(t, err)
	require.Len(t, rows, 2)

	_, err = rs.Transaction(erin, "posting", "T1")
	require.ErrorIs(t, err, records.ErrNotFound, "ACC-B is hidden from erin")
	_, err = rs.Transaction(erin, "posting", "T-unknown")
	require.ErrorIs(t, err, records.ErrNotFound)

	rows, err = rs.Transaction(erin, "posting", "T2")
	require.NoError(t, err, "every leg of T2 is on an account erin reads")
	require.Len(t, rows, 2)
}

// TestRecords_ReverseNeedsUpdateOnEveryOwner runs Reverse against a real
// acl.yaml: the caller must be able to update the owner of every leg.
func TestRecords_ReverseNeedsUpdateOnEveryOwner(t *testing.T) {
	svc := ledgerServices(t)
	seedLedger(t, svc)
	rs := appbuild.Records(svc)
	frank := boundAs(t, svc, principal.Principal{User: "frank", Tool: principal.ToolMCP})
	erin := boundAs(t, svc, principal.Principal{User: "erin", Tool: principal.ToolMCP})

	_, err := rs.Reverse(frank, "posting", "T1")
	require.ErrorIs(t, err, records.ErrForbidden, "frank reads ACC-B but may not update it")
	_, err = rs.Reverse(erin, "posting", "T1")
	require.ErrorIs(t, err, records.ErrNotFound, "erin cannot read ACC-B")
	_, err = rs.Reverse(erin, "posting", "T2")
	require.ErrorIs(t, err, records.ErrForbidden, "erin reads ACC-A but may not update it")

	rev, err := rs.Reverse(frank, "posting", "T2")
	require.NoError(t, err, "frank updates ACC-A, the only owner of T2")
	require.Len(t, rev, 2)

	alice := asAlice(t, svc)
	rev, err = rs.Reverse(alice, "posting", "T1")
	require.NoError(t, err)
	require.Len(t, rev, 2)
	_, err = rs.Reverse(alice, "posting", "T1")
	require.ErrorIs(t, err, records.ErrGroupExists, "a transaction is reversed at most once")
}

// requireOrderedTier skips on the fs and memory tiers: their record files
// share no transaction with the entity store, so neither fix applies there.
func requireOrderedTier(t *testing.T, svc *appbuild.Services) {
	t.Helper()
	if !store.TxRollsBack(svc.Store()) {
		t.Skip("records are ordered against entity writes on the database tiers only")
	}
}

// TestRecords_ClosingAPeriodIsOrderedAgainstAppends pins that the lock-date
// check and the insert of an append run in one store transaction, ordered
// after an entity transaction that is closing the period. The append
// starts while that transaction holds the write lock, so it must wait, and
// then read the new lock date on its own transaction view. Without the
// lock it reads the committed date, which is still open, and books into the
// closed period. The order is fixed by the write lock, not by timing: the
// wait below only lets a broken build fail sooner.
func TestRecords_ClosingAPeriodIsOrderedAgainstAppends(t *testing.T) {
	svc := ledgerServices(t)
	requireOrderedTier(t, svc)
	createAccount(t, svc, "ACC-A", "public")
	rs := appbuild.Records(svc)
	alice := asAlice(t, svc)
	posting := []records.Input{leg("ACC-A", "T1", "1.00", ledgerDay), leg("ACC-A", "T1", "-1.00", ledgerDay)}

	done := make(chan error, 1)
	ctx := t.Context()
	err := svc.Store().Tx(ctx, func(view store.Store) error {
		e, err := view.GetEntity(ctx, entity.Ref{ID: "ACC-A", Face: "public"})
		if err != nil {
			return err
		}
		e.Properties["locked_until"] = "2026-01-31"
		if err := view.UpdateEntity(ctx, e); err != nil {
			return err
		}
		go func() {
			_, err := rs.Append(alice, "posting", posting)
			done <- err
		}()
		select {
		case err := <-done:
			return errors.Join(errors.New("the append did not wait for the entity transaction"), err)
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})
	require.NoError(t, err)
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the append did not finish after the entity transaction committed")
	}
	require.ErrorIs(t, err, records.ErrClosed)

	_, err = rs.Append(alice, "posting", []records.Input{
		leg("ACC-A", "T2", "1.00", ledgerDay.AddDate(0, 1, 0)),
		leg("ACC-A", "T2", "-1.00", ledgerDay.AddDate(0, 1, 0)),
	})
	require.NoError(t, err, "a posting after the lock date is still open")
}

func readings(ctx context.Context, t *testing.T, rs *records.Service, owner string) int {
	t.Helper()
	page, err := rs.List(ctx, "reading", owner, records.ListQuery{})
	require.NoError(t, err)
	return len(page.Records)
}

func createRawMeter(t *testing.T, svc *appbuild.Services, id string) {
	t.Helper()
	require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
		ID: id, Type: "meter", Properties: map[string]any{"title": id},
	}))
}

// TestRecords_RowsFollowOwnerInsideTheEntityWrite pins that the database
// drops and rekeys an owner's rows inside the entity write itself. The
// writes go to the raw store, past the entity manager and its post-commit
// hooks, so only the database can be doing it. A delete that rolls back
// keeps the rows; a delete that commits drops them, so an entity that reuses
// the id starts without rows.
func TestRecords_RowsFollowOwnerInsideTheEntityWrite(t *testing.T) {
	svc := recordsServices(t)
	requireOrderedTier(t, svc)
	rs := appbuild.Records(svc)
	alice := asAlice(t, svc)
	ctx := t.Context()
	appendReading := func(owner string) {
		t.Helper()
		_, err := rs.Append(alice, "reading", []records.Input{
			{Owner: owner, TS: ledgerDay, Fields: map[string]any{"kwh": "1"}},
			{Owner: owner, TS: ledgerDay.Add(time.Hour), Fields: map[string]any{"kwh": "2"}},
		})
		require.NoError(t, err)
	}

	createRawMeter(t, svc, "MTR-1")
	appendReading("MTR-1")

	errRollback := errors.New("roll back")
	err := svc.Store().Tx(ctx, func(view store.Store) error {
		if _, err := view.DeleteFamily(ctx, "MTR-1", true); err != nil {
			return err
		}
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)
	require.Equal(t, 2, readings(alice, t, rs, "MTR-1"), "a delete that rolled back keeps the rows")

	_, err = svc.Store().DeleteFamily(ctx, "MTR-1", true)
	require.NoError(t, err)
	createRawMeter(t, svc, "MTR-1")
	require.Equal(t, 0, readings(alice, t, rs, "MTR-1"), "a new entity at a reused id inherits no rows")

	createRawMeter(t, svc, "MTR-2")
	appendReading("MTR-2")
	_, err = svc.Store().RenameFamily(ctx, "MTR-2", "MTR-3")
	require.NoError(t, err)
	require.Equal(t, 2, readings(alice, t, rs, "MTR-3"), "the rows moved with the rename")

	sd, ok := store.Innermost(svc.Store()).(store.SoftDeleteProvider)
	require.True(t, ok)
	createRawMeter(t, svc, "MTR-4")
	appendReading("MTR-4")
	_, err = sd.SoftDelete().MarkDeleted(ctx, "MTR-4", "alice")
	require.NoError(t, err)
	_, err = sd.SoftDelete().Unmark(ctx, "MTR-4")
	require.NoError(t, err)
	require.Equal(t, 2, readings(alice, t, rs, "MTR-4"), "a soft delete keeps the rows for a restore")
	_, err = sd.SoftDelete().MarkDeleted(ctx, "MTR-4", "alice")
	require.NoError(t, err)
	_, err = sd.SoftDelete().PurgeMarked(ctx, "MTR-4")
	require.NoError(t, err)
	createRawMeter(t, svc, "MTR-4")
	require.Equal(t, 0, readings(alice, t, rs, "MTR-4"), "the purge dropped the rows")
}
