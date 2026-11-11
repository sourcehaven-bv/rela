package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
relations:
  blocks:
    label: Blocks
    from: [item]
    to: [item]
    properties:
      weight: {type: integer}
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
	// A meta field on blocks, so the relation push has a field to merge.
	blocks := app.State().Meta.Relations["blocks"]
	blocks.Properties = map[string]metamodel.PropertyDef{"weight": {Type: "integer"}}
	app.State().Meta.Relations["blocks"] = blocks
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
		Gate:        visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope())),
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
	if err != nil || len(list) != 1 || list[0].Field != "title" || string(list[0].RemoteValue) != `"one, theirs"` {
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

// TestRemotePush_RelationsAndLatest pushes a relation and its meta to a real
// data-entry server, and settles a field both sides changed under the
// latest rule with the `_updated` times the server serves.
func TestRemotePush_RelationsAndLatest(t *testing.T) {
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
    conflict: latest
    entities:
      ticket: {type: item, ref: src, fields: {title: title, status: status}}
    relations:
      blocks: {fields: {weight: weight}}
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
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: pullTemplater{}, Audit: audit.NewMemory(), ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}
	kv := &pullKV{}
	gate := visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope()))
	deps := remotesync.Deps{
		Source: src, Pusher: src, Meta: meta, Writer: mgr, Store: st, State: kv,
		Gate: gate, Fingerprint: visibility.SumHeld, Logger: slog.New(slog.DiscardHandler), RunID: "e2e",
		RelationMetaGated: func(relType string) bool { return visibility.RelationMetaGatedBy(gate, relType) },
	}
	run := func(what string) remotesync.Report {
		t.Helper()
		rep, runErr := remotesync.Run(ctx, m, deps, remotesync.Options{})
		if runErr != nil {
			t.Fatalf("%s: %v (%+v)", what, runErr, rep)
		}
		return rep
	}
	run("first sync")
	one, two := pulledByTitle(t, st, "one"), pulledByTitle(t, st, "two")
	localKey := entity.RelationKey{From: one.ID, FromFace: one.Face, Type: "blocks", To: two.ID}

	// A new local relation between two linked rows is created on the remote.
	if _, err = mgr.CreateRelation(ctx, localKey,
		entity.RelationOptions{Properties: map[string]any{"weight": 3}}); err != nil {
		t.Fatal(err)
	}
	if rep := run("relation sync"); rep.PushedRelations == nil || rep.PushedRelations.Created != 1 {
		t.Fatalf("relation sync: %+v %+v", rep, rep.PushedRelations)
	}
	remoteWeight := func() any {
		t.Helper()
		rel, getErr := f.app.store.GetRelation(ctx,
			entity.RelationKey{From: "TKT-001", Type: "blocks", To: "TKT-002"})
		if getErr != nil {
			t.Fatal(getErr)
		}
		return rel.Properties["weight"]
	}
	if w := remoteWeight(); w != 3 && w != int64(3) && w != float64(3) {
		t.Fatalf("remote weight = %v (%T)", w, w)
	}

	// A local meta change is sent with If-Match and lands.
	if _, err = mgr.UpdateRelation(ctx, localKey,
		entity.RelationOptions{Properties: map[string]any{"weight": 5}}); err != nil {
		t.Fatal(err)
	}
	if rep := run("meta sync"); rep.PushedRelations.Updated != 1 || rep.Held != 0 {
		t.Fatalf("meta sync: %+v %+v", rep, rep.PushedRelations)
	}
	if w := remoteWeight(); w != 5 && w != int64(5) && w != float64(5) {
		t.Fatalf("remote weight after update = %v", w)
	}
	if rep := run("idle sync"); rep.PushedRelations.Updated+rep.PushedRelations.Created != 0 {
		t.Fatalf("idle sync pushed relations: %+v", rep.PushedRelations)
	}

	// Both sides change the title; the remote changes it last, so the
	// latest rule takes the remote value and keeps the local one.
	if _, err = mgr.PatchEntity(ctx, one.ID, entity.Patch{Properties: map[string]any{"title": "one, ours"}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	got, err := f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-001"})
	if err != nil {
		t.Fatal(err)
	}
	got.Properties["title"] = "one, theirs"
	if err = f.app.store.UpdateEntity(ctx, got); err != nil {
		t.Fatal(err)
	}
	rep := run("latest sync")
	if rep.Held != 0 || rep.Superseded != 1 {
		t.Fatalf("latest sync: %+v %+v", rep, rep.Issues)
	}
	if got, _ := st.GetEntity(ctx, one.Ref()); got.Properties["title"] != "one, theirs" {
		t.Fatalf("local title = %v", got.Properties["title"])
	}
	list, err := remotesync.LoadConflicts(ctx, kv, "acme")
	if err != nil || len(list) != 1 || list[0].Superseded == nil ||
		string(list[0].Superseded.LocalValue) != `"one, ours"` {

		t.Fatalf("conflict list: %+v, %v", list, err)
	}
	if n, err := remotesync.Dismiss(ctx, kv, m, one.ID, "title"); err != nil || n != 1 {
		t.Fatalf("dismiss: %d, %v", n, err)
	}
}
