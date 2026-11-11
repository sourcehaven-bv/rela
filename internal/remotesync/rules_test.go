package remotesync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestLatest(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	synced := now.Add(-time.Hour)
	at := func(d time.Duration) stamp { return stamp{At: now.Add(d)} }
	skew := 5 * time.Minute
	cases := []struct {
		name           string
		local, remote  stamp
		synced         time.Time
		winner, reason string
	}{
		{"remote later", at(-2 * time.Minute), at(-time.Minute), synced, TakeRemote, ""},
		{"local later", at(-time.Minute), at(-2 * time.Minute), synced, TakeLocal, ""},
		{"equal", at(-time.Minute), at(-time.Minute), synced, "", FallbackSameTime},
		{"remote withheld", at(-time.Minute), stamp{}, synced, "", FallbackTimeWithheld},
		{"local withheld", stamp{}, at(-time.Minute), synced, "", FallbackTimeWithheld},
		{"remote unparsable", at(-time.Minute), parseStamp("yesterday"), synced, "", FallbackTimeUntrusted},
		{"remote within skew ahead", at(-time.Minute), at(4 * time.Minute), synced, TakeRemote, ""},
		{"remote beyond skew ahead", at(-time.Minute), at(6 * time.Minute), synced, "", FallbackTimeUntrusted},
		{"local beyond skew ahead", at(6 * time.Minute), at(-time.Minute), synced, "", FallbackTimeUntrusted},
		{"remote before last sync", at(-time.Minute), at(-2 * time.Hour), synced, "", FallbackTimeUntrusted},
		{"remote just before last sync, within skew", at(-time.Minute), at(-time.Hour - 4*time.Minute), synced,
			TakeLocal, ""},
		{"no last sync", at(-time.Minute), at(-48 * time.Hour), time.Time{}, TakeLocal, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := latest(tc.local, tc.remote, tc.synced, now, skew)
			if v.winner != tc.winner || v.fallback != tc.reason {
				t.Fatalf("latest = (%q, %q), want (%q, %q)", v.winner, v.fallback, tc.winner, tc.reason)
			}
		})
	}
}

func TestDecide(t *testing.T) {
	won := verdict{winner: TakeRemote}
	held := verdict{fallback: FallbackSameTime}
	cases := []struct {
		rule           string
		v              verdict
		winner, reason string
	}{
		{ConflictHold, won, "", ""},
		{ConflictLocal, held, TakeLocal, ""},
		{ConflictRemote, held, TakeRemote, ""},
		{ConflictLatest, won, TakeRemote, ""},
		{ConflictLatest, held, "", FallbackSameTime},
	}
	for _, tc := range cases {
		if w, r := decide(tc.rule, tc.v); w != tc.winner || r != tc.reason {
			t.Errorf("decide(%s, %+v) = (%q, %q), want (%q, %q)", tc.rule, tc.v, w, r, tc.winner, tc.reason)
		}
	}
}

func stampAt(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339Nano) }

// conflictSetup syncs once with conflict rule rule, then changes the title
// on both sides, the remote's with updated as its time.
func conflictSetup(t *testing.T, rule, updated string) (*target, *Mapping, *pushSource, *entity.Entity) {
	t.Helper()
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) { p.Conflict = rule })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Ours"})
	src.ents["R1"].Props = map[string]any{"title": "Theirs", "state": "open"}
	src.ents["R1"].Updated = updated
	return tg, m, src, r1
}

