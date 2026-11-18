package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// fakeAuditLog serves fixed entries, newest first, and records the queries
// it was asked.
type fakeAuditLog struct {
	entries []audit.Entry
	queries []audit.SubjectQuery
}

func (f *fakeAuditLog) ForEntity(
	_ context.Context, q audit.SubjectQuery, before int64, limit int,
) ([]audit.Entry, error) {
	f.queries = append(f.queries, q)
	return f.page(before, limit), nil
}

func (f *fakeAuditLog) LifecycleStart(_ context.Context, q audit.SubjectQuery) (audit.Entry, bool, error) {
	for _, e := range f.entries {
		for _, s := range []*audit.Subject{e.Record.Subject, e.Record.After} {
			if audit.StartsLifecycle(e.Record, s) && s.ID == q.ID && s.Face == q.Face {
				return e, true, nil
			}
		}
	}
	return audit.Entry{}, false, nil
}

func (f *fakeAuditLog) Newest(_ context.Context, before int64, limit int) ([]audit.Entry, error) {
	return f.page(before, limit), nil
}

func (f *fakeAuditLog) page(before int64, limit int) []audit.Entry {
	var out []audit.Entry
	for _, e := range f.entries {
		if before > 0 && e.Seq >= before {
			continue
		}
		if len(out) == limit {
			break
		}
		out = append(out, e)
	}
	return out
}

func entityEntry(seq int64, op, typ, id string) audit.Entry {
	return audit.Entry{Seq: seq, Record: audit.Record{
		Time: time.Unix(seq, 0).UTC(), Op: op,
		Subject: &audit.Subject{Kind: "entity", Type: typ, ID: id},
	}}
}

func relationEntry(seq int64, from, to string) audit.Entry {
	return audit.Entry{Seq: seq, Record: audit.Record{
		Time: time.Unix(seq, 0).UTC(), Op: audit.OpCreateRelation,
		Subject: &audit.Subject{Kind: "relation", RelationType: "depends_on", FromID: from, ToID: to},
	}}
}

func doAuditGet(ctx context.Context, app *App, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	if strings.HasPrefix(path, "/api/v1/_changes/") {
		handleV1Changes(app, rec, req)
	} else {
		handleV1Audit(app, rec, req)
	}
	return rec
}

func TestSetAuditLog_RejectsNil(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	if err := SetAuditLog(app, nil); err == nil {
		t.Fatal("SetAuditLog(nil) succeeded")
	}
}

// TestAuditRoutes_AbsentWithoutLog pins "offered only when the configured
// audit implementation can read": no log, no route and no sidebar entry.
func TestAuditRoutes_AbsentWithoutLog(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	for _, path := range []string{"/api/v1/_changes/ticket/TKT-1", "/api/v1/_audit"} {
		if rec := doAuditGet(t.Context(), app, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s without a log = %d, want 404", path, rec.Code)
		}
	}
	if changes, global := sidebarAudit(t.Context(), app.auditRead, app.acl); changes || global {
		t.Errorf("sidebar offers changes=%v global=%v without a log", changes, global)
	}
}

func TestHandleV1Changes(t *testing.T) {
	f := newFixture()
	f.AddNode(entity.New("TKT-1", "ticket"))
	f.AddNode(entity.New("TKT-2", "ticket"))
	app := newAppFromParts(nil, testMeta(), f)
	log := &fakeAuditLog{entries: []audit.Entry{
		relationEntry(5, "TKT-1", "TKT-HIDDEN"),
		relationEntry(4, "TKT-1", "TKT-2"),
		entityEntry(3, audit.OpUpdateEntity, "note", "TKT-1"),
		entityEntry(2, audit.OpUpdateEntity, "ticket", "TKT-1"),
		entityEntry(1, audit.OpCreateEntity, "ticket", "TKT-1"),
	}}
	if err := SetAuditLog(app, log); err != nil {
		t.Fatal(err)
	}

	t.Run("shows only what the reader may see", func(t *testing.T) {
		rec := doAuditGet(t.Context(), app, "/api/v1/_changes/ticket/TKT-1")
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d; body=%s", rec.Code, rec.Body.String())
		}
		var body changesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var seqs []int64
		for _, c := range body.Changes {
			seqs = append(seqs, c.Seq)
		}
		if want := []int64{4, 2, 1}; !equalSeqs(seqs, want) {
			t.Errorf("seqs = %v, want %v", seqs, want)
		}
		if bodyMentions(rec, "TKT-HIDDEN") {
			t.Error("a record naming an unreadable entity was served")
		}
	})

	t.Run("pages with a cursor", func(t *testing.T) {
		rec := doAuditGet(t.Context(), app, "/api/v1/_changes/ticket/TKT-1?limit=1")
		var body changesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Changes) != 1 || body.Changes[0].Seq != 4 || body.NextBefore != 4 {
			t.Fatalf("page = %+v", body)
		}
	})

	t.Run("cross-type is the uniform 404", func(t *testing.T) {
		rec := doAuditGet(t.Context(), app, "/api/v1/_changes/component/TKT-1")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", rec.Code)
		}
	})

	t.Run("deleted entity without history:read is the uniform 404", func(t *testing.T) {
		ctx := withReadGate(t.Context(), fakeGate{holdsPermission: false})
		rec := doAuditGet(ctx, app, "/api/v1/_changes/ticket/GONE-1")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", rec.Code)
		}
	})

	for _, q := range []string{"limit=0", "limit=201", "limit=x", "before=0", "before=-3", "before=x"} {
		t.Run("bad query "+q, func(t *testing.T) {
			rec := doAuditGet(t.Context(), app, "/api/v1/_changes/ticket/TKT-1?"+q)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400", rec.Code)
			}
		})
	}
}

