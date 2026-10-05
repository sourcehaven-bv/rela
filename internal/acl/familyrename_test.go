package acl

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// familyRenamePolicy holds the roles the family-rename tests decide for.
// renamer holds the family grant; editor holds update on every face, which a
// family decision does not consult; typewide holds update on the type.
const familyRenamePolicy = `
roles:
  renamer:
    read: ["*"]
    rename: [policy]
  anyrenamer:
    read: ["*"]
    rename: ["*"]
  editor:
    read: ["*"]
    update: ["policy@draft", "policy@published"]
  typewide:
    read: ["*"]
    update: [policy]
  owner:
    read: ["*"]
    rename: [policy]
assignments:
  renamer: renamer
  anyrenamer: anyrenamer
  editor: editor
  typewide: typewide
client_baselines:
  readonly:
    applies_to: [readonly-client]
    deny_write: ["*"]
role_relations:
  owns:
    confers: owner
`

// TestDecideFamily_RenameNeedsTheFamilyGrant pins the family rule: a rename
// of a faced type is decided once from the `rename:` grants, never from a
// grant on each face, and every other op is refused at family level. A
// denial names the type and no face.
func TestDecideFamily_RenameNeedsTheFamilyGrant(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, familyRenamePolicy)
	for _, tc := range []struct {
		user string
		op   Op
		want bool
	}{
		{"renamer", OpRename, true},
		{"anyrenamer", OpRename, true},
		{"editor", OpRename, false},
		{"typewide", OpRename, false},
		{"renamer", OpUpdate, false},
		{"renamer", OpDelete, false},
		{"editor", OpUpdate, false},
	} {
		t.Run(tc.user+"/"+string(tc.op), func(t *testing.T) {
			t.Parallel()
			got := mustRequest(t, d, tc.user).AuthorizeWrite(t.Context(),
				WriteRequest{Op: tc.op, Subject: NewFamilySubject("policy", "POL-1")})
			if got.Allow != tc.want {
				t.Fatalf("%s on the POL-1 family as %s: allow = %v, want %v (%s)",
					tc.op, tc.user, got.Allow, tc.want, got.Reason)
			}
			if !got.Allow {
				for _, face := range []string{"draft", "published"} {
					if strings.Contains(got.Reason, face) {
						t.Errorf("denial %q names the face %q", got.Reason, face)
					}
				}
			}
		})
	}
}

// TestDecideFamily_RenameOnFacelessType pins that a faceless type is renamed
// by either list: its family is its one implicit row, which `update:` has
// always covered and `rename:` covers too.
func TestDecideFamily_RenameOnFacelessType(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, strings.ReplaceAll(familyRenamePolicy, "policy", "ticket"))
	for _, tc := range []struct {
		user string
		want bool
	}{
		{"renamer", true},
		{"typewide", true},
		{"editor", false},
	} {
		t.Run(tc.user, func(t *testing.T) {
			t.Parallel()
			got := mustRequest(t, d, tc.user).AuthorizeWrite(t.Context(),
				WriteRequest{Op: OpRename, Subject: NewFacelessEntitySubject("ticket", "TKT-1")})
			if got.Allow != tc.want {
				t.Errorf("rename of TKT-1 as %s: allow = %v, want %v (%s)", tc.user, got.Allow, tc.want, got.Reason)
			}
		})
	}
}

// TestDecideFamily_LocalRoles pins the role rule. A local role is conferred
// through an identity-scoped relation (a content-scoped conferring relation
// is a load error, see TestValidateAgainstMetamodel_IdentityStructure), so it
// applies to the entity as a whole and confers a family rename on that
// entity, and on no other.
func TestDecideFamily_LocalRoles(t *testing.T) {
	t.Parallel()
	g := newFakeGraph()
	g.add("alice", "owns", "POL-1")
	d := newTestDeclarative(t, mustPolicy(t, familyRenamePolicy), g)
	r, err := d.ForPrincipal(principal.Principal{User: "alice", Tool: principal.ToolCLI})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"POL-1", true},
		{"POL-2", false},
	} {
		got := r.AuthorizeWrite(t.Context(), WriteRequest{Op: OpRename, Subject: NewFamilySubject("policy", tc.id)})
		if got.Allow != tc.want {
			t.Errorf("rename of %s by the owner of POL-1: allow = %v, want %v (%s)", tc.id, got.Allow, tc.want, got.Reason)
		}
	}
}

// TestDecideFamily_CeilingClamps pins that a client ceiling withholding
// writes withholds a family rename too: a ceiling only narrows.
func TestDecideFamily_CeilingClamps(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, familyRenamePolicy)
	for _, tc := range []struct {
		principalType string
		want          bool
	}{
		{"user", true},
		{"readonly-client", false},
	} {
		r, err := d.ForPrincipal(verifiedClient("renamer", tc.principalType))
		if err != nil {
			t.Fatal(err)
		}
		got := r.AuthorizeWrite(t.Context(), WriteRequest{Op: OpRename, Subject: NewFamilySubject("policy", "POL-1")})
		if got.Allow != tc.want {
			t.Errorf("rename as a %s: allow = %v, want %v (%s)", tc.principalType, got.Allow, tc.want, got.Reason)
		}
	}
}

// TestValidateRenameGrants pins the load error for a face-qualified rename
// grant, and that a type or "*" loads.
func TestValidateRenameGrants(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, grant, wantErr string
	}{
		{"type", "[policy]", ""},
		{"wildcard", `["*"]`, ""},
		{"face", `["policy@draft"]`, `roles.r: rename: "policy@draft" names the face "draft"; a rename ` +
			`moves every face of an entity, so a rename grant names the type alone: "policy"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := LoadPolicyBytes([]byte("roles:\n  r:\n    read: [\"*\"]\n    rename: " + tc.grant + "\n"))
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("rename: %s: %v", tc.grant, err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("rename: %s = %v, want an error containing %q", tc.grant, err, tc.wantErr)
			}
		})
	}
}
