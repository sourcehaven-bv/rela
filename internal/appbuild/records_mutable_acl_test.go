package appbuild_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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

const mutableACLMetamodel = `version: "1.0"
entities:
  device:
    label: Device
    plural: devices
    id_type: manual
    properties:
      title: {type: string}
  gauge:
    label: Gauge
    plural: gauges
    id_type: manual
    faces:
      draft: { label: Draft }
      published: { label: Published }
    properties:
      title: {type: string}
records:
  note:
    owner: device
    mutable: true
    fields:
      text: {type: string, required: true}
  gnote:
    owner: gauge
    mutable: true
    fields:
      text: {type: string, required: true}
`

// kim reads and updates everything; rita reads devices but updates none;
// olly reads no device; dave reads both gauge faces but updates only the
// draft face.
const mutableACLPolicy = `roles:
  keeper:
    read: [device, "gauge@draft", "gauge@published"]
    update: [device, "gauge@draft", "gauge@published"]
  reader:
    read: [device]
  outsider:
    read: []
  draftwriter:
    read: ["gauge@draft", "gauge@published"]
    update: ["gauge@draft"]
assignments:
  kim: keeper
  rita: reader
  olly: outsider
  dave: draftwriter
`

var mutableACLT0 = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// mutableACLServices builds the project with a memory audit sink, stores
// DEV-1 and GAU-1 (at both faces), and appends one row to each as kim. It
// returns the ids of the note and gnote rows.
func mutableACLServices(t *testing.T) (svc *appbuild.Services, rs *records.Service, sink *audit.Memory, noteID, gnoteID string) {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, mutableACLMetamodel)
	writePolicy(t, root, mutableACLPolicy)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	sink = audit.NewMemory()
	svc, err = appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: sink, DatabaseURL: backendtest.DSN(t),
	}, backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
		ID: "DEV-1", Type: "device", Properties: map[string]any{"title": "Pump"},
	}))
	for _, face := range []string{"draft", "published"} {
		require.NoError(t, svc.Store().CreateEntity(t.Context(), &entity.Entity{
			ID: "GAU-1", Type: "gauge", Face: entity.Face(face),
			Properties: map[string]any{"title": face},
		}))
	}
	rs = appbuild.Records(svc)
	require.NotNil(t, rs)

	kim := mutableACLCaller(t, svc, "kim")
	note, err := rs.Append(kim, "note", []records.Input{
		{Owner: "DEV-1", TS: mutableACLT0, Fields: map[string]any{"text": "first"}},
	})
	require.NoError(t, err)
	gnote, err := rs.Append(kim, "gnote", []records.Input{
		{Owner: "GAU-1", TS: mutableACLT0, Fields: map[string]any{"text": "first"}},
	})
	require.NoError(t, err)
	return svc, rs, sink, note[0].ID, gnote[0].ID
}

func mutableACLCaller(t *testing.T, svc *appbuild.Services, user string) context.Context {
	t.Helper()
	return boundAs(t, svc, principal.Principal{User: user, Tool: principal.ToolMCP})
}

// auditOf returns the audit records: from the database log on a database
// tier, where it replaces the configured sink, else from sink.
func auditOf(t *testing.T, svc *appbuild.Services, sink *audit.Memory) []audit.Record {
	t.Helper()
	if log := svc.AuditLog(); log != nil {
		return settledSQLAudit(t, log)
	}
	return sink.Records()
}

// recordAuditLines returns the audit lines about record id.
func recordAuditLines(t *testing.T, svc *appbuild.Services, sink *audit.Memory, id string) []audit.Record {
	t.Helper()
	var out []audit.Record
	for _, r := range auditOf(t, svc, sink) {
		if r.Subject != nil && r.Subject.Kind == "record" && r.Subject.ID == id {
			out = append(out, r)
		}
	}
	return out
}

// requireRowUnchanged checks, as kim, that row id of typ under owner is
// still at revision 0 with its first text.
func requireRowUnchanged(t *testing.T, svc *appbuild.Services, rs *records.Service, typ, owner, id string) {
	t.Helper()
	page, err := rs.List(mutableACLCaller(t, svc, "kim"), typ, owner, records.ListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)
	require.Equal(t, id, page.Records[0].ID)
	require.Equal(t, int64(0), page.Records[0].Revision)
	require.Equal(t, "first", page.Records[0].Fields["text"])
}

