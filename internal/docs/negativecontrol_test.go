package docs

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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

// TestNegativeControls_Read covers reads{}/hidden{}, which need the FACED
// fixture (fixtureMeta declares no faces, so no face claim could be made
// against it).
//
// The rows here guard the failure that motivated the verb. The worlds fixture's
// reader holds `read: [policy@published]`; dropping the `@published` widens it
// to every face, and NOTHING in the write-side assertions notices — every
// refuses{} still passes, because being unable to write a draft was never what
// concealed it. Only a read claim turns that red.
func TestNegativeControls_Read(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
		why     string
	}{
		{
			name:    "hidden about an id that was never seeded",
			body:    `hidden{who="pub", type="policy", id="POL-404"}`,
			wantErr: "would pass against any policy",
			why:     "a nonexistent row is hidden from everyone, so the claim holds against any policy",
		},
		{
			name:    "typo'd principal in a read claim",
			body:    `hidden{who="typo-nobody", type="policy", id="POL-1"}`,
			wantErr: "no such principal",
			why:     "an unassigned principal reads nothing, so hidden{} would pass forever",
		},
		{
			name:    "typo'd entity type in a read claim",
			body:    `reads{who="pub", type="polcy", id="POL-1"}`,
			wantErr: "no such entity type",
		},
		{
			name:    "a read claim naming no row",
			body:    `reads{who="pub", type="policy"}`,
			wantErr: "`who`, `type` and `id` are all required",
			why:     "a claim without a subject asserts nothing about the policy",
		},
		{
			name:    "a field-redaction claim is refused rather than checked against a no-op redactor",
			body:    `reads{who="pub", type="policy", id="POL-1", face="published", redacted={"title"}}`,
			wantErr: "unknown key redacted",
			why:     "docs cannot import the real redactor, so such a claim could only pass vacuously",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" + readSeed + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{
				Meta:   worldFixtureMeta(t),
				Policy: readFixturePolicy(),
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

// TestReadClaimCatchesAWidenedFaceGrant is the negative control the whole
// read-side verb exists for, run as a MUTATION of the policy rather than of the
// manual: it widens `policy@published` to a bare `policy` grant and asserts
// that the write-side claims stay green while the read-side claim turns red.
//
// Without this, "the read side needs its own verb" is an argument. With it, it
// is a failing build.
func TestReadClaimCatchesAWidenedFaceGrant(t *testing.T) {
	widened := readFixturePolicy()
	widened.Roles["reader"] = acl.RoleDef{Read: []string{"policy", "control"}}

	t.Run("the write-side claim still passes, so it cannot catch this", func(t *testing.T) {
		src := "```rela\n" + readSeed + `refuses{who="pub", op="update", type="policy"}` + "\n```\n"
		if _, err := Build(context.Background(), src, Options{
			Meta: worldFixtureMeta(t), Policy: widened,
		}); err != nil {
			t.Fatalf("the write claim should be unaffected by a read grant: %v", err)
		}
	})

	t.Run("the read-side claim turns red", func(t *testing.T) {
		src := "```rela\n" + readSeed + `hidden{who="pub", type="policy", id="POL-1"}` + "\n```\n"
		_, err := Build(context.Background(), src, Options{
			Meta: worldFixtureMeta(t), Policy: widened,
		})
		if err == nil {
			t.Fatal("a widened face grant went undetected — the read claim is vacuous")
		}
		if !strings.Contains(err.Error(), "disclosure") {
			t.Fatalf("want a disclosure failure, got:\n%v", err)
		}
	})
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
		{
			name:    "absent= with no status claim beside it",
			body:    `api{path="/a", absent={"secret"}}`,
			wantErr: "absent= needs a status= claim",
			why: "an error body contains none of the strings either, so absent= alone " +
				"passes on a 500 while proving nothing about a successful response",
		},
		{
			name:    "has= given a bare string instead of a list",
			body:    `api{path="/a", status=403, has="denied"}`,
			wantErr: "has must be a list",
			why:     "a bare value reads as an empty list, asserting nothing at all",
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
