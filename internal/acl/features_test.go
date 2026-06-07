package acl_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// TestFeature_UC1_GroupConferredWrite pins the smallest interesting
// design property of v1: a role assigned to a group reaches the
// principals who are members of that group.
//
// Scenario. A small engineering team runs a shared issue tracker.
// Every engineer is a member of the `engineering` team; the team
// itself is assigned the `editor` role. Nobody is named individually
// in the policy.
//
// As Alice (member of `engineering`), I expect to be able to update
// TKT-001 — my team has the editor role even though I'm not
// personally listed. An operator looking at the audit record expects
// to see that the role was conferred via my group membership, not
// direct assignment.
func TestFeature_UC1_GroupConferredWrite(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  editor:   { write: [ticket], read: [ticket] }
  everyone: { read: [project] }
assignments:
  engineering: editor
`).
		Person("alice").
		Team("engineering").
		Ticket("TKT-001").
		Relation("alice", "member-of", "engineering").
		Build(t)

	w.AssertAllow("alice", acl.OpUpdate, acl.EntitySubject{Type: "ticket", ID: "TKT-001"})
	w.AssertPrimarySource("alice", "TKT-001", "editor",
		acl.Source{Kind: acl.SourceGroup, Group: "engineering"})
}

// TestFeature_UC2_NestedGroupsAllowAll exercises the transitive
// member-of walk and the AllowAll fast-path on list reads.
//
// Scenario. A larger company organizes engineering as
// `engineering ⊂ all-staff`. The `all-staff` team is assigned the
// `viewer` role — everyone in the company can read everything.
// Alice is in `engineering`, two hops away from the role grant.
//
// As Alice opening the ticket list view, I expect to see every
// ticket — my nested membership in `all-staff` (via `engineering`)
// confers read-on-everything. As a backend engineer, I expect the
// server to compute Alice's group set once per request, not once per
// entity.
func TestFeature_UC2_NestedGroupsAllowAll(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  viewer: { read: ["*"] }
assignments:
  all-staff: viewer
`).
		Person("alice").
		Team("engineering").
		Team("all-staff").
		Ticket("TKT-001").
		Ticket("TKT-002").
		Ticket("TKT-003").
		Relation("alice", "member-of", "engineering").
		Relation("engineering", "member-of", "all-staff").
		Build(t)

	w.AssertVisible("alice", "ticket", "TKT-001", "TKT-002", "TKT-003")
}

// TestFeature_UC3_ContainmentRead pins the containment-inheritance
// design property: a role granted on a folder propagates to every
// entity inside it via `belongs-to`.
//
// Scenario. A document-management workspace organizes files in
// nested folders:
//
//	F-root
//	├── F-eng        ← alice editor-of
//	│     ├── D-overview
//	│     └── F-mobile
//	│           ├── D-roadmap
//	│           └── F-leak
//	│                 └── D-secret
//	└── F-public
//	      └── D-readme
//
// Alice has `editor-of` on F-eng. Per the policy
// (`inherit_roles_through: [belongs-to]`), her grant should flow
// down to every document inside F-eng's subtree.
//
// As Alice opening the documents list, I expect to see exactly the
// three documents inside F-eng's subtree; D-readme (sibling
// subtree) should be hidden. As Alice editing D-secret three
// levels deep, I expect the write to be allowed and the audit
// record to attribute the grant to F-eng (the ancestor that holds
// the role-relation), not to the document itself.
func TestFeature_UC3_ContainmentRead(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  editor: { write: [folder, document], read: [folder, document] }
inherit_roles_through: [belongs-to]
role_relations:
  editor-of: { confers: editor }
`).
		Folder("F-root").
		Folder("F-eng", Inside("F-root")).
		Folder("F-public", Inside("F-root")).
		Folder("F-mobile", Inside("F-eng")).
		Folder("F-leak", Inside("F-mobile")).
		Document("D-overview", Inside("F-eng")).
		Document("D-roadmap", Inside("F-mobile")).
		Document("D-secret", Inside("F-leak")).
		Document("D-readme", Inside("F-public")).
		Person("alice").
		Relation("alice", "editor-of", "F-eng").
		Build(t)

	w.AssertVisible("alice", "document", "D-overview", "D-roadmap", "D-secret")

	w.AssertAllow("alice", acl.OpUpdate, acl.EntitySubject{Type: "document", ID: "D-secret"})
	w.AssertPrimarySource("alice", "D-secret", "editor",
		acl.Source{Kind: acl.SourceLocalViaAncestor, Ancestor: "F-eng", Relation: "editor-of"})

	w.AssertDeny("alice", acl.OpUpdate, acl.EntitySubject{Type: "document", ID: "D-readme"})
}

// TestFeature_UC4_MultiParentUnion pins the union semantics for
// containment: an entity filed under multiple parents inherits the
// union of grants, not the intersection.
//
// Scenario. Extends UC3. D-roadmap is filed under BOTH F-mobile
// (inside F-eng's subtree) AND F-public (where alice has no grant).
// Operators expect "filing in more places makes more people see it",
// which is the union rule.
//
// As Alice (editor-of F-eng), I expect to still see and edit
// D-roadmap — my grant on F-eng reaches it via the F-mobile parent.
// The fact that D-roadmap also lives in F-public (where I have no
// grant) does not revoke my access.
func TestFeature_UC4_MultiParentUnion(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  editor: { write: [folder, document], read: [folder, document] }
inherit_roles_through: [belongs-to]
role_relations:
  editor-of: { confers: editor }
`).
		Folder("F-root").
		Folder("F-eng", Inside("F-root")).
		Folder("F-public", Inside("F-root")).
		Folder("F-mobile", Inside("F-eng")).
		Document("D-roadmap", Inside("F-mobile")).
		Document("D-readme", Inside("F-public")).
		Person("alice").
		Relation("D-roadmap", "belongs-to", "F-public"). // second parent
		Relation("alice", "editor-of", "F-eng").
		Build(t)

	w.AssertContains("alice", "document", "D-roadmap")
	w.AssertHidden("alice", "document", "D-readme")
	w.AssertAllow("alice", acl.OpUpdate, acl.EntitySubject{Type: "document", ID: "D-roadmap"})
}

