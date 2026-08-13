package acl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ErrUnstampedPrincipal is the sentinel returned by Declarative.ForPrincipal
// when the principal carries User="" / User="unknown" or Tool="" /
// Tool="unknown". Code paths that forgot to stamp identity must fail
// loud rather than silently degrade.
var ErrUnstampedPrincipal = errors.New("acl: principal is unstamped (User or Tool is unknown)")

// GlobalRoles is the result of computing the global (no-entity) role
// set for a principal. Carries both the attributions and the
// transitively-walked member-of closure so a follow-up per-entity
// call can reuse the Members slice without re-walking.
type GlobalRoles struct {
	Attributions []RoleAttribution
	Members      []string
}

// Request is a per-request resolver scope. Constructed via
// Declarative.ForPrincipal; methods are not safe for concurrent use
// by multiple goroutines — open one Request per logical operation and
// let it go out of scope when the operation completes.
//
// Carries a memoised GlobalRoles so multiple per-entity calls on the
// same Request reuse the one member-of walk. The principal is bound
// at construction and never revalidated by Request methods.
type Request struct {
	d             *Declarative
	principal     principal.Principal
	globals       GlobalRoles
	globalsLoaded bool

	// ceiling is the client attenuation resolved from the principal's verified
	// principal_type + scope claims (TKT-IAC8TX). Computed once at
	// construction: it depends only on the bound principal and the immutable
	// policy, so there is nothing to invalidate and no reason to recompute it
	// per call. Inactive (and fully transparent) for an interactive user or any
	// principal whose type no baseline covers.
	ceiling compiledCeiling
}

// ForPrincipal opens a Request scope for `p`. Returns
// ErrUnstampedPrincipal if `p.User` or `p.Tool` is empty or "unknown".
// The Declarative is always constructed with a non-nil Graph (the
// [NewDeclarative] constructor rejects nil), so the resolver always
// has the read-side access it needs.
func (d *Declarative) ForPrincipal(p principal.Principal) (*Request, error) {
	if isUnstamped(p) {
		return nil, fmt.Errorf("%w: User=%q Tool=%q", ErrUnstampedPrincipal, p.User, p.Tool)
	}
	return &Request{
		d:         d,
		principal: p,
		ceiling:   d.policy.ceilingFor(p.PrincipalType(), p.Scopes()),
	}, nil
}

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

// Globals returns the principal's global role set, computing it on
// first call and caching for the lifetime of the Request. Subsequent
// calls reuse the cached value with no graph traffic.
func (r *Request) Globals(ctx context.Context) GlobalRoles {
	if !r.globalsLoaded {
		r.globals = r.computeGlobals(ctx)
		r.globalsLoaded = true
	}
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
		return r.Globals(ctx).Attributions
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
func (r *Request) ReadQuery(ctx context.Context, entityType string) ReadQueryResult {
	return r.readQuery(ctx, entityType)
}

// PermitsRead reports whether this Request's principal is permitted
// to read entityID of type entityType under the active policy. Used
// by the dataentry per-entity GET gate (and writes-to-hidden 404
// parity) to answer one-shot ACL questions without invoking a full
// list query.
//
// Semantics:
//
//   - AllowAll → (true, nil) immediately; existence/type are NOT
//     verified. Callers that need existence MUST follow up with
//     getEntity — this method answers "permits read", not "exists".
//   - DenyAll  → (false, nil) immediately.
//   - Query    → MatchingIDs with {entityID}; map[entityID] → result.
//
// Returns any backing error verbatim so the caller can map it to the
// right HTTP status (typically 500; context.Canceled → no response;
// context.DeadlineExceeded → 504).
func (r *Request) PermitsRead(ctx context.Context, entityType, entityID string) (bool, error) {
	m, err := r.PermitsReadMany(ctx, entityType, []string{entityID})
	if err != nil {
		return false, err
	}
	return m[entityID], nil
}

// PermitsReadMany returns a permissions map keyed by every input id
// (true = principal may read, false = denied) for the given type. Used
// by the dataentry include filter and any future batched gate. All
// input ids appear in the result map regardless of outcome.
//
// Semantics mirror [Request.PermitsRead]:
//
//   - AllowAll → every id maps to true.
//   - DenyAll  → empty map (every lookup returns false zero-value).
//   - Query    → store.MatchingIDs result, verbatim.
func (r *Request) PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error) {
	rqr := r.readQuery(ctx, entityType)
	switch {
	case rqr.AllowAll:
		m := make(map[string]bool, len(ids))
		for _, id := range ids {
			m[id] = true
		}
		return m, nil
	case rqr.DenyAll:
		return map[string]bool{}, nil
	case rqr.Query == nil:
		return nil, errors.New("acl: PermitsReadMany: readQuery returned zero ReadQueryResult")
	}
	return r.d.graphQueryer.MatchingIDs(ctx, *rqr.Query, ids)
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

// WithRequest attaches r to ctx so downstream resolvers (notably the
// affordance resolver) can reuse the same per-request scope —
// amortizing the member-of walk across every per-entity call in a
// list response (RR-JJYW). The dataentry list handler opens one
// Request at the top and threads the derived ctx through every
// FieldVerdicts / RelationVerdicts call.
//
// When ctx already carries a Request, the latest one wins; this is
// the right behavior for nested handlers (rare today).
func WithRequest(ctx context.Context, r *Request) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext returns the Request previously attached via
// [WithRequest], or nil when no Request is attached. The affordance
// resolver consults this in bindingFor; a nil return means "build a
// fresh Request for this call" (back-compat for callers that don't
// thread a Request).
func FromContext(ctx context.Context) *Request {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(ctxKey{}).(*Request)
	return r
}

// unmatchedVerifiedKey is the ctx key for the "verified-but-unmatched" flag.
type unmatchedVerifiedKey struct{}

// WithUnmatchedVerified marks ctx to record that the request's principal was
// CRYPTOGRAPHICALLY VERIFIED but resolved to no user entity (the
// principal_property lookup found no match).
//
// It is set by the data-entry middleware, which is the only layer that knows
// both facts: that a verified-JWT gate is the identity source (wiring state,
// not a per-principal marker — a JWT and a proxy-header principal are
// indistinguishable on the Principal), and that resolution found no entity. The
// flag lets [Declarative.AuthorizeWrite] — the single point every entity write
// funnels through — enforce `unmatched_principal: reject` uniformly across
// every data-entry write path (CRUD, sync, action, attachment) without the
// write layer learning about transports.
//
// A boolean fact on ctx, set on the read path; it performs no write and is
// gone when the request ends. CLI/MCP/scheduler/header requests never set it.
func WithUnmatchedVerified(ctx context.Context) context.Context {
	return context.WithValue(ctx, unmatchedVerifiedKey{}, true)
}

// UnmatchedVerifiedFrom reports whether ctx was marked by
// [WithUnmatchedVerified].
func UnmatchedVerifiedFrom(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(unmatchedVerifiedKey{}).(bool)
	return v
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
