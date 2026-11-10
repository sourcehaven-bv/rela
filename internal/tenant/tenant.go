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
// # Where a tenant lives
//
// Pratique's verified `org_id` says which org a request belongs to; rela
// derives where that org's data lives (TKT-HF5KE12). No operator file maps
// orgs to schemas. [SchemaFor] derives the schema name from the org id, the
// DSN is the deployment's base DSN with `search_path` pinned to it, and a
// small control table, `rela_control.tenants`, records which orgs exist and
// whether each is served. [ClusterResolver] reads it; [Provisioner] is the
// only writer.
//
// # What lives here and what does not
//
// Resolution ([ClusterResolver]), per-tenant store acquisition ([Registry])
// and the lifecycle writes ([Provisioner]: create, suspend, resume, erase)
// live here. The request path receives only a [Resolver], so it can never
// create or erase a tenant. Automatic provisioning from the IdP webhook
// (TKT-TNPRV8) is not built yet; until then an operator creates a tenant
// with `rela-server tenant create`.
package tenant

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrUnknownTenant reports that an org is not a tenant of this deployment.
//
// Distinguished from a lookup failure on purpose. Both deny the request — see
// the package doc — but they are different operational events: an unknown org
// is routine (a stale session, or an org awaiting provisioning), whereas a
// lookup that broke is an outage. Callers that log them identically will not be
// able to tell a quiet afternoon from a failing control table.
var ErrUnknownTenant = errors.New("tenant: unknown organization")

// ErrTenantInactive marks an [ErrUnknownTenant] for an org that is a tenant
// but is not served now: suspended, or not yet provisioned. Requests are
// refused alike, but an inbound event for such an org must not be dropped:
// it applies once the tenant is served again, so the sender has to retry.
// An erasing tenant is not inactive: its data is being destroyed, so its
// events have nothing to apply to.
var ErrTenantInactive = errors.New("tenant: organization is not served now")

// ErrSchemaCollision reports that an org's derived schema is recorded for a
// different org. Two org ids whose hashes share a prefix would otherwise share
// one schema, which is a cross-tenant leak, so the second org is refused and
// never served. It is an integrity failure, not a routine refusal: it maps to
// 503 and is logged as an error.
var ErrSchemaCollision = errors.New("tenant: the derived schema belongs to another organization")

// schemaNamePattern constrains a tenant's PostgreSQL schema name.
//
// Schema names are a trust boundary: they are interpolated into DDL and into
// a connection's `search_path`. [SchemaFor] derives them from a hash, so the
// org id itself never reaches either, and the derived name is checked again
// against this pattern before use.
//
// The character class is deliberately narrower than PostgreSQL's: lowercase
// only, so a name cannot differ from another only by case-folding; no leading
// digit, so it is a bare identifier needing no quoting; and capped at 31
// characters, comfortably inside PostgreSQL's 63-byte NAMEDATALEN so a name can
// never be silently truncated into a collision with another tenant's.
var schemaNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)

// checkSchemaName refuses a schema name that does not match
// [schemaNamePattern] or that names a schema PostgreSQL or the deployment
// shares between tenants: `public`, `information_schema`, the `pg_` prefix
// and [ControlSchema].
//
// The pattern alone admits `public` and `information_schema`, and
// PostgreSQL reserves the `pg_` prefix. A tenant in `public` would be served
// from the schema every tenant's `search_path` falls back to, and erasing
// it would drop the `pg_trgm` extension search needs for every tenant.
func checkSchemaName(schema string) error {
	if !schemaNamePattern.MatchString(schema) {
		return fmt.Errorf("schema %q must match %s", schema, schemaNamePattern)
	}
	if schema == "public" || schema == "information_schema" || schema == ControlSchema ||
		strings.HasPrefix(schema, "pg_") {

		return fmt.Errorf("schema %q is shared by PostgreSQL or by every tenant, so no tenant may use it", schema)
	}
	return nil
}

// Tenant is one organization's storage location.
//
// The DSN is the whole point of the type: every deployment tier RES-D54281
// contemplates (one schema per tenant on a shared cluster today, a dedicated
// database for a tenant that outgrows it) is a different DSN and nothing
// else. A per-org override for a dedicated cluster will be a decorator around
// [ClusterResolver] that swaps the DSN, kept in the secret store.
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

	// DSN connects to Schema: the deployment's base DSN with `search_path`
	// pinned.
	//
	// It carries a password, so it must never reach a command line — the
	// invariant TKT-1J5KEV established for every DSN in rela.
	DSN string
}

// validate rejects a tenant record that could not safely be used.
func (t Tenant) validate() error {
	if t.OrgID == "" {
		return errors.New("org_id is required")
	}
	if err := checkSchemaName(t.Schema); err != nil {
		return err
	}
	if t.DSN == "" {
		return errors.New("dsn is required")
	}
	return nil
}

// Resolver maps a verified organization to its storage.
//
// One method, because RES-D54281 requires the lookup to be a single seam.
// [ClusterResolver] is the production implementation. The input is the
// verified org claim alone, not the principal.
//
// Implementations must return [ErrUnknownTenant] for an org they do not
// serve, and must never return a usable [Tenant] alongside an error. A
// failed lookup must not be reported as [ErrUnknownTenant]: that is a
// routine 403, while a broken lookup is an outage.
type Resolver interface {
	Resolve(ctx context.Context, orgID string) (Tenant, error)
}
