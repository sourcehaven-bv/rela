package appbuild_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const recordsMetamodel = `version: "1.0"
entities:
  meter:
    label: Meter
    plural: meters
    id_prefix: "MTR-"
    id_type: manual
    properties:
      title: {type: string}
records:
  reading:
    owner: meter
    fields:
      kwh: {type: decimal, scale: 2}
`

// alice owns meters; carol reads them but may not write; bob reads none.
const recordsPolicy = `roles:
  owner:
    read: [meter]
    create: [meter]
    update: [meter]
    delete: [meter]
  reader:
    read: [meter]
  outsider:
    read: []
assignments:
  alice: owner
  carol: reader
  bob: outsider
`

func recordsServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, recordsMetamodel)
	writePolicy(t, root, recordsPolicy)
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func createMeter(t *testing.T, svc *appbuild.Services) string {
	t.Helper()
	e := entity.New("", "meter")
	e.SetString("title", "m")
	res, err := svc.EntityManager().CreateEntity(asAlice(t, svc), e, entity.CreateOptions{ID: "MTR-1"})
	require.NoError(t, err)
	return res.Entity.ID
}

// TestRecords_GatedThroughPolicy runs the wired service against a real
// acl.yaml: a reader without the owner's read grant sees neither rows nor
// aggregates, a reader without update cannot append, and rows follow the
// owner through rename and delete.
func TestRecords_GatedThroughPolicy(t *testing.T) {
	svc := recordsServices(t)
	rs := appbuild.Records(svc)
	require.NotNil(t, rs)

	m := createMeter(t, svc)
	t0 := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	alice := asAlice(t, svc)
	_, err := rs.Append(alice, "reading", []records.Input{
		{Owner: m, TS: t0, Fields: map[string]any{"kwh": "1.5"}},
		{Owner: m, TS: t0.Add(time.Hour), Fields: map[string]any{"kwh": "2"}},
	})
	require.NoError(t, err)

	pts, err := rs.Aggregate(alice, records.AggregateQuery{Type: "reading", Field: "kwh"})
	require.NoError(t, err)
	require.Len(t, pts, 1)
	require.Equal(t, int64(2), pts[0].Count)
	require.Equal(t, "3.50", pts[0].Sum)

	bob := boundAs(t, svc, principal.Principal{User: "bob", Tool: principal.ToolMCP})
	pts, err = rs.Aggregate(bob, records.AggregateQuery{Type: "reading", Field: "kwh"})
	require.NoError(t, err)
	require.Empty(t, pts)
	_, err = rs.Aggregate(bob, records.AggregateQuery{Type: "reading", Owners: []string{m}})
	require.ErrorIs(t, err, records.ErrNotFound)
	_, err = rs.List(bob, "reading", m, records.ListQuery{})
	require.ErrorIs(t, err, records.ErrNotFound)
	_, err = rs.Append(bob, "reading", []records.Input{{Owner: m, TS: t0, Fields: map[string]any{"kwh": "1"}}})
	require.ErrorIs(t, err, records.ErrNotFound)

	carol := boundAs(t, svc, principal.Principal{User: "carol", Tool: principal.ToolMCP})
	page, err := rs.List(carol, "reading", m, records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 2)
	_, err = rs.Append(carol, "reading", []records.Input{{Owner: m, TS: t0, Fields: map[string]any{"kwh": "1"}}})
	require.ErrorIs(t, err, records.ErrForbidden)

	_, err = svc.EntityManager().RenameEntity(alice, m, "MTR-900", entity.RenameOptions{})
	require.NoError(t, err)
	page, err = rs.List(alice, "reading", "MTR-900", records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 2)

	_, err = svc.EntityManager().DeleteEntity(alice, "MTR-900", false)
	require.NoError(t, err)
	pts, err = rs.Aggregate(alice, records.AggregateQuery{Type: "reading"})
	require.NoError(t, err)
	require.Empty(t, pts)
}

const facedRecordsMetamodel = `version: "1.0"
entities:
  meter:
    label: Meter
    plural: meters
    id_type: manual
    faces:
      draft: { label: Draft }
      published: { label: Published }
    properties:
      title: {type: string}
records:
  reading:
    owner: meter
    fields:
      kwh: {type: decimal, scale: 2}
`

