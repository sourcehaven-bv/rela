package acl

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ErrUnstampedPrincipal is the sentinel returned by
// [IdentityResolver.Resolve] and [IdentityResolver.ResolveDirect] when the
// principal carries User="" / User="unknown" or Tool="" / Tool="unknown".
// Code paths that forgot to stamp identity must fail loud rather than
// silently degrade.
var ErrUnstampedPrincipal = errors.New("acl: principal is unstamped (User or Tool is unknown)")

// GlobalRoles is the global (no-entity) role set of a [Request]. Carries
// both the attributions and the membership closure of the [Identity] it
// was computed from, so per-entity probes reuse the closure.
type GlobalRoles struct {
	Attributions []RoleAttribution
	Members      []string
}

// Request is a per-operation evaluation scope. Constructed via
// [Declarative.ForIdentity]; methods are not safe for concurrent use
// by multiple goroutines — open one Request per logical operation and
// let it go out of scope when the operation completes.
//
// Its global roles are computed at construction from the [Identity] and
// the policy; no Request method walks membership. The principal is bound
// at construction and never revalidated by Request methods.
type Request struct {
	d         *Declarative
	principal principal.Principal
	globals   GlobalRoles

	// ceiling is the client attenuation resolved from the principal's verified
	// principal_type + scope claims (TKT-IAC8TX). Computed once at
	// construction: it depends only on the bound principal and the immutable
	// policy, so there is nothing to invalidate and no reason to recompute it
	// per call. Inactive (and fully transparent) for an interactive user or any
	// principal whose type no baseline covers.
	ceiling compiledCeiling

	// roleVerbAnswers memoises [Request.RoleGrantsVerbOnFace], which the field
	// verdicts ask per role per row. Under an active ceiling each answer
	// clamps the role, so a list page would otherwise clamp the same role
	// once per row. Allocated on first use.
	roleVerbAnswers map[roleVerbQuestion]bool
	// unmatchedVerified is set by [Declarative.ForIdentity] when the
	// Identity was unmatched and came from a verified assertion; see
	// [Request.UnmatchedVerified].
	unmatchedVerified bool
}

// ForIdentity opens a Request scope for a resolved [Identity]. The global
// role set is a pure function of the identity and the policy, so this does
// no graph traffic.
//
// It returns [ErrNoIdentity] for the zero Identity and [ErrForeignIdentity]
// for an identity that another Declarative's resolver produced.
func (d *Declarative) ForIdentity(id Identity) (*Request, error) {
	if id.origin == nil {
		return nil, ErrNoIdentity
	}
	if id.origin != d.identities {
		return nil, ErrForeignIdentity
	}
	p := id.principal.Clone()
	return &Request{
		d:                 d,
		principal:         p,
		globals:           d.globalRoles(p, id.members),
		ceiling:           d.policy.ceilingFor(p.PrincipalType(), p.Scopes()),
		unmatchedVerified: id.unmatched && id.verified,
	}, nil
}

// UnmatchedVerified reports that the request's principal was
// cryptographically verified but resolved to no user entity: the
// `principal_property` lookup ran and found no match. The data-entry edge
// sets it through [Identity.WithVerifiedAssertion]; the CLI, MCP, the
// scheduler and header-authenticated requests never do.
//
// [Declarative.AuthorizeWrite], the single point every entity write reaches,
// reads it to enforce `unmatched_principal`. Lazy provisioning reads it to
// decide whether to create a stub user, and then binds a new Request on the
// provisioned identity, which is not unmatched.
func (r *Request) UnmatchedVerified() bool { return r.unmatchedVerified }

// roleFor resolves a declared role name to its definition, NARROWED by the
// request's client ceiling.
//
// This is THE clamp point for client attenuation. Every evaluation path in this
// package resolves a role name and then asks a predicate of the result, so
// narrowing here means read gating, write authorization and permission checks
// all inherit the ceiling without any of them knowing it exists. Reaching into
// `r.d.policy.Roles[...]` directly from an evaluation path BYPASSES the ceiling
// — use this instead. Pinned by TestNoDirectRoleLookupInEvaluationPaths.
//
// The returned RoleDef holds plain allowlists; the runtime never learns the
// word "deny". See ceiling.go for why that is load-bearing.
func (r *Request) roleFor(name string) (RoleDef, bool) {
	role, ok := r.d.policy.Roles[name]
	if !ok {
		return RoleDef{}, false
	}
	return r.ceiling.clamp(role), true
}

