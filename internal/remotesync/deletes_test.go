package remotesync

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// deleteMapping is a remote with push and send_deletes.
func deleteMapping(t *testing.T, tg *target, edit func(p *Remote)) *Mapping {
	t.Helper()
	return pushMapping(t, tg, func(p *Remote) {
		p.SendDeletes = true
		if edit != nil {
			edit(p)
		}
	})
}

// deleteLocal deletes the local row linked to source id, with its relations.
func deleteLocal(t *testing.T, tg *target, id string) string {
	t.Helper()
	e := tg.byRef(t, id)
	if e == nil {
		t.Fatalf("no local row for %s", id)
	}
	if _, err := tg.mgr.DeleteEntity(context.Background(), e.ID, true); err != nil {
		t.Fatal(err)
	}
	return e.ID
}

// ledgerState returns the ledger state of the row for req/R2.
func ledgerState(t *testing.T, tg *target, m *Mapping) string {
	t.Helper()
	const addr = "req/R2"
	l, err := loadLedger(context.Background(), tg.kv, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"E", "R"} {
		if row := l.rows[kind+addr]; row != nil {
			return row.State
		}
	}
	return ""
}

func TestDeletes_SendsLocalDelete(t *testing.T) {
	tg := newTarget(t)
	m := deleteMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})

	deleteLocal(t, tg, "R2")
	rep := runSync(t, tg, m, src, Options{DryRun: true})
	if rep.Pushed.Deleted != 1 || rep.Deleting != 1 || src.ents["R2"] == nil {
		t.Fatalf("dry run: %+v, remote has R2: %v", rep.Pushed, src.ents["R2"] != nil)
	}
	rep = runSync(t, tg, m, src, Options{})
	if rep.Pushed.Deleted != 1 || src.ents["R2"] != nil {
		t.Fatalf("delete: %+v, remote has R2: %v", rep.Pushed, src.ents["R2"] != nil)
	}
	if got := ledgerState(t, tg, m); got != "" {
		t.Fatalf("the ledger keeps the deleted row as %q", got)
	}
	if !strings.Contains(strings.Join(src.sends, "\n"), "DELETE reqs/R2") {
		t.Fatalf("sends: %v", src.sends)
	}
	rep = runSync(t, tg, m, src, Options{})
	if rep.Pushed.Deleted != 0 || rep.Entities.Created != 0 || tg.byRef(t, "R2") != nil {
		t.Fatalf("not idempotent: %+v %+v", rep.Entities, rep.Pushed)
	}
}

// A local relation delete is sent too, and an entity's relations go before
// it.
func TestDeletes_RelationsFirst(t *testing.T) {
	tg := newTarget(t)
	m := deleteMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})

	deleteLocal(t, tg, "R1")
	rep := runSync(t, tg, m, src, Options{AllowGone: 2})
	if rep.Pushed.Deleted != 1 || rep.PushedRelations.Deleted != 1 {
		t.Fatalf("deleted: %+v %+v", rep.Pushed, rep.PushedRelations)
	}
	var deletes []string
	for _, s := range src.sends {
		if strings.HasPrefix(s, http.MethodDelete) {
			deletes = append(deletes, s)
		}
	}
	if len(deletes) != 2 || !strings.Contains(deletes[0], "/relations/") {
		t.Fatalf("deletes: %v", deletes)
	}
}

func TestDeletes_OffByDefault(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})

	deleteLocal(t, tg, "R2")
	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Deleted != 0 || src.ents["R2"] == nil || tg.byRef(t, "R2") != nil {
		t.Fatalf("%+v", rep.Pushed)
	}
	if got := ledgerState(t, tg, m); got != stateDropped {
		t.Fatalf("state %q", got)
	}
	// A row dropped before send_deletes was turned on stays dropped.
	m = deleteMapping(t, tg, nil)
	if rep = runSync(t, tg, m, src, Options{}); rep.Pushed.Deleted != 0 || src.ents["R2"] == nil {
		t.Fatalf("a dropped row was sent: %+v", rep.Pushed)
	}
}

