package acl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Declarative is the policy-driven [ACL] implementation. It composes
// a [Policy] (loaded from `acl.yaml`) with the [principal.Principal]
// carried on each request's context to answer [ACL.AuthorizeWrite].
//
// Every [WriteRequest] carries a [Subject] (sum of [EntitySubject] /
// [RelationSubject]) — the resolver dispatches on the variant and
// runs the full path: group expansion via member-of, containment
// inheritance through `inherit_roles_through`, and typed-Source
// attribution for the audit log.
//
// Semantics:
//
//   - **Union semantics.** Any role granting write on the target
//     entity type → allow. The returned RuleID names the first role
//     that matched, for debuggability.
//   - **Delegate-X tamper resistance.** Writes to a relation type
//     listed in [Policy.RoleRelations] require the writer to hold the
//     declared permission. See [RoleRelationDef.RequiresPermission].
//   - **Unstamped principals are hard-denied.** A principal with
//     User="" / User="unknown" or Tool="" / Tool="unknown" fails
//     identity resolution, so no [Request] exists for it and every
//     evaluation path fails closed.
//
// # Identity resolution is a separate step
//
// The Declarative evaluates; it does not resolve who the caller is. The
// request edge (HTTP middleware, MCP call, CLI command, scheduler run)
// resolves the principal once with [Declarative.IdentityResolver], opens a
// [Request] with [Declarative.ForIdentity], and attaches it with
// [WithRequest]. Evaluation paths read that Request from ctx. When none is
// attached they fail closed with [ErrNoIdentity]; they never resolve the
// principal themselves.
type Declarative struct {
	policy          *Policy
	graph           Graph           // required: NewDeclarative rejects nil
	faceMatcher     FaceMatcher     // required: needed by Request.ReadableFacesMany
	principalLookup PrincipalLookup // optional: required only when principal_property lookup is enabled

	// identities resolves principals for this Declarative's policy and
	// store. Built by NewDeclarative; evaluator code never calls it (pinned
	// by TestEvaluatorDoesNotResolveIdentity).
	identities *IdentityResolver

	// provisionWarn logs "provision not yet implemented" at most once per
	// Declarative (i.e. per policy load), so a `unmatched_principal: provision`
	// deployment isn't silently treated as anonymous but also isn't a per-request
	// log flood. Remove when provision lands (its own ticket).
	provisionWarn sync.Once
}

// DeclarativeOption configures optional [Declarative] collaborators.
type DeclarativeOption func(*Declarative)

// WithPrincipalLookup supplies the [PrincipalLookup] used to resolve the
// raw principal to a user entity ID when the policy sets both
// `user_entity_type` and `principal_property`. Wiring (appbuild) passes
// aclstore.PrincipalLookup over the store. When the policy enables the
// lookup but no PrincipalLookup was supplied, [NewDeclarative] fails — a
// silent no-op would degrade every authenticated user to the raw
// principal without any signal (constructors-reject-nil rule).
func WithPrincipalLookup(l PrincipalLookup) DeclarativeOption {
	return func(d *Declarative) { d.principalLookup = l }
}