// TestHandleV1Changes_DeletedIDOfAnotherType: a holder of history:read who
// asks for a deleted id under a type it never had gets an empty list, even
// when relation records of a readable entity name the id.
func TestHandleV1Changes_DeletedIDOfAnotherType(t *testing.T) {
	f := newFixture()
	f.AddNode(entity.New("TKT-2", "ticket"))
	app := newAppFromParts(nil, testMeta(), f)
	log := &fakeAuditLog{entries: []audit.Entry{
		relationEntry(3, "TKT-2", "GONE-1"),
		entityEntry(2, audit.OpDeleteEntity, "secret", "GONE-1"),
		entityEntry(1, audit.OpCreateEntity, "secret", "GONE-1"),
	}}
	if err := SetAuditLog(app, log); err != nil {
		t.Fatal(err)
	}
	ctx := withReadGate(t.Context(), fakeGate{holdsPermission: true})
	rec := doAuditGet(ctx, app, "/api/v1/_changes/ticket/GONE-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d; body=%s", rec.Code, rec.Body.String())
	}
	var body changesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Changes) != 0 {
		t.Errorf("served %d records of another type's deleted id", len(body.Changes))
	}
}

func equalSeqs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPermitsAuditRead_ClosedSwitch pins that the global view is an elevated
// read: no policy and read-only refuse it even when the read gate would say
// the permission is held.
func TestPermitsAuditRead_ClosedSwitch(t *testing.T) {
	holds := permGate{perms: map[string]bool{acl.PermAuditRead: true}}
	lacks := permGate{perms: map[string]bool{acl.PermHistoryRead: true}}
	for _, tc := range []struct {
		name    string
		aclImpl acl.ACL
		gate    readGate
		want    bool
	}{
		{name: "nil ACL denies", aclImpl: nil, gate: holds},
		{name: "NopACL denies", aclImpl: acl.NopACL{}, gate: holds},
		{name: "NopACL face denies", aclImpl: &acl.NopACL{}, gate: holds},
		{name: "ReadOnlyACL denies", aclImpl: acl.ReadOnlyACL{}, gate: holds},
		{name: "ReadOnlyACL face denies", aclImpl: &acl.ReadOnlyACL{}, gate: holds},
		{name: "unknown implementation denies", aclImpl: stubACL{}, gate: holds},
		{name: "policy without audit:read denies", aclImpl: &acl.Declarative{}, gate: lacks},
		{name: "policy with audit:read allows", aclImpl: &acl.Declarative{}, gate: holds, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withReadGate(t.Context(), tc.gate)
			if got := permitsAuditRead(ctx, tc.aclImpl); got != tc.want {
				t.Errorf("permitsAuditRead = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHandleV1Audit(t *testing.T) {
	newApp := func(t *testing.T) (*App, *capturingAudit) {
		t.Helper()
		app := newAppFromParts(nil, testMeta(), &fixture{})
		sink := &capturingAudit{}
		app.auditSink = sink
		deleted := entityEntry(2, audit.OpDeleteEntity, "ticket", "TKT-GONE")
		denied := audit.Entry{Seq: 1, Record: audit.Record{Op: audit.OpDeniedWrite}}
		if err := SetAuditLog(app, &fakeAuditLog{entries: []audit.Entry{deleted, denied}}); err != nil {
			t.Fatal(err)
		}
		return app, sink
	}

	t.Run("refused without a policy, nothing recorded", func(t *testing.T) {
		app, sink := newApp(t)
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{acl.PermAuditRead: true}})
		rec := doAuditGet(ctx, app, "/api/v1/_audit")
		if rec.Code != http.StatusForbidden || !bodyMentions(rec, acl.PermAuditRead) {
			t.Fatalf("got %d %s, want 403 naming %s", rec.Code, rec.Body.String(), acl.PermAuditRead)
		}
		if len(sink.ops()) != 0 {
			t.Errorf("a refused read was recorded: %v", sink.ops())
		}
	})

	t.Run("holder reads deleted and denied records, and the read is recorded", func(t *testing.T) {
		app, sink := newApp(t)
		app.acl = &acl.Declarative{}
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{acl.PermAuditRead: true}})
		rec := doAuditGet(ctx, app, "/api/v1/_audit?limit=1")
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d; body=%s", rec.Code, rec.Body.String())
		}
		var body auditResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Records) != 1 || body.Records[0].Record.Subject.ID != "TKT-GONE" || body.NextBefore != 2 {
			t.Fatalf("first page = %+v", body)
		}
		rec = doAuditGet(ctx, app, "/api/v1/_audit?before=2")
		body = auditResponse{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Records) != 1 || body.Records[0].Record.Op != audit.OpDeniedWrite || body.NextBefore != 0 {
			t.Fatalf("second page = %+v", body)
		}
		if ops := sink.ops(); len(ops) != 2 || ops[0] != audit.OpAuditRead || ops[1] != audit.OpAuditRead {
			t.Errorf("recorded ops = %v, want two audit-read records", ops)
		}
	})
}