// A deleting row is dropped, not sent, when send_deletes is turned off
// before the push.
func TestDeletes_TurnedOff(t *testing.T) {
	tg := newTarget(t)
	m := deleteMapping(t, tg, func(p *Remote) { p.MaxGone = new(int) })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "R2")
	if _, err := Run(context.Background(), m, tg.pushDeps(src), Options{}); !errors.Is(err, ErrGoneGuard) {
		t.Fatalf("err %v", err)
	}
	if got := ledgerState(t, tg, m); got != stateDeleting {
		t.Fatalf("state %q", got)
	}
	m = pushMapping(t, tg, nil)
	rep := runSync(t, tg, m, src, Options{})
	if got := ledgerState(t, tg, m); got != stateDropped || src.ents["R2"] == nil {
		t.Fatalf("state %q", got)
	}
	if !hasKeptIssue(rep, "send_deletes is off") {
		t.Fatalf("the dropped delete is not reported: %+v", rep.Issues)
	}
}

// A delete that waits longer than deleteRetention is dropped with its
// digests, the remote row is left alone, and the report says so.
func TestDeletes_RetentionDropsWaitingDelete(t *testing.T) {
	tg := newTarget(t)
	m := deleteMapping(t, tg, func(p *Remote) { p.MaxGone = new(int) })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "R2")
	if _, err := Run(context.Background(), m, tg.pushDeps(src), Options{}); !errors.Is(err, ErrGoneGuard) {
		t.Fatalf("err %v", err)
	}
	d := tg.pushDeps(src)
	d.Now = func() time.Time { return time.Now().Add(deleteRetention + time.Hour) }
	rep, err := Run(context.Background(), m, d, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := ledgerState(t, tg, m); got != stateDropped || src.ents["R2"] == nil {
		t.Fatalf("state %q", got)
	}
	if !hasKeptIssue(rep, "30 days") {
		t.Fatalf("the expired delete is not reported: %+v", rep.Issues)
	}
	l, err := loadLedger(context.Background(), tg.kv, m)
	if err != nil {
		t.Fatal(err)
	}
	if row := l.rows["Ereq/R2"]; row == nil || row.Base != nil {
		t.Fatalf("digests outlive the expired delete: %+v", row)
	}
}

func hasKeptIssue(rep Report, why string) bool {
	for _, is := range rep.Issues {
		if is.Kind == IssueKept && strings.Contains(is.Reason, why) {
			return true
		}
	}
	return false
}

// A row deleted on both sides is forgotten without a request.
func TestDeletes_GoneOnBothSides(t *testing.T) {
	tg := newTarget(t)
	m := deleteMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "R2")
	delete(src.ents, "R2")
	src.sends = nil
	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Deleted != 0 || rep.Deleting != 0 || src.writes() != 0 {
		t.Fatalf("%+v, sends %v", rep.Pushed, src.sends)
	}
	if got := ledgerState(t, tg, m); got != "" {
		t.Fatalf("state %q", got)
	}
}

