package acl

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// defaultMembershipRelation is the relation type the resolver walks
// for group membership when [Policy.MembershipRelation] is unset (or
// blank/whitespace). Promoting this from a hard-coded literal to a
// policy field (TKT-Z8A62F) lets operators point the resolver at a
// domain relation they already model (e.g. `heeft_rol` in a Dutch
// ISMS) instead of maintaining a parallel `member-of` edge system.
// The default preserves existing deployments verbatim.
const defaultMembershipRelation = "member-of"

// EveryoneRole is the one built-in role name. A role declared under
// this name in `acl.yaml` is held implicitly by every principal,
// authenticated or not — it is how an operator expresses "applies to
// everyone" without enumerating users. It is appended to every
// principal's effective role set in both the Subject-driven write
// path ([Request]) and the affordances resolver, and is the single
// source of truth for the name so the two paths can't drift.
//
// (No `anonymous` / `authenticated` built-ins yet: rela-server has no
// authentication layer — see docs/server-security.md. Those would be added
// here when auth lands, so both write and affordance paths see them.)
const EveryoneRole = "everyone"

// PermHistoryRead is the global named permission that gates reading the
// version history of a DELETED entity (TKT-9INY0Y). A live entity's history
// is gated by the ordinary per-entity read verdict; but once an entity is
// deleted the conferring relations are gone, so there is nothing to evaluate
// a per-entity verdict against — deleted-history read is therefore an
// all-or-nothing global capability, granted via a role's `permissions:` list
// like the delegate-X permissions. Documented in docs/acl-security.md as an
// "audit-everything-deleted" super-permission.
const PermHistoryRead = "history:read"

// PermHistoryReadRedacted is the global named permission that reveals fields a
// historical snapshot would otherwise redact (TKT-73C6B2). Historical field
// redaction fails CLOSED: any `visible:` grant whose subject-world inputs cannot
// be affirmed for a historical/deleted entity (the live store no longer holds
// the entity's as-of-version edges, so a conditional `has_relation` /
// `count_relations` grant cannot be trusted) hides the field. A holder of this
// permission sees ALL frozen fields — OVERRIDE semantics, the field-grained
// sibling of [PermHistoryRead]'s all-or-nothing audit power. Granted via a
// role's `permissions:` list like the delegate-X permissions and PermHistoryRead;
// documented in docs/acl-security.md alongside it.
const PermHistoryReadRedacted = "history:read-redacted"

// BuiltinPermissions returns every global named permission rela itself ships
// and consumes. These are granted through a role's `permissions:` list exactly
// like the operator-defined delegate-X permissions, but — unlike those — they
// are consumed by rela's own read paths, never by a
// `role_relations.requires_permission` gate.
//
// It exists so consumers that reason about which permissions are *referenced*
// (notably the `rela acl audit` dead-permission check) can tell a live
// built-in from an operator's typo. A new global permission constant MUST be
// added here; the alternative — each consumer hardcoding its own list — is
// what let history:read be reported as dead config while it was in use.
func BuiltinPermissions() []string {
	return []string{PermHistoryRead, PermHistoryReadRedacted}
}

// Policy is the declarative ACL configuration parsed from `acl.yaml`
// at the project root.
//
//   - [Policy.UserEntityType] names the entity type that represents
//     a user (e.g. "person", "user"). Consulted together with
//     [Policy.PrincipalProperty] to resolve the raw principal to a user
//     entity (see below); also the type a membership edge is expected to
//     originate from.
//   - [Policy.PrincipalProperty] names a property on [Policy.UserEntityType]
//     whose value equals the authenticated principal's raw identifier
//     (e.g. `email` holding the value of `X-Forwarded-User`). When both
//     this and UserEntityType are set, the resolver looks the raw
//     principal up against that property once per request and, on exactly
//     one match, substitutes the matched entity's ID for `principal.User`
//     so membership/local-role walks operate from a real entity. The
//     referenced property MUST be declared `unique: true` in the metamodel
//     (enforced at load, see [Policy.ValidateAgainstMetamodel]).
//   - [Policy.MembershipRelation] names the relation type the resolver
//     walks from a principal to resolve group membership (TKT-Z8A62F).
//     Blank/whitespace means the default ("member-of") — read the
//     effective value via Policy.membershipRelation, never the raw
//     field, since a blank type would otherwise match *all* relations.
//   - [Policy.Roles] declares the named capability bundles. The
//     built-in role name [EveryoneRole] ("everyone") is appended to
//     every principal's effective role set in both the write path
//     and the affordances resolver.
//   - [Policy.Assignments] maps `principal.User` → role name.
//     Unknown role names (assigned but not declared in Roles) log a
//     warning at load and are dropped from the effective set.
//   - [Policy.RoleRelations] declares which relation types grant a
//     role to their source entity, and which permission the writer
//     must hold (delegate-X tamper resistance — see [Declarative]).
//   - [Policy.InheritRolesThrough] declares the containment relation
//     types through which a role granted at an ancestor flows down
//     to its descendants (e.g. folder → document).
//
// **Tolerant by design.** Unknown top-level keys emit one
// `slog.Warn` per key and are otherwise ignored. Operators iterate
// on `acl.yaml` frequently and a typo shouldn't brick the server —
// the metamodel loader follows the same convention. Hard errors
// reserved for unparseable YAML, undecodable values within a known
// key, and security-critical invariants — see [Policy.Validate].
type Policy struct {
	// Description is optional operator-facing prose describing what this
	// deployment's access model is for. It is never consulted by the
	// authorization path — it exists solely so the `rela docs` generator
	// can narrate the role model (rela-docs phase 1b, TKT-JO2SAD).
	Description string `yaml:"description,omitempty"`

	UserEntityType      string                     `yaml:"user_entity_type"`
	PrincipalProperty   string                     `yaml:"principal_property"`
	MembershipRelation  string                     `yaml:"membership_relation"`
	Roles               map[string]RoleDef         `yaml:"roles"`
	Assignments         map[string]string          `yaml:"assignments"`
	AssertedRoles       map[string]RoleList        `yaml:"asserted_role_assignments"`
	RoleRelations       map[string]RoleRelationDef `yaml:"role_relations"`
	InheritRolesThrough []string                   `yaml:"inherit_roles_through"`

	// ClientBaselines and ScopeGrants implement client attenuation
	// (TKT-IAC8TX): a ceiling that restricts a non-interactive client BELOW
	// the user it acts as. Unlike every other field here, these can only
	// REMOVE capability — see the package-level commentary in ceiling.go for
	// the invariant and why it compiles at load time.
	ClientBaselines map[string]ClientBaseline `yaml:"client_baselines"`
	ScopeGrants     map[string]ScopeGrant     `yaml:"scope_grants"`

	// RelationWriteGrants names, per relation type, the ACL permissions that
	// may satisfy a relation write's source-type verb grant (TKT-K2VN9D).
	// Keyed by RELATION type — unlike [RoleDef.Relations], which is keyed by
	// entity type under a role. See [RelationWriteGrant].
	RelationWriteGrants map[string]RelationWriteGrant `yaml:"relation_grants"`

	// UnmatchedPrincipal decides what happens when a verified principal's
	// identifier resolves to no [Policy.UserEntityType] entity (the
	// principal_property lookup found no match). It governs the data-entry
	// write path only; reads, and the CLI/MCP/scheduler entry points, are
	// unaffected.
	//
	//   - "" / "anonymous" (default): the request proceeds as today — the
	//     principal keeps its asserted roles but has no resolved user entity,
	//     so anything keyed on that entity (local roles, ancestry) does not
	//     apply.
	//   - "reject": a graph-is-authority posture — the unknown identity's
	//     WRITES are denied (403). Requires [Policy.PrincipalProperty] and
	//     [Policy.UserEntityType] (else "unmatched" is undefined, since the
	//     lookup is disabled and every principal looks unmatched); [Policy.Validate]
	//     rejects "reject" without both.
	//   - "provision": RESERVED for a future ticket (lazy stub creation). Accepted
	//     at load so the vocabulary is stable, but currently behaves as
	//     "anonymous" with a one-time warning.
	UnmatchedPrincipal string `yaml:"unmatched_principal"`
}

