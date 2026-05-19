package acl

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// Policy is the declarative ACL configuration parsed from `acl.yaml`
// at the project root. v0 fields:
//
//   - [Policy.UserEntityType] declares which entity type represents
//     a user (e.g. "person", "user"). Currently informational —
//     reserved for v1 group expansion when [Policy.RoleRelations]
//     binds principals to entities.
//   - [Policy.Roles] declares the named capability bundles. The
//     special role name `default` is appended to every principal's
//     effective role set; see [Declarative.effectiveRoles].
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
type RoleDef struct {
	Write       []string `yaml:"write"`
	Read        []string `yaml:"read"`
	Permissions []string `yaml:"permissions"`
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
