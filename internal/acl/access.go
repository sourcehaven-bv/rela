package acl

import (
	"context"
	"fmt"
	"sort"
)

// Verb is a read-or-write access verb the who-can query asks about. It
// widens [Op] (create/update/delete/rename) with read, which has no Op
// because the write path never authorizes reads — the read path is
// Request.readQuery. VerbRead routes through the read machinery;
// every other verb routes through the per-verb write grant.
type Verb string

// Verb constants. VerbRead maps to the read path; the rest map 1:1 to
// the write [Op] of the same name.
const (
	VerbRead   Verb = "read"
	VerbCreate Verb = "create"
	VerbUpdate Verb = "update"
	VerbDelete Verb = "delete"
)

// Valid reports whether v is one of the known verbs. Unknown verbs must
// be rejected rather than defaulted — a fail-closed access tool never
// silently treats a garbage verb as "read".
func (v Verb) Valid() bool {
	switch v {
	case VerbRead, VerbCreate, VerbUpdate, VerbDelete:
		return true
	default:
		return false
	}
}

// op maps a write Verb to its [Op]. VerbRead has no Op (read never goes
// through the write path) and returns ("", false).
func (v Verb) op() (Op, bool) {
	switch v {
	case VerbCreate:
		return OpCreate, true
	case VerbUpdate:
		return OpUpdate, true
	case VerbDelete:
		return OpDelete, true
	default:
		return "", false
	}
}

// EveryoneGrant reports how a policy grants a verb to *every* principal
// via the built-in "everyone" role — the case that must be reported
// once, globally, rather than per enumerated principal (the `everyone`
// role is appended to every principal's effective set, so enumerating
// principals for it would be both redundant and wrong-shaped).
type EveryoneGrant struct {
	// Granted is true when the "everyone" role is declared and grants
	// the verb on the target type (directly or via a "*" wildcard).
	Granted bool
	// Wildcard is true when the grant came from a "*" entry rather than
	// an exact type match — worth surfacing because it means the grant
	// covers every type, not just this one.
	Wildcard bool
}

// EveryoneGrants reports whether the built-in "everyone" role grants
// verb on entityType. Callers use this to print a single global
// "everyone (all principals, incl. unauthenticated)" line instead of
// attributing the everyone role to each enumerated principal.
//
// Read and write verbs are checked against the same policy fields the
// runtime consults (roleGrantsRead for read, grantsVerb for writes), so
// this can never disagree with an actual authorization decision.
func (d *Declarative) EveryoneGrants(verb Verb, entityType string) EveryoneGrant {
	role, ok := d.policy.Roles[EveryoneRole]
	if !ok {
		return EveryoneGrant{}
	}
	return grantForRole(role, verb, entityType)
}

// AssertedGrant is one (claim → role) mapping that grants a verb, for reporting
// grants whose holders are NOT enumerable from the graph: the population that
// presents a given claim lives in the IdP.
//
// This is deliberately NOT folded into [EveryoneGrant]. An everyone grant
// applies to every principal — reporting it globally is a true statement. An
// asserted grant applies to an unknown subset, so reporting it the same way
// would tell an operator that everyone holds the role.
type AssertedGrant struct {
	Claim    string
	Role     string
	Wildcard bool
}

