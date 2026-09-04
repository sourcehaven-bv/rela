package docs

import (
	"strings"
	"testing"
)

// TestCheckRegionText covers the matcher directly — no browser, no Lua. The
// failure TEXT is asserted as well as the pass/fail, because a doctest's value
// is its failure output and prose that only appears on a red build is prose
// nobody proofreads.
func TestCheckRegionText(t *testing.T) {
	tests := []struct {
		name      string
		got       []string
		has       []string
		absent    []string
		wantOK    bool
		wantInMsg []string
		notInMsg  []string
	}{
		{
			name:   "substring hit passes",
			got:    []string{"Policies", "Pipeline", "Guides"},
			has:    []string{"Policies", "Pipeline"},
			wantOK: true,
		},
		{
			name:   "substring is a substring, not equality",
			got:    []string{"Data Retention (draft)"},
			has:    []string{"Data Retention"},
			wantOK: true,
		},
		{
			name:      "case sensitive",
			got:       []string{"Policies"},
			has:       []string{"policies"},
			wantOK:    false,
			wantInMsg: []string{`missing "policies"`},
		},
		{
			name:   "absent passes when truly absent",
			got:    []string{"POL-1", "POL-3"},
			absent: []string{"POL-2"},
			wantOK: true,
		},
		{
			name:      "absent fails when present",
			got:       []string{"POL-1", "POL-2"},
			absent:    []string{"POL-2"},
			wantOK:    false,
			wantInMsg: []string{`unexpectedly present "POL-2"`},
		},
		{
			name:      "missing and unexpected reported together",
			got:       []string{"POL-2"},
			has:       []string{"POL-1"},
			absent:    []string{"POL-2"},
			wantOK:    false,
			wantInMsg: []string{`missing "POL-1"`, `unexpectedly present "POL-2"`},
		},
		{
			// The load-bearing rule: an empty region satisfies every absent=
			// claim, so it must fail rather than pass vacuously.
			name:      "empty region is a FAILURE even for a satisfiable absent claim",
			got:       nil,
			absent:    []string{"POL-2"},
			wantOK:    false,
			wantInMsg: []string{"matched no element", "absent="},
		},
		{
			name:      "empty region fails a has claim too",
			got:       []string{},
			has:       []string{"anything"},
			wantOK:    false,
			wantInMsg: []string{"matched no element"},
		},
		{
			name:   "matching spans elements, not within one",
			got:    []string{"Access", "Control"},
			has:    []string{"Access", "Control"},
			wantOK: true,
		},
		{
			name:      "duplicate misses are deduped",
			got:       []string{"x"},
			has:       []string{"y", "y"},
			wantOK:    false,
			wantInMsg: []string{`missing "y"`},
			notInMsg:  []string{`"y", "y"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := checkRegionText("badge", tc.got, tc.has, tc.absent)
			if tc.wantOK {
				if msg != "" {
					t.Fatalf("expected pass, got failure: %s", msg)
				}
				return
			}
			if msg == "" {
				t.Fatal("expected failure, got pass")
			}
			for _, want := range tc.wantInMsg {
				if !strings.Contains(msg, want) {
					t.Errorf("failure message lacks %q:\n%s", want, msg)
				}
			}
			for _, no := range tc.notInMsg {
				if strings.Contains(msg, no) {
					t.Errorf("failure message unexpectedly contains %q:\n%s", no, msg)
				}
			}
		})
	}
}

// TestPageClaimsValidate pins the structural rules, above all "a call that
// asserts nothing is an error".
func TestPageClaimsValidate(t *testing.T) {
	tests := []struct {
		name      string
		claims    pageClaims
		wantErr   bool
		wantInMsg string
	}{
		{
			name:      "no claims at all is refused",
			claims:    pageClaims{},
			wantErr:   true,
			wantInMsg: "asserts nothing",
		},
		{
			name:   "menu_has alone is enough",
			claims: pageClaims{menuHas: []string{"Policies"}},
		},
		{
			name:   "has_card alone is enough",
			claims: pageClaims{hasCard: []string{"Data Retention"}},
		},
		{
			name:   "card_absent alone is enough",
			claims: pageClaims{cardAbsent: []string{"POL-2"}},
		},
		{
			name:   "region with has is fine",
			claims: pageClaims{region: "badge", has: []string{"en"}},
		},
		{
			name:      "has without region has no subject",
			claims:    pageClaims{has: []string{"en"}},
			wantErr:   true,
			wantInMsg: "need a region=",
		},
		{
			name:      "absent without region has no subject",
			claims:    pageClaims{absent: []string{"POL-2"}},
			wantErr:   true,
			wantInMsg: "need a region=",
		},
		{
			name:      "region with nothing claimed about it",
			claims:    pageClaims{region: "kanban-card", menuHas: []string{"Policies"}},
			wantErr:   true,
			wantInMsg: "nothing is claimed about it",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.claims.validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				if !strings.Contains(err.Error(), tc.wantInMsg) {
					t.Errorf("error %q lacks %q", err, tc.wantInMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestPageClaimsRegionsUsed pins which regions a call fetches: one browser trip
// per distinct region, and the sugar keys resolving to kanban-card.
func TestPageClaimsRegionsUsed(t *testing.T) {
	tests := []struct {
		name   string
		claims pageClaims
		want   []string
	}{
		{"menu only", pageClaims{menuHas: []string{"x"}}, []string{"menu"}},
		{"has_card is kanban-card sugar", pageClaims{hasCard: []string{"x"}}, []string{"kanban-card"}},
		{"card_absent is kanban-card sugar", pageClaims{cardAbsent: []string{"x"}}, []string{"kanban-card"}},
		{
			"both card keys fetch kanban-card once",
			pageClaims{hasCard: []string{"a"}, cardAbsent: []string{"b"}},
			[]string{"kanban-card"},
		},
		{
			"explicit region plus menu",
			pageClaims{menuHas: []string{"x"}, region: "badge", has: []string{"en"}},
			[]string{"menu", "badge"},
		},
		{
			"region == kanban-card is not fetched twice",
			pageClaims{region: "kanban-card", has: []string{"a"}, hasCard: []string{"b"}},
			[]string{"kanban-card"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.claims.regionsUsed()
			if len(got) != len(tc.want) {
				t.Fatalf("regionsUsed() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("regionsUsed() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestPageClaimsCheck exercises the whole-call assertion, including the dump
// that makes a failure diagnosable.
func TestPageClaimsCheck(t *testing.T) {
	t.Run("passing call reports nothing", func(t *testing.T) {
		c := pageClaims{menuHas: []string{"Policies"}}
		if msg := c.check("list", map[string][]string{"menu": {"Policies", "Pipeline"}}); msg != "" {
			t.Fatalf("expected pass, got: %s", msg)
		}
	})

	t.Run("failure dumps what was actually there", func(t *testing.T) {
		// The TKT-PI17Z6 shape: expected `en`, rendered `site-nl`.
		c := pageClaims{region: "badge", has: []string{"en"}}
		msg := c.check("entity", map[string][]string{"badge": {"site-nl"}})
		if msg == "" {
			t.Fatal("expected failure")
		}
		for _, want := range []string{`missing "en"`, "found:", "site-nl", `page{view="entity"}`} {
			if !strings.Contains(msg, want) {
				t.Errorf("failure lacks %q:\n%s", want, msg)
			}
		}
	})

	t.Run("failures in two regions are both reported", func(t *testing.T) {
		c := pageClaims{menuHas: []string{"Nope"}, region: "badge", has: []string{"en"}}
		msg := c.check("entity", map[string][]string{
			"menu":  {"Policies"},
			"badge": {"site-nl"},
		})
		if !strings.Contains(msg, "menu:") || !strings.Contains(msg, "badge:") {
			t.Fatalf("expected both regions named:\n%s", msg)
		}
	})
}

// TestDumpRegionCaps pins that a huge region cannot bury the failure message.
func TestDumpRegionCaps(t *testing.T) {
	t.Run("item cap", func(t *testing.T) {
		got := make([]string, 100)
		for i := range got {
			got[i] = "row"
		}
		out := dumpRegion(got)
		if !strings.Contains(out, "and 60 more") {
			t.Errorf("expected an item-cap notice, got:\n%s", out)
		}
		if strings.Count(out, "- row") != maxDumpItems {
			t.Errorf("expected %d items, got %d", maxDumpItems, strings.Count(out, "- row"))
		}
	})

	t.Run("byte cap", func(t *testing.T) {
		got := []string{strings.Repeat("x", 500), strings.Repeat("y", 500),
			strings.Repeat("z", 500), strings.Repeat("w", 500), strings.Repeat("v", 500)}
		out := dumpRegion(got)
		if len(out) > maxDumpBytes+200 {
			t.Errorf("dump is %d bytes, expected it capped near %d", len(out), maxDumpBytes)
		}
		if !strings.Contains(out, "output capped") {
			t.Errorf("expected a byte-cap notice, got %d bytes", len(out))
		}
	})

	t.Run("empty region says so", func(t *testing.T) {
		if !strings.Contains(dumpRegion(nil), "matched no element") {
			t.Error("empty dump should say the region matched nothing")
		}
	})
}

// TestRequireViewArgs pins the per-view argument rules shared with screenshot{}.
func TestRequireViewArgs(t *testing.T) {
	tests := []struct {
		name    string
		spec    CaptureSpec
		wantErr string
	}{
		{"list needs list", CaptureSpec{View: "list"}, "`list` is required"},
		{"kanban needs list", CaptureSpec{View: "kanban"}, "`list` is required"},
		{"kanban with list is fine", CaptureSpec{View: "kanban", List: "pipeline"}, ""},
		{"dashboard needs nothing", CaptureSpec{View: "dashboard"}, ""},
		{"analyze needs nothing", CaptureSpec{View: "analyze"}, ""},
		{"search needs q", CaptureSpec{View: "search"}, "`q` is required"},
		{"search with q is fine", CaptureSpec{View: "search", Query: "mfa"}, ""},
		{"create needs type or form", CaptureSpec{View: "create"}, "`type` or `form` is required"},
		{"create with form is fine", CaptureSpec{View: "create", Form: "new_policy"}, ""},
		{"history needs both", CaptureSpec{View: "history", Type: "policy"}, "`type` and `entity` are required"},
		{"entity needs type", CaptureSpec{View: "entity"}, "`type` is required"},
		{"entity needs entity", CaptureSpec{View: "entity", Type: "policy"}, "`entity` is required"},
		{"entity with both is fine", CaptureSpec{View: "entity", Type: "policy", Entity: "POL-1"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireViewArgs(tc.spec)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q lacks %q", err, tc.wantErr)
			}
		})
	}
}

// TestCountIsNotAKey pins that `count=` was REMOVED rather than merely
// undocumented. It was in the original design, and it is the one key that
// asserted a fact about the FIXTURE rather than a claim from the prose: adding
// one seeded entity upstream breaks every downstream count, and the break
// reports a bare number mismatch that says nothing about which claim was wrong.
// Worse, `count=3` passes on three cards showing entirely wrong content —
// uncomfortably close to the green-while-checking-nothing shape this verb
// exists to kill. has=/absent= make the same intent as a real claim.
//
// Since the key is gone, a manual still carrying one must be REFUSED by
// rejectUnknownKeys rather than silently ignored — a dropped claim is exactly
// the failure that rule was written for.
func TestCountIsNotAKey(t *testing.T) {
	for _, k := range pageKeys {
		if k == "count" {
			t.Fatal("`count` is still in page{}'s known-key list; it was removed deliberately")
		}
	}

	// The "asserts nothing" message must not advertise a key that no longer exists.
	err := pageClaims{}.validate()
	if err == nil {
		t.Fatal("an empty claim set must be refused")
	}
	if strings.Contains(err.Error(), "count=") {
		t.Errorf("the asserts-nothing message still offers count=: %s", err)
	}
	for _, want := range []string{"menu_has=", "has=", "absent=", "has_card=", "card_absent="} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the asserts-nothing message should still list %s: %s", want, err)
		}
	}

	// region= must retain a legal use now that count is gone.
	if err := (pageClaims{region: "badge", has: []string{"en"}}).validate(); err != nil {
		t.Errorf("region= with has= must remain legal: %v", err)
	}
	if err := (pageClaims{region: "badge", absent: []string{"nl"}}).validate(); err != nil {
		t.Errorf("region= with absent= must remain legal: %v", err)
	}
}