// UnmatchedPrincipal modes — the values [Policy.UnmatchedPrincipal] accepts.
const (
	// UnmatchedAnonymous is the default: an unmatched verified principal
	// proceeds with its asserted roles and no resolved entity.
	UnmatchedAnonymous = "anonymous"
	// UnmatchedReject denies an unmatched verified principal's writes.
	UnmatchedReject = "reject"
	// UnmatchedProvision is reserved for lazy stub provisioning (a future
	// ticket). Accepted at load; behaves as anonymous until implemented.
	UnmatchedProvision = "provision"
)

// effectiveUnmatchedPrincipal returns the mode with the empty default resolved
// to [UnmatchedAnonymous], so callers read one value.
func (p *Policy) effectiveUnmatchedPrincipal() string {
	if p == nil || p.UnmatchedPrincipal == "" {
		return UnmatchedAnonymous
	}
	return p.UnmatchedPrincipal
}

// EffectiveUnmatchedPrincipal is the exported form of
// Policy.effectiveUnmatchedPrincipal, for out-of-package callers (the
// data-entry provision seam) that must branch on the resolved mode.
func (p *Policy) EffectiveUnmatchedPrincipal() string {
	return p.effectiveUnmatchedPrincipal()
}

// RoleList is a list of role names that accepts either a bare scalar or a
// sequence in YAML, so the common single-role case stays terse:
//
//	asserted_role_assignments:
//	  admin: editor              # scalar
//	  auditor: [reader, auditor] # sequence
//
// Mirrors metamodel.StringOrSlice; duplicated rather than imported because
// internal/acl must not depend on internal/metamodel (see the MetamodelView
// comment below for why that boundary exists).
type RoleList []string

// UnmarshalYAML accepts a scalar or a sequence. A scalar becomes a one-element
// list; anything else is decoded as a sequence and surfaces its own error.
func (r *RoleList) UnmarshalYAML(unmarshal func(any) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*r = RoleList{single}
		return nil
	}
	var list []string
	if err := unmarshal(&list); err != nil {
		return err
	}
	*r = list
	return nil
}

// principalPropertyLookupEnabled reports whether the policy asks the
// resolver to map the raw principal (e.g. an email from
// `X-Forwarded-User`) to a user entity ID before role attribution. Both
// [Policy.UserEntityType] AND [Policy.PrincipalProperty] must be set;
// either blank leaves behavior byte-for-byte identical to a policy that
// never declared them (assignments/membership match on the raw string).
func (p *Policy) principalPropertyLookupEnabled() bool {
	return p != nil &&
		strings.TrimSpace(p.UserEntityType) != "" &&
		strings.TrimSpace(p.PrincipalProperty) != ""
}

// PrincipalPropertyLookupEnabled is the exported form of
// Policy.principalPropertyLookupEnabled, for out-of-package callers that must
// distinguish "the resolver attempted a lookup and found no entity" from "no
// lookup was configured" — e.g. the data-entry middleware deciding whether a
// no-match is a genuine unmatched principal.
func (p *Policy) PrincipalPropertyLookupEnabled() bool {
	return p.principalPropertyLookupEnabled()
}

// RejectEffective reports whether `unmatched_principal: reject` can actually
// deny anything, given whether a JWT gate is wired.
//
// Three conditions must ALL hold, and each is a separate way for the setting to
// be silently inert (TKT-M60ZF5 / issue #1274):
//
//   - the mode is "reject";
//   - a JWT gate is wired — reject keys on identity having come from the
//     fail-closed gate, and without one every principal looks unverified;
//   - the principal_property lookup is enabled — with it disabled the resolver
//     never attempts a match, so "unmatched" is not a thing that can be
//     observed rather than something that is always true.
//
// This exists so the enforcement site and the startup warning cannot disagree.
// They previously computed the answer separately, and did: the warning checked
// only gate wiring, so a reject policy with no user_entity_type /
// principal_property was equally inert and warned about nothing. A warning that
// is wrong about the thing it warns about is worse than none.
//
// Note LoadPolicy rejects "reject" without the lookup, but NewDeclarative does
// NOT call Validate — so any construction path that skips LoadPolicy can hold a
// policy in exactly that state. That is why this is checked rather than assumed.
func (p *Policy) RejectEffective(jwtWired bool) bool {
	return p != nil &&
		p.effectiveUnmatchedPrincipal() == UnmatchedReject &&
		jwtWired &&
		p.principalPropertyLookupEnabled()
}

// EffectiveMembershipRelation returns the relation type the resolver
// walks for group membership: a space-trimmed [Policy.MembershipRelation]
// when set, or [defaultMembershipRelation] ("member-of") when
// blank/whitespace.
//
// This is the single source of truth for the membership relation name.
// The resolver MUST read through it rather than the raw field, and any
// out-of-package consumer that needs to reason about the *effective*
// relation (e.g. the aclaudit linter) must use this too — so the audit
// can never disagree with what the resolver actually walks. [NewDeclarative]
// does not run [Policy.Validate], and the resolver passes the name straight
// into a [store.RelationQuery] where an empty Type means "all relation
// types" — so a blank field reaching the walk would silently follow *every*
// outgoing edge as if it were membership (an over-grant). Collapsing blank
// to the default here, on every read, closes that hole regardless of how
// the [Policy] was constructed.
//
// The value is trimmed so a stray-whitespace YAML value (e.g.
// `"heeft_rol "`) resolves to the relation the operator meant rather
// than silently matching zero edges.
func (p *Policy) EffectiveMembershipRelation() string {
	if trimmed := strings.TrimSpace(p.MembershipRelation); trimmed != "" {
		return trimmed
	}
	return defaultMembershipRelation
}

// MembershipSelfPromotionOpen reports whether this policy leaves the
// membership-relation self-promotion path open: at least one entry in
// [Policy.Assignments] targets a declared, PRIVILEGED role while writes
// to the effective membership relation carry no `requires_permission`
// gate. When true, any principal who can write an edge of that relation
// type can grant themselves the assigned role (RR-7O6Q) — see the
// escalation-risk note on [RoleRelationDef].
//
// This is the single implementation of the "is membership gated?"
// question. The aclaudit linter's A1-ungated-membership finding and the
// boot-time startup warning both call it, so the advisory and the
// enforcement view can never drift apart (TKT-T31NKT). It lives here,
// beside [Policy.EffectiveMembershipRelation], for the same reason that
// accessor does: a second hand-maintained copy of this predicate is a
// silent over- or under-report waiting to happen.
//
// Privilege-gated deliberately, and symmetric with the linter's A2
// check: a group assigned a READ-ONLY role is a visibility choice, not
// an escalation path (RR-LXI3NW / RR-UR0LJU / RR-EG5D3E). Widening this
// to "any assignment" would fire on read-only groups — the exact false
// positive the audit design fought.
//
// An assignment naming an UNDECLARED role confers nothing and is
// likewise not an escalation path; the linter reports that separately
// as A4-assignment-unknown-role.
func (p *Policy) MembershipSelfPromotionOpen() bool {
	if p.RoleRelations[p.EffectiveMembershipRelation()].RequiresPermission != "" {
		return false // gated — the delegate-X pattern is in place
	}
	for _, roleName := range p.Assignments {
		if role, ok := p.Roles[roleName]; ok && role.IsPrivileged() {
			return true
		}
	}
	return false
}

