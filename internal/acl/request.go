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
	return &Request{d: d, principal: p}, nil
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

// Principal returns the principal bound at construction. Helper for
// audit attribution; callers that already have the principal in their
// own ctx don't need this.
func (r *Request) Principal() principal.Principal { return r.principal }

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
