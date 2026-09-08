package acl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// WorldGrantPrefix is the reserved prefix marking a `read:` entry as a
// WORLD grant rather than an entity type: `read: [world:published]`
// (design doc §8.1).
//
// Reserved by documentation, not by a metamodel check. Colliding needs an
// entity type literally named `world:something`; internal/acl cannot see
// the metamodel to reject one (arch-lint), and the collision is
// fail-closed in the only direction it can go — the type becomes
// unreadable rather than over-readable.
const WorldGrantPrefix = "world:"

// DefaultWorldName is the name of the implicit default world, mirroring
// metamodel.DefaultWorldName. Duplicated rather than imported because
// internal/acl must not depend on internal/metamodel (arch-lint); the
// two are pinned together by TestDefaultWorldNameMatchesMetamodel.
const DefaultWorldName = "default"

// normalizeWorldGrants splits `world:`-prefixed entries out of every
// role's Read list into [RoleDef.Worlds], and rejects the spellings that
// cannot mean anything.
//
// Runs at the top of [Policy.Validate] AND from [NewDeclarative], because
// those are two different chokepoints: Validate is where a policy is
// LOADED, NewDeclarative is where one starts SERVING READS, and a policy
// can reach the second without passing the first (RR-LWE222). Idempotent,
// so running it twice is harmless — a second pass finds no prefixed
// entries left in Read.
//
// Rejections, both hard errors:
//
//   - `world:` with an empty name. The empty name is the dangerous one:
//     it would make a lookup with an unpopulated name succeed and return a
//     real world, the inverse of the fail-closed rule. The metamodel
//     loader rejects it for the same reason (validateWorlds).
//   - `world:*`. A glob silently absorbs worlds added later, which is
//     fail-open drift — the argument design doc §4.5 already makes for
//     refusing `world:site-*` globs in grants. Naming worlds is the point.
//
// It does NOT reject a world name that no metamodel declares: that needs
// the schema, which this package cannot see. aclaudit reports it.
//
// A role with no `world:` token is left untouched — see the early-continue
// below, which is load-bearing for shared policies, not an optimization.
func (p *Policy) normalizeWorldGrants() error {
	for name, role := range p.Roles {
		if !roleHasWorldToken(role) {
			// NOTHING TO DO — and taking this branch is what makes the
			// function safe to call on a SHARED policy. SharedBase hands one
			// *Policy face to every assembled Services and documents that
			// assembly only READS it; NewDeclarative runs on that face, so
			// an unconditional `p.Roles[name] = role` would be a write on a
			// shared map (a concurrent-map-write panic under parallel
			// Assemble, not a benign race). An already-split policy — which
			// every Validate-loaded one is — never reaches the write below.
			continue
		}
		var types []string
		// SEED from what is already there rather than starting empty. The
		// second pass sees a Read list with no world tokens left in it, so
		// rebuilding Worlds from scratch would wipe the grants the first
		// pass extracted. Caught by TestWorldGrantSplit_Idempotent.
		worlds := role.Worlds
		for _, entry := range role.Read {
			world, ok := strings.CutPrefix(entry, WorldGrantPrefix)
			if !ok {
				types = append(types, entry)
				continue
			}
			world = strings.TrimSpace(world)
			switch world {
			case "":
				return fmt.Errorf(
					"roles.%s: read grant %q names no world — write the world's "+
						"name (e.g. %qpublished); an empty name would match a "+
						"lookup with an unset world and serve a real, non-default world",
					name, entry, WorldGrantPrefix)
			case "*":
				return fmt.Errorf(
					"roles.%s: read grant %q is not allowed — world grants must "+
						"name each world explicitly, because a glob silently absorbs "+
						"worlds declared later (an auto-widening read grant)",
					name, entry)
			}
			if !slices.Contains(worlds, world) {
				worlds = append(worlds, world)
			}
		}
		role.Read = types
		role.Worlds = worlds
		p.Roles[name] = role
	}
	return nil
}

// roleHasWorldToken reports whether any Read entry still carries the
// `world:` prefix, i.e. whether this role needs splitting at all.
func roleHasWorldToken(role RoleDef) bool {
	for _, entry := range role.Read {
		if strings.HasPrefix(entry, WorldGrantPrefix) {
			return true
		}
	}
	return false
}

// roleGrantsWorldRead reports whether role may read the named world.
//
// The DEFAULT world is covered by an ordinary read grant — a bare type or
// `"*"` in Read — so an existing acl.yaml keeps meaning exactly what it
// meant (design doc §8.1). Every OTHER world must be named, which is what
// makes reading a non-default world impossible to acquire by accident.
//
// Callers must resolve the role through Request.roleFor first, so the
// client ceiling has already clamped Worlds; this predicate does not
// re-check it. The ceiling's world DENIAL is applied separately by
// [compiledCeiling.permitsWorld] — see that method for why a denial
// cannot be expressed by clamping alone.
func roleGrantsWorldRead(role RoleDef, world string) bool {
	if world == "" || world == DefaultWorldName {
		// The default world is the absence of a world grant. Any read
		// grant at all covers it; holding none covers nothing.
		return len(role.Read) > 0
	}
	return slices.Contains(role.Worlds, world)
}