// Globals returns the principal's global role set, computed at
// construction by [Declarative.ForIdentity]. No graph traffic.
func (r *Request) Globals() GlobalRoles {
	return r.globals
}

// ForEntity returns the full attribution set for (principal, entityID
// of entityType): the cached Globals plus any local-role-via-edge or
// local-role-via-ancestor sources reachable from the entity.
//
// Used by write authz (where the caller has an entity in hand) and by
// affordance verdicts. Single-entity get_entity read gates consult the
// role set returned here.
//
// Passing entityID == "" returns Globals only (no per-entity probes).
func (r *Request) ForEntity(ctx context.Context, _, entityID string) []RoleAttribution {
	if entityID == "" {
		return r.Globals().Attributions
	}
	return r.computeForEntity(ctx, entityID)
}

// AuthorizeWrite gates a single write — the entry point used by
// entitymanager.Manager.{Create,Update,Delete}{Entity,Relation} +
// RenameEntity once the migration in TKT-SVXL PR (a) lands.
func (r *Request) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
	return r.authorizeWrite(ctx, req)
}

// ReadQuery composes a ReadQueryResult for list-style reads. The
// dataentry handler consumes this and either runs an unfiltered list
// (AllowAll), returns empty (DenyAll), or runs the composed
// store.GraphQuery.
func (r *Request) ReadQuery(_ context.Context, entityType string) ReadQueryResult {
	return r.readQuery(entityType)
}

// errReadQueryZero reports a ReadQueryResult with none of its three states
// set, which readQuery never returns.
var errReadQueryZero = errors.New("acl: read gate: readQuery returned zero ReadQueryResult")

// PermitsRead reports whether the principal may read SOME stored face of
// entityID: [Request.ReadableFacesMany] for one id. It is the 404 decision
// for a bare id, which names the family rather than one face.
//
// A global grant answers true without verifying existence; callers that need
// existence read the row afterwards. A backing error is returned verbatim so
// the caller can map it to the right HTTP status.
func (r *Request) PermitsRead(ctx context.Context, entityType, entityID string) (bool, error) {
	vs, err := r.ReadableFacesMany(ctx, entityType, []string{entityID})
	if err != nil {
		return false, err
	}
	return !vs.For(entityID).None(), nil
}

// PermitsReadFace reports whether the principal may read the row of entityID
// at face: the verdict on that row AND the face allowlist a `type@face` grant
// compiles to ([ReadQueryResult.Faces]). It is [Request.ReadableFacesMany]
// asked about one face, so the two cannot disagree.
func (r *Request) PermitsReadFace(
	ctx context.Context, entityType, entityID string, face entity.Face,
) (bool, error) {
	vs, err := r.ReadableFacesMany(ctx, entityType, []string{entityID})
	if err != nil {
		return false, err
	}
	return vs.For(entityID).Contains(face), nil
}

// Principal returns the principal bound at construction. Helper for
// audit attribution; callers that already have the principal in their
// own ctx don't need this.
//
// Cloned so a caller cannot reach the roles backing array this Request is
// still authorizing against.
func (r *Request) Principal() principal.Principal { return r.principal.Clone() }

// ctxKey is the unexported type for context.WithValue. Required by
// the std-lib contract that context keys are not bare strings.
type ctxKey struct{}

// WithRequest attaches r to ctx. Every request edge does this once, after
// resolving the principal; downstream evaluation (the write gate, the
// visibility readers, the affordance resolver) reads it back with
// [Declarative.RequestFrom] and fails closed when it is missing.
//
// When ctx already carries a Request, the latest one wins; this is
// the right behavior for nested handlers (rare today).
func WithRequest(ctx context.Context, r *Request) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext returns the Request previously attached via
// [WithRequest], or nil when no Request is attached. It does not check
// that the Request belongs to a particular Declarative or to ctx's
// principal; evaluation paths use [Declarative.RequestFrom], which does.
func FromContext(ctx context.Context) *Request {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(ctxKey{}).(*Request)
	return r
}

