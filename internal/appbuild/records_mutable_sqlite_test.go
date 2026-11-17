//go:build sqlite

package appbuild_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const mutableRecordsMetamodel = `version: "1.0"
entities:
  device:
    label: Device
    plural: devices
    id_type: manual
    properties:
      title: {type: string}
records:
  note:
    owner: device
    mutable: true
    fields:
      text: {type: string, required: true}
      score: {type: integer}
`

const mutableRecordsPolicy = `roles:
  keeper:
    read: [device]
    update: [device]
assignments:
  kim: keeper
`

// TestRecords_MutableUpdateDeleteSQLite runs Update and Delete of a mutable
// record type through the sqlite wiring, where the write lock hands the
// closure a record store bound to the entity store's transaction.
func TestRecords_MutableUpdateDeleteSQLite(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, mutableRecordsMetamodel)
	writePolicy(t, root, mutableRecordsPolicy)
	fs := osFS()
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	sink := audit.NewMemory()
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: sink,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
		ID: "DEV-1", Type: "device", Properties: map[string]any{"title": "Pump"},
	}))
	rs := appbuild.Records(svc)
	require.NotNil(t, rs)
	ctx := boundAs(t, svc, principal.Principal{User: "kim", Tool: principal.ToolMCP})

	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	added, err := rs.Append(ctx, "note", []records.Input{
		{Owner: "DEV-1", TS: ts, Fields: map[string]any{"text": "first", "score": int64(1)}},
	})
	require.NoError(t, err)
	require.Len(t, added, 1)
	id := added[0].ID
	require.Equal(t, int64(0), added[0].Revision)

	later := ts.Add(time.Hour)
	got, err := rs.Update(ctx, "note", id, records.Change{
		Revision: 0, TS: later, Fields: map[string]any{"text": "second"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), got.Revision)
	require.Equal(t, "second", got.Fields["text"])
	require.EqualValues(t, 1, got.Fields["score"], "a field not named keeps its value")
	require.True(t, got.TS.Equal(later))
	require.Equal(t, "kim", got.UpdatedBy)

	page, err := rs.List(ctx, "note", "DEV-1", records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)
	require.Equal(t, int64(1), page.Records[0].Revision)
	require.Equal(t, "second", page.Records[0].Fields["text"])

	_, err = rs.Update(ctx, "note", id, records.Change{Revision: 0, Fields: map[string]any{"text": "third"}})
	require.ErrorIs(t, err, records.ErrStale)

	require.NoError(t, rs.Delete(ctx, "note", id, 1))
	page, err = rs.List(ctx, "note", "DEV-1", records.ListQuery{})
	require.NoError(t, err)
	require.Empty(t, page.Records)

	var ops []string
	for _, r := range auditOf(t, svc, sink) {
		if r.Subject != nil && r.Subject.Kind == "record" && r.Subject.ID == id {
			ops = append(ops, r.Op)
		}
	}
	require.Contains(t, ops, audit.OpUpdateRecord)
	require.Contains(t, ops, audit.OpDeleteRecord)
}
