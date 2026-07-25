package migration

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ACLSchedulerGrantMigration{})
}

// Grant shape written into acl.yaml. The role name is deliberately
// specific enough not to collide with an operator's own naming.
const (
	schedulerRoleName = "scheduler-system"
	// SchedulerPrincipal must stay in lockstep with
	// principal.UserScheduler. It is duplicated as a literal rather than
	// imported because arch-lint restricts internal/migration to
	// [storage] — importing internal/principal fails `just arch-lint`.
	//
	// Exported so a test can assert the equality directly, independent of
	// whether Apply happens to work on any given input.
	SchedulerPrincipal = "system:scheduler"
)

// ACLSchedulerGrantMigration grants the scheduler's fixed system identity
// read access in an existing acl.yaml.
//
// Why this exists: since scheduled Lua reads became ACL-bound (TKT-ZF2DTV,
// DEC-O59WM4), a task's script sees only what its identity may see. A
// project that has an acl.yaml but never assigned the scheduler a role
// therefore has tasks that read NOTHING — silently, because a gated read
// returns "not found" rather than an error. This migration restores them.
//
// It only ever edits an acl.yaml that already exists. A project without one
// runs on NopACL, where scheduled tasks already read the full graph and have
// nothing to repair; creating a policy there would flip every OTHER principal
// (humans, CLI, MCP) to deny-by-default, breaking far more than it fixed
// (RR-SVQ5HE). The migrate runner's skip-on-missing behavior gives that for
// free.
//
// The grant is read-only. Writes continue through entitymanager's own ACL.
type ACLSchedulerGrantMigration struct{}

func (m *ACLSchedulerGrantMigration) Name() string {
	return "acl-scheduler-grant"
}

func (m *ACLSchedulerGrantMigration) Description() string {
	return "Grant the scheduler identity read access in acl.yaml " +
		"(without it, scheduled tasks silently read nothing)"
}

func (m *ACLSchedulerGrantMigration) FileTypes() []FileType {
	return []FileType{FileTypeACL}
}

// Detect reports whether the scheduler still lacks a usable read grant.
//
// The predicate is "is the scheduler granted read?", NOT merely "does an
// assignment key exist". A bare key check would go quiet on a grant that
// names an undefined role, or one whose role grants no read — both of
// which leave tasks reading nothing while reporting the migration done.
// Apply asserts this same predicate as its postcondition.
//
// An operator who has already granted the scheduler read — under any role
// name, scoped however they like, via `assignments` or
// `asserted_role_assignments` — is left alone.
func (m *ACLSchedulerGrantMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return false
	}
	return !schedulerCanRead(root)
}

// Apply adds the scheduler role (if absent) and the assignment binding the
// scheduler principal to it. Existing keys are never overwritten: if the
// operator already defines a role by this name, we bind to theirs rather
// than replacing it.
//
// Apply verifies its own postcondition. Every failure mode this migration
// has had produced a file that parsed, validated, and granted nothing —
// so "wrote something" is not evidence of success. Returning an error here
// stops the runner BEFORE it writes (runner.go), leaving the operator's
// file untouched and the failure loud.
func (m *ACLSchedulerGrantMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}

	m.ensureRole(root)
	m.ensureAssignment(root)

	if !schedulerCanRead(root) {
		return fmt.Errorf(
			"acl-scheduler-grant: after applying, %q still has no role granting read; "+
				"acl.yaml was left unchanged — grant it manually",
			SchedulerPrincipal)
	}
	return nil
}

