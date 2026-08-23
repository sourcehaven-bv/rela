package aclaudit

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// fakeMetamodel is a minimal MetamodelReader for the Tier-B tests.
type fakeMetamodel struct {
	types     map[string]bool
	relations map[string][]string            // rel -> from types
	fields    map[string]map[string][]string // type -> field -> enum options (nil = non-enum/declared field)
}

func (m fakeMetamodel) HasEntityType(t string) bool { return m.types[t] }

func (m fakeMetamodel) GetRelation(name string) (RelationView, bool) {
	from, ok := m.relations[name]
	if !ok {
		return RelationView{}, false
	}
	return RelationView{From: from}, true
}

func (m fakeMetamodel) HasField(t, field string) bool {
	if m.fields[t] == nil {
		return false
	}
	_, ok := m.fields[t][field]
	return ok
}

func (m fakeMetamodel) EnumOptions(t, field string) ([]string, bool) {
	if m.fields[t] == nil {
		return nil, false
	}
	opts, ok := m.fields[t][field]
	if !ok || opts == nil {
		return nil, false // field absent, or declared but not an enum
	}
	return opts, true
}

// allPerms is a PermissionConsumer that references nothing, so A7 runs with
// full information and every unreferenced permission is genuinely dead. Tests
// about rules OTHER than A7 use it simply to opt into the check running.
type allPerms struct{}

func (allPerms) UsedPermissions() []string { return nil }

// usedPerms is a PermissionConsumer reporting a fixed set of permissions as
// referenced by data-entry UI gates.
type usedPerms []string

func (u usedPerms) UsedPermissions() []string { return []string(u) }

// hasRule reports whether findings contain a finding with the given rule.
func hasRule(findings []Finding, rule string) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool { return f.Rule == rule })
}

// ruleSeverity returns the severity of the first finding with rule, or -1.
func ruleSeverity(findings []Finding, rule string) Severity {
	for _, f := range findings {
		if f.Rule == rule {
			return f.Severity
		}
	}
	return Severity(-1)
}

// ---- Tier A ------------------------------------------------------------

func TestAudit_A1_UngatedMembership(t *testing.T) {
	t.Parallel()
	// Default member-of, an assignment to a declared role, no requires_permission
	// gate → A1 high. (Covers the DEFAULT relation, not just configured.)
	p := &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Create: []string{"ticket"}, Read: []string{"ticket"}}},
		Assignments: map[string]string{"engineering": "editor"},
	}
	got := Audit(p, nil, allPerms{})
	if !hasRule(got, "A1-ungated-membership") {
		t.Fatalf("expected A1, got %+v", got)
	}
	if sev := ruleSeverity(got, "A1-ungated-membership"); sev != High {
		t.Errorf("A1 severity = %v, want high", sev)
	}
}

func TestAudit_A1_GatedMembership_NoFinding(t *testing.T) {
	t.Parallel()
	// Same, but member-of is gated by requires_permission → no A1.
	p := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"editor": {Create: []string{"ticket"}, Read: []string{"ticket"}, Permissions: []string{"delegate-membership"}}},
		Assignments:   map[string]string{"engineering": "editor"},
		RoleRelations: map[string]acl.RoleRelationDef{"member-of": {RequiresPermission: "delegate-membership"}},
	}
	if got := Audit(p, nil, allPerms{}); hasRule(got, "A1-ungated-membership") {
		t.Errorf("gated membership must not flag A1, got %+v", got)
	}
}

func TestAudit_A1_ReadOnlyGroup_NoFinding(t *testing.T) {
	t.Parallel()
	// RR-EG5D3E: a group assigned only a READ-ONLY role is a visibility choice,
	// not an escalation path. A1 must NOT fire (symmetric with A2). This is the
	// false-positive the design fought; gating must be on isPrivileged.
	p := &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"engineering": "reader"},
	}
	if got := Audit(p, nil, allPerms{}); hasRule(got, "A1-ungated-membership") {
		t.Errorf("read-only assigned role must not flag A1, got %+v", got)
	}
}

