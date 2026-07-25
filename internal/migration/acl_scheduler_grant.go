package migration

import (
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ACLSchedulerGrantMigration{})
}

// Grant shape written into acl.yaml. The role name is deliberately
// specific enough not to collide with an operator's own naming.
const (
	schedulerRoleName = "scheduler-system"
	// schedulerPrincipal must stay in lockstep with
	// principal.UserScheduler. It is duplicated as a literal rather than
	// imported because internal/migration deliberately depends on nothing
	// but yaml — see the package doc. The pairing is pinned by a test.
	schedulerPrincipal = "system:scheduler"
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

// Detect reports whether acl.yaml lacks an assignment for the scheduler
// principal. An operator who has already assigned it a role — of any name,
// scoped however they like — is left alone.
func (m *ACLSchedulerGrantMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return false
	}
	assignments := GetMapValue(root, "assignments")
	if assignments == nil || assignments.Kind != yaml.MappingNode {
		// No assignments block at all: the scheduler is certainly unassigned.
		return true
	}
	return GetMapValue(assignments, schedulerPrincipal) == nil
}

// Apply adds the scheduler role (if absent) and the assignment binding the
// scheduler principal to it. Existing keys are never overwritten: if the
// operator already defines a role by this name, we bind to theirs rather
// than replacing it.
func (m *ACLSchedulerGrantMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}

	m.ensureRole(root)
	m.ensureAssignment(root)
	return nil
}

// ensureRole adds `scheduler-system: {read: ["*"]}` under `roles:` unless a
// role of that name is already defined.
//
// read: ["*"] restores exactly the pre-TKT-ZF2DTV behavior, where scheduled
// tasks read the whole graph. Narrowing it is the operator's call, and
// `run_as` exists so a job can be given a tighter identity instead.
func (m *ACLSchedulerGrantMigration) ensureRole(root *yaml.Node) {
	roles := GetMapValue(root, "roles")
	if roles == nil || roles.Kind != yaml.MappingNode {
		roles = &yaml.Node{Kind: yaml.MappingNode}
		// Roles are conventionally declared before assignments; when neither
		// exists this appends, which is also fine.
		InsertMapKeyAfter(root, "", "roles", roles)
	}
	if GetMapValue(roles, schedulerRoleName) != nil {
		return // operator already defines this role — bind to theirs
	}

	wildcard := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "*", Style: yaml.DoubleQuotedStyle},
	}}
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

// ensureAssignment binds the scheduler principal to the role.
func (m *ACLSchedulerGrantMigration) ensureAssignment(root *yaml.Node) {
	assignments := GetMapValue(root, "assignments")
	if assignments == nil || assignments.Kind != yaml.MappingNode {
		assignments = &yaml.Node{Kind: yaml.MappingNode}
		InsertMapKeyAfter(root, "roles", "assignments", assignments)
	}
	if GetMapValue(assignments, schedulerPrincipal) != nil {
		return
	}
	assignments.Content = append(assignments.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: schedulerPrincipal},
		&yaml.Node{Kind: yaml.ScalarNode, Value: schedulerRoleName},
	)
}