// AssertedGrants reports every asserted_role_assignments mapping that grants
// verb on entityType. Uses the same grantForRole helper as [Declarative.EveryoneGrants] and
// Request.AccessRoutes, so a reported grant can never disagree with an actual
// authorization decision.
//
// Undeclared target roles are skipped, matching what the resolver does at
// attribution time. Results are sorted by (claim, role) for a stable artifact.
func (d *Declarative) AssertedGrants(verb Verb, entityType string) []AssertedGrant {
	var out []AssertedGrant
	for claim, roles := range d.policy.AssertedRoles {
		for _, name := range roles {
			role, declared := d.policy.Roles[name]
			if !declared {
				continue
			}
			if g := grantForRole(role, verb, entityType); g.Granted {
				out = append(out, AssertedGrant{
					Claim: claim, Role: name, Wildcard: g.Wildcard,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Claim != out[j].Claim {
			return out[i].Claim < out[j].Claim
		}
		return out[i].Role < out[j].Role
	})
	return out
}

// grantForRole reports whether one role grants verb on entityType and
// whether the match came from a "*" wildcard. It is the single place
// the verb→field mapping lives, shared by EveryoneGrants and
// Request.AccessRoutes so their answers can't drift.
func grantForRole(role RoleDef, verb Verb, entityType string) EveryoneGrant {
	var list []string
	if verb == VerbRead {
		list = role.Read
	} else {
		op, ok := verb.op()
		if !ok {
			return EveryoneGrant{}
		}
		switch op {
		case OpCreate:
			list = role.Create
		case OpUpdate, OpRename:
			list = role.Update
		case OpDelete:
			list = role.Delete
		}
	}
	for _, t := range list {
		if t == "*" {
			return EveryoneGrant{Granted: true, Wildcard: true}
		}
		if t == entityType {
			return EveryoneGrant{Granted: true}
		}
	}
	return EveryoneGrant{}
}

// AccessRoutes returns the attributions by which this Request's
// principal is granted verb on the entity (entityID of entityType),
// EXCLUDING the built-in "everyone" role (reported once globally via
// [Declarative.EveryoneGrants], never per-principal).
//
// It is the who-can building block. The returned attributions carry the
// full (role, Source) provenance the resolver already computes — all
// distinct routes, deduped, never collapsed to a primary source.
//
// The BOOLEAN "has any access" answer is authoritative and matches the
// runtime exactly — no false negatives:
//
//   - Write verbs (create/update/delete): an attribution's role grants
//     the verb via the same grantsVerb the write path uses. The write
//     path IS this attribution set, so equivalence is by construction.
//   - Read: gated by [Request.PermitsRead] — the actual runtime read
//     decision. If PermitsRead is false the result is empty; if true,
//     the returned attributions are those whose role grants read
//     (roleGrantsRead). Because the boolean is the runtime's own answer,
//     "who-can read reports nobody" can never contradict a real allow.
//     An error from the read backend is returned so the caller fails
//     loud rather than silently under-reporting readers.
//
// The "everyone" role is skipped here on purpose: it lands in every
// principal's global attributions unconditionally, so including it
// would report it once per enumerated principal. Callers surface it
// separately and globally.
func (r *Request) AccessRoutes(
	ctx context.Context, verb Verb, entityType, entityID string,
) ([]RoleAttribution, error) {
	if !verb.Valid() {
		return nil, fmt.Errorf("acl: AccessRoutes: unknown verb %q", verb)
	}
	if verb == VerbRead {
		permitted, err := r.PermitsRead(ctx, entityType, entityID)
		if err != nil {
			return nil, err
		}
		if !permitted {
			return nil, nil
		}
		return r.grantingAttributions(ctx, VerbRead, entityType, entityID), nil
	}
	return r.grantingAttributions(ctx, verb, entityType, entityID), nil
}

// grantingAttributions filters the per-entity attribution set to those
// whose role grants verb on entityType, excluding the everyone role.
// The verb→predicate mapping is the same one the runtime uses
// (roleGrantsRead / grantsVerb), so the routes it credits are the real
// reasons access was granted.
//
// Create is intentionally computed with the concrete entityID, same as
// every other write verb — NOT globals-only. This mirrors the production
// create path: entitymanager.ApplyEntity authorizes create with
// EntitySubject{ID: e.ID} (the new entity's id, which exists at authz
// time), so authorizeEntityWrite takes its `s.ID != ""` branch and folds
// in local-role-via-edge / via-ancestor routes. Collapsing create to
// Globals here would report a false DENY for a principal the runtime WOULD
// let create via an edge — the worst error class for an attestation tool.
// The `s.ID == ""` globals-only branch exists for callers with no id yet;
// this report always has one.
func (r *Request) grantingAttributions(
	ctx context.Context, verb Verb, entityType, entityID string,
) []RoleAttribution {
	attrs := r.ForEntity(ctx, entityType, entityID)
	op, isWrite := verb.op()

	// An attestation tool must report EFFECTIVE access, so the client ceiling
	// applies here exactly as it does at runtime. Reporting the un-attenuated
	// role set would tell an operator a restricted client can do something it
	// cannot — the worst error class for a tool whose whole job is answering
	// "who can do what".
	if isWrite && !r.ceiling.permitsVerb(op, entityType) {
		return nil
	}
	if !isWrite && !r.ceiling.permitsRead(entityType) {
		return nil
	}

	var out []RoleAttribution
	for _, a := range attrs {
		if a.Role == EveryoneRole {
			continue
		}
		role, ok := r.roleFor(a.Role)
		if !ok {
			continue
		}
		var granted bool
		if isWrite {
			granted = grantsVerb(role, op, entityType)
		} else {
			granted = roleGrantsRead(role, entityType)
		}
		if granted {
			out = append(out, a)
		}
	}
	return out
}
