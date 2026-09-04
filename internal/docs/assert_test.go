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

// TestUnknownKeysAreRefused: a misspelled claim beside a VALID one is the
// dangerous case — the call passes on the strength of the correct claim while
// the author believes both are checked. The claimless-call rule cannot catch
// this, because the call does assert something.
func TestUnknownKeysAreRefused(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "shows: a typo beside a valid claim",
			body:    `shows{type="risico", contains={"risico-1"}, absnt={"risico-2"}}`,
			wantErr: "unknown key absnt",
		},
		{
			name:    "shows: the message lists the known keys",
			body:    `shows{type="risico", exactl={}}`,
			wantErr: "Known keys: absent, contains, emit, exactly, type",
		},
		{
			name:    "shows: several typos are reported together",
			body:    `shows{type="risico", contains={"a"}, absnt={}, exactl={}}`,
			wantErr: "unknown keys absnt, exactl",
		},
		{
			name: "refuses: a typo beside a valid claim",
			//nolint:misspell // the misspelling IS the fixture: this asserts a typo is caught
			body:    `refuses{who="vi", op="update", type="risico", becuase="x"}`,
			wantErr: "unknown key becuase", //nolint:misspell // ditto
		},
		{
			name:    "api: a typo beside a valid claim",
			body:    `api{path="/a", status=200, eror="x"}`,
			wantErr: "unknown key eror",
		},
		{
			name:    "api: a typo inside identical_to",
			body:    `api{path="/a", status=200, identical_to={path="/b", ass="viewer"}}`,
			wantErr: "identical_to: unknown key ass",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" + tc.body + "\n```\n"
			opts := Options{Meta: fixtureMeta(t), Policy: aclFixturePolicy()}
			if strings.HasPrefix(tc.body, "api") {
				opts.APIClient = &fakeAPI{responses: map[string]APIResponse{
					"/a": {Status: 200}, "/b": {Status: 200},
				}}
			}
			_, err := Build(context.Background(), src, opts)
			if err == nil {
				t.Fatalf("want failure containing %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error does not contain %q:\n%v", tc.wantErr, err)
			}
		})
	}
}

// A correctly-spelled call must still build — the guard must not reject valid
// keys, which would make every assertion unusable.
func TestKnownKeysAreAccepted(t *testing.T) {
	src := "```rela\n" + `create("risico", {titel="a", kans=1, impact=1, status="todo"})
shows{type="risico", contains={"risico-1"}, absent={"risico-9"}, exactly={"risico-1"}}
refuses{who="vi", op="update", type="risico", because="role-grant"}` + "\n```\n"
	if _, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), Policy: aclFixturePolicy()}); err != nil {
		t.Fatalf("a call using every known key must build: %v", err)
	}
}
