package acl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
//     User="" / User="unknown" or Tool="" / Tool="unknown" fails the
//     [Declarative.ForPrincipal] check; the deny surfaces as RuleKind="role-grant"
//     with a Reason that names ErrUnstampedPrincipal.
type Declarative struct {
	policy          *Policy
	graph           Graph              // required: NewDeclarative rejects nil
	graphQueryer    store.GraphQueryer // required: needed by Request.PermitsRead / PermitsReadMany
	principalLookup PrincipalLookup    // optional: required only when principal_property lookup is enabled

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
// [NewStorePrincipalLookup] over the store. When the policy enables the
// lookup but no PrincipalLookup was supplied, [NewDeclarative] fails — a
// silent no-op would degrade every authenticated user to the raw
// principal without any signal (constructors-reject-nil rule).
func WithPrincipalLookup(l PrincipalLookup) DeclarativeOption {
	return func(d *Declarative) { d.principalLookup = l }
}

// NewDeclarative wraps a [Policy] + [Graph] + [store.GraphQueryer] as
// an [ACL]. The first three must be non-nil:
//
//   - Policy is the static role / assignment definitions.
//   - Graph supplies the read-side access the resolver needs for
//     member-of walks and ancestor probes used by AuthorizeWrite.
//   - GraphQueryer supplies [store.MatchingIDs] execution used by
//     [Request.PermitsRead] / [Request.PermitsReadMany] for per-entity
//     read gating.
//
// Optional collaborators are supplied via [DeclarativeOption]. When the
// policy enables `principal_property` lookup, a [PrincipalLookup] MUST be
// supplied via [WithPrincipalLookup] or construction fails.
//
// Tests that don't exercise group expansion can pass [NullGraph];
// tests that don't exercise read gating can pass [NullGraphQueryer]
// (returns false for every id probe). Production wiring (appbuild)
// passes the store as both Graph (via [NewStoreGraph]) and as the
// GraphQueryer, and supplies [WithPrincipalLookup].
func NewDeclarative(p *Policy, g Graph, gq store.GraphQueryer, opts ...DeclarativeOption) (*Declarative, error) {
	if p == nil {
		return nil, errors.New("acl: NewDeclarative: policy must be non-nil")
	}
	if g == nil {
		return nil, errors.New("acl: NewDeclarative: graph must be non-nil")
	}
	if gq == nil {
		return nil, errors.New("acl: NewDeclarative: graphQueryer must be non-nil")
	}
	d := &Declarative{policy: p, graph: g, graphQueryer: gq}
	for _, opt := range opts {
		opt(d)
	}
	if p.principalPropertyLookupEnabled() && d.principalLookup == nil {
		return nil, errors.New("acl: NewDeclarative: policy enables principal_property lookup " +
			"but no PrincipalLookup was supplied (use WithPrincipalLookup)")
	}
	return d, nil
}

// ResolvePrincipal maps a raw principal identifier (e.g. the value of the
// `X-Forwarded-User` header) to a user entity ID via the policy's
// `principal_property` lookup. It returns:
//
//   - ("", nil)  when the lookup is disabled, rawUser is blank, or no
//     entity matches — the caller keeps the raw principal.
//   - (id, nil)  on exactly one match — the caller substitutes id.
//   - ("", err)  when the lookup errored OR more than one entity matched
//     (ambiguous natural key). The caller keeps the raw principal and
//     logs; ambiguity is a data-integrity failure the resolver refuses
//     to guess through.
//
// Substitution is performed by the wiring layer (the data-entry
// attachACLRequest middleware) so the resolved ID reaches both the ACL
// walk and the audit log; the resolver itself never mutates ctx.
//
// **Scope limitation — data-entry only (deliberate).** This method is
// wired into exactly one caller: the `/api/` data-entry middleware. The
// CLI, MCP, scheduler, and desktop entry points stamp their principal via
// [principal.With] but do NOT call ResolvePrincipal, so a write over those
// transports authorizes against the RAW principal, never the resolved
// entity ID. Consequence: an `assignments`/`member-of`/`role_relations`
// grant keyed on the resolved entity ID (e.g. `PERS-JV`) applies to
// data-entry writes but not to the same human's CLI/MCP writes — those
// fall back to whatever the raw identifier is assigned (typically
// `everyone` only). This is fail-CLOSED (an unresolved principal loses
// grants, never gains them — see the multi-match/no-match handling below),
// and it matches the intended deployment: only the reverse-proxy
// (`X-Forwarded-User`) path carries an identity worth resolving. If a
// future entry point needs the same resolution, wire this in there too —
// don't assume one `acl.yaml` means the same thing on every transport.
func (d *Declarative) ResolvePrincipal(ctx context.Context, rawUser string) (string, error) {
	if !d.policy.principalPropertyLookupEnabled() || strings.TrimSpace(rawUser) == "" {
		return "", nil
	}
	if d.principalLookup == nil {
		// Defense in depth: NewDeclarative rejects this combination, but a
		// future direct construction path must not silently over-resolve.
		return "", errors.New("acl: ResolvePrincipal: lookup enabled but no PrincipalLookup configured")
	}
	ids, err := d.principalLookup.LookupEntityByProperty(
		ctx, d.policy.UserEntityType, d.policy.PrincipalProperty, rawUser)
	if err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("acl: principal %q matches %d %s entities on %q (ambiguous natural key)",
			rawUser, len(ids), d.policy.UserEntityType, d.policy.PrincipalProperty)
	}
}

