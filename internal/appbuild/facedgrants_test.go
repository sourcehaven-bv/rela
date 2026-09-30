package appbuild_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// facedMetamodel declares a faced type with an alias, a faceless user type,
// a faced group type, and relations for the identity-structure checks.
const facedMetamodel = `version: "1.0"
entities:
  policy:
    label: Policy
    aliases: [pol]
    id_type: manual
    faces:
      draft: { label: Draft }
      published: { label: Published }
    properties:
      title: { type: string }
  person:
    label: Person
    id_type: manual
    properties:
      email: { type: string, unique: true }
  board:
    label: Board
    id_type: manual
    faces:
      draft: { label: Draft }
    properties:
      title: { type: string }
relations:
  member-of:
    label: member of
    from: [person]
    to: [board]
  cites:
    label: cites
    from: [policy]
    to: [policy]
    scope: content
`

// TestPrepare_FacedWriteGrantFailsBoot pins TKT-7IZHP0 §7 at the wiring
// site: the check runs in prepare, so it fails before any store opens, on
// every recipe, and the message names the grant and its replacements.
func TestPrepare_FacedWriteGrantFailsBoot(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, facedMetamodel)
	writePolicy(t, root, "roles:\n  editor:\n    read: [\"*\"]\n    update: [\"*\", pol]\n")

	svc, err := appbuildOnDiskNoBackend(t, root)
	if err == nil {
		svc.Close()
		t.Fatal("expected boot to fail on a bare write grant on a faced type")
	}
	for _, want := range []string{
		`roles.editor.update: "pol" names a type that declares faces`,
		`"policy@draft", "policy@published"`,
		`note: "*"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

// TestPrepare_FacedGroupTypeFailsBoot pins A16 through the real metamodel
// adapter: the default membership relation into a faced group type fails.
func TestPrepare_FacedGroupTypeFailsBoot(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, facedMetamodel)
	writePolicy(t, root, "roles:\n  reader:\n    read: [\"*\"]\n")

	svc, err := appbuildOnDiskNoBackend(t, root)
	if err == nil {
		svc.Close()
		t.Fatal("expected boot to fail on a faced group type")
	}
	if want := `membership_relation "member-of": group type "board" declares faces (draft)`; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err, want)
	}
}

// TestPrepare_InjectedDeclarativeIsValidated pins G16: a policy injected via
// WithACL as a *acl.Declarative is validated like a loaded one. Any other
// injected ACL carries no policy and is exempt.
func TestPrepare_InjectedDeclarativeIsValidated(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, facedMetamodel)
	// No acl.yaml: the only policy is the injected one.

	policy := &acl.Policy{
		MembershipRelation: "cites-nobody", // undeclared: keeps A16 out of this test
		Roles:              map[string]acl.RoleDef{"editor": {Read: []string{"*"}, Delete: []string{"policy"}}},
	}
	d, err := acl.NewDeclarative(policy, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	svc, err := newOnDisk(t, "", root, appbuild.WithACL(d))
	if err == nil {
		svc.Close()
		t.Fatal("expected boot to fail on an injected policy with a bare faced grant")
	}
	if want := `roles.editor.delete: "policy"`; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err, want)
	}

	svc, err = appbuildOnDiskWithOpts(t, root, appbuild.WithACL(acl.ReadOnlyACL{}))
	if err != nil {
		t.Fatalf("a non-declarative injected ACL is exempt, got %v", err)
	}
	svc.Close()
}

// TestValidateACLPolicy covers the exported entry point directly: nil
// arguments are rejected, and a face-qualified policy over the same schema
// is accepted, which shows the adapter resolves aliases and relations.
func TestValidateACLPolicy(t *testing.T) {
	meta, err := metamodel.Parse([]byte(facedMetamodel))
	if err != nil {
		t.Fatalf("load metamodel: %v", err)
	}
	if appbuild.ValidateACLPolicy(nil, meta) == nil {
		t.Error("a nil policy must be rejected")
	}
	if appbuild.ValidateACLPolicy(&acl.Policy{}, nil) == nil {
		t.Error("a nil metamodel must be rejected")
	}

	ok := &acl.Policy{
		UserEntityType:     "person",
		PrincipalProperty:  "email",
		MembershipRelation: "undeclared-membership",
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"*"}, Update: []string{"*", "pol@draft", "policy@published"}},
		},
	}
	if vErr := appbuild.ValidateACLPolicy(ok, meta); vErr != nil {
		t.Errorf("a face-qualified policy must validate, got %v", vErr)
	}

	inherit := &acl.Policy{MembershipRelation: "undeclared-membership", InheritRolesThrough: []string{"cites"}}
	err = appbuild.ValidateACLPolicy(inherit, meta)
	if err == nil || !strings.Contains(err.Error(), "`scope: content`") {
		t.Errorf("a content-scoped inherit_roles_through must be refused, got %v", err)
	}
}
