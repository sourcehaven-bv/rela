package dataentry

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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

// deleteSync is a local project that syncs with a [pushRemote] with
// send_deletes on.
type deleteSync struct {
	m    *remotesync.Mapping
	deps remotesync.Deps
	st   store.Store
	mgr  *entitymanager.Manager
	kv   *pullKV
}

func newDeleteSync(t *testing.T, f *pushRemote) *deleteSync {
	t.Helper()
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
    send_deletes: true
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
	return &deleteSync{m: m, st: st, mgr: mgr, kv: kv, deps: remotesync.Deps{
		Source: src, Pusher: src, Meta: meta, Writer: mgr, Store: st, State: kv,
		Gate: gate, Fingerprint: visibility.SumHeld, Logger: slog.New(slog.DiscardHandler), RunID: "e2e",
		RelationMetaGated: func(relType string) bool { return visibility.RelationMetaGatedBy(gate, relType) },
	}}
}

func (d *deleteSync) run(t *testing.T, what string, allowGone int) remotesync.Report {
	t.Helper()
	rep, err := remotesync.Run(context.Background(), d.m, d.deps, remotesync.Options{AllowGone: allowGone})
	if err != nil {
		t.Fatalf("%s: %v (%+v)", what, err, rep)
	}
	return rep
}

// TestRemoteDelete_EndToEnd: with send_deletes, a local delete reaches a
// real data-entry server through its DELETE route with If-Match, relations
// first; a row the remote changed since the last sync is held, not deleted;
// and a remote whose ACL denies the delete holds the row as refused.
func TestRemoteDelete_EndToEnd(t *testing.T) {
	remoteGone := func(t *testing.T, f *pushRemote, id string) bool {
		t.Helper()
		_, err := f.app.store.GetEntity(context.Background(), entity.Ref{ID: id})
		return errors.Is(err, store.ErrNotFound)
	}
	t.Run("sends", func(t *testing.T) {
		f := newPushRemoteAs(t, acl.RoleDef{Read: []string{"ticket"}, Create: []string{"ticket"},
			Update: []string{"ticket"}, Delete: []string{"ticket"}})
		seedRelation(f.app, &entity.Relation{From: "TKT-001", Type: "blocks", To: "TKT-002"})
		d := newDeleteSync(t, f)
		d.run(t, "first sync", 0)
		two := pulledByTitle(t, d.st, "two")
		if _, err := d.mgr.DeleteEntity(context.Background(), two.ID, true); err != nil {
			t.Fatal(err)
		}
		// The only relation goes with the entity, which empties the type
		// here, so the guard asks for confirmation.
		rep, err := remotesync.Run(context.Background(), d.m, d.deps, remotesync.Options{})
		if !errors.Is(err, remotesync.ErrGoneGuard) || rep.Pushed.Deleted != 0 || remoteGone(t, f, "TKT-002") {
			t.Fatalf("guard: %v %+v", err, rep.Pushed)
		}
		rep = d.run(t, "delete sync", 2)
		if rep.Pushed.Deleted != 1 || rep.PushedRelations.Deleted != 1 || !remoteGone(t, f, "TKT-002") {
			t.Fatalf("delete sync: %+v %+v %+v", rep.Pushed, rep.PushedRelations, rep.Issues)
		}
		if remoteGone(t, f, "TKT-001") {
			t.Fatal("the other ticket was deleted")
		}

		// The remote changes TKT-001 after the last sync; the local delete
		// is held.
		one := pulledByTitle(t, d.st, "one")
		got, err := f.app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-001"})
		if err != nil {
			t.Fatal(err)
		}
		got.Properties["status"] = "closed"
		if err = f.app.store.UpdateEntity(context.Background(), got); err != nil {
			t.Fatal(err)
		}
		if _, err = d.mgr.DeleteEntity(context.Background(), one.ID, true); err != nil {
			t.Fatal(err)
		}
		rep = d.run(t, "held sync", 1)
		list, err := remotesync.LoadConflicts(context.Background(), d.kv, "acme")
		if err != nil || rep.Pushed.Conflicts != 1 || len(list) != 1 ||
			list[0].Reason != remotesync.ReasonLocalDeleted || remoteGone(t, f, "TKT-001") {

			t.Fatalf("held sync: %+v, list %+v, err %v", rep.Pushed, list, err)
		}
	})
	t.Run("refused", func(t *testing.T) {
		f := newPushRemote(t)
		d := newDeleteSync(t, f)
		d.run(t, "first sync", 0)
		if _, err := d.mgr.DeleteEntity(context.Background(), pulledByTitle(t, d.st, "two").ID, true); err != nil {
			t.Fatal(err)
		}
		rep := d.run(t, "delete sync", 0)
		list, err := remotesync.LoadConflicts(context.Background(), d.kv, "acme")
		if err != nil || rep.Pushed.Refused != 1 || len(list) != 1 || list[0].Cause != remotesync.CauseRefused ||
			list[0].Status != 403 || remoteGone(t, f, "TKT-002") {

			t.Fatalf("refused: %+v, list %+v, err %v", rep.Pushed, list, err)
		}
	})
}