// Policy returns the policy this Declarative was constructed with.
// Exposed so downstream consumers (chiefly the affordance resolver)
// can read role/grant definitions from the same source the resolver
// uses for role attribution — eliminating the mismatched-pair foot-
// cannon where a caller might pass policyA to the affordance resolver
// while wiring a Declarative built from policyB (RR-WTLD).
//
// **The returned *Policy must be treated as immutable** (RR-9GN3).
// The resolver caches no policy state; every AuthorizeWrite reads
// fields back through this pointer. Mutating Roles, Assignments,
// or any nested map invalidates the resolver's safety guarantees
// from the next call onward, including the unstamped-principal
// check and the delegate-X gates. Callers that need a mutated
// policy build a fresh Declarative with [NewDeclarative].
func (d *Declarative) Policy() *Policy { return d.policy }

// AuthorizeWrite implements [ACL.AuthorizeWrite]. Opens a Request for
// the principal carried on ctx, then delegates to
// [Request.AuthorizeWrite] which dispatches on Subject variant. An
// unstamped principal short-circuits to a structured deny.
func (d *Declarative) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
	// unmatched_principal gate: a verified principal that resolved to no user
	// entity (flagged by the data-entry middleware) is denied its writes when
	// the policy is `reject`. This runs before role evaluation because it is a
	// deployment posture ("unknown identities don't mutate"), not a role
	// decision — and because it is the single write-authz point every entity
	// write reaches, so it covers CRUD, sync, action, and attachment writes
	// uniformly (TKT-0C3II2).
	if UnmatchedVerifiedFrom(ctx) {
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
			// TKT-ANUJDS): a successful provision re-stamps ctx to the resolved
			// entity and CLEARS this flag (acl.WithMatchedVerified), so the write
			// never reaches here still-flagged. Arriving here under provision means
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

	r, err := d.ForPrincipal(principal.From(ctx))
	if err != nil {
		return Decision{
			Allow:    false,
			RuleKind: "role-grant",
			RuleID:   "-",
			Reason:   fmt.Sprintf("acl.ForPrincipal: %v", err),
		}
	}
	return r.AuthorizeWrite(ctx, req)
}

// (roleGrantsWrite removed in TKT-4LQMWP — write grants are now per-verb;
// see grantsVerb in policy.go, dispatched by decideFromAttrs.)
