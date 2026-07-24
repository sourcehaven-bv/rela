package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

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