func TestAudit_A1b_InertMembership(t *testing.T) {
	t.Parallel()
	// Configured a non-default membership relation with no assignments → A1b low.
	p := &acl.Policy{MembershipRelation: "heeft_rol"}
	got := Audit(p, nil, allPerms{})
	if !hasRule(got, "A1b-inert-membership") {
		t.Fatalf("expected A1b, got %+v", got)
	}
	// Default member-of with no assignments must NOT flag A1b (common case).
	if got := Audit(&acl.Policy{}, nil, allPerms{}); hasRule(got, "A1b-inert-membership") {
		t.Errorf("default member-of with no assignments must not flag A1b, got %+v", got)
	}
}

func TestAudit_A2_UngatedRoleRelation(t *testing.T) {
	t.Parallel()
	// editor-of confers a privileged (write-granting) role, ungated → A2 high.
	p := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"editor": {Update: []string{"ticket"}, Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
	}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A2-ungated-role-relation") {
		t.Fatalf("expected A2, got %+v", got)
	}
	// Gated → no A2.
	p.RoleRelations["editor-of"] = acl.RoleRelationDef{Confers: "editor", RequiresPermission: "delegate-editor"}
	if got := Audit(p, nil, allPerms{}); hasRule(got, "A2-ungated-role-relation") {
		t.Errorf("gated role-relation must not flag A2, got %+v", got)
	}
}

func TestAudit_A2_NonPrivilegedRole_NoFinding(t *testing.T) {
	t.Parallel()
	// A role-relation conferring a read-only (non-privileged) role is fine ungated.
	p := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"reader": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"reader-of": {Confers: "reader"}},
	}
	if got := Audit(p, nil, allPerms{}); hasRule(got, "A2-ungated-role-relation") {
		t.Errorf("read-only role-relation must not flag A2, got %+v", got)
	}
}

func TestAudit_A3_EveryonePrivileged(t *testing.T) {
	t.Parallel()
	// everyone with a write grant → A3 critical.
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		acl.EveryoneRole: {Update: []string{"ticket"}},
	}}
	got := Audit(p, nil, allPerms{})
	if !hasRule(got, "A3-everyone-privileged") {
		t.Fatalf("expected A3, got %+v", got)
	}
	if sev := ruleSeverity(got, "A3-everyone-privileged"); sev != Critical {
		t.Errorf("A3 severity = %v, want critical", sev)
	}
}

func TestAudit_A3_EveryoneReadOnly_NoFinding(t *testing.T) {
	t.Parallel()
	// The DOCUMENTED `everyone: read: ["*"]` pattern must NOT flag A3 (RR-UR0LJU).
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		acl.EveryoneRole: {Read: []string{"*"}},
	}}
	if got := Audit(p, nil, allPerms{}); hasRule(got, "A3-everyone-privileged") {
		t.Errorf("read-only everyone must not flag A3, got %+v", got)
	}
}

func TestAudit_A4_AssignmentToUnknownRole(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "edutor"}, // typo
	}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A4-assignment-unknown-role") {
		t.Fatalf("expected A4, got %+v", got)
	}
}

func TestAudit_A5_ConfersUnknownRole(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "edutor"}},
	}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A5-confers-unknown-role") {
		t.Fatalf("expected A5, got %+v", got)
	}
}

func TestAudit_A6_UngrantablePermission(t *testing.T) {
	t.Parallel()
	// requires_permission names a perm no role grants → A6 low.
	p := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor", RequiresPermission: "delegate-nobody-has"}},
	}
	got := Audit(p, nil, allPerms{})
	if !hasRule(got, "A6-ungrantable-permission") {
		t.Fatalf("expected A6, got %+v", got)
	}
	if sev := ruleSeverity(got, "A6-ungrantable-permission"); sev != Low {
		t.Errorf("A6 severity = %v, want low (intentional lockdown may be valid)", sev)
	}
}

func TestAudit_A7_DeadPermission(t *testing.T) {
	t.Parallel()
	// A permission granted but referenced by no requires_permission → A7 low.
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"admin": {Create: []string{"*"}, Read: []string{"*"}, Permissions: []string{"delegate-unused"}},
	}}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A7-dead-permission") {
		t.Fatalf("expected A7, got %+v", got)
	}
	// Referenced permission → no A7.
	p2 := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"admin": {Create: []string{"*"}, Read: []string{"*"}, Permissions: []string{"delegate-x"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"member-of": {RequiresPermission: "delegate-x"}},
	}
	if got := Audit(p2, nil, allPerms{}); hasRule(got, "A7-dead-permission") {
		t.Errorf("referenced permission must not flag A7, got %+v", got)
	}
}