// RoleDef is the capability bundle for a single role. The per-verb
// mutation grants (Create / Update / Delete), Permissions, and the
// affordance grants are honored by the write path and the affordances
// resolver. Read drives the read-filtering path (see
// [Request.ReadQuery]).
//
// Per-verb mutation grants (TKT-4LQMWP): Create / Update / Delete each
// list the entity types the role may create / update / delete (`"*"`
// for all). They are SEPARATE because they have different read
// requirements (see [Policy.Validate]):
//
//   - Create implies NO read. A role can create a type it cannot read —
//     it then reads back only what it authored, via a role-conferring
//     relation like `created-by`. This is what lets a "submitter" create
//     tickets yet see only their own.
//   - Update and Delete require read coverage of the type (you must be
//     able to read a thing to modify or remove it). Rename routes
//     through the Update grant.
//
// Wildcard: a single entry `"*"` in any verb list grants that verb on
// every entity type. Mixing `"*"` with explicit types is allowed but
// redundant — the wildcard short-circuits the per-type check.
//
// Affordance grants (Fields / Visible / Options / Relations) drive
// the data-entry _fields / _relations wire shape via the
// affordances resolver. Each is keyed by entity type and is
// opt-in per type: a type that appears as a key is closed-world for
// that affordance dimension (only listed fields/options/relations
// are granted); a type absent from the map defaults permissive.
// A present-but-empty list (`fields: {ticket: []}`) is closed-world
// deny-all for that type, distinct from an absent or null value.
type RoleDef struct {
	// Description is optional operator-facing prose describing what this
	// role is for, in plain language. Like [Policy.Description] it is
	// documentation only — never read by the write/read/affordance paths —
	// and feeds the `rela docs` generator's role narration (rela-docs
	// phase 1b, TKT-JO2SAD).
	Description string `yaml:"description,omitempty"`

	Create      []string `yaml:"create"`
	Update      []string `yaml:"update"`
	Delete      []string `yaml:"delete"`
	Read        []string `yaml:"read"`
	Permissions []string `yaml:"permissions"`

	// Worlds holds the world names this role may read, SPLIT OUT of Read
	// at policy load by [Policy.normalizeWorldGrants]. It carries no yaml
	// tag: the wire spelling is `read: [world:published]` (design doc
	// §8.1), and the split happens so that every existing consumer of Read
	// — roleGrantsRead, grantForRole, filterTypes, aclaudit's verbLists —
	// keeps seeing a list of nothing but entity types.
	//
	// The split is load-bearing, not cosmetic. Left inline, a world token
	// is intersected with a client ceiling's TYPE allow/deny list by
	// [filterTypes]: silently DROPPED under an allowlist ceiling (the
	// token is not an entity type, so it fails the permits check) and
	// silently KEPT under a deny ceiling (it matches no denied type). Both
	// directions are wrong, and one of them is wrong in the operator's
	// favor. aclaudit's B1-undeclared-type would also report every world
	// grant as an undeclared entity type, at High.
	//
	// An EMPTY Worlds means the default world only — the default world is
	// the ABSENCE of a grant, never an entry. That is what keeps existing
	// acl.yaml files meaning exactly what they meant, and it is why a
	// ceiling cannot express "deny the default world" by intersection; see
	// [compiledCeiling.permitsWorld].
	Worlds []string `yaml:"-"`

	Fields    map[string][]FieldGrant    `yaml:"fields"`
	Visible   map[string][]FieldGrant    `yaml:"visible"`
	Options   map[string][]OptionGrant   `yaml:"options"`
	Relations map[string][]RelationGrant `yaml:"relations"`
}

// IsPrivileged reports whether the role confers escalation-relevant
// power: it grants any write verb (Create/Update/Delete, including the
// "*" wildcard) or holds any permission.
//
// Read grants are NOT privilege — a read-everything role is a
// visibility choice, not an escalation path (RR-LXI3NW, RR-UR0LJU).
// Exported so the aclaudit linter shares this definition rather than
// keeping its own copy; its A2/A3 checks reference the same notion.
func (r RoleDef) IsPrivileged() bool {
	return len(r.Create) > 0 || len(r.Update) > 0 || len(r.Delete) > 0 || len(r.Permissions) > 0
}

// grantsVerb reports whether the role may perform op on entity type
// `target`. Op selects the verb list: Create / Update / Delete; Rename
// routes through Update (it is a modification). Read is handled
// separately via roleGrantsRead. An unknown op grants nothing.
//
// STATE GRANTS GRANT NOTHING HERE, deliberately.
//
// This is the TYPE-granular question — "may this role write this type at
// all" — and its remaining callers ask exactly that: the provisioner
// create-check (Policy.principalGrantsCreate) and the who-can reporting
// in access.go, both of which report per type and have no face in hand.
//
// It is NO LONGER the write-authorization path. Since TKT-C1XUA8,
// decideFromAttrs calls [GrantsVerbOnState] with the subject's face, so
// the live path is face-granular and a state grant authorizes exactly the
// face it names.
//
// A state-shaped entry is skipped rather than matched on its type half,
// because matching would answer "yes" to a question about the DEFAULT face
// on the strength of a grant that names only `page@draft` — a grant
// widening by being made more specific. Skipping keeps this function's
// answer a lower bound: everything it grants is genuinely granted.
func grantsVerb(role RoleDef, op Op, target string) bool {
	var list []string
	switch op {
	case OpCreate:
		list = role.Create
	case OpUpdate, OpRename:
		list = role.Update
	case OpDelete:
		list = role.Delete
	default:
		return false
	}
	for _, t := range list {
		if isStateGrant(t) {
			continue // see the doc comment: fail closed until the write path is face-aware
		}
		if t == "*" || t == target {
			return true
		}
	}
	return false
}

// FieldGrant grants a per-field affordance (write under `fields:`,
// visibility under `visible:`) on the entity type it is keyed under.
// When set conditions the grant on a predicate evaluated against the
// entity; an empty When grants unconditionally. The same shape backs
// relation-meta-field grants (RelationGrant.Fields).
type FieldGrant struct {
	Field string `yaml:"field"`
	When  string `yaml:"when,omitempty"`
}

// UnmarshalYAML rejects unknown keys. See [rejectUnknownGrantKeys] for why
// leniency here fails OPEN rather than closed.
func (g *FieldGrant) UnmarshalYAML(node *yaml.Node) error {
	if err := rejectUnknownGrantKeys(node, "fields/visible", "field", "when"); err != nil {
		return err
	}
	type raw FieldGrant
	var out raw
	if err := node.Decode(&out); err != nil {
		return err
	}
	*g = FieldGrant(out)
	return nil
}

// OptionGrant grants a single enum option on a field. Used to filter
// the option set the SPA renders and to gate writes that set the
// field to that option.
type OptionGrant struct {
	Field  string `yaml:"field"`
	Option string `yaml:"option"`
	When   string `yaml:"when,omitempty"`
}

// UnmarshalYAML rejects unknown keys. See [rejectUnknownGrantKeys].
func (g *OptionGrant) UnmarshalYAML(node *yaml.Node) error {
	if err := rejectUnknownGrantKeys(node, "options", "field", "option", "when"); err != nil {
		return err
	}
	type raw OptionGrant
	var out raw
	if err := node.Decode(&out); err != nil {
		return err
	}
	*g = OptionGrant(out)
	return nil
}

