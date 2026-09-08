package docs

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// readFixturePolicy is the shape the whole feature exists for: `pub` may read
// ONLY the published face of a policy, `ed` may read every face.
//
// The face grant (`policy@published`) is the load-bearing part — a bare
// `policy` read grant permits every face, so a fixture without the `@` would
// make every hidden{} claim fail and prove nothing about face gating.
func readFixturePolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"*"}, Update: []string{"*"}},
			"reader": {Read: []string{"policy@published", "control"}},
		},
		Assignments: map[string]string{"ed": "editor", "pub": "reader"},
	}
}

// readSeed gives POL-1 both faces and POL-2 only the bare draft, so a claim can
// distinguish "this face is concealed" from "this entity is concealed".
const readSeed = `create("policy", { id = "POL-1", title = "Access Control" })
face("policy", "POL-1", "published", { title = "Access Control" })
create("policy", { id = "POL-2", title = "Unpublished draft" })
`

func TestReadIsland(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "the published face is readable by the reader",
			body: `reads{who="pub", type="policy", id="POL-1", face="published"}`,
		},
		{
			// The claim the feature exists to support, and the one no other
			// verb in this package could make.
			name: "the draft face is hidden from the reader",
			body: `hidden{who="pub", type="policy", id="POL-1"}`,
		},
		{
			name: "an editor reads the draft the reader cannot",
			body: `reads{who="ed", type="policy", id="POL-1"}`,
		},
		{
			name: "a draft-only policy is entirely hidden from the reader",
			body: `hidden{who="pub", type="policy", id="POL-2"}`,
		},
		{
			// If someone widens `reader` to a bare `policy` grant, the face
			// gate stops applying and this manual stops building.
			name:    "claiming the reader sees the draft fails",
			body:    `reads{who="pub", type="policy", id="POL-1"}`,
			wantErr: "the row is HIDDEN from them",
		},
		{
			name:    "claiming a readable row is hidden fails as a disclosure",
			body:    `hidden{who="pub", type="policy", id="POL-1", face="published"}`,
			wantErr: "the row was RETURNED — this is a disclosure",
		},
		{
			// The vacuous-pass guard: without it, every typo'd id is "hidden"
			// and the claim holds against any policy at all.
			name:    "hidden about an id that does not exist is refused",
			body:    `hidden{who="pub", type="policy", id="POL-404"}`,
			wantErr: "would pass against any policy",
		},
		{
			name:    "an unassigned principal is refused rather than reading nothing",
			body:    `hidden{who="nobody", type="policy", id="POL-1"}`,
			wantErr: "no such principal",
		},
		{
			name:    "an unknown type is refused",
			body:    `reads{who="pub", type="polcy", id="POL-1"}`,
			wantErr: "no such entity type",
		},
		{
			name:    "a missing id is refused",
			body:    `reads{who="pub", type="policy"}`,
			wantErr: "`who`, `type` and `id` are all required",
		},
		{
			name:    "an unknown key is refused",
			body:    `reads{who="pub", type="policy", id="POL-1", redacted={"x"}}`,
			wantErr: "unknown key redacted",
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
					t.Fatalf("want build to succeed, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want failure containing %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error does not contain %q:\n%v", tc.wantErr, err)
			}
		})
	}
}

// TestReadIslandNeedsPolicy pins that a project with no acl.yaml cannot make a
// read claim: with no policy there is no gate, so every claim would describe
// the absence of a policy rather than its content.
func TestReadIslandNeedsPolicy(t *testing.T) {
	src := "```rela\n" + readSeed + `hidden{who="pub", type="policy", id="POL-1"}` + "\n```\n"
	_, err := Build(context.Background(), src, Options{Meta: worldFixtureMeta(t)})
	if err == nil || !strings.Contains(err.Error(), "no acl.yaml") {
		t.Fatalf("want a no-policy refusal, got: %v", err)
	}
}
