package acl

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// faceGrantCeilingPolicy gives a role the common "everything, plus these
// faces" shape. On a write, "*" covers only the default face, so the face
// entries are what grant the faces. The ceilings name bare types, which is
// all a ceiling can name.
const faceGrantCeilingPolicy = `
roles:
  editor:
    read: ["*", "policy@draft"]
    create: ["*", "policy@draft", "secret@draft"]
    update: ["*", "policy@draft", "secret@draft"]
    delete: ["*", "policy@draft", "secret@draft"]
client_baselines:
  agents:
    applies_to: [agent]
    read: [policy, ticket]
    create: [policy, ticket]
    update: [policy, ticket]
    delete: [policy, ticket]
  open:
    applies_to: [open]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
  deny:
    applies_to: [deny]
    deny_write: [secret]
`

// TestClamp_WildcardKeepsFaceGrantsTheCeilingPermits pins BUG-0A2KPC. A role
// list holding "*" next to a face grant met a closed allowlist, and the
// allowlist replaced the whole list. The face grant was lost, so a client
// could no longer write a face that both the role and the ceiling permit.
func TestClamp_WildcardKeepsFaceGrantsTheCeilingPermits(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, faceGrantCeilingPolicy)
	role := p.Roles["editor"]

	tests := []struct {
		principalType string
		want          []string
	}{
		{"agent", []string{"policy", "ticket", "policy@draft"}},
		{"open", []string{"*", "policy@draft", "secret@draft"}},
		{"deny", []string{"*", "policy@draft"}},
	}
	for _, tc := range tests {
		t.Run(tc.principalType, func(t *testing.T) {
			t.Parallel()
			got := p.ceilingFor(tc.principalType, nil).clamp(role)
			for verb, list := range map[string][]string{
				"create": got.Create, "update": got.Update, "delete": got.Delete,
			} {
				if !slices.Equal(list, tc.want) {
					t.Errorf("%s = %v, want %v", verb, list, tc.want)
				}
			}
		})
	}

	// Read under the allowlist: the face entry survives too, and the
	// wildcard does not.
	got := p.ceilingFor("agent", nil).clamp(role)
	if want := []string{"policy", "ticket", "policy@draft"}; !slices.Equal(got.Read, want) {
		t.Errorf("read = %v, want %v", got.Read, want)
	}
}

// TestAuthorizeWrite_CeilingKeepsFaceGrant is the request-level form of
// BUG-0A2KPC: the write gate, not only the compiled list.
func TestAuthorizeWrite_CeilingKeepsFaceGrant(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, faceGrantCeilingPolicy+`
assignments:
  alice: editor
`)
	draft := entity.Face("draft")
	tests := []struct {
		name          string
		principalType string
		subject       EntitySubject
		want          bool
	}{
		{"user, face grant", "user", NewEntitySubject("policy", "POL-1", draft), true},
		{"agent, face the ceiling permits", "agent", NewEntitySubject("policy", "POL-1", draft), true},
		{"agent, face of a type the ceiling omits", "agent", NewEntitySubject("secret", "SEC-1", draft), false},
		{"agent, other face is still not granted", "agent", NewEntitySubject("policy", "POL-1", "published"), false},
		{"agent, default face via the allowlist", "agent", NewFacelessEntitySubject("ticket", "TKT-1"), true},
		{"open, face grant", "open", NewEntitySubject("secret", "SEC-1", draft), true},
		{"deny, face of the denied type", "deny", NewEntitySubject("secret", "SEC-1", draft), false},
	}
	for _, tc := range tests {
		for _, op := range []Op{OpCreate, OpUpdate, OpDelete} {
			t.Run(tc.name+"/"+string(op), func(t *testing.T) {
				t.Parallel()
				r, err := OpenRequest(t.Context(), d, verifiedClient("alice", tc.principalType))
				if err != nil {
					t.Fatal(err)
				}
				got := r.AuthorizeWrite(t.Context(), WriteRequest{Op: op, Subject: tc.subject})
				if got.Allow != tc.want {
					t.Errorf("allow = %v, want %v (%s)", got.Allow, tc.want, got.Reason)
				}
			})
		}
	}
}

// TestClamp_IsExactIntersection checks every small combination of role
// entries and ceiling lists: the clamped role, together with the match-time
// re-check, grants a (type, face) exactly when the role grants it and the
// ceiling permits the type. It covers the mix of "*", type and face entries
// that the example tests above missed (BUG-0A2KPC).
func TestClamp_IsExactIntersection(t *testing.T) {
	t.Parallel()
	entries := []string{"*", "page", "page@draft", "ticket@draft"}
	types := []string{"page", "ticket"}
	faces := []entity.Face{"", "draft"}

	var ceilings []verbCeiling
	for _, allow := range subsets([]string{"*", "page", "ticket"}) {
		ceilings = append(ceilings, verbCeiling{allow: append([]string{}, allow...)})
	}
	for _, deny := range subsets([]string{"*", "page"}) {
		if len(deny) > 0 {
			ceilings = append(ceilings, verbCeiling{deny: deny})
		}
	}
	// A scope grant re-opening a type under a wildcard denial.
	ceilings = append(ceilings, verbCeiling{deny: []string{"*"}, except: []string{"page"}})

	for _, roleList := range subsets(entries) {
		role := RoleDef{Create: roleList, Read: roleList}
		for _, v := range ceilings {
			kept := filterTypes(roleList, v)
			clamped := RoleDef{Create: kept, Read: kept}
			for _, typ := range types {
				for _, face := range faces {
					for verb, grants := range map[string]func(RoleDef) bool{
						"create": func(r RoleDef) bool { return GrantsVerbOnState(r, OpCreate, typ, face) },
						"read":   func(r RoleDef) bool { return readsFace(r, typ, face) },
					} {
						want := grants(role) && v.permits(typ)
						got := grants(clamped)
						if v.allow == nil {
							got = got && v.permits(typ) // a deny is re-checked at match time
						}
						if got != want {
							t.Errorf("%s: role %v, ceiling %+v, %s@%q: clamped %v grants %v, want %v",
								verb, roleList, v, typ, face, kept, got, want)
						}
					}
				}
			}
		}
	}
}

// readsFace reports whether role reads the face of typ: the type grant
// ([roleGrantsRead]) and the face set ([roleReadFaces]) together.
func readsFace(role RoleDef, typ string, face entity.Face) bool {
	if !roleGrantsRead(role, typ) {
		return false
	}
	faces, all := roleReadFaces(role, typ)
	return all || slices.Contains(faces, face)
}

// subsets returns every subset of items, in a fixed order.
func subsets(items []string) [][]string {
	out := [][]string{nil}
	for _, it := range items {
		for _, s := range out[:len(out):len(out)] {
			out = append(out, append(slices.Clone(s), it))
		}
	}
	return out
}