// RelationGrant grants relation-level affordances for one relation
// type on the keyed entity type. Create and Remove are pointers so
// "unset" (use the grant's implied default of true — the grant
// existing is itself the opt-in) is distinguishable from an explicit
// false. Fields grants per-meta-field writability on links of this
// type. Visible grants per-meta-field READ visibility (redaction) —
// the read-side sibling of Fields, mirroring how RoleDef keeps entity
// Fields (write) and Visible (read) as separate dimensions (TKT-B1F5Q1).
// When conditions the whole grant on a predicate.
type RelationGrant struct {
	Relation string       `yaml:"relation"`
	Create   *bool        `yaml:"create,omitempty"`
	Remove   *bool        `yaml:"remove,omitempty"`
	Fields   []FieldGrant `yaml:"fields,omitempty"`
	Visible  []FieldGrant `yaml:"visible,omitempty"`
	When     string       `yaml:"when,omitempty"`
}

// UnmarshalYAML rejects unknown keys. See [rejectUnknownGrantKeys].
func (g *RelationGrant) UnmarshalYAML(node *yaml.Node) error {
	if err := rejectUnknownGrantKeys(node, "relations",
		"relation", "create", "remove", "fields", "visible", "when"); err != nil {
		return err
	}
	type raw RelationGrant
	var out raw
	if err := node.Decode(&out); err != nil {
		return err
	}
	*g = RelationGrant(out)
	return nil
}

// rejectUnknownGrantKeys fails the policy load when an affordance grant
// mapping carries a key outside want.
//
// yaml.v3 drops a key it cannot map, and for these three structs that fails
// OPEN. Every one of them carries a `when:` conditioning the grant on a
// predicate, and an absent When means "grant unconditionally"
// (PolicyResolver.compile returns a nil program) — so a misspelling of that
// key silently promotes "editors may see salary IF they are HR" to "editors
// may see salary". That is a read-side disclosure on the `visible:` block,
// not merely a wider write gate.
//
// The sibling keys are not the motivation but are covered for free: a dropped
// `field:`/`option:`/`relation:` is already caught downstream, because ""
// matches no declared property and internal/affordances rejects it by name.
// Rejecting here simply moves that report to the line that is wrong.
//
// Nil: node is never nil — yaml.v3 only calls UnmarshalYAML with a live node.
func rejectUnknownGrantKeys(node *yaml.Node, block string, want ...string) error {
	if node.Kind != yaml.MappingNode {
		return nil // let the normal decode produce the type error
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !slices.Contains(want, key) {
			// Report the source line, not an index: the caller decodes each
			// grant on its own, so any counter here would number the KEYS of
			// one mapping and read as if it named the offending grant.
			return fmt.Errorf(
				"%s grant at line %d: unknown key %q (want %s)",
				block, node.Content[i].Line, key, strings.Join(want, ", "))
		}
	}
	return nil
}

// RelationWriteGrant declares, for ONE relation type, the ACL permission that
// may satisfy each relation-write verb. It is the `relation_grants:` block in
// acl.yaml:
//
//	relation_grants:
//	  spawnt:
//	    create: create-spawnt
//	    update: edit-spawnt
//	    delete: remove-spawnt
//
// [RelationWriteGrant.Permission] is a shorthand covering all three verbs; it
// is mutually exclusive with the per-verb fields ([Policy.Validate] rejects
// both). Values are permission names from [RoleDef.Permissions] — this block
// CONSUMES permissions, it does not define them.
//
// # Not to be confused with RelationGrant
//
// Two similarly-named types live in this package and they are not
// interchangeable:
//
//   - [RelationGrant] (`relations:` UNDER a role, keyed by ENTITY type) is an
//     affordance grant. It is consumed only by internal/affordances, enforced
//     only on the data-entry HTTP path, and is default-PERMISSIVE.
//   - RelationWriteGrant (`relation_grants:` at policy top level, keyed by
//     RELATION type) participates in the write gate itself, so it applies to
//     every write path — CLI, Lua, MCP, sync, data-entry.
//
// # It is an ALTERNATIVE SATISFIER, never "sufficient"
//
// A relation write is authorized by a CONJUNCTION (see the unexported
// Request.authorizeRelationWrite, which implements it):
//
//	allow = (delegate-X satisfied OR not configured)   // gate A
//	      AND ceiling permits the verb on FromType      // gate B, ceiling
//	      AND (source-type verb grant OR this block)    // gate B, roles
//
// This block widens ONLY the third conjunct. It can never satisfy the
// delegate-X gate (which is what stops a `member-of` edge conferring a role to
// its own writer, RR-7O6Q) and it can never escape the client ceiling (which
// only ever narrows). A grant here is also inert when the source entity's type
// could not be resolved: an empty FromType is a fail-closed sentinel, not a
// wildcard.
type RelationWriteGrant struct {
	// Description is optional operator-facing prose, documentation only —
	// never read by the write path. Mirrors [RoleDef.Description]; feeds the
	// `rela docs` generator.
	Description string `yaml:"description,omitempty"`

	// Permission is the shorthand: one permission satisfying create, update
	// AND delete. Mutually exclusive with the per-verb fields below.
	Permission string `yaml:"permission,omitempty"`

	Create string `yaml:"create,omitempty"`
	Update string `yaml:"update,omitempty"`
	Delete string `yaml:"delete,omitempty"`
}

// UnmarshalYAML rejects unknown verb keys rather than silently dropping them,
// so a typo cannot masquerade as a grant. `read:` gets its own message: an
// operator writing it has a coherent intent, so "unknown key" would read as an
// oversight and invite a workaround. It is deliberately unsupported — see the
// error text.
func (g *RelationWriteGrant) UnmarshalYAML(unmarshal func(any) error) error {
	var raw map[string]string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	for _, k := range slices.Sorted(maps.Keys(raw)) {
		switch k {
		case "description", "permission", "create", "update", "delete":
		case "read":
			return errors.New(
				"relation_grants: `read:` is not supported. Relation visibility is " +
					"DERIVED — a relation is visible exactly when BOTH its endpoints " +
					"are — so an independent read grant would reveal that an entity " +
					"you cannot read exists. To hide edges, hide an endpoint")
		default:
			return fmt.Errorf(
				"relation_grants: unknown key %q (want permission, create, update, delete)", k)
		}
	}
	g.Description = raw["description"]
	g.Permission = raw["permission"]
	g.Create = raw["create"]
	g.Update = raw["update"]
	g.Delete = raw["delete"]
	return nil
}

// permissionFor returns the permission name that may satisfy op for this
// relation type, and false when the grant names none. The shorthand answers
// for every write verb; otherwise the matching per-verb field is used.
//
// OpRename deliberately returns false rather than routing through Update the
// way [grantsVerb] does: no caller pairs OpRename with a [RelationSubject]
// (relations are re-keyed in bulk below the write gate by the store), so
// silently accepting it here would invent a semantic no path exercises.
func (g RelationWriteGrant) permissionFor(op Op) (string, bool) {
	if g.Permission != "" {
		switch op {
		case OpCreate, OpUpdate, OpDelete:
			return g.Permission, true
		default:
			return "", false
		}
	}
	var perm string
	switch op {
	case OpCreate:
		perm = g.Create
	case OpUpdate:
		perm = g.Update
	case OpDelete:
		perm = g.Delete
	default:
		return "", false
	}
	return perm, perm != ""
}

