package remotesync

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
)

func TestParseRetention(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
		bad  bool
	}{
		{raw: "", want: DefaultSupersededRetention},
		{raw: "30d", want: 30 * 24 * time.Hour},
		{raw: "12h", want: 12 * time.Hour},
		{raw: "0", want: 0},
		{raw: "0d", want: 0},
		{raw: "-1h", bad: true},
		{raw: "-1d", bad: true},
		{raw: "xd", bad: true},
		{raw: "soon", bad: true},
		{raw: "36501d", bad: true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseRetention(tc.raw)
			if tc.bad {
				if err == nil || !strings.Contains(err.Error(), "superseded_retention") {
					t.Fatalf("parseRetention(%q) = %v, %v; want an error naming the key", tc.raw, got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("parseRetention(%q) = %v, %v; want %v", tc.raw, got, err, tc.want)
			}
		})
	}
}

func TestCompile_RejectsBadRetention(t *testing.T) {
	tg := newTarget(t)
	cfg, err := Parse([]byte(testRemotes), tg.meta)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Remotes["cat"]
	p.SupersededRetention = "-5d"
	if _, err := Compile("cat", p, tg.meta); err == nil || !strings.Contains(err.Error(), "superseded_retention") {
		t.Fatalf("compile: %v", err)
	}
}

func superseded(local, field string, since time.Time) *Conflict {
	return &Conflict{Local: local, Field: field, Reason: ReasonSuperseded, Since: since, Superseded: &remotestatus.Superseded{}}
}

func TestConflicts_Expire(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	fill := func() *conflicts {
		c := &conflicts{entries: map[string]*Conflict{}}
		for _, e := range []*Conflict{
			superseded("OLD", "title", now.Add(-31*24*time.Hour)),
			superseded("NEW", "title", now.Add(-time.Hour)),
			{Local: "HELD", Field: "title", Reason: ReasonBothChanged, Since: now.Add(-90 * 24 * time.Hour)},
		} {
			c.entries[e.key()] = e
		}
		return c
	}

	c := fill()
	c.expire(now, DefaultSupersededRetention)
	if len(c.entries) != 2 || c.entries[supersededKey("OLD", "title")] != nil {
		t.Fatalf("after expire: %v", c.entries)
	}
	if c.entries[conflictKey("HELD", "title")] == nil {
		t.Fatal("a held entry expired")
	}

	c = fill()
	c.expire(now, 0)
	if len(c.entries) != 3 {
		t.Fatalf("a retention of 0 dropped entries: %v", c.entries)
	}
}

func TestPurgeSuperseded(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	now := time.Now().UTC()
	c := &conflicts{key: conflictsKey("cat"), entries: map[string]*Conflict{}}
	rel := func(key string) *Conflict {
		e := superseded(key, "note", now)
		e.Kind = KindRelation
		return e
	}
	for _, e := range []*Conflict{
		superseded("REQ-1", "title", now),
		superseded("REQ-2", "title", now),
		rel("REQ-1--satisfies--CMP-1"),
		rel("CMP-2--satisfies--REQ-1"),
		rel("REQ-10--satisfies--CMP-1"),
		{Local: "REQ-1", Field: "state", Reason: ReasonBothChanged, Since: now},
	} {
		c.entries[e.key()] = e
	}
	if err := c.save(ctx, tg.kv); err != nil {
		t.Fatal(err)
	}

	n, err := PurgeSuperseded(ctx, tg.kv, "cat", "REQ-1")
	if err != nil || n != 3 {
		t.Fatalf("purge: %d, %v", n, err)
	}
	left := map[string]bool{}
	for _, e := range entries(t, tg) {
		left[e.Local+"/"+e.Reason] = true
	}
	want := []string{"REQ-2/" + ReasonSuperseded, "REQ-10--satisfies--CMP-1/" + ReasonSuperseded, "REQ-1/" + ReasonBothChanged}
	if len(left) != len(want) {
		t.Fatalf("left = %v", left)
	}
	for _, w := range want {
		if !left[w] {
			t.Fatalf("left = %v, missing %s", left, w)
		}
	}
	if n, err := PurgeSuperseded(ctx, tg.kv, "cat", "REQ-1"); err != nil || n != 0 {
		t.Fatalf("second purge: %d, %v", n, err)
	}
}

func TestRules_RetentionZeroKeepsNoLocalValue(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) { p.Conflict = ConflictLatest; p.SupersededRetention = "0" })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Ours"})
	src.ents["R1"].Props = map[string]any{"title": "Theirs", "state": "open"}
	src.ents["R1"].Updated = stampAt(time.Minute)

	if rep := runSync(t, tg, m, src, Options{}); rep.Superseded != 1 {
		t.Fatalf("run: %+v", rep)
	}
	list := entries(t, tg)
	if len(list) != 1 || list[0].Superseded.Winner != TakeRemote || list[0].LocalDigest == "" {
		t.Fatalf("list = %+v", list)
	}
	if list[0].Superseded.LocalValue != nil || list[0].Superseded.LocalValueOmitted {
		t.Fatalf("a retention of 0 kept the local value: %+v", list[0].Superseded)
	}
}

func TestRun_ExpiresSupersededEntries(t *testing.T) {
	tg, m, src, _ := conflictSetup(t, ConflictLatest, stampAt(time.Minute))
	runSync(t, tg, m, src, Options{})
	if len(entries(t, tg)) != 1 {
		t.Fatal("no superseded entry")
	}

	deps := tg.pushDeps(src)
	deps.Now = func() time.Time { return time.Now().Add(DefaultSupersededRetention + time.Hour) }
	if _, err := Run(context.Background(), m, deps, Options{}); err != nil {
		t.Fatal(err)
	}
	if list := entries(t, tg); len(list) != 0 {
		t.Fatalf("the expired entry stayed: %+v", list)
	}
}