// AM-acl-builtin-permissions-audit-exempt. rela's own global permissions are
// granted through a role's `permissions:` list but consumed by read paths, not
// by a requires_permission gate — A7 must not call them dead. Driving the
// exemption off acl.BuiltinPermissions() (rather than a literal list here)
// means a newly added global constant fails this test instead of silently
// producing a false "dead" finding in an operator's audit output.
func TestAudit_A7_BuiltinPermissionsAreNotDead(t *testing.T) {
	t.Parallel()
	builtins := acl.BuiltinPermissions()
	if len(builtins) == 0 {
		t.Fatal("acl.BuiltinPermissions() is empty; A7 exemption would be a no-op")
	}
	for _, perm := range builtins {
		t.Run(perm, func(t *testing.T) {
			t.Parallel()
			// The reproduction from the bug report: a role granting only a
			// built-in, with no role_relations at all.
			p := &acl.Policy{Roles: map[string]acl.RoleDef{
				"admin": {Read: []string{"persoon"}, Permissions: []string{perm}},
			}}
			if got := Audit(p, nil, allPerms{}); hasRule(got, "A7-dead-permission") {
				t.Errorf("built-in permission %q reported dead, got %+v", perm, got)
			}
		})
	}
}

// AM-acl-audit-permission-consumers-complete, part 2 (the fail-safe half).
// A nil PermissionConsumer means the caller could not determine what the UI
// gates reference. A7 must then stay silent rather than assert config is dead
// on information it knows is incomplete.
func TestAudit_A7_NilConsumerSuppressesCheck(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"admin": {Create: []string{"*"}, Read: []string{"*"}, Permissions: []string{"report:sales"}},
	}}
	// Sanity: with full information this permission IS dead.
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A7-dead-permission") {
		t.Fatalf("precondition: expected A7 with a consumer present, got %+v", got)
	}
	if got := Audit(p, nil, nil); hasRule(got, "A7-dead-permission") {
		t.Errorf("nil consumer must suppress A7, got %+v", got)
	}
}

// A consumer that reports nothing because it holds nothing (a nil-receiver
// implementation) must be treated as a real answer, not as "could not look".
// Pins the distinction the CLI's typed-nil trap violated: a non-nil interface
// wrapping a nil pointer reaches the check and must not panic.
func TestAudit_A7_TypedNilConsumerIsAnAnswer(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"admin": {Create: []string{"*"}, Read: []string{"*"}, Permissions: []string{"report:sales"}},
	}}
	var typed *nilConsumer
	if got := Audit(p, nil, typed); !hasRule(got, "A7-dead-permission") {
		t.Errorf("a consumer reporting no permissions is complete information; expected A7, got %+v", got)
	}
}

// nilConsumer implements PermissionConsumer with a nil-safe receiver.
type nilConsumer struct{}

func (*nilConsumer) UsedPermissions() []string { return nil }

// AM-acl-audit-permission-consumers-complete, part 1. A permission referenced
// only by a data-entry UI gate is live config, not dead. Table-driven over the
// surfaces so the intent is explicit; the CLI adapter's per-surface coverage
// (that it actually collects each one) is tested in internal/cli.
func TestAudit_A7_UIGatedPermissionIsNotDead(t *testing.T) {
	t.Parallel()
	for _, surface := range []string{"document", "dashboard card", "navigation entry", "command"} {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			p := &acl.Policy{Roles: map[string]acl.RoleDef{
				"sales": {Read: []string{"persoon"}, Permissions: []string{"report:sales"}},
			}}
			if got := Audit(p, nil, usedPerms{"report:sales"}); hasRule(got, "A7-dead-permission") {
				t.Errorf("permission gating a %s reported dead, got %+v", surface, got)
			}
		})
	}
}

