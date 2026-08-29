// Package tenant resolves a verified organization to the storage it — and only
// it — may reach.
//
// # Why this package exists
//
// rela's ACL is union semantics with no deny primitive, so "deny unless the
// principal's org matches the row's org" cannot be written as a role rule. That
// is not an oversight to be patched: it means tenant isolation must come from
// somewhere other than the ACL.
//
// It comes from the store handle. Under the schema-per-tenant design
// (RES-D54281) each tenant's data lives in its own PostgreSQL schema, and a
// tenant's store is a connection pool pinned to that schema by `search_path`.
// A request resolved to tenant T therefore *cannot* address tenant U's rows:
// the attempt is `relation "entities" does not exist`, enforced by PostgreSQL,
// not a missing `WHERE` clause a reviewer has to spot. The ACL keeps doing what
// it already does, within a tenant.
//
// This package is the lookup that decides which handle a request gets.
//
// # Fail closed
//
// The requirement is one sentence — no cross-tenant data leaks — and every
// ambiguous case resolves the same way: no tenant, no store, no request. An
// unknown org, an absent org, and a malformed tenant record are all errors, and
// never a zero value or a default store.
//
// That last clause is deliberate. rela already carries fail-open traps where a
// zero value means "allow" — a zero `ReadQueryResult` aliases `AllowAll`, and
// `nopReadGate.HoldsPermission` returns true unconditionally. A resolver
// returning a zero-valued store on an unknown org would be the worst member of
// that family: it would not widen permissions inside a tenant, it would hand
// one tenant another tenant's database. So [Resolver] returns an error, and
// callers must not treat a failed resolution as a reason to fall back.
//
// # What lives here and what does not
//
// Resolution and per-tenant store acquisition live here. Provisioning
// (creating and migrating a new tenant's schema, TKT-TNPRV8) and erasure
// (dropping it, TKT-TNERAS) deliberately do not: this package resolves tenants
// that already exist and denies ones that do not. Nothing in it issues DDL.
package tenant

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrUnknownTenant reports that an org is not a tenant of this deployment.
//
// Distinguished from a lookup failure on purpose. Both deny the request — see
// the package doc — but they are different operational events: an unknown org
// is routine (a stale session, or an org awaiting provisioning), whereas a
// lookup that broke is an outage. Callers that log them identically will not be
// able to tell a quiet afternoon from a failing tenant map.
var ErrUnknownTenant = errors.New("tenant: unknown organization")

// schemaNamePattern constrains a tenant's PostgreSQL schema name.
//
// Schema names are a trust boundary. Today they come from an operator-authored
// file, which is inside the boundary, but they are interpolated into DDL and
// into a connection's `search_path`, and the moment provisioning derives one
// from an `org_id` claim (TKT-TNPRV8) the input becomes external. Validating
// now — where the rule is cheap and the file is small — means that change is a
// new caller rather than a new attack surface.
//
// The character class is deliberately narrower than PostgreSQL's: lowercase
// only, so a name cannot differ from another only by case-folding; no leading
// digit, so it is a bare identifier needing no quoting; and capped at 31
// characters, comfortably inside PostgreSQL's 63-byte NAMEDATALEN so a name can
// never be silently truncated into a collision with another tenant's.
var schemaNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)

// Tenant is one organization's storage location.
//
// The DSN is the whole point of the type: every deployment tier RES-D54281
// contemplates — one schema per tenant on a shared cluster today, a dedicated
// database for a tenant that outgrows it, a different cluster once tenants
// shard — is a different DSN and nothing else. Keeping storage topology behind
// this one string is what makes those tiers a config change rather than a
// redesign.
type Tenant struct {
	// OrgID is the verified `org_id` claim this tenant is keyed by. It is the
	// entire interface between rela and the identity provider: Pratique is the
	// authority on org identity, rela is the authority on where an org's data
	// lives, and exactly one stable string joins them.
	OrgID string

	// Schema is the PostgreSQL schema holding this tenant's data. Recorded
	// separately from the DSN because provisioning and erasure need to name it
	// as an identifier, not just connect through it.
	Schema string

	// DSN connects to Schema. For the shared-cluster tier this is the
	// deployment's base DSN with `search_path` pinned; for a promoted tenant it
	// may point at another database or another host entirely.
	//
	// It carries a password, so it must never reach a command line — the
	// invariant TKT-1J5KEV established for every DSN in rela.
	DSN string
}

