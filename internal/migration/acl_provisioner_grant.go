package migration

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ACLProvisionerGrantMigration{})
}

// Grant shape written into acl.yaml. The role name is deliberately specific
// enough not to collide with an operator's own naming.
const (
	provisionerRoleName = "provisioner-system"
	// ProvisionerPrincipal must stay in lockstep with
	// principal.UserProvisioner. It is duplicated as a literal rather than
	// imported because arch-lint restricts internal/migration to [storage] —
	// importing internal/principal fails `just arch-lint`.
	//
	// Exported so a test can assert the equality directly, independent of
	// whether Apply happens to work on any given input.
	ProvisionerPrincipal = "system:provisioner"
)

// ACLProvisionerGrantMigration grants the lazy-provisioner's fixed system
// identity `create` access to the policy's user entity type in an existing
// acl.yaml.
//
// Why this exists: `unmatched_principal: provision` (TKT-ANUJDS) lazily creates
// a stub user entity, on the first write by a verified principal that resolves
// to no user entity, under the `system:provisioner` identity. That create is
// ACL-authorized like any other write, so without a grant the provisioner can
// create NOTHING and every unmatched principal's first write fails at the
// provision step. This migration injects the minimal grant that makes provision
// work.
//
// The grant is deliberately MINIMAL — `create: [<user_entity_type>]` and
// nothing else (the bare-stub containment, RR-28SCW3). Create implies no read
// (policy.go), and the provisioner is given no update/delete/read and no other
// type, so it cannot author group edges or touch anything but a fresh stub of
// the one user type. Group/local-role membership arrives later via the webhook,
// a reconcile, or an admin — never through this identity.
//
// It only ever edits an acl.yaml that already exists AND declares
// `unmatched_principal: provision` with a `user_entity_type`. A project without
// an acl.yaml runs on NopACL, where provision is not configurable and nothing
// needs granting; the migrate runner's skip-on-missing behavior gives that for
// free. A project whose policy is not `provision` is left entirely alone.
type ACLProvisionerGrantMigration struct{}

func (m *ACLProvisionerGrantMigration) Name() string {
	return "acl-provisioner-grant"
}

func (m *ACLProvisionerGrantMigration) Description() string {
	return "Grant the provisioner identity create access to the user entity type " +
		"in acl.yaml (required by unmatched_principal: provision)"
}

func (m *ACLProvisionerGrantMigration) FileTypes() []FileType {
	return []FileType{FileTypeACL}
}

// Detect reports whether the policy opts into provision AND the provisioner
// still lacks a create grant on the declared user entity type.
//
// The predicate mirrors Apply's postcondition: "is the provisioner granted
// create on the user type?", NOT merely "does an assignment key exist" — a bare
// key check would go quiet on a grant naming an undefined role, or one that
// grants create on the wrong type, both of which leave provision broken while
// reporting the migration done.
//
// A policy that is not `unmatched_principal: provision`, or that declares no
// `user_entity_type`, needs nothing and is not detected. (A provision policy
// without a user_entity_type is itself a load error — Policy.Validate rejects
// it — so this migration never has to invent a target type.)
func (m *ACLProvisionerGrantMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return false
	}
	userType := provisionUserType(root)
	if userType == "" {
		return false
	}
	return !provisionerCanCreate(root, userType)
}

// Apply adds the provisioner role (if absent) granting create on the user type,
// and the assignment binding the provisioner principal to it. Existing keys are
// never overwritten: if the operator already defines a role by this name, we
// bind to theirs, repairing only a missing create grant.
//
// Apply verifies its own postcondition. Returning an error here stops the
// runner BEFORE it writes (runner.go), leaving the operator's file untouched
// and the failure loud, rather than persisting a file that parses but grants
// nothing.
func (m *ACLProvisionerGrantMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	userType := provisionUserType(root)
	if userType == "" {
		// Not a provision policy (or no user type): nothing to grant. Detect
		// would not have selected this, but Apply stays defensive.
		return nil
	}

	m.ensureRole(root, userType)
	m.ensureAssignment(root)

	if !provisionerCanCreate(root, userType) {
		return fmt.Errorf(
			"acl-provisioner-grant: after applying, %q still cannot create %q; "+
				"acl.yaml was left unchanged — grant it manually",
			ProvisionerPrincipal, userType)
	}
	return nil
}

// provisionUserType returns the declared user_entity_type IFF the policy is
// unmatched_principal: provision, else "". Both must be present for the
// migration to have a target.
func provisionUserType(root *yaml.Node) string {
	mode := GetMapValue(root, "unmatched_principal")
	if mode == nil || strings.TrimSpace(mode.Value) != "provision" {
		return ""
	}
	ut := GetMapValue(root, "user_entity_type")
	if ut == nil {
		return ""
	}
	return strings.TrimSpace(ut.Value)
}