// A UI-referenced permission must not blanket-suppress A7 for others: a typo'd
// permission alongside a live one is still reported.
func TestAudit_A7_UIGateDoesNotMaskRealDeadPermission(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"sales": {Read: []string{"persoon"}, Permissions: []string{"report:sales", "report:sales-typo"}},
	}}
	got := Audit(p, nil, usedPerms{"report:sales"})
	if !hasRule(got, "A7-dead-permission") {
		t.Fatalf("expected A7 for the typo'd permission, got %+v", got)
	}
	for _, f := range got {
		if f.Rule == "A7-dead-permission" && !strings.Contains(f.Detail, "report:sales-typo") {
			t.Errorf("A7 fired for a UI-referenced permission: %+v", f)
		}
	}
}

// A built-in must not blanket-suppress A7: a genuinely dead permission granted
// alongside one is still reported. Guards against "fixing" the false positive
// by weakening the rule itself.
func TestAudit_A7_BuiltinDoesNotMaskRealDeadPermission(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"admin": {
			Read:        []string{"persoon"},
			Permissions: []string{acl.PermHistoryRead, "delegate-typoed"},
		},
	}}
	got := Audit(p, nil, allPerms{})
	if !hasRule(got, "A7-dead-permission") {
		t.Fatalf("expected A7 for the typo'd permission, got %+v", got)
	}
	for _, f := range got {
		if f.Rule == "A7-dead-permission" && strings.Contains(f.Detail, acl.PermHistoryRead) {
			t.Errorf("A7 fired for built-in %q: %+v", acl.PermHistoryRead, f)
		}
	}
}

func TestAudit_A9_WildcardWrite_NotRead(t *testing.T) {
	t.Parallel()
	// A non-everyone role with create:["*"] → A9. read:["*"] alone → no A9.
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"power": {Create: []string{"*"}, Read: []string{"*"}},
	}}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A9-wildcard-write") {
		t.Fatalf("expected A9 for create:[*], got %+v", got)
	}
	readOnly := &acl.Policy{Roles: map[string]acl.RoleDef{"viewer": {Read: []string{"*"}}}}
	if got := Audit(readOnly, nil, allPerms{}); hasRule(got, "A9-wildcard-write") {
		t.Errorf("read:[*] must not flag A9, got %+v", got)
	}
}

func TestAudit_A10_NameWhitespace(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{MembershipRelation: "heeft_rol ", Assignments: map[string]string{"x": "editor"}, Roles: map[string]acl.RoleDef{"editor": {Read: []string{"t"}}}}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A10-name-whitespace") {
		t.Fatalf("expected A10 for trailing space, got %+v", got)
	}
}

func TestAudit_A10_AssignmentKeyWhitespace(t *testing.T) {
	t.Parallel()
	// RR-KUOAVH: a padded assignment KEY silently matches no member.
	p := &acl.Policy{
		Assignments: map[string]string{"engineering ": "editor"},
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
	}
	if got := Audit(p, nil, allPerms{}); !hasRule(got, "A10-name-whitespace") {
		t.Fatalf("expected A10 for padded assignment key, got %+v", got)
	}
}

// ---- Tier B ------------------------------------------------------------

func TestAudit_B1_UndeclaredType(t *testing.T) {
	t.Parallel()
	meta := fakeMetamodel{types: map[string]bool{"ticket": true}}
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"editor": {Create: []string{"ticket", "tickets"}, Read: []string{"ticket"}}, // "tickets" is a typo
	}}
	if got := Audit(p, meta, allPerms{}); !hasRule(got, "B1-undeclared-type") {
		t.Fatalf("expected B1, got %+v", got)
	}
}

func TestAudit_B1_WildcardSkipped(t *testing.T) {
	t.Parallel()
	// A wildcard grant must NOT flag B1 — "*" is not an entity type (RR-TZ2S3G).
	meta := fakeMetamodel{types: map[string]bool{"ticket": true}}
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"admin": {Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"}, Read: []string{"*"}},
	}}
	if got := Audit(p, meta, allPerms{}); hasRule(got, "B1-undeclared-type") {
		t.Errorf("wildcard role must not flag B1, got %+v", got)
	}
}