// parseStateGrant splits a write grant of the form "type@face" into its
// parts, reporting ok=false when the entry carries no separator (a plain
// type grant, which addresses the DEFAULT state).
//
// The face half is parsed by [entity.ParseFace], so a grant and the
// storage layer can never disagree about what a face name is.
//
// The TYPE half is deliberately NOT validated here, and NOT parsed with
// [entity.ParseStateRef]. That codec validates its left side with
// entity.ValidateID — the ENTITY-ID grammar, which rejects the internal
// spaces metamodel.ValidateSchemaName deliberately permits in a type name
// ("some property" is that function's own documented example). Reusing it
// would reject legal type names with an error message about entity IDs.
// Whether the type exists at all is a schema question, so aclaudit
// reports it (B1-undeclared-type).
func parseStateGrant(s string) (typeName string, p entity.Face, isState bool, err error) {
	typeName, ptr, found := strings.Cut(s, entity.StateRefSeparator)
	if !found {
		return s, "", false, nil
	}
	if strings.Contains(ptr, entity.StateRefSeparator) {
		return "", "", true, fmt.Errorf(
			"write grant %q: more than one %q — a state cannot have a state",
			s, entity.StateRefSeparator)
	}
	parsed, perr := entity.ParseFace(ptr)
	if perr != nil {
		return "", "", true, fmt.Errorf("write grant %q: %w", s, perr)
	}
	if typeName == "" {
		return "", "", true, fmt.Errorf(
			"write grant %q: names a state but no entity type", s)
	}
	return typeName, parsed, true, nil
}

// grantTypeOf returns the entity type a write-grant entry addresses:
// the whole entry for a plain type grant, the part before "@" for a
// state grant. Used where only the TYPE matters — the write⊆read
// coverage check, and aclaudit's undeclared-type checks.
func grantTypeOf(entry string) string {
	typeName, _, _ := strings.Cut(entry, entity.StateRefSeparator)
	return typeName
}

// validateStateGrants checks every write-grant entry that carries a state
// separator, so a malformed face fails at load rather than silently
// granting nothing.
func (p *Policy) validateStateGrants() error {
	for _, name := range sortedRoleNames(p.Roles) {
		role := p.Roles[name]
		for _, verb := range []struct {
			name string
			list []string
		}{
			{"create", role.Create}, {"update", role.Update}, {"delete", role.Delete},
		} {
			for _, entry := range verb.list {
				if _, _, isState, err := parseStateGrant(entry); isState && err != nil {
					return fmt.Errorf("roles.%s: %s: %w", name, verb.name, err)
				}
			}
		}
	}
	return nil
}

// sortedRoleNames returns the policy's role names in a deterministic
// order, so a load error names the same role on every run rather than
// whichever one Go's map iteration reached first.
func sortedRoleNames(roles map[string]RoleDef) []string {
	names := make([]string, 0, len(roles))
	for name := range roles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// GrantsAnyNonDefaultWorldRead reports whether any role in this policy
// grants read on a world other than the default one.
//
// This is the trigger half of the membership-gate load refusal
// ([Policy.WorldGrantRefusalReason]): the refusal exists because a
// non-default world read grant changes the stakes of an ungated
// role-relation from over-granting inside a trusted team to a working
// mechanism for reading unpublished content.
//
// A policy whose only world grant is `world:default` does NOT count — that
// grant means exactly what an ordinary read grant already meant.
func (p *Policy) GrantsAnyNonDefaultWorldRead() bool {
	for _, role := range p.Roles {
		for _, w := range role.Worlds {
			if w != DefaultWorldName {
				return true
			}
		}
	}
	return false
}

// GrantsVerbOnState reports whether role may perform op on the specific
// FACE (target, p) of an entity — the face-granular write check.
//
// Matching is EXACT and there is no inheritance between states (design
// doc §8.2, fail closed):
//
//   - `update: ["page"]` grants the DEFAULT state of page, and nothing
//     else. An operator who adds faces to an existing type finds their
//     existing grants now cover one face — the correct fail-closed reading,
//     and the reason it is documented as a migration note.
//   - `update: ["page@draft"]` grants the draft face only; it does NOT
//     grant published, nor the default state.
//   - `update: ["*"]` grants every type's DEFAULT state. It is a wildcard
//     over TYPES, never over faces — otherwise the one grant every
//     admin policy already holds would silently acquire authority over
//     every face the moment a type declares faces, which is precisely
//     the "no role holds update on published" invariant (§8.2) inverted.
func GrantsVerbOnState(role RoleDef, op Op, target string, p entity.Face) bool {
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
	for _, entry := range list {
		if entry == "*" {
			if p.IsDefault() {
				return true
			}
			continue
		}
		entryType, entryPtr, isState, err := parseStateGrant(entry)
		if err != nil {
			continue // malformed grants are rejected at load; never grant here
		}
		if entryType != target {
			continue
		}
		if !isState {
			entryPtr = "" // a bare type grant addresses the default state
		}
		if entryPtr == p {
			return true
		}
	}
	return false
}

// isStateGrant reports whether a write-grant entry names a specific face
// (`page@draft`) rather than a bare entity type.
//
// Kept separate from [grantTypeOf] because the two answer opposite
// questions and the distinction is load-bearing: grantTypeOf is for code
// that wants the TYPE regardless of face (the write⊆read coverage check,
// aclaudit's undeclared-type checks), while this is for code that must
// treat a face-specific grant as out of its scope.
func isStateGrant(entry string) bool {
	return strings.Contains(entry, entity.StateRefSeparator)
}