func entries(t *testing.T, tg *target) []Conflict {
	t.Helper()
	list, err := LoadConflicts(context.Background(), tg.kv, "cat")
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestRules_LatestRemoteWinsAndKeepsTheLocalValue(t *testing.T) {
	ctx := context.Background()
	tg, m, src, r1 := conflictSetup(t, ConflictLatest, stampAt(time.Minute))

	rep := runSync(t, tg, m, src, Options{})
	if rep.Held != 0 || rep.Superseded != 1 || rep.Entities.Settled != 1 || rep.Entities.Conflicts != 0 {
		t.Fatalf("run: %+v", rep)
	}
	if got := tg.byRef(t, "R1").Properties["title"]; got != "Theirs" {
		t.Fatalf("local title = %v, want the remote's", got)
	}
	list := entries(t, tg)
	if len(list) != 1 || list[0].Reason != ReasonSuperseded || list[0].Superseded.Winner != TakeRemote ||
		string(list[0].Superseded.LocalValue) != `"Ours"` || list[0].Rule != ConflictLatest {

		t.Fatalf("list = %+v", list)
	}
	// The entry blocks nothing: a later local edit is pushed.
	patch(t, tg, r1.ID, map[string]any{"title": "Ours again"})
	if rep := runSync(t, tg, m, src, Options{}); rep.Pushed.Updated != 1 || src.ents["R1"].Props["title"] != "Ours again" {
		t.Fatalf("push after supersede: %+v, remote %v", rep.Pushed, src.ents["R1"].Props)
	}

	if _, err := CheckResolve(entries(t, tg), r1.ID, "title", TakeLocal); err == nil {
		t.Fatal("a superseded entry was resolvable")
	}
	if n, err := Dismiss(ctx, tg.kv, m, r1.ID, ""); err != nil || n != 1 {
		t.Fatalf("dismiss: %d, %v", n, err)
	}
	if len(entries(t, tg)) != 0 {
		t.Fatal("the dismissed entry stayed")
	}
	if _, err := Dismiss(ctx, tg.kv, m, r1.ID, ""); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("second dismiss: %v", err)
	}
}

func TestRules_LatestLocalWinsAndIsPushed(t *testing.T) {
	tg, m, src, r1 := conflictSetup(t, ConflictLatest, stampAt(-time.Minute))
	// The local edit is later than the remote's.
	patch(t, tg, r1.ID, map[string]any{"title": "Ours, later"})

	rep := runSync(t, tg, m, src, Options{})
	if rep.Held != 0 || rep.Superseded != 1 || rep.Pushed.Updated != 1 {
		t.Fatalf("run: %+v %+v", rep, rep.Pushed)
	}
	if src.ents["R1"].Props["title"] != "Ours, later" || tg.byRef(t, "R1").Properties["title"] != "Ours, later" {
		t.Fatalf("remote %v, local %v", src.ents["R1"].Props, tg.byRef(t, "R1").Properties)
	}
	list := entries(t, tg)
	if len(list) != 1 || list[0].Superseded.Winner != TakeLocal || string(list[0].RemoteValue) != `"Theirs"` {
		t.Fatalf("list = %+v", list)
	}
}

func TestRules_LatestFallsBackToHold(t *testing.T) {
	cases := map[string]struct{ updated, reason string }{
		"withheld":    {"", FallbackTimeWithheld},
		"future":      {stampAt(time.Hour), FallbackTimeUntrusted},
		"before sync": {stampAt(-time.Hour), FallbackTimeUntrusted},
		"not a time":  {"soon", FallbackTimeUntrusted},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tg, m, src, _ := conflictSetup(t, ConflictLatest, tc.updated)
			rep := runSync(t, tg, m, src, Options{})
			if rep.Held != 1 || rep.Entities.Conflicts != 1 {
				t.Fatalf("run: %+v", rep)
			}
			list := entries(t, tg)
			if len(list) != 1 || list[0].Reason != ReasonBothChanged || list[0].Fallback != tc.reason ||
				list[0].Rule != ConflictLatest {

				t.Fatalf("list = %+v", list)
			}
			if tg.byRef(t, "R1").Properties["title"] != "Ours" || src.ents["R1"].Props["title"] != "Theirs" {
				t.Fatal("a held field changed")
			}
		})
	}
}

// A local row the principal sees only in part has no usable time: the rule
// holds, as the API withholds the time from such a reader.
func TestRules_LatestHoldsAPartlyHiddenLocalRow(t *testing.T) {
	tg, m, src, _ := conflictSetup(t, ConflictLatest, stampAt(time.Minute))
	tg.gate = redactGate{inner: tg.gate, field: "note"}
	rep := runSync(t, tg, m, src, Options{})
	list := entries(t, tg)
	if rep.Held != 1 || len(list) != 1 || list[0].Fallback != FallbackTimeWithheld {
		t.Fatalf("run %+v, list %+v", rep, list)
	}
}

func TestRules_LocalAndRemote(t *testing.T) {
	for _, rule := range []string{ConflictLocal, ConflictRemote} {
		t.Run(rule, func(t *testing.T) {
			tg, m, src, _ := conflictSetup(t, rule, "")
			rep := runSync(t, tg, m, src, Options{})
			want := map[string]string{ConflictLocal: "Ours", ConflictRemote: "Theirs"}[rule]
			if rep.Held != 0 || rep.Superseded != 0 || rep.Entities.Settled != 1 {
				t.Fatalf("run: %+v", rep)
			}
			if tg.byRef(t, "R1").Properties["title"] != want || src.ents["R1"].Props["title"] != want {
				t.Fatalf("local %v, remote %v, want %s", tg.byRef(t, "R1").Properties["title"],
					src.ents["R1"].Props["title"], want)
			}
			if len(entries(t, tg)) != 0 {
				t.Fatal("a local or remote rule left an entry")
			}
		})
	}
}