// TestRecords_MutableChangeGatedThroughPolicy pins the access rules of
// Update and Delete of a mutable record type against a real acl.yaml.
func TestRecords_MutableChangeGatedThroughPolicy(t *testing.T) {
	change := records.Change{Revision: 0, Fields: map[string]any{"text": "changed"}}

	t.Run("read without update is forbidden and audited", func(t *testing.T) {
		svc, rs, sink, id, _ := mutableACLServices(t)
		rita := mutableACLCaller(t, svc, "rita")
		before := len(recordAuditLines(t, svc, sink, id))

		_, err := rs.Update(rita, "note", id, change)
		require.ErrorIs(t, err, records.ErrForbidden)
		require.ErrorIs(t, rs.Delete(rita, "note", id, 0), records.ErrForbidden)

		lines := recordAuditLines(t, svc, sink, id)[before:]
		require.Len(t, lines, 2, "one denied-write line per refusal")
		for _, l := range lines {
			require.Equal(t, audit.OpDeniedWrite, l.Op)
			require.Equal(t, "record", l.Subject.Kind)
			require.Equal(t, "note", l.Subject.Type)
			require.Equal(t, "rita", l.Principal.User)
		}
		require.Contains(t, lines[0].Summary, audit.OpUpdateRecord)
		require.Contains(t, lines[1].Summary, audit.OpDeleteRecord)
		requireRowUnchanged(t, svc, rs, "note", "DEV-1", id)
	})

	t.Run("hidden owner is not found and not audited", func(t *testing.T) {
		svc, rs, sink, id, _ := mutableACLServices(t)
		olly := mutableACLCaller(t, svc, "olly")
		before := len(auditOf(t, svc, sink))

		_, err := rs.Update(olly, "note", id, change)
		require.ErrorIs(t, err, records.ErrNotFound)
		require.NotContains(t, err.Error(), "DEV-1", "the error names the row, not its hidden owner")
		require.ErrorIs(t, rs.Delete(olly, "note", id, 0), records.ErrNotFound)
		// A stale revision tells nothing about a hidden row.
		require.ErrorIs(t, rs.Delete(olly, "note", id, 7), records.ErrNotFound)

		require.Len(t, auditOf(t, svc, sink), before, "no audit line shows that a hidden row exists")
		requireRowUnchanged(t, svc, rs, "note", "DEV-1", id)
	})

	t.Run("update on only some faces is forbidden", func(t *testing.T) {
		svc, rs, sink, _, gid := mutableACLServices(t)
		dave := mutableACLCaller(t, svc, "dave")
		before := len(recordAuditLines(t, svc, sink, gid))

		_, err := rs.Update(dave, "gnote", gid, change)
		require.ErrorIs(t, err, records.ErrForbidden)
		require.ErrorIs(t, rs.Delete(dave, "gnote", gid, 0), records.ErrForbidden)

		lines := recordAuditLines(t, svc, sink, gid)[before:]
		require.Len(t, lines, 2)
		for _, l := range lines {
			require.Equal(t, audit.OpDeniedWrite, l.Op)
		}
		requireRowUnchanged(t, svc, rs, "gnote", "GAU-1", gid)

		got, err := rs.Update(mutableACLCaller(t, svc, "kim"), "gnote", gid, change)
		require.NoError(t, err, "a caller who updates every face may change the row")
		require.Equal(t, int64(1), got.Revision)
	})

	t.Run("no bound identity fails closed", func(t *testing.T) {
		svc, rs, sink, id, _ := mutableACLServices(t)
		before := len(auditOf(t, svc, sink))
		for name, ctx := range map[string]context.Context{
			"bare":         context.Background(),
			"stamped only": principal.With(context.Background(), principal.Principal{User: "kim"}),
		} {
			t.Run(name, func(t *testing.T) {
				_, err := rs.Update(ctx, "note", id, change)
				require.ErrorIs(t, err, acl.ErrNoIdentity)
				require.ErrorIs(t, rs.Delete(ctx, "note", id, 0), acl.ErrNoIdentity)
			})
		}
		require.Len(t, auditOf(t, svc, sink), before)
		requireRowUnchanged(t, svc, rs, "note", "DEV-1", id)
	})
}