// RelationWriteGrantTypes lists the relation types covered by
// `relation_grants:`, sorted. Empty when the block is absent.
//
// Exists so the WIRING site can log a positive confirmation that the block is
// active: an older binary does not know the key and warn-and-ignores it, so
// silence at load is otherwise indistinguishable from "loaded and enforcing" —
// and an operator who believes the block is live might relax an entity-type
// grant on the strength of it. Logging inside LoadPolicy would fire for every
// read-only reporting command too, which trains operators to ignore it.
func (p *Policy) RelationWriteGrantTypes() []string {
	if len(p.RelationWriteGrants) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(p.RelationWriteGrants))
}

// relationPermissionFor reports the permission that may satisfy op on relation
// type relType, if the policy declares one.
func (p *Policy) relationPermissionFor(relType string, op Op) (string, bool) {
	g, ok := p.RelationWriteGrants[relType]
	if !ok {
		return "", false
	}
	return g.permissionFor(op)
}

// HasAffordanceGrants reports whether any role in the policy declares
// at least one of the affordance grant blocks (fields / visible /
// options / relations). The resolver-selection logic in the entry
// points uses this to decide between the policy-backed resolver and
// the permissive default: a policy that only carries write/read
// grants has no affordances to compute, so it falls through to the
// Nop resolver and the wire stays byte-identical to no-policy.
func (p *Policy) HasAffordanceGrants() bool {
	for _, role := range p.Roles {
		if roleHasAffordanceGrants(role) {
			return true
		}
	}
	return false
}

func roleHasAffordanceGrants(role RoleDef) bool {
	return len(role.Fields) > 0 || len(role.Visible) > 0 ||
		len(role.Options) > 0 || len(role.Relations) > 0
}

// RoleRelationDef declares that a graph relation type confers a role
// on its source entity. Writes to relations of this type are gated by
// [RoleRelationDef.RequiresPermission] — the writer (principal) must
// hold that permission via one of their effective roles. This is the
// Plone delegate-X tamper-resistance pattern: granting role X requires
// permission delegate-X, so the principal who can hand out access is
// distinct from the principal who has access.
//
// Empty [RoleRelationDef.RequiresPermission] disables the delegate-X
// gate — the relation type is recognized as role-conferring (for
// future group expansion) but no permission check fires on writes.
//
// **Escalation risk for the configured membership relation** (RR-7O6Q).
// v1 confers group roles by walking the membership relation —
// [Policy.MembershipRelation], default `member-of`. By default that
// relation is a regular relation type with no `requires_permission`
// gate, so anyone with write access on the relation's source type can
// create their own membership edge into any group named in
// [Policy.Assignments]. If a group is assigned a privileged role
// (e.g. `assignments: { admins: admin }`), an attacker with write
// access on `person` can self-promote by writing
// `alice --member-of--> admins`.
//
// Operators using groups for role attribution MUST gate writes to the
// membership relation. Recommended shape (substitute the configured
// relation name for `member-of` when [Policy.MembershipRelation] is
// set):
//
//	role_relations:
//	  member-of:
//	    requires_permission: delegate-membership
//	roles:
//	  admin:
//	    permissions: [delegate-membership]
//
// This restricts membership-edge creation to principals holding
// `delegate-membership` — typically only admins. See
// `docs/server-security.md` for the full hardening pattern. The UC1 example
// policy in features_test.go is intentionally minimal and would be
// wide-open if copy-pasted into a deployment.
// # The gate is all-verbs, deliberately
//
// [RoleRelationDef.RequiresPermission] is a single permission covering
// every write verb on the type — there is no per-verb form, and one is
// not an oversight. The escalation this gate exists to stop is a
// CREATE (`alice --member-of--> admins`), so a policy that gated only
// some verbs would read as hardened while stopping nothing; and
// revoking a conferred edge is an availability attack, so `delete` is
// not safely un-gated either. Per-verb create/update/delete
// permissions exist for relation types that do NOT confer roles — see
// [RelationWriteGrant] (`relation_grants:`), which is refused on any
// type carrying this gate precisely so the two cannot disagree.
type RoleRelationDef struct {
	Confers            string `yaml:"confers"`
	RequiresPermission string `yaml:"requires_permission"`
}

// UnmarshalYAML rejects unknown keys rather than silently dropping them.
// This struct's fields are a closed set, and yaml.v3 ignores anything it
// cannot map — so `create: add-member` under a `role_relations:` entry
// would load clean and leave the relation UNGATED, which fails open on
// the one gate whose whole job is tamper-resistance.
//
// The write-verb keys get their own message: an operator reaching for
// them has a coherent intent (per-verb relation permissions), it is just
// spelled `relation_grants:` and only applies to relation types that do
// not confer a role. "unknown key" would read as a typo and hide that.
func (d *RoleRelationDef) UnmarshalYAML(unmarshal func(any) error) error {
	var raw map[string]string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	for _, k := range slices.Sorted(maps.Keys(raw)) {
		switch k {
		case "confers", "requires_permission":
		case "create", "update", "delete", "read":
			return fmt.Errorf(
				"role_relations: %q is not supported — `requires_permission:` gates ALL "+
					"write verbs on a role-conferring relation, by design. Per-verb "+
					"permissions are `relation_grants:`, which is refused on a type that "+
					"confers a role", k)
		default:
			return fmt.Errorf(
				"role_relations: unknown key %q (want confers, requires_permission)", k)
		}
	}
	d.Confers = raw["confers"]
	d.RequiresPermission = raw["requires_permission"]
	return nil
}

// knownPolicyKeys is the allowlist used for unknown-key warnings.
// Keep in sync with [Policy]'s yaml tags.
var knownPolicyKeys = map[string]bool{
	"description":               true,
	"user_entity_type":          true,
	"principal_property":        true,
	"membership_relation":       true,
	"roles":                     true,
	"assignments":               true,
	"asserted_role_assignments": true,
	"role_relations":            true,
	"inherit_roles_through":     true,
	"unmatched_principal":       true,
	"client_baselines":          true,
	"scope_grants":              true,
	"relation_grants":           true,
}

// LoadPolicy reads and parses `acl.yaml` at the given path.
//
// Errors:
//   - The caller distinguishes "no policy file" from "broken policy
//     file" via [os.ErrNotExist]. Use `errors.Is(err, os.ErrNotExist)`
//     to fall back to [NopACL] when no policy is present.
//   - Any other I/O error, YAML parse error, or [Policy.Validate]
//     failure returns wrapped.
//
// Unknown top-level keys emit one `slog.Warn` per key and are
// otherwise ignored. The returned [Policy] is non-nil on success
// even if every field is zero (matches "empty file is valid").
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err // preserves os.ErrNotExist for errors.Is
	}

	// First pass: discover unknown top-level keys. Decoding into
	// map[string]any rather than KnownFields(true) on Policy lets
	// us warn-and-continue rather than fail.
	if len(data) > 0 {
		var raw map[string]any
		if uErr := yaml.Unmarshal(data, &raw); uErr == nil {
			for k := range raw {
				if !knownPolicyKeys[k] {
					slog.Warn("acl: unknown key in acl.yaml; ignored",
						"path", path, "key", k)
				}
			}
		}
		// Parse failure here is not fatal — the typed decode below
		// will surface the same error with better context.
	}

	var policy Policy
	if uErr := yaml.Unmarshal(data, &policy); uErr != nil {
		return nil, fmt.Errorf("acl: parse %s: %w", path, uErr)
	}
	if vErr := policy.Validate(); vErr != nil {
		return nil, fmt.Errorf("acl: validate %s: %w", path, vErr)
	}
	return &policy, nil
}