// alice reads and updates both faces; dave reads and updates the draft
// face only, so the published face is hidden from him.
const facedRecordsPolicy = `roles:
  both:
    read: ["meter@draft", "meter@published"]
    update: ["meter@draft", "meter@published"]
  draftonly:
    read: ["meter@draft"]
    update: ["meter@draft"]
assignments:
  alice: both
  dave: draftonly
`

// TestRecords_HiddenFaceRefusesAppend pins the per-face write check against
// a real acl.yaml: rows belong to the whole family, so a caller who cannot
// see one face of the owner may not append, even though the face it sees is
// writable to it.
func TestRecords_HiddenFaceRefusesAppend(t *testing.T) {
	svc, rs := facedRecordsServices(t)
	in := []records.Input{{Owner: "MTR-1", TS: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
		Fields: map[string]any{"kwh": "1"}}}

	dave := boundAs(t, svc, principal.Principal{User: "dave", Tool: principal.ToolMCP})
	_, err := rs.Append(dave, "reading", in)
	require.ErrorIs(t, err, records.ErrForbidden)

	_, err = rs.Append(asAlice(t, svc), "reading", in)
	require.NoError(t, err)
}

// TestRecords_HiddenFaceHidesRows pins the read gate to the write gate: rows
// belong to the whole family, so a caller who reads only one face of the
// owner sees no rows, folds nothing into an aggregate, and gets not found.
// A caller who reads every face sees the rows.
func TestRecords_HiddenFaceHidesRows(t *testing.T) {
	svc, rs := facedRecordsServices(t)
	alice := asAlice(t, svc)
	_, err := rs.Append(alice, "reading", []records.Input{{Owner: "MTR-1",
		TS: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), Fields: map[string]any{"kwh": "1"}}})
	require.NoError(t, err)

	dave := boundAs(t, svc, principal.Principal{User: "dave", Tool: principal.ToolMCP})
	_, err = rs.List(dave, "reading", "MTR-1", records.ListQuery{})
	require.ErrorIs(t, err, records.ErrNotFound)
	pts, err := rs.Aggregate(dave, records.AggregateQuery{Type: "reading", Field: "kwh"})
	require.NoError(t, err)
	require.Empty(t, pts, "no row of a partly hidden owner is folded")
	_, err = rs.Aggregate(dave, records.AggregateQuery{Type: "reading", Owners: []string{"MTR-1"}})
	require.ErrorIs(t, err, records.ErrNotFound)

	page, err := rs.List(alice, "reading", "MTR-1", records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)
	pts, err = rs.Aggregate(alice, records.AggregateQuery{Type: "reading", Field: "kwh"})
	require.NoError(t, err)
	require.Len(t, pts, 1)
	require.Equal(t, "1.00", pts[0].Sum)
}

// facedRecordsServices builds the faced meter project with MTR-1 stored at
// both faces.
func facedRecordsServices(t *testing.T) (*appbuild.Services, *records.Service) {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, facedRecordsMetamodel)
	writePolicy(t, root, facedRecordsPolicy)
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	rs := appbuild.Records(svc)
	require.NotNil(t, rs)
	for _, face := range []string{"draft", "published"} {
		require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
			ID: "MTR-1", Type: "meter", Face: entity.Face(face),
			Properties: map[string]any{"title": face},
		}))
	}
	return svc, rs
}

const ledgerRecordsMetamodel = `version: "1.0"
entities:
  meter:
    label: Meter
    plural: meters
    id_prefix: "MTR-"
    id_type: manual
    properties:
      title: {type: string}
  person:
    label: Person
    plural: people
    id_type: manual
    properties:
      sub: {type: string, unique: true}
records:
  reading:
    owner: meter
    fields:
      kwh: {type: decimal, scale: 2}
  posting:
    owner: meter
    ledger:
      amount: amount
      group_by: txn
    fields:
      amount: {type: decimal, scale: 2, required: true}
      txn: {type: string, required: true}
`

// ledgerRecordsPolicy matches principals to people by sub and rejects
// writes from a verified principal that matches none. ALICE is alice's
// person; mallory keeps the owner role but matches no person.
const ledgerRecordsPolicy = `user_entity_type: person
principal_property: sub
unmatched_principal: reject
roles:
  owner:
    read: [meter]
    create: [meter]
    update: [meter]
    delete: [meter]
assignments:
  ALICE: owner
  mallory: owner
`

var ledgerT0 = time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)

