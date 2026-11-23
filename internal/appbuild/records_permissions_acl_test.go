package appbuild_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const recordPermMetamodel = `version: "1.0"
entities:
  device:
    label: Device
    plural: devices
    id_type: manual
    properties:
      title: {type: string}
records:
  reading:
    owner: device
    permissions: {read: "readings:read", append: "readings:append"}
    fields:
      kwh: {type: integer, required: true}
`

// All three users read and update every device; they differ only in the
// record permissions their role grants.
const recordPermPolicy = `roles:
  full:
    read: [device]
    update: [device]
    permissions: ["readings:read", "readings:append"]
  viewer:
    read: [device]
    update: [device]
    permissions: ["readings:read"]
  bare:
    read: [device]
    update: [device]
assignments:
  kim: full
  lou: viewer
  sam: bare
`

// TestRecords_TypePermissionsThroughPolicy pins the per-type permissions
// against a real acl.yaml: they narrow an owner the caller may update.
func TestRecords_TypePermissionsThroughPolicy(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, recordPermMetamodel)
	writePolicy(t, root, recordPermPolicy)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.NewMemory(), DatabaseURL: backendtest.DSN(t),
	}, backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
		ID: "DEV-1", Type: "device", Properties: map[string]any{"title": "Pump"},
	}))
	rs := appbuild.Records(svc)
	require.NotNil(t, rs)
	as := func(user string) *principal.Principal {
		return &principal.Principal{User: user, Tool: principal.ToolMCP}
	}
	in := []records.Input{{Owner: "DEV-1", Fields: map[string]any{"kwh": 3}}}

	_, err = rs.Append(boundAs(t, svc, *as("kim")), "reading", in)
	require.NoError(t, err)

	// May update the owner, lacks records append: refused.
	_, err = rs.Append(boundAs(t, svc, *as("lou")), "reading", in)
	require.ErrorIs(t, err, records.ErrForbidden)
	_, err = rs.Append(boundAs(t, svc, *as("sam")), "reading", in)
	require.ErrorIs(t, err, records.ErrForbidden)

	// Reads need the read permission.
	page, err := rs.List(boundAs(t, svc, *as("lou")), "reading", "DEV-1", records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)
	_, err = rs.List(boundAs(t, svc, *as("sam")), "reading", "DEV-1", records.ListQuery{})
	require.ErrorIs(t, err, records.ErrForbidden)
	_, err = rs.Aggregate(boundAs(t, svc, *as("sam")), records.AggregateQuery{Type: "reading"})
	require.ErrorIs(t, err, records.ErrForbidden)
}
