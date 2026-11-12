package remotesync

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestLink_SettlesAnUncertainCreate pins the operator path out of an
// uncertain create whose row did reach the remote: linking the two rows
// sends nothing again, and the next sync treats the pair as a pushed row.
func TestLink_SettlesAnUncertainCreate(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local := newRow(t, tg, "requirement", map[string]any{"title": "Maybe"})
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPost {
			return http.StatusBadGateway, "upstream timeout"
		}
		return 0, ""
	}
	if _, err := Run(ctx, m, tg.pushDeps(src), Options{}); err == nil {
		t.Fatal("uncertain create: want a failed run")
	}
	// The create did reach the remote.
	src.before = nil
	src.put(&fakeEnt{Type: "req", ID: "N9", Props: map[string]any{"title": "Maybe"}})
	r1 := tg.byRef(t, "R1")

	list, _ := LoadConflicts(ctx, tg.kv, "cat")
	for _, tc := range []struct {
		name, local, address string
		noEntry              bool
	}{
		{name: "no entry", local: r1.ID, address: "req/N9", noEntry: true},
		{name: "other type", local: local, address: "component/N9"},
		{name: "no id", local: local, address: "req"},
		{name: "bad id", local: local, address: "req/../x"},
		{name: "linked address", local: local, address: "req/R1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Link(ctx, tg.kv, m, Target{Local: tc.local}, tc.address)
			if err == nil || errors.Is(err, ErrNoConflict) != tc.noEntry || errors.Is(err, ErrRefused) == tc.noEntry {
				t.Fatalf("Link(%s, %s) = %v", tc.local, tc.address, err)
			}
			if tc.noEntry != (CheckLink(list, Target{Local: tc.local}) != nil) {
				t.Fatalf("CheckLink disagrees with Link for %s", tc.local)
			}
		})
	}
	if err := Link(ctx, tg.kv, nil, Target{Local: local}, "req/N9"); err == nil {
		t.Fatal("Link without a mapping")
	}
	// A caller that checked another field set is refused, for a link and
	// for a resolve: the entry is not the one it checked.
	stale := Target{Local: local, Exact: true, Fields: []string{"title"}}
	if err := Link(ctx, tg.kv, m, stale, "req/N9"); !errors.Is(err, ErrRefused) {
		t.Fatalf("Link with a stale field set: %v", err)
	}
	if _, err := CheckResolve(list, stale, TakeLocal); !errors.Is(err, ErrRefused) {
		t.Fatalf("CheckResolve with a stale field set: %v", err)
	}
	var sent []string
	for i := range list {
		if list[i].Local == local {
			sent = list[i].RefusedLocalFields()
		}
	}
	if len(sent) < 2 {
		t.Fatalf("the create's fields: %v", sent)
	}
	checked := Target{Local: local, Exact: true, Fields: append(slices.Clone(sent), sent[0])}
	if _, err := CheckResolve(list, checked, TakeLocal); err != nil {
		t.Fatalf("CheckResolve with the checked field set: %v", err)
	}

	if err := Link(ctx, tg.kv, m, Target{Local: local}, "req/N9"); err != nil {
		t.Fatal(err)
	}
	if got := heldReasons(t, tg); len(got) != 0 {
		t.Fatalf("the entry stayed: %v", got)
	}
	linked, err := LoadLinked(ctx, tg.kv, m)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range linked {
		found = found || l.Local == local && l.Type == "requirement" && l.Kind == ""
	}
	if !found {
		t.Fatalf("not linked: %+v", linked)
	}
	if err := Link(ctx, tg.kv, m, Target{Local: local}, "req/N9"); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("a second link: %v", err)
	}

	src.sends = nil
	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Created != 0 || rep.Held != 0 {
		t.Fatalf("after the link: %+v held %d", rep.Pushed, rep.Held)
	}
	if got := tg.byRef(t, "N9"); got.ID != local {
		t.Fatalf("the local ref was not set: %s", got.ID)
	}
	if _, err := LoadLinked(ctx, tg.kv, nil); err == nil {
		t.Fatal("LoadLinked without a mapping")
	}
}

func TestLink_RefusesAnAlreadyLinkedRow(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	// An uncertain entry of a row the ledger already links, as a hand-edited
	// list could hold.
	c, err := loadConflicts(ctx, tg.kv, "cat")
	if err != nil {
		t.Fatal(err)
	}
	c.entries[conflictKey(r1.ID, "")] = &Conflict{Local: r1.ID, Type: "requirement", Reason: ReasonUncertain,
		Since: time.Now(), Seen: time.Now()}
	if err := c.save(ctx, tg.kv); err != nil {
		t.Fatal(err)
	}
	src.put(&fakeEnt{Type: "req", ID: "N9", Props: map[string]any{"title": "x"}})
	if err := Link(ctx, tg.kv, m, Target{Local: r1.ID}, "req/N9"); err == nil {
		t.Fatal("linked a row the ledger already links")
	}
}

func TestCauseOfError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{context.Canceled, CauseCancelled},
		{context.DeadlineExceeded, CauseTimeout},
		{errors.New("reset"), CauseNoAnswer},
	} {
		if got := causeOfError(tc.err); got != tc.want {
			t.Errorf("causeOfError(%v) = %s, want %s", tc.err, got, tc.want)
		}
	}
}

func TestConflictRelationKey(t *testing.T) {
	for _, tc := range []struct {
		c    Conflict
		want entity.RelationKey
		ok   bool
	}{
		{Conflict{Kind: KindRelation, Local: "A-1--refines--B-2"},
			entity.RelationKey{From: "A-1", Type: "refines", To: "B-2"}, true},
		{Conflict{Local: "A-1--refines--B-2"}, entity.RelationKey{}, false},
		{Conflict{Kind: KindRelation, Local: "A-1--refines"}, entity.RelationKey{}, false},
		{Conflict{Kind: KindRelation, Local: "A-1----B-2"}, entity.RelationKey{}, false},
	} {
		got, ok := tc.c.RelationKey()
		if ok != tc.ok || got != tc.want {
			t.Errorf("RelationKey(%q) = %+v, %v", tc.c.Local, got, ok)
		}
	}
}

func TestTargetExact(t *testing.T) {
	row := &Conflict{Local: "A"}
	field := &Conflict{Local: "A", Field: "title"}
	for _, tc := range []struct {
		t          Target
		row, field bool
	}{
		{Target{Local: "A"}, true, true},
		{Target{Local: "A", Exact: true}, true, false},
		{Target{Local: "A", Field: "title"}, false, true},
		{Target{Local: "A", Field: "title", Exact: true}, false, true},
		{Target{Local: "B"}, false, false},
	} {
		if tc.t.names(row) != tc.row || tc.t.names(field) != tc.field {
			t.Errorf("%+v: row %v field %v", tc.t, tc.t.names(row), tc.t.names(field))
		}
	}
}

func TestSettings(t *testing.T) {
	meta := testMeta(t)
	m := testMapping(t, meta, nil)
	s := m.Remote.Settings()
	if s.URL != "https://catalog.example" || s.Conflict != ConflictHold || s.Push || s.CredentialKind == "" {
		t.Fatalf("settings: %+v", s)
	}
}

func TestRefusedLocalFields(t *testing.T) {
	c := Conflict{RefusedFields: map[string]string{"title<title": "a", "status<state": "b", "content<content": "c"}}
	got := c.RefusedLocalFields()
	if len(got) != 3 || got[0] != "content" || got[1] != "status" || got[2] != "title" {
		t.Fatalf("RefusedLocalFields = %v", got)
	}
	if got := (&Conflict{}).RefusedLocalFields(); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
}