// validate rejects a tenant record that could not safely be used.
//
// Called at load rather than at first request. A tenant map is small,
// operator-authored, and read once, so there is no reason for a typo to surface
// as a failed request hours later when it can surface as a failed boot.
func (t Tenant) validate() error {
	if t.OrgID == "" {
		return errors.New("org_id is required")
	}
	if !schemaNamePattern.MatchString(t.Schema) {
		return fmt.Errorf("schema %q must match %s", t.Schema, schemaNamePattern)
	}
	if t.DSN == "" {
		return errors.New("dsn is required")
	}
	return nil
}

// Resolver maps a verified organization to its storage.
//
// One method, because RES-D54281 requires the lookup to be a single seam: keep
// it true and "where the tenant map lives" stays an implementation choice. The
// file-backed [MapResolver] is the current implementation; a control-schema
// table becomes the natural one once provisioning needs to add a tenant without
// a deploy.
//
// Implementations must return [ErrUnknownTenant] for an org they do not know,
// and must never return a usable [Tenant] alongside an error.
type Resolver interface {
	Resolve(orgID string) (Tenant, error)
}

// MapResolver resolves from an in-memory table built at load.
//
// Immutable after construction, hence safe for concurrent use without a lock
// and without a cache: the map *is* the cache. That is a property of the
// current file-backed source, not of the [Resolver] interface — a DB-backed
// implementation will need both, which is why invalidation is the interface's
// problem to solve later and not a shape imposed on it now.
type MapResolver struct {
	byOrg map[string]Tenant
}

// NewMapResolver builds a resolver over the given tenants, rejecting a table
// that cannot be safely served.
//
// Two orgs mapping to one schema is refused, and it is the check worth
// understanding: it is a cross-tenant data leak spelled as a typo. Nothing
// downstream would report it — both orgs would resolve, both would connect,
// both would read and write the same rows, and the symptom would be one
// tenant seeing another's data with every layer behaving exactly as designed.
// Duplicate org IDs are refused for the adjacent reason: whichever entry lost
// would be silently ignored, so a well-meant edit could point an org at the
// wrong schema.
func NewMapResolver(tenants []Tenant) (*MapResolver, error) {
	byOrg := make(map[string]Tenant, len(tenants))
	bySchema := make(map[string]string, len(tenants))
	for i, t := range tenants {
		if err := t.validate(); err != nil {
			return nil, fmt.Errorf("tenant %d: %w", i, err)
		}
		if _, dup := byOrg[t.OrgID]; dup {
			return nil, fmt.Errorf("tenant %d: duplicate org_id %q", i, t.OrgID)
		}
		if other, dup := bySchema[t.Schema]; dup {
			return nil, fmt.Errorf(
				"tenant %d: org_id %q and %q both map to schema %q, which would "+
					"share one tenant's data with the other",
				i, t.OrgID, other, t.Schema)
		}
		byOrg[t.OrgID] = t
		bySchema[t.Schema] = t.OrgID
	}
	return &MapResolver{byOrg: byOrg}, nil
}

// Resolve returns the tenant for orgID, or [ErrUnknownTenant].
//
// An empty orgID is unknown rather than a distinct error. A principal without
// an org is either unverified or came from an issuer that asserts no org;
// either way it addresses no tenant, and giving that case its own error would
// invite a caller to handle it separately — which is the shape a fallback grows
// out of.
func (r *MapResolver) Resolve(orgID string) (Tenant, error) {
	t, ok := r.byOrg[orgID]
	if !ok {
		return Tenant{}, fmt.Errorf("%w: %q", ErrUnknownTenant, orgID)
	}
	return t, nil
}

// Len reports how many tenants this resolver knows. Exposed so a host can log
// its tenant count at boot and so an operator can confirm the file it deployed
// is the file that loaded.
func (r *MapResolver) Len() int { return len(r.byOrg) }
