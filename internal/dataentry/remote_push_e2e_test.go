package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const pushTargetSchema = `
version: "1.0"
entities:
  item:
    label: Item
    id_prefix: "ITM-"
    properties:
      title: {type: string}
      status: {type: string}
      note: {type: string}
      src: {type: external_ref, system: source}
`

// pushRemote is a data-entry server behind the identity proxy of
// [newPullSource], whose principal may read, create and update tickets.
// Its entity manager records to sink, so the test sees every write the
// pushes caused there.
type pushRemote struct {
	app  *App
	url  string
	sink *audit.Memory
}

func newPushRemote(t *testing.T) *pushRemote {
	t.Helper()
	app := newTestAppV1(t)
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "one", "status": "open"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "two", "status": "open"}},
	} {
		seedEntity(app, e)
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"tickets-sync": {Read: []string{"ticket"}, Create: []string{"ticket"}, Update: []string{"ticket"}},
		},
		Assignments: map[string]string{"catalog-mirror": "tickets-sync"},
	}, app.store)
	sink := audit.NewMemory()
	svc := appbuildtest.New(app.State().Meta, appbuildtest.WithStore(app.store),
		appbuildtest.WithFS(app.fs, app.paths), appbuildtest.WithACL(d), appbuildtest.WithAudit(sink))
	rebindApp(app, app.fs, app.paths, svc)
	setTestACL(t, app, d)
	if err := SetReconcile(app, localElements(t, app, d)); err != nil {
		t.Fatal(err)
	}
	const header = "X-Remote-User"
	app.SetPrincipalResolver(HeaderPrincipalResolver(header))
	app.SetPrincipalHeader(header)
	router := app.NewRouter()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(header)
		if r.Header.Get("Authorization") == "Bearer "+pullToken {
			r.Header.Set(header, "catalog-mirror")
		}
		r.Header.Del("Authorization")
		router.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return &pushRemote{app: app, url: srv.URL, sink: sink}
}

// versions returns the version of every row of typ in st.
func versions(t *testing.T, st store.Store, typ string) map[string]store.EntityVersion {
	t.Helper()
	out := map[string]store.EntityVersion{}
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		out[e.ID] = store.VersionOf(e)
	}
	return out
}

