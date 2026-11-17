package aclaudit

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// TestAudit_A16_ConditionalGrant pins that a conditional grant is reported
// as an Info fact that no `--fail-on` threshold reaches (TKT-TKSCSBT).
func TestAudit_A16_ConditionalGrant(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{
		Roles: map[string]acl.RoleDef{"auditor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"grants":    {Confers: "auditor", RequiresPermission: "delegate", When: "edge.valid_until >= today()"},
			"member-of": {RequiresPermission: "admin", When: "edge.valid_until >= today()"},
			"owns":      {Confers: "auditor", RequiresPermission: "delegate"},
		},
	}
	var got []Finding
	for _, f := range Audit(p, fakeMetamodel{}, allPerms{}) {
		if f.Rule == "A16-conditional-grant" {
			got = append(got, f)
		}
	}
	if len(got) != 2 {
		t.Fatalf("A16 findings = %+v, want one per `when:`", got)
	}
	if got[0].Subject != "grants" || !strings.Contains(got[0].Detail, "confers auditor") ||
		!strings.Contains(got[0].Detail, "edge.valid_until >= today()") {

		t.Errorf("grants finding = %+v", got[0])
	}
	if got[1].Subject != "member-of" || !strings.Contains(got[1].Detail, "membership relation") {
		t.Errorf("membership finding = %+v", got[1])
	}
	for _, f := range got {
		if f.Severity != Info || f.Severity.String() != "info" {
			t.Errorf("severity = %v, want info", f.Severity)
		}
	}
	threshold, _ := ParseSeverity("any")
	if HasAtLeast(got, threshold) {
		t.Error("an Info finding must not trip --fail-on=any")
	}
}
