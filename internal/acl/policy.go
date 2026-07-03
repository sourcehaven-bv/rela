package acl

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
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
//     effective value via [Policy.membershipRelation], never the raw
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
	UserEntityType      string                     `yaml:"user_entity_type"`
	PrincipalProperty   string                     `yaml:"principal_property"`
	MembershipRelation  string                     `yaml:"membership_relation"`
	Roles               map[string]RoleDef         `yaml:"roles"`
	Assignments         map[string]string          `yaml:"assignments"`
	RoleRelations       map[string]RoleRelationDef `yaml:"role_relations"`
	InheritRolesThrough []string                   `yaml:"inherit_roles_through"`
}

// principalPropertyLookupEnabled reports whether the policy asks the
// resolver to map the raw principal (e.g. an email from
// `X-Forwarded-User`) to a user entity ID before role attribution. Both
// [Policy.UserEntityType] AND [Policy.PrincipalProperty] must be set;
// either blank leaves behavior byte-for-byte identical to a policy that
// never declared them (assignments/membership match on the raw string).
func (p *Policy) principalPropertyLookupEnabled() bool {
	return strings.TrimSpace(p.UserEntityType) != "" &&
		strings.TrimSpace(p.PrincipalProperty) != ""
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

// RoleDef is the capability bundle for a single role. The per-verb
// mutation grants (Create / Update / Delete), Permissions, and the
// affordance grants are honored by the write path and the affordances
// resolver. Read drives the read-filtering path (see
// [Declarative.ReadQuery]).
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
	Create      []string `yaml:"create"`
	Update      []string `yaml:"update"`
	Delete      []string `yaml:"delete"`
	Read        []string `yaml:"read"`
	Permissions []string `yaml:"permissions"`

	Fields    map[string][]FieldGrant    `yaml:"fields"`
	Visible   map[string][]FieldGrant    `yaml:"visible"`
	Options   map[string][]OptionGrant   `yaml:"options"`
	Relations map[string][]RelationGrant `yaml:"relations"`
}

// grantsVerb reports whether the role may perform op on entity type
// `target`. Op selects the verb list: Create / Update / Delete; Rename
// routes through Update (it is a modification). Read is handled
// separately via roleGrantsRead. An unknown op grants nothing.
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

// OptionGrant grants a single enum option on a field. Used to filter
// the option set the SPA renders and to gate writes that set the
// field to that option.
type OptionGrant struct {
	Field  string `yaml:"field"`
	Option string `yaml:"option"`
	When   string `yaml:"when,omitempty"`
}

// RelationGrant grants relation-level affordances for one relation
// type on the keyed entity type. Create and Remove are pointers so
// "unset" (use the grant's implied default of true — the grant
// existing is itself the opt-in) is distinguishable from an explicit
// false. Fields grants per-meta-field writability on links of this
// type. When conditions the whole grant on a predicate.
type RelationGrant struct {
	Relation string       `yaml:"relation"`
	Create   *bool        `yaml:"create,omitempty"`
	Remove   *bool        `yaml:"remove,omitempty"`
	Fields   []FieldGrant `yaml:"fields,omitempty"`
	When     string       `yaml:"when,omitempty"`
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
type RoleRelationDef struct {
	Confers            string `yaml:"confers"`
	RequiresPermission string `yaml:"requires_permission"`
}

// knownPolicyKeys is the allowlist used for unknown-key warnings.
// Keep in sync with [Policy]'s yaml tags.
var knownPolicyKeys = map[string]bool{
	"user_entity_type":      true,
	"principal_property":    true,
	"membership_relation":   true,
	"roles":                 true,
	"assignments":           true,
	"role_relations":        true,
	"inherit_roles_through": true,
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

// Validate enforces security-critical invariants on the parsed
// policy. Run automatically by [LoadPolicy] / [LoadPolicyBytes].
// Operators can also call it before persisting a generated policy.
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
func (p *Policy) Validate() error {
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
				if !roleGrantsRead(role, t) {
					hint := fmt.Sprintf("add %q (or \"*\")", t)
					if t == "*" {
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
	return nil
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