// LoadPolicyBytes parses an acl.yaml from in-memory bytes. Used by
// tests (and any future caller that builds policy from non-filesystem
// sources); production wiring uses [LoadPolicy] with a path. Unknown
// top-level keys are NOT warned here — the bytes form is for callers
// who already control the schema.
func LoadPolicyBytes(data []byte) (*Policy, error) {
	if len(data) == 0 {
		return &Policy{}, nil
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("acl: parse policy bytes: %w", err)
	}
	if vErr := p.Validate(); vErr != nil {
		return nil, fmt.Errorf("acl: validate policy bytes: %w", vErr)
	}
	return &p, nil
}

// normalizeAssertedRoles trims surrounding whitespace from every
// asserted_role_assignments key so the stored form matches what the resolver
// looks up.
//
// Without this, a padded key like `" admin": editor` loads clean and is
// PERMANENTLY INERT: the resolver trims the incoming claim before an exact map
// lookup, so no claim value can ever match the untrimmed key. That fails safe
// (the grant silently doesn't happen) but it is precisely the
// looks-configured-but-does-nothing trap [Policy.Validate]'s blank-key check
// exists to prevent. [Policy.EffectiveMembershipRelation] trims its value for
// the identical reason.
//
// A key that is entirely blank trims to "" and is then caught by the blank-key
// check in [Policy.Validate], so it still fails loudly rather than normalizing
// into something matchable. Two keys that differ only by padding collapse to
// one entry; their role lists are unioned, so no grant is lost.
//
// Two details the merge depends on, both load-bearing:
//
//   - Lists are CLONED on store, never aliased. Go map iteration order is
//     random, so which key is stored first and which merges into it varies per
//     run; appending into a caller-owned slice with spare capacity would write
//     past its length into the caller's backing array. Unreachable via
//     [LoadPolicy] (the YAML unmarshaler hands over fresh slices), but this
//     method exists precisely because policies also arrive hand-built — and
//     [Policy.Validate] documents that as supported.
//   - The merged list is SORTED. Otherwise the same policy text yields a
//     different in-memory order on every load, which propagates into the
//     resolver's attribution append order. Downstream reporting sorts anyway,
//     so nothing observable breaks today; this keeps a future consumer from
//     inheriting an ordering hazard that nothing in the type documents.
func (p *Policy) normalizeAssertedRoles() {
	if len(p.AssertedRoles) == 0 {
		return
	}
	out := make(map[string]RoleList, len(p.AssertedRoles))
	merged := map[string]bool{}
	for claim, roles := range p.AssertedRoles {
		key := strings.TrimSpace(claim)
		existing, dup := out[key]
		if !dup {
			out[key] = slices.Clone(roles)
			continue
		}
		for _, r := range roles {
			if !slices.Contains(existing, r) {
				existing = append(existing, r)
			}
		}
		out[key] = existing
		merged[key] = true
	}
	// Sort only the merged lists: an unmerged list keeps the operator's
	// authored order, which is what they see in `rela acl map` output.
	for key := range merged {
		slices.Sort(out[key])
	}
	p.AssertedRoles = out
}

// normalizeRelationWriteGrants trims the relation-type keys and every
// permission name in `relation_grants:`. A padded key ("spawnt ") would load
// clean and then never match a real relation type, and a padded permission
// would never match a granted one — both are silently-inert config, the
// failure mode an operator is least likely to notice. Trimming first also
// means [Policy.Validate]'s blank checks see "" rather than whitespace.
func (p *Policy) normalizeRelationWriteGrants() {
	if len(p.RelationWriteGrants) == 0 {
		return
	}
	out := make(map[string]RelationWriteGrant, len(p.RelationWriteGrants))
	for relType, g := range p.RelationWriteGrants {
		g.Permission = strings.TrimSpace(g.Permission)
		g.Create = strings.TrimSpace(g.Create)
		g.Update = strings.TrimSpace(g.Update)
		g.Delete = strings.TrimSpace(g.Delete)
		out[strings.TrimSpace(relType)] = g
	}
	p.RelationWriteGrants = out
}

// validateRelationWriteGrants enforces the structural invariants of the
// `relation_grants:` block. Everything here is checkable without a metamodel;
// whether the relation type is DECLARED is checked in
// [Policy.ValidateAgainstMetamodel], where the schema is available.
func (p *Policy) validateRelationWriteGrants() error {
	for _, relType := range slices.Sorted(maps.Keys(p.RelationWriteGrants)) {
		g := p.RelationWriteGrants[relType]
		if relType == "" {
			return errors.New(
				"relation_grants: relation type key must not be empty or whitespace")
		}
		if err := g.validate(relType); err != nil {
			return err
		}
		if err := p.validateNotRoleConferring(relType); err != nil {
			return err
		}
	}
	return nil
}

// validateNotRoleConferring rejects a relation_grants entry on a relation type
// whose creation CONFERS a role, in any of the three ways the policy allows.
//
// This is the load-bearing half of the delegate-X hardening (RR-7O6Q). A
// relation permission satisfies the source-type verb grant, which is precisely
// the conjunct that stands between a principal and a self-granted role edge —
// so allowing it on a role-conferring type hands over the escalation primitive.
//
// Checking only role_relations would miss two mechanisms, and the miss is not
// theoretical: the membership relation NEED NOT appear in role_relations at all
// (it is a plain relation type by default), which is exactly the RR-7O6Q attack
// shape.
func (p *Policy) validateNotRoleConferring(relType string) error {
	const hint = "a relation permission would satisfy the source-type grant, " +
		"which is the only thing standing between a principal and a " +
		"self-granted role — grant the entity-type verb instead"

	if relType == p.EffectiveMembershipRelation() {
		return fmt.Errorf(
			"relation_grants.%s: %q is the membership relation (group roles are "+
				"conferred by walking it), so it must not be grantable here; %s",
			relType, relType, hint)
	}
	if slices.Contains(p.InheritRolesThrough, relType) {
		return fmt.Errorf(
			"relation_grants.%s: %q is listed in inherit_roles_through (local "+
				"roles are conferred across it), so it must not be grantable here; %s",
			relType, relType, hint)
	}
	if rr, ok := p.RoleRelations[relType]; ok && rr.RequiresPermission != "" {
		return fmt.Errorf(
			"relation_grants.%s: %q is a role-relation gated by requires_permission "+
				"%q; a relation_grants entry cannot satisfy that gate and would only "+
				"confuse which rule applies — remove one of the two",
			relType, relType, rr.RequiresPermission)
	}
	return nil
}

// validate checks one relation type's grant. relType names it in errors.
func (g RelationWriteGrant) validate(relType string) error {
	perVerb := g.Create != "" || g.Update != "" || g.Delete != ""
	if g.Permission != "" && perVerb {
		return fmt.Errorf(
			"relation_grants.%s: `permission` and the per-verb keys are mutually "+
				"exclusive. The shorthand `permission: %s` means "+
				"`create: %s, update: %s, delete: %s` — write those three "+
				"explicitly and change the one you need",
			relType, g.Permission, g.Permission, g.Permission, g.Permission)
	}
	if g.Permission == "" && !perVerb {
		return fmt.Errorf(
			"relation_grants.%s: grants nothing; name a permission under "+
				"`permission:` (all verbs) or under create/update/delete",
			relType)
	}
	return nil
}