// A remote edit since the last sync holds the delete under hold; resolving
// to local deletes the remote row, to remote restores the row here.
func TestDeletes_RemoteEditHolds(t *testing.T) {
	for _, take := range []string{TakeLocal, TakeRemote} {
		t.Run(take, func(t *testing.T) {
			ctx := context.Background()
			tg := newTarget(t)
			m := deleteMapping(t, tg, nil)
			src := newPushSource(seedSource())
			runSync(t, tg, m, src, Options{})
			local := deleteLocal(t, tg, "R2")
			src.ents["R2"].Props["title"] = "Logout now"

			rep := runSync(t, tg, m, src, Options{})
			if rep.Pushed.Conflicts != 1 || src.ents["R2"] == nil {
				t.Fatalf("%+v", rep.Pushed)
			}
			list, err := LoadConflicts(ctx, tg.kv, "cat")
			if err != nil || len(list) != 1 || list[0].Reason != ReasonLocalDeleted || list[0].Cause != CauseEdited ||
				list[0].Local != local {

				t.Fatalf("list %+v, err %v", list, err)
			}
			// Held: the next run sends nothing.
			if rep = runSync(t, tg, m, src, Options{}); rep.Pushed.Deleted != 0 || len(heldReasons(t, tg)) != 1 {
				t.Fatalf("%+v %v", rep.Pushed, heldReasons(t, tg))
			}

			if n, err := Resolve(ctx, tg.kv, m, Target{Local: local}, take); err != nil || n != 1 {
				t.Fatalf("resolve: %d %v", n, err)
			}
			rep = runSync(t, tg, m, src, Options{})
			if take == TakeLocal {
				if rep.Pushed.Deleted != 1 || src.ents["R2"] != nil {
					t.Fatalf("%+v", rep.Pushed)
				}
				return
			}
			if rep.Entities.Created != 1 || src.ents["R2"] == nil {
				t.Fatalf("%+v", rep.Entities)
			}
			if got := tg.byRef(t, "R2"); got == nil || got.Properties["title"] != "Logout now" {
				t.Fatalf("restored %+v", got)
			}
			if len(heldReasons(t, tg)) != 0 {
				t.Fatalf("held: %v", heldReasons(t, tg))
			}
		})
	}
}

func TestDeletes_RemoteEditRules(t *testing.T) {
	for _, rule := range []string{ConflictLocal, ConflictRemote, ConflictLatest} {
		t.Run(rule, func(t *testing.T) {
			tg := newTarget(t)
			m := deleteMapping(t, tg, func(p *Remote) { p.Conflict = rule })
			src := newPushSource(seedSource())
			runSync(t, tg, m, src, Options{})
			deleteLocal(t, tg, "R2")
			src.ents["R2"].Props["title"] = "Logout now"

			rep := runSync(t, tg, m, src, Options{})
			switch rule {
			case ConflictLocal:
				if rep.Pushed.Deleted != 1 || src.ents["R2"] != nil {
					t.Fatalf("%+v", rep.Pushed)
				}
			case ConflictRemote:
				if rep.Pushed.Deleted != 0 || src.ents["R2"] == nil || len(heldReasons(t, tg)) != 0 {
					t.Fatalf("%+v", rep.Pushed)
				}
				runSync(t, tg, m, src, Options{})
				if tg.byRef(t, "R2") == nil {
					t.Fatal("not restored here")
				}
			default:
				if rep.Pushed.Conflicts != 1 || len(heldReasons(t, tg)) != 1 {
					t.Fatalf("latest must hold: %+v", rep.Pushed)
				}
			}
		})
	}
}

// A remote that refuses the delete holds it; a remote that changes the row
// between the fetch and the delete gets one more fetch, which finds the
// edit.
func TestDeletes_RefusedAndRaced(t *testing.T) {
	t.Run("refused", func(t *testing.T) {
		tg := newTarget(t)
		m := deleteMapping(t, tg, nil)
		src := newPushSource(seedSource())
		runSync(t, tg, m, src, Options{})
		local := deleteLocal(t, tg, "R2")
		src.before = func(method, _ string) (int, string) {
			if method == http.MethodDelete {
				return http.StatusForbidden, "no"
			}
			return 0, ""
		}
		rep := runSync(t, tg, m, src, Options{})
		list, _ := LoadConflicts(context.Background(), tg.kv, "cat")
		if rep.Pushed.Refused != 1 || len(list) != 1 || list[0].Cause != CauseRefused || list[0].Local != local {
			t.Fatalf("%+v %+v", rep.Pushed, list)
		}
	})
	t.Run("raced", func(t *testing.T) {
		tg := newTarget(t)
		m := deleteMapping(t, tg, nil)
		src := newPushSource(seedSource())
		runSync(t, tg, m, src, Options{})
		deleteLocal(t, tg, "R2")
		src.before = func(method, _ string) (int, string) {
			if method == http.MethodDelete {
				src.mu.Lock()
				src.ents["R2"].Props["title"] = "Raced"
				src.mu.Unlock()
				src.before = nil
			}
			return 0, ""
		}
		rep := runSync(t, tg, m, src, Options{})
		if rep.Pushed.Conflicts != 1 || src.ents["R2"] == nil || heldReasons(t, tg) == nil {
			t.Fatalf("%+v", rep.Pushed)
		}
	})
}