// schedulerCanRead reports whether the policy grants the scheduler
// principal read via a role that actually exists and lists read targets.
//
// Both assignment maps are consulted: `assignments` (principal → one role)
// and `asserted_role_assignments` (principal → role or list of roles). An
// operator who scoped the scheduler through the asserted map has already
// thought about this, and must not have a wildcard grant added on top.
func schedulerCanRead(root *yaml.Node) bool {
	roles := GetMapValue(root, "roles")
	if roles == nil || roles.Kind != yaml.MappingNode {
		return false
	}
	grantsRead := func(roleName string) bool {
		return hasReadTargets(GetMapValue(roles, roleName))
	}

	assignments := GetMapValue(root, "assignments")
	if assignments != nil && assignments.Kind == yaml.MappingNode {
		assigned := GetMapValue(assignments, SchedulerPrincipal)
		if assigned != nil && grantsRead(assigned.Value) {
			return true
		}
	}

	asserted := GetMapValue(root, "asserted_role_assignments")
	if asserted == nil || asserted.Kind != yaml.MappingNode {
		return false
	}
	entry := GetMapValue(asserted, SchedulerPrincipal)
	if entry == nil {
		return false
	}
	// RoleList is scalar-or-sequence.
	if entry.Kind == yaml.SequenceNode {
		for _, r := range entry.Content {
			if grantsRead(r.Value) {
				return true
			}
		}
		return false
	}
	return grantsRead(entry.Value)
}

// ensureRole adds `scheduler-system: {read: ["*"]}` under `roles:` unless a
// role of that name is already defined.
//
// read: ["*"] restores exactly the pre-TKT-ZF2DTV behavior, where scheduled
// tasks read the whole graph. Narrowing it is the operator's call, and
// `run_as` exists so a job can be given a tighter identity instead.
func (m *ACLSchedulerGrantMigration) ensureRole(root *yaml.Node) {
	// EnsureMapping, not InsertMapKeyAfter: `roles:` with nothing under it
	// is a null scalar, and inserting over an existing key no-ops.
	roles := EnsureMapping(root, "", "roles")
	if roles == nil {
		return
	}
	if GetMapValue(roles, schedulerRoleName) != nil {
		// The operator defines a role by this name. Keep their definition,
		// but make sure it grants read — binding to a role that grants
		// nothing just relocates the bug this migration exists to fix.
		m.ensureRoleGrantsRead(root, schedulerRoleName)
		return
	}

	wildcard := wildcardRead()
	roleDef := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "read"},
		wildcard,
	}}
	roleKey := &yaml.Node{
		Kind:        yaml.ScalarNode,
		Value:       schedulerRoleName,
		HeadComment: "Added by migration: lets scheduled tasks read. Narrow or\nremove this once each job has a run_as identity of its own.",
	}
	roles.Content = append(roles.Content, roleKey, roleDef)
}

// wildcardRead builds the `read: ["*"]` value node.
func wildcardRead() *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "*", Style: yaml.DoubleQuotedStyle},
	}}
}

// ensureAssignment binds the scheduler principal to a role that grants read.
//
// When an assignment already exists it is KEPT — repointing an operator's
// deliberate choice would be presumptuous — but the role it names is
// repaired if it grants nothing, since an assignment to a dead or undefined
// role reads exactly like no assignment at all.
func (m *ACLSchedulerGrantMigration) ensureAssignment(root *yaml.Node) {
	assignments := EnsureMapping(root, "roles", "assignments")
	if assignments == nil {
		return
	}
	if existing := GetMapValue(assignments, SchedulerPrincipal); existing != nil {
		m.ensureRoleGrantsRead(root, existing.Value)
		return
	}
	assignments.Content = append(assignments.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: SchedulerPrincipal},
		&yaml.Node{Kind: yaml.ScalarNode, Value: schedulerRoleName},
	)
}

// ensureRoleGrantsRead gives `roleName` a read list if it has none —
// covering both an undefined role (a dangling assignment) and one defined
// but empty. Roles that already grant read are untouched.
func (m *ACLSchedulerGrantMigration) ensureRoleGrantsRead(root *yaml.Node, roleName string) {
	if roleName == "" {
		return
	}
	roles := EnsureMapping(root, "", "roles")
	if roles == nil {
		return
	}
	def := EnsureMapping(roles, "", roleName)
	if def == nil {
		return
	}
	if hasReadTargets(def) {
		return
	}
	SetMapNode(def, "read", wildcardRead())
}

// hasReadTargets reports whether a role definition lists at least one read
// target. An empty or missing `read:` grants nothing, which is
// indistinguishable from having no role at all.
func hasReadTargets(roleDef *yaml.Node) bool {
	if roleDef == nil || roleDef.Kind != yaml.MappingNode {
		return false
	}
	read := GetMapValue(roleDef, "read")
	return read != nil && read.Kind == yaml.SequenceNode && len(read.Content) > 0
}