// isUnstamped reports whether the principal looks like a default /
// missing-stamp value. The acl package treats "" and "unknown" as
// equivalent — the principal package's SystemUser() default is the
// literal "unknown", and code that bypasses From(ctx) may construct
// Principal{User: ""}; both indicate an entry point that forgot to
// stamp identity.
func isUnstamped(p principal.Principal) bool {
	if isBlankOrUnknown(p.User) {
		return true
	}
	if isBlankOrUnknown(p.Tool) {
		return true
	}
	return false
}

func isBlankOrUnknown(s string) bool {
	t := strings.TrimSpace(s)
	return t == "" || t == "unknown"
}

// PermitsWorld reports whether this Request's principal may read the named
// world (design doc §8.1, §4.4). It is the gate a caller must pass BEFORE
// constructing a world-resolved reader — the per-entity visibility gate
// still runs after, on whatever the resolved world contains.
//
// Semantics:
//
//   - The DEFAULT world (name "" or "default") is permitted by any ordinary
//     read grant, so an existing acl.yaml keeps meaning what it meant.
//   - Every other world must be named by a `read: [world:X]` grant.
//   - The client ceiling is applied twice, deliberately: through roleFor
//     (which clamps the role's world list) and then through permitsWorld
//     (which is the only place a denial of the DEFAULT world can be
//     expressed). See [compiledCeiling.permitsWorld].
//
// # Why this returns an error
//
// A world grant is a READ capability, and the read paths in this package
// carry errors on purpose ([Request.PermitsRead], [Request.ReadableFacesMany],
// visibility's listPushdown). Checking a world grant conferred through a
// relation probes the graph, and a store failure there must not read as
// "no" for a principal who genuinely holds the grant.
//
// That matters because a denial must render as an EMPTY result rather than
// a 403 (a 403 would be an existence oracle for the world's contents). So
// without an error channel a store outage becomes a silently empty page
// with no operator signal. Callers map: false+nil → empty, err → 500
// (RR-4TFZNL).
//
// The membership closure is resolved before the Request exists, and a walk
// failure there is an error at the edge ([IdentityResolver.ResolveDirect]),
// so the global roles read here are never partial. The error this method
// returns comes from the per-subject probes in permitsWorldThroughRelations.
func (r *Request) PermitsWorld(ctx context.Context, world string) (bool, error) {
	if !r.ceiling.permitsWorld(world) {
		return false, nil
	}
	globals := r.Globals()
	for _, a := range globals.Attributions {
		// roleFor, never policy.Roles[...] — it is THE clamp point, and a
		// direct lookup here would bypass the client ceiling entirely
		// (pinned by TestNoDirectRoleLookupInEvaluationPaths).
		role, ok := r.roleFor(a.Role)
		if !ok {
			continue
		}
		if roleGrantsWorldRead(role, world, r.d.policy.DefaultWorld()) {
			return true, nil
		}
	}
	// A world is a GLOBAL lens, so a world grant on a relation-conferred role
	// is honored globally: holding the role through any relation to any
	// entity opens the world, and the per-entity and per-face gates then
	// decide what it shows. That is the honest reading of "may this reader
	// see drafts at all" — the world is a convenience filter, not a
	// per-entity right — and it is what the load-time refusal for an ungated
	// conferring relation defends against (membershiprefusal.go).
	return r.permitsWorldThroughRelations(ctx, globals.Members, world)
}

// permitsWorldThroughRelations reports whether any role conferred by a
// relation the principal (or a group it belongs to) holds ANYWHERE grants
// world. The graph lookup errors propagate: a partial answer here would
// silently deny a principal who holds the grant.
func (r *Request) permitsWorldThroughRelations(ctx context.Context, members []string, world string) (bool, error) {
	rels := make([]string, 0, len(r.d.policy.RoleRelations))
	for rel := range r.d.policy.RoleRelations {
		rels = append(rels, rel)
	}
	slices.Sort(rels)
	for _, rel := range rels {
		role, ok := r.roleFor(r.d.policy.RoleRelations[rel].Confers)
		if !ok || !roleGrantsWorldRead(role, world, r.d.policy.DefaultWorld()) {
			continue
		}
		for _, m := range members {
			targets, err := r.d.graph.OutgoingRelations(ctx, m, rel)
			if err != nil {
				return false, err
			}
			if len(targets) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}