// Over the guard, the deletes wait and the rest of the run applies.
func TestDeletes_GuardHoldsOnlyDeletes(t *testing.T) {
	tg := newTarget(t)
	one := 1
	m := deleteMapping(t, tg, func(p *Remote) { p.MaxGone = &one })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "R2")
	deleteLocal(t, tg, "C2")
	src.ents["C1"].Props["name"] = "Authn"

	rep, err := Run(context.Background(), m, tg.pushDeps(src), Options{})
	if !errors.Is(err, ErrGoneGuard) || !strings.Contains(rep.Refused, "--allow-gone 3") {
		t.Fatalf("err %v, report %+v", err, rep)
	}
	if src.ents["R2"] == nil || src.ents["C2"] == nil {
		t.Fatal("a held delete was sent")
	}
	if got := tg.byRef(t, "C1").Properties["name"]; got != "Authn" {
		t.Fatalf("the pull was not applied: %v", got)
	}
	rep = runSync(t, tg, m, src, Options{AllowGone: 3})
	if rep.Pushed.Deleted != 2 || rep.PushedRelations.Deleted != 1 {
		t.Fatalf("%+v %+v", rep.Pushed, rep.PushedRelations)
	}
}

// Deletes count against max_gone in the 24-hour window with the pull's
// deletes.
func TestDeletes_Window(t *testing.T) {
	tg := newTarget(t)
	two := 2
	m := deleteMapping(t, tg, func(p *Remote) { p.MaxGone = &two })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "R2")
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "C2")
	rep, err := Run(context.Background(), m, tg.pushDeps(src), Options{})
	if !errors.Is(err, ErrGoneGuard) || !strings.Contains(rep.Refused, "in the last") {
		t.Fatalf("err %v, report %+v", err, rep)
	}
}

func TestDeletes_EmptiedHereIsRefused(t *testing.T) {
	tg := newTarget(t)
	m := deleteMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	deleteLocal(t, tg, "R1")
	deleteLocal(t, tg, "R2")
	rep, err := Run(context.Background(), m, tg.pushDeps(src), Options{})
	if !errors.Is(err, ErrGoneGuard) || !strings.Contains(rep.Refused, `entity type "req"`) {
		t.Fatalf("err %v, report %+v", err, rep)
	}
	if rep = runSync(t, tg, m, src, Options{AllowGone: 3}); rep.Pushed.Deleted != 2 {
		t.Fatalf("%+v", rep.Pushed)
	}
}

// A deleting row whose local row is back before the push is linked again.
func TestDeletes_RestoredBeforeSend(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := deleteMapping(t, tg, func(p *Remote) { p.MaxGone = new(int) })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	e := tg.byRef(t, "R2")
	deleteLocal(t, tg, "R2")
	if _, err := Run(ctx, m, tg.pushDeps(src), Options{}); !errors.Is(err, ErrGoneGuard) {
		t.Fatalf("err %v", err)
	}
	if err := tg.st.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	runSync(t, tg, m, src, Options{})
	if got := ledgerState(t, tg, m); got != stateLinked || src.ents["R2"] == nil {
		t.Fatalf("state %q", got)
	}
}

