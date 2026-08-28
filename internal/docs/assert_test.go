package docs

import (
	"context"
	"strings"
	"testing"
)

// TestCheckShows covers the assertion rules. The failure TEXT is asserted, not
// just the pass/fail bit: a doctest's output is read by someone who did not
// write the manual, and prose that only appears on a red build is prose nobody
// proofreads.
func TestCheckShows(t *testing.T) {
	tests := []struct {
		name       string
		got        []string
		contains   []string
		absent     []string
		exactly    []string
		hasExactly bool
		wantOK     bool
		wantParts  []string
	}{
		{
			name:     "contains satisfied",
			got:      []string{"POL-1", "POL-2"},
			contains: []string{"POL-1"},
			wantOK:   true,
		},
		{
			name:      "contains missing names the id and shows the seeded set",
			got:       []string{"POL-2"},
			contains:  []string{"POL-1"},
			wantParts: []string{"missing:  POL-1", "seeded policy: POL-2"},
		},
		{
			name:   "absent satisfied",
			got:    []string{"POL-1"},
			absent: []string{"POL-9"},
			wantOK: true,
		},
		{
			name:      "absent violated",
			got:       []string{"POL-1", "POL-9"},
			absent:    []string{"POL-9"},
			wantParts: []string{"unexpected: POL-9"},
		},
		{
			name:       "exactly matches regardless of order",
			got:        []string{"POL-1", "POL-2"},
			exactly:    []string{"POL-2", "POL-1"},
			hasExactly: true,
			wantOK:     true,
		},
		{
			// The defect `exactly` exists for: `contains` cannot see an
			// over-inclusive result, and over-inclusion is the leak.
			name:       "exactly catches an extra the contains form would miss",
			got:        []string{"CTL-1", "CTL-2"},
			exactly:    []string{"CTL-2"},
			hasExactly: true,
			wantParts:  []string{"unexpected: CTL-1"},
		},
		{
			name:       "exactly empty asserts the set is empty",
			got:        []string{"POL-1"},
			exactly:    nil,
			hasExactly: true,
			wantParts:  []string{"unexpected: POL-1"},
		},
		{
			name:       "exactly empty is satisfied by an empty set",
			got:        nil,
			exactly:    nil,
			hasExactly: true,
			wantOK:     true,
		},
		{
			name:      "an empty seeded set says so rather than printing nothing",
			got:       nil,
			contains:  []string{"POL-1"},
			wantParts: []string{"seeded policy: (none)"},
		},
		{
			name:       "missing and unexpected are reported together",
			got:        []string{"POL-2"},
			exactly:    []string{"POL-1"},
			hasExactly: true,
			wantParts:  []string{"missing:  POL-1", "unexpected: POL-2"},
		},
		{
			// contains= and exactly= can name the same absent id; reporting it
			// twice reads as two different problems.
			name:       "a doubly-claimed missing id is reported once",
			got:        nil,
			contains:   []string{"POL-1"},
			exactly:    []string{"POL-1"},
			hasExactly: true,
			wantParts:  []string{"missing:  POL-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := checkShows("policy", tc.got, tc.contains, tc.absent, tc.exactly, tc.hasExactly)
			if tc.wantOK {
				if msg != "" {
					t.Fatalf("want pass, got failure:\n%s", msg)
				}
				return
			}
			if msg == "" {
				t.Fatalf("want failure, got pass")
			}
			for _, part := range tc.wantParts {
				if !strings.Contains(msg, part) {
					t.Errorf("failure text missing %q; full text:\n%s", part, msg)
				}
			}
		})
	}
}

func TestDedupeKeepsFirstOrder(t *testing.T) {
	got := dedupe([]string{"b", "a", "b", "c", "a"})
	want := []string{"b", "a", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dedupe = %v, want %v", got, want)
	}
}

// TestShowsIsland drives shows{} through the real Build path, so the binding —
// argument extraction, the refusal rules, the seeded-store read — is covered,
// not just the pure core.
func TestShowsIsland(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string // "" ⇒ the manual must build
	}{
		{
			name: "a satisfied claim builds",
			body: `create("risico", {titel="Leak", kans=3, impact=4, status="todo"})
shows{type="risico", contains={"risico-1"}}`,
		},
		{
			name: "a violated claim fails the build",
			body: `create("risico", {titel="Leak", kans=3, impact=4, status="todo"})
shows{type="risico", contains={"risico-99"}}`,
			wantErr: "missing:  risico-99",
		},
		{
			// The rule this verb exists for: a call that claims nothing must
			// not pass. It looks like a test and is not one.
			name:    "a claimless call is refused",
			body:    `shows{type="risico"}`,
			wantErr: "asserts nothing",
		},
		{
			name:    "a missing type is refused",
			body:    `shows{contains={"risico-1"}}`,
			wantErr: "`type` is required",
		},
		{
			name:    "a non-table argument is refused",
			body:    `shows("risico")`,
			wantErr: "expects a table",
		},
		{
			name: "exactly on an unseeded type asserts emptiness",
			body: `shows{type="maatregel", exactly={}}`,
		},
		{
			name: "exactly catches an entity the manual did not mention",
			body: `create("maatregel", {titel="A"})
create("maatregel", {titel="B"})
shows{type="maatregel", exactly={"maatregel-1"}}`,
			wantErr: "unexpected: maatregel-2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t)})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want build to succeed, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want build to fail with %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}