func TestRules_FieldOverrideBeatsTheRemoteRule(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) {
		p.Conflict = ConflictRemote
		em := p.Entities["req"]
		em.FieldConflicts = map[string]string{"title": ConflictHold}
		p.Entities["req"] = em
	})
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Ours", "status": "done"})
	src.ents["R1"].Props = map[string]any{"title": "Theirs", "state": "open"}
	rep := runSync(t, tg, m, src, Options{})
	got := tg.byRef(t, "R1").Properties
	if rep.Held != 1 || got["title"] != "Ours" || src.ents["R1"].Props["state"] != "done" {
		t.Fatalf("run %+v, local %v", rep, got)
	}
}

func TestDismiss_RefusesAHeldEntry(t *testing.T) {
	list := []Conflict{{Local: "REQ-1", Field: "title", Reason: ReasonBothChanged}}
	if _, err := CheckDismiss(list, "REQ-1", ""); err == nil || errors.Is(err, ErrNoConflict) {
		t.Fatalf("dismiss of a held entry: %v", err)
	}
	if _, err := CheckDismiss(list, "REQ-2", ""); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("dismiss of nothing: %v", err)
	}
}

// relKey returns the local key of the source relation from--satisfies--to.
func relKey(t *testing.T, tg *target, from, to string) entity.RelationKey {
	t.Helper()
	return entity.RelationKey{From: tg.byRef(t, from).ID, Type: "satisfies", To: tg.byRef(t, to).ID}
}

func TestRelationPush_CreatesAndStopsTheEcho(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R2", "C2")
	if _, err := tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{
		Properties: map[string]any{"weight": 4},
	}); err != nil {
		t.Fatal(err)
	}
	rep := runSync(t, tg, m, src, Options{})
	if rep.PushedRelations == nil || rep.PushedRelations.Created != 1 {
		t.Fatalf("push: %+v", rep.PushedRelations)
	}
	got := src.rels["R2--satisfies--C2"]
	if got == nil || got.Meta["weight"] == nil {
		t.Fatalf("remote relation = %+v", got)
	}
	src.sends = nil
	runSync(t, tg, m, src, Options{})
	src.sends = nil
	if rep := runSync(t, tg, m, src, Options{}); src.writes() != 0 || rep.Relations.Updated != 0 {
		t.Fatalf("echo: %+v, sends %v", rep.Relations, src.sends)
	}
}

func TestRelationPush_UpdatesMetaWithIfMatch(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R1", "C1")
	if _, err := tg.mgr.UpdateRelation(ctx, key, entity.RelationOptions{
		Properties: map[string]any{"weight": 7},
	}); err != nil {
		t.Fatal(err)
	}
	// A concurrent remote write to another relation does not matter; one to
	// this one is a 412 the push recovers from once.
	stale := true
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPatch && stale {
			stale = false
			return http.StatusPreconditionFailed, "stale"
		}
		return 0, ""
	}
	rep := runSync(t, tg, m, src, Options{})
	if rep.PushedRelations.Updated != 1 {
		t.Fatalf("push: %+v, sends %v", rep.PushedRelations, src.sends)
	}
	if w, _ := json.Marshal(src.rels["R1--satisfies--C1"].Meta["weight"]); string(w) != "7" {
		t.Fatalf("remote weight = %s", w)
	}
}

