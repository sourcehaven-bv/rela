package tenant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

// ControlSchema is the PostgreSQL schema that holds the tenant control
// table, `rela_control.tenants`.
//
// The table records, per derived schema, the org id it belongs to and its
// status. It lives in a schema of its own because every other place is
// wrong for it:
//
//   - Not in a tenant schema. The table is how a tenant is found, so it
//     cannot live inside one, and erasing that tenant would drop it.
//   - Not in `public`. Every tenant connection has `public` on its
//     `search_path`, so an unqualified name there is reachable from every
//     tenant. And [Erase] refuses while `public` holds a table, because
//     `public` is what a server that lost its tenant lock falls back to.
//   - In the same database as the tenants, reached through the base DSN in
//     $RELA_DATABASE_URL, so finding a tenant needs no second credential.
//
// No tenant's `search_path` names this schema, and [checkSchemaName]
// refuses it as a tenant schema.
const ControlSchema = "rela_control"

// Status is a tenant's lifecycle state in the control table. Only
// [StatusActive] is served; every other status fails closed.
type Status string

// The statuses a tenant moves through. Provisioning is set while `tenant
// create` builds the schema; erasing is one-way.
const (
	StatusProvisioning Status = "provisioning"
	StatusActive       Status = "active"
	StatusSuspended    Status = "suspended"
	StatusErasing      Status = "erasing"
)

// schemaPrefix starts every derived schema name, so a derived name can never
// be `public`, a `pg_` name or [ControlSchema].
const schemaPrefix = "org_"

// schemaHashChars is how many hex characters of the org id's SHA-256 the
// schema name keeps: 64 bits, so an accidental collision is negligible, and
// a name of 20 characters, inside [schemaNamePattern]'s cap. A collision is
// still detected and refused (see [ErrSchemaCollision]).
const schemaHashChars = 16

// maxOrgIDBytes bounds an org id the control table stores.
const maxOrgIDBytes = 256

// SchemaFor returns the schema that holds orgID's data: `org_` plus the first
// 16 hex characters of the SHA-256 of the org id. The name is checked
// against [schemaNamePattern] before it is returned, so an invalid derived
// name is refused rather than used. An empty org is [ErrUnknownTenant].
func SchemaFor(orgID string) (string, error) {
	if orgID == "" {
		return "", fmt.Errorf("%w: empty org_id", ErrUnknownTenant)
	}
	sum := sha256.Sum256([]byte(orgID))
	schema := schemaPrefix + hex.EncodeToString(sum[:])[:schemaHashChars]
	if err := checkSchemaName(schema); err != nil {
		return "", fmt.Errorf("tenant: derived schema for org_id %q: %w", orgID, err)
	}
	return schema, nil
}

// checkOrgID refuses an org id that should not be stored: empty, too long,
// not UTF-8, or holding control characters. Pratique's org ids are short
// opaque strings, so anything else is a mistake worth stopping at create.
func checkOrgID(orgID string) error {
	switch {
	case orgID == "":
		return errors.New("tenant: org_id is required")
	case len(orgID) > maxOrgIDBytes:
		return fmt.Errorf("tenant: org_id is longer than %d bytes", maxOrgIDBytes)
	case !utf8.ValidString(orgID):
		return errors.New("tenant: org_id is not valid UTF-8")
	}
	for _, r := range orgID {
		if unicode.IsControl(r) {
			return errors.New("tenant: org_id holds a control character")
		}
	}
	return nil
}

// Record is one row of the control table, plus whether its schema exists.
type Record struct {
	OrgID        string
	Schema       string
	Status       Status
	SchemaExists bool
}

// controlReader reads the control table. The postgres build reads it over a
// small pool; tests substitute a map.
type controlReader interface {
	// lookup returns the record for schema; found is false when there is
	// no row.
	lookup(ctx context.Context, schema string) (rec Record, found bool, err error)
	// ready reports an error when the control table cannot be read.
	ready(ctx context.Context) error
}

// ClusterResolver is the production [Resolver]. It derives an org's schema
// with [SchemaFor], reads the control table, and returns the base DSN with
// `search_path` pinned to the schema.
//
// It is read-only: lifecycle writes are on [Provisioner], which the request
// path never receives. It keeps no cache, so a suspension applies to the
// next request; the cost is one primary-key lookup per request.
type ClusterResolver struct {
	base    string
	control controlReader
	derive  func(string) (string, error)
	closer  io.Closer
}

// newClusterResolver builds a resolver over a control reader. closer may be
// nil.
func newClusterResolver(base string, control controlReader, closer io.Closer) *ClusterResolver {
	return &ClusterResolver{base: base, control: control, derive: SchemaFor, closer: closer}
}

// Resolve returns orgID's tenant when its control row is active and its
// schema exists.
//
// It refuses with [ErrUnknownTenant] an org with no row or with a status
// other than active, and with [ErrSchemaCollision] an org whose derived
// schema is recorded for another org. An active row whose schema is missing
// is an integrity failure, reported as neither, so it maps to 503.
func (r *ClusterResolver) Resolve(ctx context.Context, orgID string) (Tenant, error) {
	rec, err := r.Lookup(ctx, orgID)
	if err != nil {
		return Tenant{}, err
	}
	if rec.Status != StatusActive {
		return Tenant{}, fmt.Errorf("%w: org_id %q is %s", ErrUnknownTenant, orgID, rec.Status)
	}
	if !rec.SchemaExists {
		return Tenant{}, fmt.Errorf("tenant: org_id %q is active, but its schema %q does not exist", orgID, rec.Schema)
	}
	dsn, err := dsnForSchema(r.base, rec.Schema)
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant: org_id %q: %w", orgID, err)
	}
	return Tenant{OrgID: orgID, Schema: rec.Schema, DSN: dsn}, nil
}

// Lookup returns orgID's control record whatever its status, for an operator
// command that reports or changes it. It refuses as [ClusterResolver.Resolve]
// does an org with no row or a colliding one.
func (r *ClusterResolver) Lookup(ctx context.Context, orgID string) (Record, error) {
	schema, err := r.derive(orgID)
	if err != nil {
		return Record{}, err
	}
	rec, found, err := r.control.lookup(ctx, schema)
	if err != nil {
		// Never ErrUnknownTenant: a broken lookup is an outage.
		return Record{}, fmt.Errorf("tenant: look up org_id %q: %w", orgID, err)
	}
	if !found {
		return Record{}, fmt.Errorf("%w: %q", ErrUnknownTenant, orgID)
	}
	if rec.OrgID != orgID {
		return Record{}, fmt.Errorf("%w: org_id %q derives schema %q, which is recorded for another org",
			ErrSchemaCollision, orgID, schema)
	}
	return rec, nil
}

// Check reports an error when the control table cannot be read, for a host
// to refuse to start rather than refuse every request.
func (r *ClusterResolver) Check(ctx context.Context) error {
	return r.control.ready(ctx)
}

// Close releases the resolver's connections. Safe on a nil resolver.
func (r *ClusterResolver) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	return r.closer.Close()
}