// TestFeature_UC5_MultiSourceAttribution pins the multi-source case:
// the same role conferred via multiple paths produces multiple
// attributions, and the primary is picked deterministically.
//
// Scenario. Alice is in the `eng-leads` group, which is assigned
// the `editor` role globally (group path). The `eng-leads` group
// also has `editor-of` on PRJ-flagship (local-via-group path).
// Alice also has `editor-of` on PRJ-flagship directly (local path).
//
// Three distinct Source values should land for the role `editor`,
// all preserved in the attribution chain. The primary credited in
// RuleID is the highest-priority one (Group < Local <
// LocalViaGroup, per AC8a's sort).
func TestFeature_UC5_MultiSourceAttribution(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  editor: { write: [project], read: [project] }
assignments:
  eng-leads: editor
role_relations:
  editor-of: { confers: editor }
`).
		Person("alice").
		Team("eng-leads").
		Project("PRJ-flagship").
		Relation("alice", "member-of", "eng-leads").
		Relation("eng-leads", "editor-of", "PRJ-flagship").
		Relation("alice", "editor-of", "PRJ-flagship").
		Build(t)

	w.AssertAttribution("alice", "PRJ-flagship", "editor",
		acl.Source{Kind: acl.SourceGroup, Group: "eng-leads"},
		acl.Source{Kind: acl.SourceLocal, Relation: "editor-of"},
		acl.Source{Kind: acl.SourceLocalViaGroup, Group: "eng-leads", Relation: "editor-of"},
	)

	w.AssertPrimarySource("alice", "PRJ-flagship", "editor",
		acl.Source{Kind: acl.SourceGroup, Group: "eng-leads"})
}

// TestFeature_UC6_DelegateXRelationWrite pins the existing v0 design
// property — granting a role requires holding the corresponding
// delegate-X permission — as a regression guard at the feature layer.
//
// Scenario. The policy declares `editor-of` as a role-relation that
// confers `editor`, and gates writing it on the `delegate-editor`
// permission. Jeroen (admin) holds `delegate-editor`; Alice (editor)
// does not.
//
// As Jeroen, I can author `someone --editor-of--> something` because
// I hold the delegation permission. As Alice, the same write is
// denied — even though I have the `editor` role myself, that
// doesn't authorize me to grant it to others.
func TestFeature_UC6_DelegateXRelationWrite(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  admin:  { write: ["*"], permissions: [delegate-editor] }
  editor: { write: [ticket] }
assignments:
  jeroen: admin
  alice:  editor
role_relations:
  editor-of: { confers: editor, requires_permission: delegate-editor }
`).
		Person("jeroen").
		Person("alice").
		Ticket("TKT-001").
		Build(t)

	rs := acl.RelationSubject{
		Type:     "editor-of",
		FromType: "person", FromID: "alice",
	}

	w.AssertAllow("jeroen", acl.OpCreate, rs)
	w.AssertDeny("alice", acl.OpCreate, rs)
}

