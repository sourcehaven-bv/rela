package acl

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// EveryoneRole is the one built-in role name. A role declared under
// this name in `acl.yaml` is held implicitly by every principal,
// authenticated or not — it is how an operator expresses "applies to
// everyone" without enumerating users. It is appended to every
// principal's effective role set (see [Declarative.effectiveRoles]
// and the affordances resolver), and is the single source of truth
// for the name so the two paths can't drift.
//
// (No `anonymous` / `authenticated` built-ins yet: rela-server has no
// authentication layer — see docs/security.md. Those would be added
// here when auth lands, so both write and affordance paths see them.)
const EveryoneRole = "everyone"

// Policy is the declarative ACL configuration parsed from `acl.yaml`
// at the project root. v0 fields:
//
//   - [Policy.UserEntityType] declares which entity type represents
//     a user (e.g. "person", "user"). Currently informational —
//     reserved for v1 group expansion when [Policy.RoleRelations]
//     binds principals to entities.
//   - [Policy.Roles] declares the named capability bundles. The
//     built-in role name [EveryoneRole] ("everyone") is appended to
//     every principal's effective role set; see
//     [Declarative.effectiveRoles].
//   - [Policy.Assignments] maps `principal.User` → role name.
//     Unknown role names (assigned but not declared in Roles) log a
//     warning at load and are dropped from the effective set.
//   - [Policy.RoleRelations] declares which relation types grant a
//     role to their source entity, and which permission the writer
//     must hold (delegate-X tamper resistance — see [Declarative]).
//
// **Tolerant by design.** Unknown top-level keys emit one
// `slog.Warn` per key and are otherwise ignored. Operators iterate
// on `acl.yaml` frequently and a typo shouldn't brick the server —
// the metamodel loader follows the same convention. Hard errors
// reserved for unparseable YAML or undecodable values within a
// known key.
type Policy struct {
	UserEntityType string                     `yaml:"user_entity_type"`
	Roles          map[string]RoleDef         `yaml:"roles"`
	Assignments    map[string]string          `yaml:"assignments"`
	RoleRelations  map[string]RoleRelationDef `yaml:"role_relations"`
}

// RoleDef is the capability bundle for a single role. v0 honors
// Write and Permissions; Read is parsed but unused (v1 reads).
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
type RoleRelationDef struct {
	Confers            string `yaml:"confers"`
	RequiresPermission string `yaml:"requires_permission"`
}

// knownPolicyKeys is the allowlist used for unknown-key warnings.
// Keep in sync with [Policy]'s yaml tags.
var knownPolicyKeys = map[string]bool{
	"user_entity_type": true,
	"roles":            true,
	"assignments":      true,
	"role_relations":   true,
}

// LoadPolicy reads and parses `acl.yaml` at the given path.
//
// Errors:
//   - The caller distinguishes "no policy file" from "broken policy
//     file" via [os.ErrNotExist]. Use `errors.Is(err, os.ErrNotExist)`
//     to fall back to [NopACL] when no policy is present.
//   - Any other I/O error or YAML parse error returns wrapped.
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
	return &policy, nil
}