// Validate enforces security-critical invariants on the parsed
// policy. Run automatically by [LoadPolicy] / [LoadPolicyBytes].
// Operators can also call it before persisting a generated policy.
//
// It also normalizes asserted_role_assignments keys in place (see
// Policy.normalizeAssertedRoles) — the one mutation it performs, placed here
// so it cannot be bypassed by a policy that reaches the resolver through a
// path other than LoadPolicy.
//
// Current checks (RR-NIGK, RR-W2J6):
//
//   - InheritRolesThrough entries must be non-empty and non-whitespace.
//     A blank entry would expand ancestor sets through every relation
//     type (StoreGraph treats RelationQuery.Type=="" as "all relations"),
//     silently turning a typo into a containment widening.
//
//   - RoleRelations keys must be non-empty and non-whitespace, for the
//     same reason — an empty key would gate "all relation writes" on
//     a delegate permission, breaking writes the operator didn't mean
//     to gate.
//
//   - A role's UPDATE and DELETE grants must be covered by its read
//     grants (update ⊆ read, delete ⊆ read, wildcard-aware). You must
//     be able to read a type to modify or remove it. CREATE is EXEMPT
//     (TKT-4LQMWP): a role may create a type it cannot read — it reads
//     back only what it authored via a role-conferring relation (e.g.
//     `created-by`), which is what lets a "submitter" create tickets yet
//     see only their own. (Was the broader write ⊆ read invariant,
//     RR-W2J6, before create was split out.)
//
//     Scope: the invariant covers [RoleDef.Update] and [RoleDef.Delete]
//     — the fields that authorize modification. Both entity and relation
//     authz resolve through decideFromAttrs against the per-verb grant
//     (grantsVerb). The affordance grant maps (Fields / Options /
//     Relations) are deliberately NOT checked: they restrict
//     field/option/relation surfaces *within* a write the verb grant
//     already authorized and never confer writability by themselves, so
//     a fields-only role without read grants is inert, not incoherent.
//
// Validation is intentionally narrow: misspelled role names, unknown
// entity types in grants, etc. remain warnings (or analyze-tool
// findings) per the "tolerant by design" stance. Security-relevant
// invariants like the ones above are the exception.
//
// Validate is a pure structural gate: it does NOT flag escalation
// foot-guns, dead/inert config, or policy-vs-metamodel drift. Those
// advisory checks (including the un-gated / inert membership relation
// warnings that briefly lived here in TKT-Z8A62F) belong to the
// on-demand `rela acl audit` linter — see internal/aclaudit and
// TKT-TS0J5K — which can rank findings by severity and cross-check the
// metamodel, neither of which fits a boot gate.
//
// ONE EXCEPTION, added by TKT-DN37J2: a policy that grants read on a
// non-default world while leaving a self-promotion path open is REFUSED
// here ([Policy.WorldGrantRefusalReason]). It is an escalation foot-gun,
// so it looks like it belongs to the linter — but the linter is advisory
// by construction and this is a shipping gate the content-states design
// requires, docs/acl-security.md promises operators in writing, and
// TKT-T31NKT made a hard acceptance criterion. It qualifies for the boot
// gate on the terms the paragraph above sets out: it needs no metamodel
// and no store, only the policy. The un-worlded case still warns and
// boots, exactly as before.
// validateUnmatchedPrincipal checks the unmatched_principal enum and its
// dependency on the principal_property lookup. Extracted from [Policy.Validate]
// to keep that function's cognitive complexity in bounds.
func (p *Policy) validateUnmatchedPrincipal() error {
	switch p.UnmatchedPrincipal {
	case "", UnmatchedAnonymous, UnmatchedReject, UnmatchedProvision:
		// ok
	default:
		return fmt.Errorf(
			"unmatched_principal: %q is not a valid mode (want %q, %q, or %q)",
			p.UnmatchedPrincipal, UnmatchedAnonymous, UnmatchedReject, UnmatchedProvision)
	}
	// reject and provision key off the principal_property lookup: "unmatched"
	// only has meaning when the resolver maps an identifier to a user entity.
	// Without the lookup enabled, ResolvePrincipal returns no-match for EVERY
	// request, so every principal would look unmatched — a foot-gun that would
	// reject (or provision for) everyone. Fail loud at load rather than mis-gate
	// at runtime.
	if p.UnmatchedPrincipal == UnmatchedReject || p.UnmatchedPrincipal == UnmatchedProvision {
		if !p.principalPropertyLookupEnabled() {
			return fmt.Errorf(
				"unmatched_principal: %q requires both user_entity_type and "+
					"principal_property to be set (else every principal is unmatched)",
				p.UnmatchedPrincipal)
		}
	}
	return nil
}

func (p *Policy) Validate() error {
	p.normalizeAssertedRoles()
	p.normalizeClientAttenuation()
	// Runs before every check below, because those checks read the grant
	// lists and must see them in their split form: Read holding only
	// entity types, Worlds holding the world names.
	if err := p.normalizeWorldGrants(); err != nil {
		return err
	}
	if err := p.validateStateGrants(); err != nil {
		return err
	}
	p.normalizeRelationWriteGrants()

	if err := p.validateUnmatchedPrincipal(); err != nil {
		return err
	}
	if err := p.validateClientAttenuation(); err != nil {
		return err
	}
	if err := p.validateRelationWriteGrants(); err != nil {
		return err
	}

	for i, t := range p.InheritRolesThrough {
		if isBlank(t) {
			return fmt.Errorf("inherit_roles_through[%d]: relation type must not be empty or whitespace", i)
		}
	}
	for k := range p.RoleRelations {
		if isBlank(k) {
			return errors.New("role_relations: relation type key must not be empty or whitespace")
		}
	}
	for claim, roles := range p.AssertedRoles {
		// A blank claim key can never match a real claim value, so the mapping
		// is silently inert — the failure mode an operator is least likely to
		// notice.
		//
		// normalizeAssertedRoles has already run, so a key that TrimSpace
		// considers blank arrives here as "". isBlank uses a narrower
		// definition (ASCII space/tab/CR/LF) than TrimSpace (all Unicode
		// space), and the difference is safe in this direction only: anything
		// TrimSpace strips is already gone, so isBlank sees "" and rejects it.
		// Widening isBlank would not break this; narrowing TrimSpace would.
		if isBlank(claim) {
			return errors.New("asserted_role_assignments: claim key must not be empty or whitespace")
		}
		// EveryoneRole already enters every principal's set as
		// Source{Kind: SourceGlobal}. Granting it again from a claim would add a
		// SECOND attribution for the same role under a different Source,
		// double-reporting it in `rela acl map` and in denial diagnostics — and
		// buying nothing, since everyone already has it.
		if slices.Contains(roles, EveryoneRole) {
			return fmt.Errorf(
				"asserted_role_assignments.%s: cannot grant the %q role — "+
					"it already applies to every principal", claim, EveryoneRole)
		}
	}
	if err := p.validateWriteReadCoverage(); err != nil {
		return err
	}

	// The membership-gate refusal is the LAST check, so a policy that is
	// structurally broken fails on that first — a refusal naming an
	// escalation path is confusing advice for a policy that does not parse.
	//
	// This is a deliberate, narrow exception to the "Validate is a pure
	// structural gate" contract stated above. It earns the exception by
	// being a security invariant expressible in policy alone (no metamodel,
	// no store), and by firing only for policies that grant read on a
	// non-default world — see [Policy.WorldGrantRefusalReason].
	if reason := p.WorldGrantRefusalReason(); reason != "" {
		return fmt.Errorf("refusing to load: %s", reason)
	}
	return nil
}