func TestAudit_B2_UndeclaredRelation(t *testing.T) {
	t.Parallel()
	meta := fakeMetamodel{types: map[string]bool{"ticket": true}}
	// membership_relation names a relation the schema lacks → B2.
	p := &acl.Policy{MembershipRelation: "heeft_rol"}
	if got := Audit(p, meta, allPerms{}); !hasRule(got, "B2-undeclared-relation") {
		t.Fatalf("expected B2 for membership_relation, got %+v", got)
	}
}

func TestAudit_B3_MembershipFromMismatch(t *testing.T) {
	t.Parallel()
	// heeft_rol exists but its from is [persoon]; user_entity_type is "user".
	meta := fakeMetamodel{
		types:     map[string]bool{"user": true, "persoon": true, "rol": true},
		relations: map[string][]string{"heeft_rol": {"persoon"}},
	}
	p := &acl.Policy{UserEntityType: "user", MembershipRelation: "heeft_rol"}
	if got := Audit(p, meta, allPerms{}); !hasRule(got, "B3-membership-from-mismatch") {
		t.Fatalf("expected B3, got %+v", got)
	}
	// Compatible from → no B3.
	p.UserEntityType = "persoon"
	if got := Audit(p, meta, allPerms{}); hasRule(got, "B3-membership-from-mismatch") {
		t.Errorf("compatible from must not flag B3, got %+v", got)
	}
}

func TestAudit_B4_UndeclaredField(t *testing.T) {
	t.Parallel()
	meta := fakeMetamodel{
		types:  map[string]bool{"ticket": true},
		fields: map[string]map[string][]string{"ticket": {"status": {"open", "done"}}},
	}
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"triager": {Read: []string{"ticket"}, Fields: map[string][]acl.FieldGrant{
			"ticket": {{Field: "stutus"}}, // typo
		}},
	}}
	if got := Audit(p, meta, allPerms{}); !hasRule(got, "B4-undeclared-field") {
		t.Fatalf("expected B4, got %+v", got)
	}
}

func TestAudit_B5_UndeclaredOption(t *testing.T) {
	t.Parallel()
	meta := fakeMetamodel{
		types:  map[string]bool{"ticket": true},
		fields: map[string]map[string][]string{"ticket": {"status": {"open", "done"}}},
	}
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"triager": {Read: []string{"ticket"}, Options: map[string][]acl.OptionGrant{
			"ticket": {{Field: "status", Option: "finished"}}, // not in enum
		}},
	}}
	if got := Audit(p, meta, allPerms{}); !hasRule(got, "B5-undeclared-option") {
		t.Fatalf("expected B5, got %+v", got)
	}
}

func TestAudit_B5_AbsentField_ReportsUndeclaredNotNonEnum(t *testing.T) {
	t.Parallel()
	// RR-O50E4R: an options grant on a field that doesn't exist must report
	// B4-undeclared-field (a typo), not B5-options-non-enum (wrong diagnosis).
	meta := fakeMetamodel{
		types:  map[string]bool{"ticket": true},
		fields: map[string]map[string][]string{"ticket": {"status": {"open", "done"}}},
	}
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"triager": {Read: []string{"ticket"}, Options: map[string][]acl.OptionGrant{
			"ticket": {{Field: "stutus", Option: "open"}}, // field typo
		}},
	}}
	got := Audit(p, meta, allPerms{})
	if !hasRule(got, "B4-undeclared-field") {
		t.Errorf("absent options field must report B4-undeclared-field, got %+v", got)
	}
	if hasRule(got, "B5-options-non-enum") {
		t.Errorf("absent field must NOT be reported as non-enum, got %+v", got)
	}
}

func TestAudit_B7_UndeclaredUserType(t *testing.T) {
	t.Parallel()
	meta := fakeMetamodel{types: map[string]bool{"ticket": true}}
	p := &acl.Policy{UserEntityType: "persoon"} // not declared
	if got := Audit(p, meta, allPerms{}); !hasRule(got, "B7-undeclared-user-type") {
		t.Fatalf("expected B7, got %+v", got)
	}
}