// NewDeclarative wraps a [Policy] + [Graph] + [FaceMatcher] as
// an [ACL]. The first three must be non-nil:
//
//   - Policy is the static role / assignment definitions.
//   - Graph supplies the per-subject probes the evaluator needs (local
//     roles, containment ancestors, world grants through relations), and
//     the membership reads of the [IdentityResolver] it builds.
//   - FaceMatcher runs the per-row read verdict for
//     [Request.ReadableFacesMany] and the gates built on it.
//
// Optional collaborators are supplied via [DeclarativeOption]. When the
// policy enables `principal_property` lookup, a [PrincipalLookup] MUST be
// supplied via [WithPrincipalLookup] or construction fails.
//
// Tests that don't exercise group expansion can pass [NullGraph];
// tests that don't exercise read gating can pass [NullGraphQueryer]
// (matches no row). Production wiring (appbuild) passes the store as both
// Graph (via aclstore.Graph) and as the FaceMatcher, and supplies
// [WithPrincipalLookup].
func NewDeclarative(p *Policy, g Graph, fm FaceMatcher, opts ...DeclarativeOption) (*Declarative, error) {
	if p == nil {
		return nil, errors.New("acl: NewDeclarative: policy must be non-nil")
	}
	if g == nil {
		return nil, errors.New("acl: NewDeclarative: graph must be non-nil")
	}
	if fm == nil {
		return nil, errors.New("acl: NewDeclarative: faceMatcher must be non-nil")
	}
	// Split world grants here as well as in [Policy.Validate]. Validate is
	// the LOAD chokepoint; this constructor is the SERVE chokepoint, and a
	// policy can reach the second without passing the first — NewDeclarative
	// does not call Validate, and callers construct a Policy directly
	// (internal/visibility/visibilitytest, appbuild.NewFromCollaborators,
	// every test literal). Left unsplit, `world:published` stays inline in
	// Read where roleGrantsRead never matches it and the client ceiling
	// mis-clamps it — silently, and in the fail-open direction under a deny
	// ceiling (RR-LWE222).
	//
	// normalizeWorldGrants is idempotent, so paying it twice is free — a
	// policy that came through Validate has no prefixed entries left in
	// Read and this is a no-op scan.
	//
	// It can only fail on a malformed grant (`world:` with no name, or
	// `world:*`), which for a Validate-loaded policy is already impossible.
	// Failing construction is right for the paths that skip Validate: the
	// alternative is serving reads against a grant nobody could interpret.
	if err := p.normalizeWorldGrants(); err != nil {
		return nil, fmt.Errorf("acl: NewDeclarative: %w", err)
	}
	// State-grant GRAMMAR is deliberately NOT re-checked here, unlike the
	// world split above. The split must run because a policy that skipped
	// Validate would otherwise serve reads with an uninterpretable grant
	// sitting in the wrong field. A malformed state grant has no such
	// failure mode: GrantsVerbOnState treats an unparseable entry as
	// granting nothing, so the policy is already fail-closed and Validate
	// remains the place that says so with a useful message.
	d := &Declarative{policy: p, graph: g, faceMatcher: fm}
	for _, opt := range opts {
		opt(d)
	}
	if p.principalPropertyLookupEnabled() && d.principalLookup == nil {
		return nil, errors.New("acl: NewDeclarative: policy enables principal_property lookup " +
			"but no PrincipalLookup was supplied (use WithPrincipalLookup)")
	}
	d.identities = newIdentityResolver(p, g, d.principalLookup)
	return d, nil
}

// IdentityResolver returns the resolver for this Declarative's policy and
// store. Request edges use it to resolve the principal once per operation;
// [Declarative.ForIdentity] accepts only identities it produced.
func (d *Declarative) IdentityResolver() *IdentityResolver { return d.identities }

// Policy returns the policy this Declarative was constructed with.
// Exposed so downstream consumers (chiefly the affordance resolver)
// can read role/grant definitions from the same source the resolver
// uses for role attribution — eliminating the mismatched-pair foot-
// cannon where a caller might pass policyA to the affordance resolver
// while wiring a Declarative built from policyB (RR-WTLD).
//
// **The returned *Policy must be treated as immutable** (RR-9GN3).
// The resolver caches no policy state; every AuthorizeWrite reads
// fields back through this face. Mutating Roles, Assignments,
// or any nested map invalidates the resolver's safety guarantees
// from the next call onward, including the unstamped-principal
// check and the delegate-X gates. Callers that need a mutated
// policy build a fresh Declarative with [NewDeclarative].
// Nil-receiver safe: a typed-nil *Declarative in an ACL interface is not nil,
// so callers that guard with `d != nil` are easy to get wrong, and the failure
// is a panic rather than a bad answer. Returning a nil *Policy lets the
// predicates on Policy (each of which handles a nil receiver) give the same
// "no policy" answer they would for an absent one.
func (d *Declarative) Policy() *Policy {
	if d == nil {
		return nil
	}
	return d.policy
}