// validateWriteReadCoverage enforces the write⊆read invariant (TKT-4LQMWP):
// Update and Delete require read coverage of the type, Create is exempt.
// Extracted from [Policy.Validate] to keep that function's cognitive
// complexity in bounds, the same way validateUnmatchedPrincipal was.
func (p *Policy) validateWriteReadCoverage() error {
	for name, role := range p.Roles {
		// Update and Delete require read coverage: you must be able to read a
		// type to modify or remove it (TKT-4LQMWP, was the write⊆read invariant
		// RR-W2J6). Create is EXEMPT — a role may create a type it cannot read,
		// reading back only what it authored via a role-conferring relation.
		for _, verb := range []struct {
			name  string
			types []string
		}{{"update", role.Update}, {"delete", role.Delete}} {
			for _, t := range verb.types {
				// Compare on the TYPE half. A state-shaped grant
				// (`update: ["policy@draft"]`) still requires read coverage
				// of `policy` — the face narrows WHICH FACE is writable,
				// not which type — so checking the joined string would
				// reject every state grant with a hint telling the operator
				// to add "policy@draft" to their read list, which is not a
				// thing a read list can hold.
				target := grantTypeOf(t)
				if !roleGrantsRead(role, target) {
					hint := fmt.Sprintf("add %q (or \"*\")", target)
					if target == "*" {
						hint = `add "*"`
					}
					return fmt.Errorf(
						"roles.%s: grants %s on %q without a covering read grant; "+
							"%s to the role's read list — a principal must be able to "+
							"read every type it can %s (create is exempt)",
						name, verb.name, t, hint, verb.name)
				}
			}
		}
	}

	return nil
}

// PropertyInfo describes a property to [Policy.ValidateAgainstMetamodel].
// Exists is false when the type has no such property (in which case the
// other fields are meaningless). A named struct instead of a tuple of
// bools so the fields can't be transposed at the call site.
type PropertyInfo struct {
	Exists bool // the type declares this property
	Unique bool // declared `unique: true`
	List   bool // declared `list: true` (multi-valued)
}

// MetamodelView is the narrow, consumer-side contract
// [Policy.ValidateAgainstMetamodel] needs from the metamodel. Declared
// here (rather than importing internal/metamodel, which the acl package
// deliberately does not depend on — see .go-arch-lint.yml) so the wiring
// site supplies the schema without coupling the domain package to it.
// *metamodel.Metamodel is adapted to this at the wiring site (appbuild).
type MetamodelView interface {
	// HasEntityType reports whether entityType is declared.
	HasEntityType(entityType string) bool
	// PropertyInfo describes property on entityType (existence, unique,
	// list). A missing type or property yields PropertyInfo{Exists:false}.
	PropertyInfo(entityType, property string) PropertyInfo
	// HasRelationType reports whether relationType is declared.
	HasRelationType(relationType string) bool
}

// ValidateAgainstMetamodel enforces the schema-dependent invariants that
// [Policy.Validate] cannot check in isolation. It is run at the wiring
// site (where the metamodel is available) after LoadPolicy. Errors are
// hard — a policy that references a non-existent type/property, or a
// principal_property that is not a unique natural key, is an operator
// mistake that must fail loud at load rather than silently mis-resolve
// identities at runtime.
//
// Checks (all gated on the relevant keys being set):
//
//   - principal_property set but user_entity_type empty → error.
//   - user_entity_type not a declared entity type → error.
//   - principal_property not a declared property on user_entity_type →
//     error.
//   - principal_property not declared `unique: true` → error. A
//     non-unique identity key admits duplicates, which makes resolution
//     ambiguous; requiring uniqueness is what makes the property a
//     primary key (see internal/entitymanager checkUniqueProperties).
//   - principal_property declared `list: true` → error. A multi-valued
//     property can't be an identity key: the write-time unique check
//     skips list properties and the lookup reads a scalar, so a list
//     principal_property would silently resolve nobody. Fail loud at boot
//     rather than ship a wired-but-inert lookup.
//
// user_entity_type set WITHOUT principal_property is NOT an error — it is
// meaningful on its own (the type a membership edge originates from).
func (p *Policy) ValidateAgainstMetamodel(meta MetamodelView) error {
	if meta == nil {
		return errors.New("acl: ValidateAgainstMetamodel: metamodel view must be non-nil")
	}
	prop := strings.TrimSpace(p.PrincipalProperty)
	userType := strings.TrimSpace(p.UserEntityType)

	if prop != "" && userType == "" {
		return errors.New("acl: principal_property requires user_entity_type to be set")
	}
	if userType != "" && !meta.HasEntityType(userType) {
		return fmt.Errorf("acl: user_entity_type %q is not a declared entity type", userType)
	}
	if prop != "" {
		info := meta.PropertyInfo(userType, prop)
		if !info.Exists {
			return fmt.Errorf("acl: principal_property %q is not a declared property on %q",
				prop, userType)
		}
		if info.List {
			return fmt.Errorf("acl: principal_property %q on %q is declared `list: true`; "+
				"a multi-valued property cannot be an identity key", prop, userType)
		}
		if !info.Unique {
			return fmt.Errorf("acl: principal_property %q on %q must be declared `unique: true` "+
				"(it is used as an identity key; a duplicate value makes resolution ambiguous)",
				prop, userType)
		}
	}
	if err := p.validateRelationTypesDeclared(meta); err != nil {
		return err
	}
	return p.validateProvisionerGrant(userType)
}

// validateRelationTypesDeclared rejects a `relation_grants:` key naming a
// relation type the schema does not declare. A boot error rather than an audit
// finding, matching how an undeclared user_entity_type is treated above: the
// entry can never match a real write, so it is a grant the operator believes
// they made and did not — and unlike a dead permission, nothing at runtime will
// ever surface it.
func (p *Policy) validateRelationTypesDeclared(meta MetamodelView) error {
	for _, relType := range slices.Sorted(maps.Keys(p.RelationWriteGrants)) {
		if !meta.HasRelationType(relType) {
			return fmt.Errorf(
				"acl: relation_grants.%s is not a declared relation type", relType)
		}
	}
	return nil
}

// validateProvisionerGrant enforces that under `unmatched_principal: provision`
// the provisioner system identity is actually granted create on the user entity
// type (TKT-ANUJDS AC4). Without it the lazy stub create is ACL-denied and every
// unmatched principal's first write fails at the provision step — a
// misconfiguration this catches at LOAD rather than at the first unmatched
// request. The `acl-provisioner-grant` migration injects the grant; this is the
// guard that the operator ran it (or wrote an equivalent grant themselves).
func (p *Policy) validateProvisionerGrant(userType string) error {
	if p.effectiveUnmatchedPrincipal() != UnmatchedProvision {
		return nil
	}
	// userType is guaranteed non-empty here: validateUnmatchedPrincipal (run by
	// Validate) already rejects provision without the principal_property lookup,
	// which requires user_entity_type.
	if !p.principalGrantsCreate(principal.UserProvisioner, userType) {
		return fmt.Errorf(
			"acl: unmatched_principal: provision requires %q to be granted `create: [%s]` "+
				"(run `rela db migrate` / the acl-provisioner-grant migration, or grant it manually); "+
				"without it every unmatched principal's first write fails at provision",
			principal.UserProvisioner, userType)
	}
	return nil
}

// principalGrantsCreate reports whether principalUser is assigned — via
// `assignments` or `asserted_role_assignments` — a defined role that grants
// create on target. Type-granular, which is the right question here: the
// provisioner creates DEFAULT-face user entities, and a face-specific grant
// would not be one it could act on. (The write path itself became
// face-granular in TKT-C1XUA8; this deliberately did not.)
func (p *Policy) principalGrantsCreate(principalUser, target string) bool {
	roleGrants := func(roleName string) bool {
		role, ok := p.Roles[roleName]
		return ok && grantsVerb(role, OpCreate, target)
	}
	if roleName, ok := p.Assignments[principalUser]; ok && roleGrants(roleName) {
		return true
	}
	return slices.ContainsFunc(p.AssertedRoles[principalUser], roleGrants)
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