// On a remote with push, a row the remote deleted that was edited here
// follows the row rule. Under hold it is kept and held; resolving to local
// creates it on the remote again, to remote deletes it here.
func TestDeletes_RemoteDeleteOfEditedRow(t *testing.T) {
	for _, take := range []string{TakeLocal, TakeRemote} {
		t.Run(take, func(t *testing.T) {
			ctx := context.Background()
			tg := newTarget(t)
			m := pushMapping(t, tg, func(p *Remote) { p.OnGone = OnGoneDelete })
			src := newPushSource(seedSource())
			runSync(t, tg, m, src, Options{})
			local := tg.byRef(t, "R2").ID
			patch(t, tg, local, map[string]any{"title": "Ours"})
			delete(src.ents, "R2")

			rep := runSync(t, tg, m, src, Options{})
			if rep.Entities.Conflicts != 1 || heldReasons(t, tg)[local] != ReasonRemoteDeleted {
				t.Fatalf("%+v %v", rep.Entities, heldReasons(t, tg))
			}
			if n, err := Resolve(ctx, tg.kv, m, Target{Local: local}, take); err != nil || n != 1 {
				t.Fatalf("resolve: %d %v", n, err)
			}
			rep = runSync(t, tg, m, src, Options{})
			_, err := tg.st.GetEntity(ctx, entity.Ref{ID: local})
			if take == TakeRemote {
				if rep.Entities.Deleted != 1 || !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("%+v %v", rep.Entities, err)
				}
				return
			}
			if rep.Pushed.Created != 1 || err != nil {
				t.Fatalf("%+v %v", rep.Pushed, err)
			}
			if e := tg.byRef(t, "N1"); e == nil || e.ID != local || src.ents["N1"].Props["title"] != "Ours" {
				t.Fatalf("restored on the remote as %+v", src.ents["N1"])
			}
			if rep = runSync(t, tg, m, src, Options{}); rep.Pushed.Created != 0 || len(heldReasons(t, tg)) != 0 {
				t.Fatalf("%+v %v", rep.Pushed, heldReasons(t, tg))
			}
		})
	}
}

func TestDeletes_RemoteDeleteOfEditedRowRules(t *testing.T) {
	for _, rule := range []string{ConflictLocal, ConflictRemote} {
		t.Run(rule, func(t *testing.T) {
			ctx := context.Background()
			tg := newTarget(t)
			m := pushMapping(t, tg, func(p *Remote) { p.OnGone, p.Conflict = OnGoneDelete, rule })
			src := newPushSource(seedSource())
			runSync(t, tg, m, src, Options{})
			local := tg.byRef(t, "R2").ID
			patch(t, tg, local, map[string]any{"title": "Ours"})
			delete(src.ents, "R2")

			rep := runSync(t, tg, m, src, Options{})
			_, err := tg.st.GetEntity(ctx, entity.Ref{ID: local})
			if rule == ConflictRemote {
				if rep.Entities.Deleted != 1 || !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("%+v %v", rep.Entities, err)
				}
				return
			}
			if rep.Pushed.Created != 1 || err != nil || src.ents["N1"] == nil {
				t.Fatalf("%+v %v", rep.Pushed, err)
			}
		})
	}
}

func TestDeletes_NeedsPush(t *testing.T) {
	tg := newTarget(t)
	cfg, err := Parse([]byte(testRemotes), tg.meta)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Remotes["cat"]
	p.SendDeletes = true
	if _, err := Compile("cat", p, tg.meta); err == nil || !strings.Contains(err.Error(), "send_deletes needs push") {
		t.Fatalf("err %v", err)
	}
}

func countDeletes(src *pushSource) int {
	n := 0
	for _, s := range src.sends {
		if strings.HasPrefix(s, http.MethodDelete) {
			n++
		}
	}
	return n
}

// A held delete is not sent again and does not count against the guard
// until it is resolved to the local side.
func TestDeletes_HeldDeleteWaits(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := deleteMapping(t, tg, func(p *Remote) { p.MaxGone = new(int) })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local := deleteLocal(t, tg, "R2")
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodDelete {
			return http.StatusForbidden, "no"
		}
		return 0, ""
	}
	if rep := runSync(t, tg, m, src, Options{AllowGone: 1}); rep.Pushed.Refused != 1 {
		t.Fatalf("%+v", rep.Pushed)
	}
	src.sends = nil
	rep := runSync(t, tg, m, src, Options{})
	if countDeletes(src) != 0 || rep.Pushed.Refused != 0 || rep.Deleting != 0 || len(heldReasons(t, tg)) != 1 {
		t.Fatalf("a held delete was sent again: %+v, sends %v", rep.Pushed, src.sends)
	}
	src.before = nil
	if n, err := Resolve(ctx, tg.kv, m, Target{Local: local}, TakeLocal); err != nil || n != 1 {
		t.Fatalf("resolve: %d %v", n, err)
	}
	if rep = runSync(t, tg, m, src, Options{AllowGone: 1}); rep.Pushed.Deleted != 1 || src.ents["R2"] != nil {
		t.Fatalf("%+v", rep.Pushed)
	}
}

