package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Doc-drift guard: the YAML below is copied verbatim from the "Restricting a
// client below its user" section of docs/acl-overview.md, and the assertions
// are the claims that section makes in prose.
//
// A documented ACL example that no longer matches behavior is worse than none:
// a reader will copy it into a deployment and believe a client is restricted.
func TestDocs_AclOverviewClientAttenuationExampleWorks(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  hr:
    read: [person, ticket]
    create: [ticket]
    update: [person, ticket]
    delete: [ticket]
assignments:
  alice: hr
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
    redact:
      person: [salary, bsn]
scope_grants:
  rela.tickets.write:
    update: [ticket]
`))
	if err != nil {
		t.Fatalf("the YAML in docs/acl-overview.md does not parse: %v", err)
	}
	st := memstore.New()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	asClient := func(scopes []string) *acl.Request {
		t.Helper()
		req, rErr := d.ForPrincipal(principal.VerifiedFrom(
			"alice", principal.ToolDataEntry,
			principal.Claims{PrincipalType: "app", Scopes: scopes}))
		if rErr != nil {
			t.Fatal(rErr)
		}
		return req
	}

	// "deny_write: [\"*\"] is shorthand for denying create, update and delete."
	noScope := asClient(nil)
	for _, op := range []acl.Op{acl.OpCreate, acl.OpUpdate, acl.OpDelete} {
		dec := noScope.AuthorizeWrite(ctx, acl.WriteRequest{
			Op: op, Subject: acl.EntitySubject{Type: "ticket", ID: "TKT-1"},
		})
		if dec.Allow {
			t.Errorf("%s allowed under deny_write: [\"*\"]", op)
		}
	}

	// "A ceiling never grants" — reads the user holds are untouched.
	if res := noScope.ReadQuery(ctx, "ticket"); !res.AllowAll {
		t.Error("ticket read denied; the baseline only denies writes")
	}

	// "a scope ... hands one capability back", and only the one it names.
	scoped := asClient([]string{"rela.tickets.write"})
	if dec := scoped.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: acl.OpUpdate, Subject: acl.EntitySubject{Type: "ticket", ID: "TKT-1"},
	}); !dec.Allow {
		t.Errorf("rela.tickets.write did not re-open ticket update: %s", dec.Reason)
	}
	if dec := scoped.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: acl.OpUpdate, Subject: acl.EntitySubject{Type: "person", ID: "PERS-1"},
	}); dec.Allow {
		t.Error("the scope named only ticket, but person update was allowed")
	}

	// "A `principal_type` that matches no baseline is unrestricted."
	unmatched, err := d.ForPrincipal(principal.VerifiedFrom(
		"alice", principal.ToolDataEntry, principal.Claims{PrincipalType: "user"}))
	if err != nil {
		t.Fatal(err)
	}
	if dec := unmatched.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: acl.OpUpdate, Subject: acl.EntitySubject{Type: "person", ID: "PERS-1"},
	}); !dec.Allow {
		t.Errorf("an interactive user was attenuated: %s", dec.Reason)
	}
}

// Doc-drift guard: the YAML below is copied verbatim from the
// "Roles from a verified identity assertion" section of docs/acl-overview.md.
// If the policy schema changes and the docs are not updated, this fails —
// which is the point. A documented example that no longer parses is worse
// than no example, because a reader will trust it.
func TestDocs_AclOverviewExampleWorks(t *testing.T) {
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor: {read: [ticket], update: [ticket]}
  auditor: {read: [ticket]}
asserted_role_assignments:
  admin: editor
  compliance: [editor, auditor]
`))
	if err != nil {
		t.Fatalf("the YAML in docs/acl-overview.md does not parse: %v", err)
	}
	st := memstore.New()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	// "A principal holding [admin, compliance] gets the union, deduplicated."
	req, err := d.ForPrincipal(principal.Verified(
		"usr_1", principal.ToolDataEntry, "org", "slug",
		[]string{"admin", "compliance"}))
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]int{}
	for _, a := range req.Globals(context.Background()).Attributions {
		roles[a.Role]++
	}
	if roles["editor"] == 0 || roles["auditor"] == 0 {
		t.Errorf("documented union not produced: %v", roles)
	}
	// editor arrives via BOTH claims; distinct sources, so two attributions —
	// but the ROLE set is what authorizes, and that is deduplicated.
	if len(roles) != 2 {
		t.Errorf("role set = %v, want exactly {editor, auditor}", roles)
	}
}