// AuthorizeWrite implements [ACL.AuthorizeWrite]. It evaluates with the
// [Request] the operation's edge attached to ctx (see [Declarative.RequestFrom])
// and delegates to [Request.AuthorizeWrite], which dispatches on Subject
// variant. A ctx with no usable Request is a structured deny.
//
//archplan:guard=authz reason="checks the write against the ACL policy"
func (d *Declarative) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
	r, err := d.RequestFrom(ctx)
	if err != nil {
		// The error text is fixed (sentinels only), so it is safe to return
		// to a caller; RequestFrom logs any principal ids.
		return Decision{
			Allow:    false,
			RuleKind: "role-grant",
			RuleID:   "-",
			Reason:   fmt.Sprintf("acl: no request scope: %v", err),
		}
	}
	// unmatched_principal gate: a verified principal that resolved to no user
	// entity (the edge marks it on the Request) is denied its writes when the
	// policy is `reject`. This runs before role evaluation because it is a
	// deployment posture ("unknown identities don't mutate"), not a role
	// decision — and because it is the single write-authz point every entity
	// write reaches, so it covers CRUD, sync, action, and attachment writes
	// uniformly (TKT-0C3II2).
	if r.UnmatchedVerified() {
		switch d.policy.effectiveUnmatchedPrincipal() {
		case UnmatchedReject:
			return Decision{
				Allow:    false,
				RuleKind: "unmatched-principal",
				RuleID:   UnmatchedReject,
				Reason:   "verified principal resolves to no user entity; writes are rejected",
			}
		case UnmatchedProvision:
			// Provision is implemented at the data-entry write seam (maybeProvision,
			// TKT-ANUJDS): a successful provision binds a new Request on the
			// provisioned identity, which is not unmatched, so the write never
			// reaches here still-flagged. Arriving here under provision means
			// provisioning did NOT fire or failed — a non-data-entry transport (this
			// gate is transport-agnostic), or a create/re-resolve error the seam
			// logged. Fall through to normal role evaluation (the principal's
			// asserted roles still apply, TKT-RP3X3Q); warn once so a stuck
			// provision deployment is visible without a per-request flood.
			d.provisionWarn.Do(func() {
				slog.Warn("acl: unmatched_principal: provision reached the write-authz gate " +
					"still unmatched; provisioning did not fire (non-data-entry transport) or " +
					"failed (see earlier provision warnings) — treating as anonymous")
			})
		}
	}
	return r.AuthorizeWrite(ctx, req)
}

// PermitsReadFace reports whether the principal on ctx may read the row of
// entityID at face, resolving the [Request] the way [Declarative.AuthorizeWrite]
// does. It is for a write whose response must depend only on what the caller
// can read, such as a family rename or a face delete, and which therefore may
// not judge by the raw stored family. An unresolvable principal is an error;
// callers fail closed.
func (d *Declarative) PermitsReadFace(
	ctx context.Context, entityType, entityID string, face entity.Face,
) (bool, error) {
	r, err := d.RequestFrom(ctx)
	if err != nil {
		return false, err
	}
	return r.PermitsReadFace(ctx, entityType, entityID, face)
}

// RequestFrom returns the per-operation [Request] the operation's edge
// attached to ctx via [WithRequest]. It fails closed:
//
//   - no Request on ctx → [ErrNoIdentity];
//   - a Request from another Declarative → [ErrForeignIdentity], because it
//     would evaluate another tenant's or another load's policy;
//   - a Request bound to a principal other than ctx's →
//     [ErrIdentityMismatch], because it would evaluate the wrong identity
//     (the provisioning seam re-stamps ctx and must attach a new Request
//     with it). The two principal ids are logged, not put in the error,
//     because the error text can reach a client.
//
// It never resolves the principal itself; resolution is the edge's job. The
// write gate and the read paths outside this package (visibility,
// affordances) all use it, so they fail closed the same way.
//
// Equal compares the whole principal — identity AND the verified claims
// (type, scopes, roles) the Request's client ceiling was compiled from — so a
// scope-attenuated client can never borrow a wider scope bound to the same
// user by an earlier layer.
func (d *Declarative) RequestFrom(ctx context.Context) (*Request, error) {
	r := FromContext(ctx)
	if r == nil {
		return nil, ErrNoIdentity
	}
	if r.d != d {
		return nil, ErrForeignIdentity
	}
	if p := principal.From(ctx); !r.principal.Equal(p) {
		slog.WarnContext(ctx, "acl: attached request is bound to another principal",
			"request_user", r.principal.User, "ctx_user", p.User)
		return nil, ErrIdentityMismatch
	}
	return r, nil
}

// (roleGrantsWrite removed in TKT-4LQMWP — write grants are now per-verb;
// see grantsVerb in policy.go, dispatched by decideFromAttrs.)