// A choice to delete a row here survives a run the guard held.
func TestDeletes_ResolveRemoteSurvivesGuard(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) { p.OnGone, p.MaxGone = OnGoneDelete, new(int) })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local := tg.byRef(t, "R2").ID
	patch(t, tg, local, map[string]any{"title": "Ours"})
	delete(src.ents, "R2")
	runSync(t, tg, m, src, Options{AllowGone: 1})
	if n, err := Resolve(ctx, tg.kv, m, Target{Local: local}, TakeRemote); err != nil || n != 1 {
		t.Fatalf("resolve: %d %v", n, err)
	}
	if _, err := Run(ctx, m, tg.pushDeps(src), Options{}); !errors.Is(err, ErrGoneGuard) {
		t.Fatalf("err %v", err)
	}
	rep := runSync(t, tg, m, src, Options{AllowGone: 1})
	_, err := tg.st.GetEntity(ctx, entity.Ref{ID: local})
	if rep.Entities.Deleted != 1 || !errors.Is(err, store.ErrNotFound) || len(heldReasons(t, tg)) != 0 {
		t.Fatalf("%+v %v %v", rep.Entities, err, heldReasons(t, tg))
	}
}

// A relation the remote deleted that was edited here is created on the
// remote again under the local rule, when both its ends are still linked.
// When an end is gone too, it is held, and resolving it to local is
// refused.
func TestDeletes_RemoteDeletedRelation(t *testing.T) {
	setup := func(t *testing.T) (*target, *Mapping, *pushSource, entity.RelationKey) {
		t.Helper()
		tg := newTarget(t)
		m := pushMapping(t, tg, func(p *Remote) { p.OnGone, p.Conflict = OnGoneDelete, ConflictLocal })
		src := newPushSource(seedSource())
		runSync(t, tg, m, src, Options{})
		key := relKey(t, tg, "R1", "C1")
		if _, err := tg.mgr.UpdateRelation(context.Background(), key, entity.RelationOptions{
			Properties: map[string]any{"weight": 7},
		}); err != nil {
			t.Fatal(err)
		}
		return tg, m, src, key
	}
	t.Run("ends linked", func(t *testing.T) {
		tg, m, src, _ := setup(t)
		delete(src.rels, "R1--satisfies--C1")
		rep := runSync(t, tg, m, src, Options{AllowGone: 1})
		if rep.PushedRelations.Created != 1 || src.rels["R1--satisfies--C1"] == nil {
			t.Fatalf("%+v %+v", rep.Relations, rep.PushedRelations)
		}
		if rep = runSync(t, tg, m, src, Options{}); rep.PushedRelations.Created != 0 || len(heldReasons(t, tg)) != 0 {
			t.Fatalf("%+v %v", rep.PushedRelations, heldReasons(t, tg))
		}
	})
	t.Run("end gone", func(t *testing.T) {
		tg, m, src, key := setup(t)
		delete(src.ents, "C1")
		delete(src.rels, "R1--satisfies--C1")
		delete(src.rels, "C2--depends--C1")
		rep := runSync(t, tg, m, src, Options{AllowGone: 3})
		if rep.PushedRelations.Created != 0 || heldReasons(t, tg)[key.String()] != ReasonRemoteDeleted {
			t.Fatalf("%+v %v", rep.PushedRelations, heldReasons(t, tg))
		}
		_, err := Resolve(context.Background(), tg.kv, m, Target{Local: key.String()}, TakeLocal)
		if !errors.Is(err, ErrRefused) {
			t.Fatalf("resolve local: %v", err)
		}
	})
}