// provisionerCanCreate reports whether the policy grants the provisioner
// principal create on userType via a role that actually exists and lists it (or
// "*") under create.
//
// Both assignment maps are consulted: `assignments` (principal → one role) and
// `asserted_role_assignments` (principal → role or list of roles). An operator
// who scoped the provisioner through the asserted map has already thought about
// this, and must not have a grant added on top.
func provisionerCanCreate(root *yaml.Node, userType string) bool {
	roles := GetMapValue(root, "roles")
	if roles == nil || roles.Kind != yaml.MappingNode {
		return false
	}
	grantsCreate := func(roleName string) bool {
		return hasCreateTarget(GetMapValue(roles, roleName), userType)
	}

	assignments := GetMapValue(root, "assignments")
	if assignments != nil && assignments.Kind == yaml.MappingNode {
		assigned := GetMapValue(assignments, ProvisionerPrincipal)
		if assigned != nil && grantsCreate(assigned.Value) {
			return true
		}
	}

	asserted := GetMapValue(root, "asserted_role_assignments")
	if asserted == nil || asserted.Kind != yaml.MappingNode {
		return false
	}
	entry := GetMapValue(asserted, ProvisionerPrincipal)
	if entry == nil {
		return false
	}
	if entry.Kind == yaml.SequenceNode {
		for _, r := range entry.Content {
			if grantsCreate(r.Value) {
				return true
			}
		}
		return false
	}
	return grantsCreate(entry.Value)
}

// ensureRole adds `provisioner-system: {create: [<userType>]}` under `roles:`
// unless a role of that name is already defined.
func (m *ACLProvisionerGrantMigration) ensureRole(root *yaml.Node, userType string) {
	roles := EnsureMapping(root, "", "roles")
	if roles == nil {
		return
	}
	if GetMapValue(roles, provisionerRoleName) != nil {
		// The operator defines a role by this name. Keep their definition, but
		// make sure it grants create on the user type — binding to a role that
		// grants nothing just relocates the bug this migration exists to fix.
		m.ensureRoleGrantsCreate(root, provisionerRoleName, userType)
		return
	}

	roleDef := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "create"},
		createList(userType),
	}}
	roleKey := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: provisionerRoleName,
		HeadComment: "Added by migration: lets unmatched_principal: provision create\n" +
			"a stub user entity. Deliberately create-only on the user type.",
	}
	roles.Content = append(roles.Content, roleKey, roleDef)
}

// createList builds the `create: [<userType>]` value node.
func createList(userType string) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: userType},
	}}
}

// ensureAssignment binds the provisioner principal to a role that grants
// create. An existing assignment is KEPT — repointing an operator's deliberate
// choice would be presumptuous — but the role it names is repaired if it lacks
// the create grant, since an assignment to a dead or under-granting role reads
// exactly like no assignment at all.
func (m *ACLProvisionerGrantMigration) ensureAssignment(root *yaml.Node) {
	assignments := EnsureMapping(root, "roles", "assignments")
	if assignments == nil {
		return
	}
	if existing := GetMapValue(assignments, ProvisionerPrincipal); existing != nil {
		m.ensureRoleGrantsCreate(root, existing.Value, provisionUserType(root))
		return
	}
	assignments.Content = append(assignments.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: ProvisionerPrincipal},
		&yaml.Node{Kind: yaml.ScalarNode, Value: provisionerRoleName},
	)
}

// ensureRoleGrantsCreate gives `roleName` a create grant on userType if it has
// none — covering both an undefined role (a dangling assignment) and one
// defined but not granting the user type. Roles that already grant it are
// untouched.
func (m *ACLProvisionerGrantMigration) ensureRoleGrantsCreate(root *yaml.Node, roleName, userType string) {
	if roleName == "" || userType == "" {
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
	if hasCreateTarget(def, userType) {
		return
	}
	SetMapNode(def, "create", createList(userType))
}

// hasCreateTarget reports whether a role definition grants create on userType,
// either by listing it explicitly or via the "*" wildcard. An empty or missing
// `create:` grants nothing, which is indistinguishable from having no role.
func hasCreateTarget(roleDef *yaml.Node, userType string) bool {
	if roleDef == nil || roleDef.Kind != yaml.MappingNode {
		return false
	}
	create := GetMapValue(roleDef, "create")
	if create == nil || create.Kind != yaml.SequenceNode {
		return false
	}
	for _, t := range create.Content {
		if t.Value == "*" || t.Value == userType {
			return true
		}
	}
	return false
}