func TestAudit_NilMetamodel_SkipsTierB(t *testing.T) {
	t.Parallel()
	// With nil meta, an undeclared-type grant cannot be checked → no B findings,
	// but Tier A still runs.
	p := &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Create: []string{"nonsense"}, Read: []string{"nonsense"}}},
		Assignments: map[string]string{"g": "editor"},
	}
	got := Audit(p, nil, allPerms{})
	for _, f := range got {
		if f.Rule[0] == 'B' {
			t.Errorf("nil meta must skip Tier B, got %+v", f)
		}
	}
}

// ---- Golden: the documented worked-example policy stays clean -----------

func TestAudit_CleanPolicy_NoFindings(t *testing.T) {
	t.Parallel()
	// A well-gated policy (everyone read-only, all relations gated, all names
	// declared) must produce ZERO findings. This is the anti-false-positive
	// guard: if a future check starts flagging a legitimate baseline, this fails.
	meta := fakeMetamodel{
		types:     map[string]bool{"person": true, "ticket": true, "group": true},
		relations: map[string][]string{"member-of": {"person"}},
	}
	p := &acl.Policy{
		UserEntityType: "person",
		Roles: map[string]acl.RoleDef{
			"admin":    {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}, Read: []string{"ticket"}, Permissions: []string{"delegate-membership"}},
			"everyone": {Read: []string{"*"}},
		},
		Assignments:   map[string]string{"ops-team": "admin"},
		RoleRelations: map[string]acl.RoleRelationDef{"member-of": {RequiresPermission: "delegate-membership"}},
	}
	if got := Audit(p, meta, allPerms{}); len(got) != 0 {
		t.Errorf("clean policy must produce zero findings, got %+v", got)
	}
}

func TestAudit_DeterministicOrder(t *testing.T) {
	t.Parallel()
	// Findings sort by severity, then rule, then subject — stable across runs.
	p := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			acl.EveryoneRole: {Update: []string{"ticket"}}, // A3 critical
			"power":          {Create: []string{"*"}},      // A9 medium
		},
		Assignments: map[string]string{"g": "power"}, // A1 high (ungated member-of)
	}
	first := Audit(p, nil, allPerms{})
	for range 20 {
		got := Audit(p, nil, allPerms{})
		if !slices.EqualFunc(got, first, func(a, b Finding) bool { return a == b }) {
			t.Fatalf("non-deterministic order:\n first=%+v\n got  =%+v", first, got)
		}
	}
	// Critical must sort before high before medium.
	if len(first) >= 2 && first[0].Severity > first[1].Severity {
		t.Errorf("findings not severity-sorted: %+v", first)
	}
}

func TestParseSeverity(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want Severity
		ok   bool
	}{
		"critical": {Critical, true},
		"high":     {High, true},
		"medium":   {Medium, true},
		"low":      {Low, true},
		"nit":      {Nit, true},
		"any":      {Nit, true}, // "any" == least-severe → matches every finding
		"bogus":    {0, false},
		"":         {0, false},
	}
	for in, want := range cases {
		got, ok := ParseSeverity(in)
		if ok != want.ok || (ok && got != want.want) {
			t.Errorf("ParseSeverity(%q) = (%v, %v), want (%v, %v)", in, got, ok, want.want, want.ok)
		}
	}
}

func TestHasAtLeast(t *testing.T) {
	t.Parallel()
	findings := []Finding{{Severity: Medium}, {Severity: Low}}
	if HasAtLeast(findings, High) {
		t.Error("HasAtLeast(High) should be false when worst is medium")
	}
	if !HasAtLeast(findings, Medium) {
		t.Error("HasAtLeast(Medium) should be true")
	}
	if !HasAtLeast([]Finding{{Severity: Critical}}, High) {
		t.Error("HasAtLeast(High) should be true when a critical is present")
	}
}