// ledgerRecordsServices builds the ledger project over root with opts and
// registers its Close.
func ledgerRecordsServices(t *testing.T, root string, opts []appbuild.Option) *appbuild.Services {
	t.Helper()
	svc, err := appbuild.Discover(root, script.NewEngine(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func seedFailClosed(t *testing.T, svc *appbuild.Services) {
	t.Helper()
	for _, e := range []*entity.Entity{
		{ID: "MTR-1", Type: "meter", Properties: map[string]any{"title": "a"}},
		{ID: "MTR-2", Type: "meter", Properties: map[string]any{"title": "b"}},
		{ID: "ALICE", Type: "person", Properties: map[string]any{"sub": "alice"}},
	} {
		require.NoError(t, svc.Store().CreateEntity(t.Context(), e))
	}
}

func postingIn(txn string) []records.Input {
	return []records.Input{
		{Owner: "MTR-1", TS: ledgerT0, Fields: map[string]any{"amount": "5", "txn": txn}},
		{Owner: "MTR-2", TS: ledgerT0, Fields: map[string]any{"amount": "-5", "txn": txn}},
	}
}

// verifiedRecordsCaller binds a verified assertion for sub, as the
// data-entry edge does for a JWT.
func verifiedRecordsCaller(t *testing.T, svc *appbuild.Services, sub string) context.Context {
	t.Helper()
	return acltest.BindVerified(principal.With(context.Background(),
		principal.Verified(sub, principal.ToolDataEntry, "", "", nil)), t, svc.ACLDeclarative())
}

// TestRecords_FailClosed pins that records, which skip the entity manager,
// still refuse what the ACL refuses: writes in read-only mode, every call
// without a bound identity, and writes by a verified principal that matches
// no user entity under unmatched_principal: reject.
func TestRecords_FailClosed(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, ledgerRecordsMetamodel)
	writePolicy(t, root, ledgerRecordsPolicy)
	opts := backendtest.Options(t)

	svc := ledgerRecordsServices(t, root, opts)
	seedFailClosed(t, svc)
	rs := appbuild.Records(svc)
	alice := verifiedRecordsCaller(t, svc, "alice")
	_, err := rs.Append(alice, "posting", postingIn("T1"))
	require.NoError(t, err)

	t.Run("no identity", func(t *testing.T) {
		for name, ctx := range map[string]context.Context{
			"bare":         context.Background(),
			"stamped only": principal.With(context.Background(), principal.Principal{User: "ALICE"}),
		} {
			t.Run(name, func(t *testing.T) {
				_, err := rs.List(ctx, "posting", "MTR-1", records.ListQuery{})
				require.ErrorIs(t, err, acl.ErrNoIdentity)
				_, err = rs.Aggregate(ctx, records.AggregateQuery{Type: "posting", Field: "amount"})
				require.ErrorIs(t, err, acl.ErrNoIdentity)
				_, err = rs.Append(ctx, "posting", postingIn("T2"))
				require.ErrorIs(t, err, acl.ErrNoIdentity)
				_, err = rs.Reverse(ctx, "posting", "T1")
				require.ErrorIs(t, err, acl.ErrNoIdentity)
			})
		}
	})

	t.Run("unmatched principal rejected", func(t *testing.T) {
		mallory := verifiedRecordsCaller(t, svc, "mallory")
		_, err := rs.Append(mallory, "posting", postingIn("T3"))
		require.ErrorIs(t, err, records.ErrForbidden)
		_, err = rs.Reverse(mallory, "posting", "T1")
		require.ErrorIs(t, err, records.ErrForbidden)
	})

	t.Run("read-only", func(t *testing.T) {
		require.NoError(t, svc.Close())
		ro := ledgerRecordsServices(t, root, append(slices.Clone(opts), appbuild.WithACL(acl.ReadOnlyACL{})))
		rs := appbuild.Records(ro)
		// The read-only override replaces the policy, so there is no user
		// entity lookup: bind the principal as the edge does.
		alice := boundAs(t, ro, principal.Principal{User: "ALICE", Tool: principal.ToolMCP})
		page, err := rs.List(alice, "posting", "MTR-1", records.ListQuery{})
		require.NoError(t, err)
		require.Len(t, page.Records, 1, "reads still work")
		_, err = rs.Append(alice, "posting", postingIn("T4"))
		require.ErrorIs(t, err, records.ErrForbidden)
		_, err = rs.Reverse(alice, "posting", "T1")
		require.ErrorIs(t, err, records.ErrForbidden)
	})
}
