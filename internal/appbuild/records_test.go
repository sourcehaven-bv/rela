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
	in := []records.Input{{Owner: "MTR-1", TS: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
		Fields: map[string]any{"kwh": "1"}}}

	dave := boundAs(t, svc, principal.Principal{User: "dave", Tool: principal.ToolMCP})
	_, err = rs.Append(dave, "reading", in)
	require.ErrorIs(t, err, records.ErrForbidden)

	_, err = rs.Append(asAlice(t, svc), "reading", in)
	require.NoError(t, err)
}