func sameVersions(a, b map[string]store.EntityVersion) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestRemotePush_EndToEnd syncs both ways with a real data-entry server as
// an ordinary principal. Each side's change reaches the other through its
// normal write path; a new local row is created on the remote under the id
// the remote picks; a round trip with no new edits changes no version and
// writes no audit row on either side; and a field both sides changed is
// held, written on neither side.
func TestRemotePush_EndToEnd(t *testing.T) {
	f := newPushRemote(t)
	ctx := context.Background()

	meta, err := metamodel.Parse([]byte(pushTargetSchema))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := remotesync.Parse([]byte(`
remotes:
  acme:
    url: `+f.url+`
    credential: {secret: remote_credential.acme}
    push: true
    entities:
      ticket: {type: item, ref: src, fields: {title: title, status: status}}
`), meta)
	if err != nil {
		t.Fatal(err)
	}
	m := cfg.Mappings["acme"]
	src, err := remotesync.Dial(m.Remote, pullToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	localAudit := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: pullTemplater{}, Audit: localAudit, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}
	kv := &pullKV{}
	deps := remotesync.Deps{
		Source: src, Pusher: src, Meta: meta, Writer: mgr, Store: st, State: kv,
		Fingerprint: visibility.SumHeld, Logger: slog.New(slog.DiscardHandler), RunID: "e2e",
	}
	run := func(what string) remotesync.Report {
		t.Helper()
		rep, runErr := remotesync.Run(ctx, m, deps, remotesync.Options{})
		if runErr != nil {
			t.Fatalf("%s: %v (%+v %+v)", what, runErr, rep, rep.Pushed)
		}
		return rep
	}

	if rep := run("first sync"); rep.Entities.Created != 2 || rep.Pushed.Created != 0 {
		t.Fatalf("first sync: %+v %+v", rep, rep.Pushed)
	}

	// A local edit, a new local row, and a remote edit of another row.
	one := pulledByTitle(t, st, "one")
	if _, err = mgr.PatchEntity(ctx, one.ID, entity.Patch{
		Properties: map[string]any{"status": "done", "note": "local only"},
	}); err != nil {
		t.Fatal(err)
	}
	three, err := mgr.CreateEntity(ctx, &entity.Entity{Type: "item",
		Properties: map[string]any{"title": "three", "status": "open"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	two, err := f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-002"})
	if err != nil {
		t.Fatal(err)
	}
	two.Properties["title"] = "two, renamed"
	if err = f.app.store.UpdateEntity(ctx, two); err != nil {
		t.Fatal(err)
	}

	rep := run("second sync")
	if rep.Entities.Updated != 1 || rep.Pushed.Updated != 1 || rep.Pushed.Created != 1 {
		t.Fatalf("second sync: %+v %+v %+v", rep, rep.Pushed, rep.Issues)
	}
	got, err := f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-001"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties["status"] != "done" || got.Properties["note"] != nil {
		t.Fatalf("remote TKT-001 = %+v", got.Properties)
	}
	var created *entity.Entity
	for e, err := range f.app.store.ListEntities(ctx, store.EntityQuery{Type: "ticket", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		if e.Properties["title"] == "three" {
			created = e
		}
	}
	if created == nil || created.ID == three.Entity.ID {
		t.Fatalf("the new row is not on the remote under a remote id: %+v", created)
	}
	localThree, err := st.GetEntity(ctx, three.Entity.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := metamodel.ExternalRefID(localThree.Properties["src"]); id != created.ID {
		t.Fatalf("the local ref names %q, want %q", id, created.ID)
	}
	if pulledByTitle(t, st, "two, renamed") == nil {
		t.Fatal("the remote edit was not pulled")
	}
	for _, r := range f.sink.Records() {
		if r.Principal.User != "catalog-mirror" {
			t.Fatalf("a push was written as %q, not the remote's principal: %+v", r.Principal.User, r)
		}
	}

	// The echo stop: no new edits, so no version moves and no audit row
	// is written on either side, on the run that sees the pushed values
	// come back nor on the run after it.
	remoteBefore, localBefore := versions(t, f.app.store, "ticket"), versions(t, st, "item")
	remoteAudit, localAuditN := len(f.sink.Records()), len(localAudit.Records())
	for _, what := range []string{"echo sync", "idle sync"} {
		idle := run(what)
		if idle.Pushed.Updated+idle.Pushed.Created != 0 || idle.Entities.Updated+idle.Entities.Created != 0 {
			t.Fatalf("%s did work: %+v %+v", what, idle, idle.Pushed)
		}
		if !sameVersions(remoteBefore, versions(t, f.app.store, "ticket")) ||
			!sameVersions(localBefore, versions(t, st, "item")) {

			t.Fatalf("%s moved a version", what)
		}
		if len(f.sink.Records()) != remoteAudit || len(localAudit.Records()) != localAuditN {
			t.Fatalf("%s wrote audit rows: remote %d -> %d, local %d -> %d", what, remoteAudit,
				len(f.sink.Records()), localAuditN, len(localAudit.Records()))
		}
	}

	// Both sides change the same field: hold writes neither.
	if _, err = mgr.PatchEntity(ctx, one.ID, entity.Patch{Properties: map[string]any{"title": "one, ours"}}); err != nil {
		t.Fatal(err)
	}
	got, err = f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-001"})
	if err != nil {
		t.Fatal(err)
	}
	got.Properties["title"] = "one, theirs"
	if err = f.app.store.UpdateEntity(ctx, got); err != nil {
		t.Fatal(err)
	}
	rep = run("conflict sync")
	if rep.Entities.Conflicts != 1 || rep.Held != 1 || rep.Pushed.Updated != 0 {
		t.Fatalf("conflict sync: %+v %+v", rep, rep.Pushed)
	}
	if got, _ := f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-001"}); got.Properties["title"] != "one, theirs" {
		t.Fatalf("remote title = %v", got.Properties["title"])
	}
	if got, _ := st.GetEntity(ctx, one.Ref()); got.Properties["title"] != "one, ours" {
		t.Fatalf("local title = %v", got.Properties["title"])
	}
	list, err := remotesync.LoadConflicts(ctx, kv, "acme")
	if err != nil || len(list) != 1 || list[0].Field != "title" || list[0].RemoteValue != "one, theirs" {
		t.Fatalf("conflict list: %+v, %v", list, err)
	}

	// Resolved to the local side, the next sync sends it.
	if _, err := remotesync.Resolve(ctx, kv, m, one.ID, "title", remotesync.TakeLocal); err != nil {
		t.Fatal(err)
	}
	if rep := run("resolved sync"); rep.Pushed.Updated != 1 || rep.Held != 0 {
		t.Fatalf("resolved sync: %+v %+v", rep, rep.Pushed)
	}
	if got, _ := f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-001"}); got.Properties["title"] != "one, ours" {
		t.Fatalf("remote title after resolve = %v", got.Properties["title"])
	}
}