// TestGrantEntityType_MatchesACLSplit pins the duplication this package
// accepts: grantEntityType mirrors internal/acl's unexported grantTypeOf,
// and the two must agree or the audit reports a type the runtime never
// evaluated.
//
// Verified through acl's exported behavior rather than by calling the
// unexported helper: a role granting update on `T@p` must be reported by
// the audit against type T, which is only true if both split the same way.
func TestGrantEntityType_MatchesACLSplit(t *testing.T) {
	for _, tc := range []struct{ entry, wantType string }{
		{"page", "page"},
		{"page@draft", "page"},
		{"*", "*"},
		{"some property@draft", "some property"},
		{"review-response@published", "review-response"},
	} {
		p := &acl.Policy{Roles: map[string]acl.RoleDef{
			"r": {Update: []string{tc.entry}, Read: []string{"*"}},
		}}
		findings := Audit(p, fakeMetamodel{types: map[string]bool{tc.wantType: true}}, nil)
		for _, f := range findings {
			if f.Rule == "B1-undeclared-type" {
				t.Errorf("entry %q: audit did not resolve it to type %q (finding: %s)",
					tc.entry, tc.wantType, f.Detail)
			}
		}
	}
}

// TestRefusalIsWiderThanOrEqualToA2 pins the DIRECTIONAL relationship
// between the advisory finding and the boot refusal, which is the property
// that actually matters — not that the two are identical.
//
// The invariant: whenever A2 flags a policy, the boot refusal also refuses
// it (given the refusal's own trigger, a non-default world read grant).
// Stated as sets: refusal ⊇ A2. An operator can therefore never see
// `rela acl audit` report clean on a policy the server refuses for an
// A2-shaped reason.
//
// The reverse gap is deliberate and is NOT a violation: the refusal may
// fire where A2 is silent, because the refusal is scoped to policies that
// grant a non-default world and A2 is not. That case is
// TestRefusal_ReadOnlyRoleHoldingWorldGrantCountsAsEscalation in
// internal/acl.
//
// If this ever fails, the two predicates have been allowed to drift apart
// in the dangerous direction — the audit has become MORE permissive than
// the gate, so a policy would pass the linter and fail to boot.
func TestRefusalIsWiderThanOrEqualToA2(t *testing.T) {
	roles := map[string]acl.RoleDef{
		"writer":      {Update: []string{"page"}, Read: []string{"page"}},
		"permholder":  {Permissions: []string{"something"}},
		"creator":     {Create: []string{"page"}},
		"deleter":     {Delete: []string{"page"}, Read: []string{"page"}},
		"reader":      {Read: []string{"page"}},
		"worldreader": {Read: []string{"page", "world:published"}},
	}
	for roleName := range roles {
		t.Run(roleName, func(t *testing.T) {
			p := &acl.Policy{
				// A non-default world grant somewhere in the policy is the
				// refusal's trigger; without it the refusal never evaluates
				// the escalation arms at all.
				Roles: mergeRoles(roles, map[string]acl.RoleDef{
					"everyone": {Read: []string{"world:published"}},
				}),
				RoleRelations: map[string]acl.RoleRelationDef{
					"owns": {Confers: roleName},
				},
			}
			if err := p.Validate(); err != nil && !strings.Contains(err.Error(), "refusing to load") {
				t.Fatalf("unexpected non-refusal load error: %v", err)
			}
			flaggedByA2 := len(checkUngatedRoleRelations(p)) > 0
			refused := p.WorldGrantRefusalReason() != ""
			if flaggedByA2 && !refused {
				t.Errorf("A2 flags role %q but the boot refusal does not refuse it — "+
					"the audit is now MORE permissive than the gate, so this policy "+
					"passes the linter and fails to boot", roleName)
			}
		})
	}
}

// TestA2_ReadOnlyRoleStillSilent pins that widening A2's criterion to
// include world grants did NOT reintroduce the read-only false positive the
// audit design fought (RR-LXI3NW / RR-UR0LJU / RR-EG5D3E).
func TestA2_ReadOnlyRoleStillSilent(t *testing.T) {
	p := &acl.Policy{
		Roles:         map[string]acl.RoleDef{"viewer": {Read: []string{"page", "*"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"owns": {Confers: "viewer"}},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := checkUngatedRoleRelations(p); len(got) != 0 {
		t.Errorf("a role holding only ordinary read grants — even read: [\"*\"] — "+
			"is a visibility choice, not an escalation path; got %d finding(s): %+v",
			len(got), got)
	}
}

func mergeRoles(a, b map[string]acl.RoleDef) map[string]acl.RoleDef {
	out := make(map[string]acl.RoleDef, len(a)+len(b))
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
}
