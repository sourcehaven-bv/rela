package docs

import (
	"context"
	"strings"
	"testing"
)

// TestNegativeControls is the suite's positive control: every row is a mistake
// an author would plausibly make, and each must turn the build RED.
//
// # Why this table exists
//
// Every other test here asserts that a CORRECT assertion behaves correctly.
// None of them can catch a guard that was never written — and a missing guard
// is exactly how an assertion becomes vacuous: a typo'd principal has no
// grants and is therefore always refused; an unknown entity type yields an
// empty set and therefore satisfies `absent=`; an unknown role falls back to a
// privileged default and therefore passes as someone else. Each of those
// PASSES, forever, while appearing to check something.
//
// Mutation testing cannot find these either: mutating absent code kills no
// mutants. So the rule is that every new claim, argument, or verb gets a row
// here showing how it fails when misused — this table is the natural home for
// "what does this look like when it's wrong?"
func TestNegativeControls(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
		why     string
	}{
		{
			name:    "typo'd principal",
			body:    `refuses{who="typo-nobody", op="update", type="risico"}`,
			wantErr: "no such principal",
			why:     "an unassigned principal has no grants, so a refusal claim would pass forever",
		},
		{
			name:    "an intended unassigned principal must say so",
			body:    `refuses{who="nobody@example.com", op="update", type="risico", unassigned=true}`,
			wantErr: "", // legitimate: the claim IS that this principal has no role
		},
		{
			name:    "unassigned=true on an assigned principal",
			body:    `refuses{who="vi", op="update", type="risico", unassigned=true}`,
			wantErr: "IS assigned",
			why:     "the manual describes the wrong thing; the reader would be misled",
		},
		{
			name:    "typo'd entity type in shows",
			body:    `shows{type="risicoo", exactly={}}`,
			wantErr: "no such entity type",
			why:     "an unknown type is an empty set, so exactly={} and absent= pass vacuously",
		},
		{
			name:    "typo'd entity type in an authorization claim",
			body:    `refuses{who="vi", op="update", type="risicoo"}`,
			wantErr: "no such entity type",
		},
		{
			name:    "scalar exactly reads as 'assert emptiness'",
			body:    `shows{type="risico", exactly="risico-1"}`,
			wantErr: "must be a list",
			why:     "a bare value is present-but-empty, asserting the OPPOSITE of what was written",
		},
		{
			name:    "non-string element would be dropped",
			body:    `shows{type="risico", exactly={"risico-1", 42}}`,
			wantErr: "non-string",
			why:     "dropping it silently shrinks the asserted set",
		},
		{
			name:    "misspelled claim key beside a valid one",
			body:    `shows{type="risico", contains={"risico-1"}, absnt={"x"}}`,
			wantErr: "unknown key absnt",
			why:     "the call passes on the valid claim while the author believes both are checked",
		},
		{
			name:    "a call that claims nothing",
			body:    `shows{type="risico"}`,
			wantErr: "asserts nothing",
		},
		{
			name:    "a one-character because pins nothing",
			body:    `refuses{who="vi", op="update", type="risico", because="a"}`,
			wantErr: "reason was not",
			why:     "a substring that short matches almost any deny",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" +
				`create("risico", {titel="Leak", kans=1, impact=1, status="todo"})` + "\n" +
				tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{
				Meta:   fixtureMeta(t),
				Policy: aclFixturePolicy(),
			})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("this is a LEGITIMATE claim and must build: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("this mistake was accepted, so the assertion is vacuous.\n  why it matters: %s", tc.why)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("failed for the wrong reason: want %q, got:\n%v", tc.wantErr, err)
			}
		})
	}
}

// The api{} negative controls need a client, so they live in their own table.
func TestNegativeControls_API(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
		why     string
	}{
		{
			name:    "identical_to comparing a request with itself",
			body:    `api{path="/a", status=403, identical_to={path="/a"}}`,
			wantErr: "names the SAME request",
			why:     "a self-comparison is trivially true — a claimless call wearing a claim's clothes",
		},
		{
			name:    "misspelled key inside identical_to",
			body:    `api{path="/a", status=403, identical_to={path="/b", ass="viewer"}}`,
			wantErr: "unknown key ass",
		},
		{
			name:    "a call that claims nothing",
			body:    `api{path="/a"}`,
			wantErr: "asserts nothing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAPI{responses: map[string]APIResponse{
				"/a": {Status: 403, Body: `{"type":"denied"}`},
				"/b": {Status: 403, Body: `{"type":"denied"}`},
			}}
			src := "```rela\n" + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), APIClient: f})
			if err == nil {
				t.Fatalf("this mistake was accepted, so the assertion is vacuous.\n  why it matters: %s", tc.why)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("failed for the wrong reason: want %q, got:\n%v", tc.wantErr, err)
			}
		})
	}
}
