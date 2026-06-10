package acl

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// EveryoneRole is the one built-in role name. A role declared under
// this name in `acl.yaml` is held implicitly by every principal,
// authenticated or not — it is how an operator expresses "applies to
// everyone" without enumerating users. It is appended to every
// principal's effective role set in both the Subject-driven write
// path ([Request]) and the affordances resolver, and is the single
// source of truth for the name so the two paths can't drift.
//
// (No `anonymous` / `authenticated` built-ins yet: rela-server has no
// authentication layer — see docs/security.md. Those would be added
// here when auth lands, so both write and affordance paths see them.)
const EveryoneRole = "everyone"

// Policy is the declarative ACL configuration parsed from `acl.yaml`
// at the project root.
//
//   - [Policy.UserEntityType] names the entity type that represents
//     a user (e.g. "person", "user"). Reserved for a future
//     check that validates `member-of` edges originate from a user
//     entity; not consulted by the resolver today (RR-NIGK).
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
	Roles               map[string]RoleDef         `yaml:"roles"`
	Assignments         map[string]string          `yaml:"assignments"`
	RoleRelations       map[string]RoleRelationDef `yaml:"role_relations"`
	InheritRolesThrough []string                   `yaml:"inherit_roles_through"`
}

// RoleDef is the capability bundle for a single role. Write,
// Permissions, and the affordance grants are honored by the write
// path and the affordances resolver. Read drives the read-filtering
// path (see [Declarative.ReadQuery]).
//
// Wildcard write: a single entry `"*"` grants write on every entity
// type. Mixing `"*"` with explicit types is allowed but redundant —
// the wildcard short-circuits the per-type check.
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
	Write       []string `yaml:"write"`
	Read        []string `yaml:"read"`
	Permissions []string `yaml:"permissions"`

	Fields    map[string][]FieldGrant    `yaml:"fields"`
	Visible   map[string][]FieldGrant    `yaml:"visible"`
	Options   map[string][]OptionGrant   `yaml:"options"`
	Relations map[string][]RelationGrant `yaml:"relations"`
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
// **Escalation risk for the `member-of` relation** (RR-7O6Q). v1
// confers group roles by walking `member-of` edges. By default
// `member-of` is a regular relation type with no `requires_permission`
// gate, so anyone with write access on the relation's source type can
// create their own `member-of` edge into any group named in
// [Policy.Assignments]. If a group is assigned a privileged role
// (e.g. `assignments: { admins: admin }`), an attacker with write
// access on `person` can self-promote by writing
// `alice --member-of--> admins`.
//
// Operators using groups for role attribution MUST gate
// `member-of` writes. Recommended shape:
//
//	role_relations:
//	  member-of:
//	    requires_permission: delegate-membership
//	roles:
//	  admin:
//	    permissions: [delegate-membership]
//
// This restricts `member-of` creation to principals holding
// `delegate-membership` — typically only admins. See
// `docs/security.md` for the full hardening pattern. The UC1 example
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
//   - RoleRelations keys must be non-empty and non-whitespace, for the
//     same reason — an empty key would gate "all relation writes" on
//     a delegate permission, breaking writes the operator didn't mean
//     to gate.
//   - Every role's write grants must be covered by its read grants
//     (write ⊆ read, wildcard-aware). A role that can create or update
//     a type it cannot read produces incoherent UX (empty list with a
//     working Create button) and would force every affordance consumer
//     to handle the combination. Rejecting it at load lets downstream
//     read-side logic assume "writable implies readable" (RR-W2J6).
//
// Validation is intentionally narrow: misspelled role names, unknown
// entity types in grants, etc. remain warnings (or analyze-tool
// findings) per the "tolerant by design" stance. Security-relevant
// invariants like the ones above are the exception.
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
		for _, w := range role.Write {
			if !roleGrantsRead(role, w) {
				hint := fmt.Sprintf("add %q (or \"*\")", w)
				if w == "*" {
					hint = `add "*"`
				}
				return fmt.Errorf(
					"roles.%s: grants write on %q without a covering read grant; "+
						"%s to the role's read list — a principal must "+
						"be able to read every type it can write", name, w, hint)
			}
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
