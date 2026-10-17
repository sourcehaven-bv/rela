package acl

import (
	"context"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Verb is a read-or-write access verb. The who-can query asks about one,
// and so does the affordance resolver when it decides what a role without a
// field block contributes ([Request.RoleGrantsVerbOnFace]).
//
// A Verb is an [Op] or read. Read has no Op because the write path never
// authorizes reads; the read path is Request.readQuery. Every write verb is
// the Op of the same name, so the verb set follows [Ops] and a new write Op
// cannot be left out of the reports that enumerate [Verbs].
type Verb string

// VerbRead is the one verb that is not an [Op].
const VerbRead Verb = "read"

// The write verbs, one per [Op].
const (
	VerbCreate = Verb(OpCreate)
	VerbUpdate = Verb(OpUpdate)
	VerbDelete = Verb(OpDelete)
	VerbRename = Verb(OpRename)
)

// Verbs returns [VerbRead] followed by one verb per [Ops] entry, in that
// order.
func Verbs() []Verb {
	ops := Ops()
	out := make([]Verb, 0, len(ops)+1)
	out = append(out, VerbRead)
	for _, op := range ops {
		out = append(out, Verb(op))
	}
	return out
}

// Valid reports whether v is one of the known verbs. Unknown verbs must
// be rejected rather than defaulted — a fail-closed access tool never
// silently treats a garbage verb as "read".
//
// [VerbRename] is valid, but the grant helpers that answer reports
// (AccessRoutes, EveryoneGrants, AssertedGrants) read it from the `update:`
// list, which is wrong in both directions. aclmap keeps rename out of its
// reports until TKT-4APGVF fixes them; a new caller must do the same.
func (v Verb) Valid() bool {
	return v == VerbRead || Op(v).Valid()
}

// op maps a write Verb to its [Op]. VerbRead has no Op (read never goes
// through the write path) and returns ("", false), as does an unknown verb.
func (v Verb) op() (Op, bool) {
	if v == VerbRead {
		return "", false
	}
	if op := Op(v); op.Valid() {
		return op, true
	}
	return "", false
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

// ClampedEveryoneGrants is [Declarative.EveryoneGrants] for r's principal: the client ceiling applies first, then the everyone role as
// roleFor narrows it. A report about one principal's effective access asks
// this one. The policy-wide answer would show an everyone grant that a
// `client_baselines:` entry removes, so a restricted client (a webhook, an
// app) would appear to hold it.
//
// It and [ClampedAssertedGrants] are functions rather than [Request] methods
// because Request is at its plimsoll method load line.
func ClampedEveryoneGrants(r *Request, verb Verb, entityType string) EveryoneGrant {
	if !r.ceilingPermitsVerb(verb, entityType) {
		return EveryoneGrant{}
	}
	role, ok := r.roleFor(EveryoneRole)
	if !ok {
		return EveryoneGrant{}
	}
	return grantForRole(role, verb, entityType)
}

// ClampedAssertedGrants is [Declarative.AssertedGrants] for r's principal,
// with the client ceiling applied the same way as in [ClampedEveryoneGrants].
func ClampedAssertedGrants(r *Request, verb Verb, entityType string) []AssertedGrant {
	if !r.ceilingPermitsVerb(verb, entityType) {
		return nil
	}
	var out []AssertedGrant
	for _, g := range r.d.AssertedGrants(verb, entityType) {
		role, ok := r.roleFor(g.Role)
		if !ok {
			continue
		}
		if clamped := grantForRole(role, verb, entityType); clamped.Granted {
			out = append(out, AssertedGrant{Claim: g.Claim, Role: g.Role, Wildcard: clamped.Wildcard})
		}
	}
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
		// A state-shaped write grant is skipped for the same reason
		// [grantsVerb] skips it: it authorizes no face today, so reporting
		// it as a grant here would put a false all-clear into `rela acl
		// who-can` / `rela acl map` — the operator-facing security reports
		// this helper exists to keep honest (RR-Q1LI2Y).
		if verb != VerbRead && isStateGrant(t) {
			continue
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
//     the verb via grantsVerb, the TYPE-granular predicate.
//
//     Equivalence with the runtime is no longer by construction, and the
//     divergence is deliberate: since TKT-C1XUA8 the write path is
//     face-granular (decideFromAttrs → GrantsVerbOnState), while this
//     report has no face in hand — "who can update page?" is a question
//     about a type. So a role holding ONLY a face-specific grant
//     (`update: ["page@draft"]`) is not credited here, and this answer is
//     a LOWER BOUND on write access rather than an exact one.
//
//     That is the safe direction for an attestation tool — a false
//     negative, never a false all-clear — but it is a real limitation, not
//     a guarantee. A report that names a face is the fix if operators need
//     one; it needs a face threaded through AccessRoutes.
//
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
// The verb→predicate mapping matches the runtime for reads; for writes it is
// the TYPE-granular grantsVerb, so a face-specific grant is not credited.
// Every route it credits is a real reason access was granted; it may omit a
// face-specific one. See [Request.AccessRoutes] for why.
//
// Create is intentionally computed with the concrete entityID, same as
// every other write verb — NOT globals-only. This mirrors the production
// create path: entitymanager.RecreateEntity authorizes create with
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
		// coverage-ignore: defensive: ForEntity attributions only carry roles the resolver already confirmed defined in
		// policy.Roles, so ok is
		// always true here
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

// GrantsRead reports whether role's read list covers entityType — exact
// match or the "*" wildcard.
//
// GrantsRead and [GrantsVerb] are exported so consumers outside this
// package (the `rela docs` role-matrix generator) match grants with the
// SAME predicate the runtime uses, rather than hand-copying the
// wildcard-or-exact loop. A second copy silently disagrees the moment the
// grant grammar changes — which is exactly what happened when write grants
// gained the `type@face` form (TKT-DN37J2): the copy reported "cannot
// update page" for a role holding `update: ["page@draft"]`, a false
// all-clear in an operator-facing security document (RR-Q1LI2Y).
//
// These take a plain RoleDef and so do NOT apply a client ceiling; they
// answer "what does this role grant", not "what may this principal do".
// For the latter, resolve through Request.roleFor first.
func GrantsRead(role RoleDef, entityType string) bool {
	return roleGrantsRead(role, entityType)
}

// GrantsVerb reports whether role may perform op on entityType, honoring
// the "*" wildcard and the `type@face` state form. Rename routes
// through the update grant. See [GrantsRead] for why this is exported.
func GrantsVerb(role RoleDef, op Op, entityType string) bool {
	return grantsVerb(role, op, entityType)
}

// ReadFaces returns the faces of entityType this role may read, and whether it
// may read EVERY face. Exported for the same reason [GrantsRead] is: a
// capability DISPLAY that recomputed this would drift from the runtime the
// moment the grant grammar changed.
//
// [GrantsRead] alone is not enough to describe read access, and reporting only
// it overstates the grant: it answers true for a role holding
// `read: [policy@published]`, because a face-scoped grant does grant its type
// for the purposes of composing a query. Which FACES is a separate question,
// and this is it — the pair is what a role's read access actually is.
//
// all=true means every face (a bare `read: [policy]` grant, or "*"); faces is
// then nil. Note the asymmetry with the write side, documented on
// roleReadFaces: a bare READ grant covers every face, while a bare update grant
// covers only the default one.
func ReadFaces(role RoleDef, entityType string) (faces []entity.Face, all bool) {
	return roleReadFaces(role, entityType)
}