// TestFeature_UC7_LocalRoleOnEntity pins the v1 write-side lift: a
// local role-relation on a specific entity allows the principal to
// write that entity, even without any global role grant.
//
// Scenario. Alice has no global role and no group memberships. The
// policy declares `assigned-to` as a role-relation that confers
// `editor`. Alice has an `assigned-to` edge to TKT-042 only.
//
// As Alice, I can update TKT-042 because my local edge confers
// editor on it. TKT-099 (no edge to me) stays denied. The audit
// record attributes the grant to the local edge.
func TestFeature_UC7_LocalRoleOnEntity(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  editor: { write: [ticket], read: [ticket] }
role_relations:
  assigned-to: { confers: editor }
`).
		Person("alice").
		Ticket("TKT-042").
		Ticket("TKT-099").
		Relation("alice", "assigned-to", "TKT-042").
		Build(t)

	w.AssertAllow("alice", acl.OpUpdate, acl.EntitySubject{Type: "ticket", ID: "TKT-042"})
	w.AssertPrimarySource("alice", "TKT-042", "editor",
		acl.Source{Kind: acl.SourceLocal, Relation: "assigned-to"})

	w.AssertDeny("alice", acl.OpUpdate, acl.EntitySubject{Type: "ticket", ID: "TKT-099"})
}

// TestFeature_UC8_ClosedWorldDeny pins the closed-world read shape:
// `RoleDef.Read: []` is deny-all for that type; combined with no
// other roles, no entities of the type are visible.
//
// Scenario. The policy declares `everyone: { read: [] }`. No other
// roles grant read on `ticket`. A principal with no other affiliations
// hits the `everyone` role only.
//
// As Alice, an authenticated user with no special grants, opening
// the ticket list — I expect to see nothing. The closed-world rule
// of `read: []` is intentional ("you have a role, but it lists no
// readable types").
func TestFeature_UC8_ClosedWorldDeny(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  everyone: { read: [] }
`).
		Person("alice").
		Ticket("TKT-001").
		Ticket("TKT-002").
		Build(t)

	w.AssertVisible("alice", "ticket") // empty
}

// TestFeature_UC9_LocalViaGroupRead pins that a local role-relation
// from a *group* to an entity confers read on that entity to every
// member of the group, transitively.
//
// Scenario. The `engineering` group has `viewer-of` on PRJ-foo.
// Alice is in `engineering`. The `viewer-of` role-relation confers
// the `viewer` role, which grants `read: [project]`.
//
// As Alice, I expect to see PRJ-foo because my group transitively
// holds the viewer role on it — even though I have no global role
// granting read on projects and no direct edge to PRJ-foo.
func TestFeature_UC9_LocalViaGroupRead(t *testing.T) {
	w := NewWorld().
		Policy(`
roles:
  viewer: { read: [project] }
role_relations:
  viewer-of: { confers: viewer }
`).
		Person("alice").
		Team("engineering").
		Project("PRJ-foo").
		Project("PRJ-bar").
		Relation("alice", "member-of", "engineering").
		Relation("engineering", "viewer-of", "PRJ-foo").
		Build(t)

	w.AssertVisible("alice", "project", "PRJ-foo")
	// PRJ-bar has no role-relation edge → not visible.
}

// UC10 and UC11 are property-level redaction scenarios. They belong
// in internal/affordances/features_test.go, not here — the verdicts
// they assert on (`visible: {ticket: [...]}`) are computed by the
// affordances package, not the ACL resolver. Putting them here would
// invert the dependency direction (acl importing affordances).
//
// See internal/affordances/features_test.go for the property-redaction
// feature tests.

// TestFeature_UC10_PropertyRedaction — moved to internal/affordances.
func TestFeature_UC10_PropertyRedaction(t *testing.T) {
	t.Skip("UC10 lives in internal/affordances/features_test.go (property redaction is an affordance-layer concern).")
}

// TestFeature_UC11_ReadAndVisibleCompose — moved to internal/affordances.
func TestFeature_UC11_ReadAndVisibleCompose(t *testing.T) {
	t.Skip("UC11 lives in internal/affordances/features_test.go (Read + visible composition is an affordance-layer concern).")
}

// TestFeature_UC12_MCPScopeIntersection pins the MCP transport
// intersection: the tool surface advertised to a principal is
// the intersection of their capability and the agent scope.
//
// Like UC10/UC11, this belongs in the package that owns the wiring
// (internal/mcp), not here. The acl.Policy needs `mcp_scopes` /
// `mcp_scope_assignments` schema additions and the mcp.Server needs
// to filter its tool-advertise response per request. Both pieces
// are separate from the core ACL resolver landed by TKT-SVXL.
//
// Tracked for a follow-up ticket once the MCP transport's
// per-request filtering shape is designed.
func TestFeature_UC12_MCPScopeIntersection(t *testing.T) {
	t.Skip("UC12 lives in internal/mcp (transport-layer intersection); follow-up ticket once the MCP filtering shape is designed.")
}