func TestRelationPush_ConflictIsHeldAndResolved(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R1", "C1")
	if _, err := tg.mgr.UpdateRelation(ctx, key, entity.RelationOptions{
		Properties: map[string]any{"weight": 7},
	}); err != nil {
		t.Fatal(err)
	}
	src.rels["R1--satisfies--C1"].Meta = map[string]any{"weight": 9}

	rep := runSync(t, tg, m, src, Options{})
	list := entries(t, tg)
	if rep.Held != 1 || len(list) != 1 || list[0].Kind != KindRelation || list[0].Local != key.String() {
		t.Fatalf("run %+v, list %+v", rep, list)
	}
	if n, err := Resolve(ctx, tg.kv, m, key.String(), "weight", TakeRemote); err != nil || n != 1 {
		t.Fatalf("resolve: %d, %v", n, err)
	}
	runSync(t, tg, m, src, Options{})
	rel, err := tg.st.GetRelation(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if w, _ := json.Marshal(rel.Properties["weight"]); string(w) != "9" || len(entries(t, tg)) != 0 {
		t.Fatalf("after resolve: local weight %s, list %+v", w, entries(t, tg))
	}
}

func TestRelationPush_LatestRuleOnMeta(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) {
		rm := p.Relations["satisfies"]
		rm.Conflict = ConflictLatest
		p.Relations["satisfies"] = rm
	})
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R1", "C1")
	if _, err := tg.mgr.UpdateRelation(ctx, key, entity.RelationOptions{
		Properties: map[string]any{"weight": 7},
	}); err != nil {
		t.Fatal(err)
	}
	r := src.rels["R1--satisfies--C1"]
	r.Meta, r.Updated = map[string]any{"weight": 9}, stampAt(time.Minute)

	rep := runSync(t, tg, m, src, Options{})
	rel, err := tg.st.GetRelation(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	list := entries(t, tg)
	if w, _ := json.Marshal(rel.Properties["weight"]); string(w) != "9" || rep.Superseded != 1 ||
		len(list) != 1 || string(list[0].Superseded.LocalValue) != "7" {

		t.Fatalf("local weight %s, run %+v, list %+v", w, rep, list)
	}
}

func TestRelationPush_RefusalIsHeld(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R2", "C2")
	if _, err := tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{}); err != nil {
		t.Fatal(err)
	}
	src.before = func(method, path string) (int, string) {
		if method == http.MethodPost && strings.Contains(path, "/relations/") {
			return http.StatusUnprocessableEntity, "no"
		}
		return 0, ""
	}
	rep := runSync(t, tg, m, src, Options{})
	list := entries(t, tg)
	if rep.PushedRelations.Refused != 1 || len(list) != 1 || list[0].Kind != KindRelation ||
		list[0].Reason != ReasonRefused {

		t.Fatalf("run %+v, list %+v", rep.PushedRelations, list)
	}
	src.sends = nil
	runSync(t, tg, m, src, Options{})
	for _, s := range src.sends {
		if strings.HasPrefix(s, http.MethodPost) {
			t.Fatalf("a held relation was sent again: %v", src.sends)
		}
	}
	if len(entries(t, tg)) != 1 {
		t.Fatal("the refusal was forgotten")
	}
}

func TestRelationPush_GatedTypeIsNotPushed(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	tg.metaGated = func(typ string) bool { return typ == "satisfies" }
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R2", "C2")
	if _, err := tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{}); err != nil {
		t.Fatal(err)
	}
	rep := runSync(t, tg, m, src, Options{})
	if _, ok := src.rels["R2--satisfies--C2"]; ok || rep.PushedRelations.Created != 0 {
		t.Fatalf("a gated type was pushed: %+v", rep.PushedRelations)
	}
	found := false
	for _, is := range rep.Issues {
		found = found || strings.Contains(is.Reason, "satisfies")
	}
	if !found {
		t.Fatalf("no issue names the gated type: %+v", rep.Issues)
	}
	// Its meta is pulled source-wins, as on a pull-only remote.
	src.rels["R1--satisfies--C1"].Meta = map[string]any{"weight": 9}
	runSync(t, tg, m, src, Options{})
	rel, err := tg.st.GetRelation(ctx, relKey(t, tg, "R1", "C1"))
	if err != nil {
		t.Fatal(err)
	}
	if w, _ := json.Marshal(rel.Properties["weight"]); string(w) != "9" {
		t.Fatalf("local weight %s", w)
	}
}

// A relation whose endpoint the pusher cannot read is not sent.
func TestRelationPush_UnreadableEndpointIsNotSent(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	key := relKey(t, tg, "R2", "C2")
	if _, err := tg.mgr.CreateRelation(ctx, key, entity.RelationOptions{}); err != nil {
		t.Fatal(err)
	}
	tg.gate = hideGate{inner: tg.gate, hidden: map[string]bool{key.To: true}}
	runSync(t, tg, m, src, Options{})
	if _, ok := src.rels["R2--satisfies--C2"]; ok {
		t.Fatal("a relation to an unreadable endpoint was pushed")
	}
}
