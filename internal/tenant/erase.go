package tenant

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotMarkedForErasure reports that an org is a served tenant, not one
// marked `erase: true` in the tenant map. Erasing a served tenant would drop
// a schema that a server loading the same map still routes requests to.
var ErrNotMarkedForErasure = errors.New("tenant: not marked for erasure")

// ErrTenantInUse reports that the tenant's lock is held: a server has the
// tenant open, or another erasure of it is running. [Erase] refuses rather
// than drop a schema under a live store.
var ErrTenantInUse = errors.New("tenant: locked: open in a running server or being erased")

// ErrPublicHasTables reports that the `public` schema holds tables. Every
// tenant connection has `public` on its `search_path`, so a server that still
// had the tenant open would read and write those tables once the tenant's
// schema is gone. [Erase] refuses rather than risk that.
var ErrPublicHasTables = errors.New("tenant: the public schema holds tables, which a server that still " +
	"had the tenant open would fall back to")

// AuditRoot returns the directory under a project's cache directory that
// holds one audit sink per tenant, each named after its schema
// (TKT-SNVR8UV). The server writes there and [Erase] removes from there, so
// both take the path from here.
func AuditRoot(cacheDir string) string {
	return filepath.Join(cacheDir, "audit", "tenants")
}

// Erasure reports what [Erase] removed.
type Erasure struct {
	// SchemaDropped is false when the schema did not exist, for example on
	// a second run after an erasure that already dropped it.
	SchemaDropped bool
	// AuditRemoved is false when the tenant had no audit directory.
	AuditRemoved bool
}

// Erase removes all of one tenant's data (TKT-TNERAS): its schema, dropped
// with everything in it, and its audit directory under auditRoot.
//
// A schema holds everything the store keeps for a tenant: entities,
// relations, attachment bytes, search, version history, jobs, leases and
// piles. The audit sink is the one store outside it, which is why it is a
// directory per tenant. Backups are outside both; see the erasure section of
// the PostgreSQL backend guide.
//
// The order is the safe one. The schema goes first, under an exclusive lock
// that every open tenant holds shared, so a server that has the tenant open
// makes this return [ErrTenantInUse] and a server that tries to open it
// meanwhile is refused. The audit directory goes second, so a failed drop
// leaves the record of what the tenant did beside the data it describes.
//
// Rerunning after a partial erasure is safe: a missing schema or directory
// is not an error.
//
// t must come from [Config.ErasureTarget], which checks the erase mark.
func Erase(ctx context.Context, t Tenant, auditRoot string) (Erasure, error) {
	if err := t.validate(); err != nil {
		return Erasure{}, fmt.Errorf("tenant: erase: %w", err)
	}
	if auditRoot == "" {
		return Erasure{}, errors.New("tenant: erase: audit root is required")
	}
	var out Erasure
	dropped, err := dropSchema(ctx, t.DSN, t.Schema)
	if err != nil {
		return out, fmt.Errorf("tenant: erase org_id %q: %w", t.OrgID, err)
	}
	out.SchemaDropped = dropped
	// The schema name passed checkSchemaName (validate above), so it is a
	// single path element and cannot leave auditRoot.
	dir := filepath.Join(auditRoot, t.Schema)
	switch fi, err := os.Lstat(dir); {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return out, fmt.Errorf("tenant: erase org_id %q: check audit directory: %w", t.OrgID, err)
	case fi.Mode()&os.ModeSymlink != 0:
		// Removing the link would leave the records it points at and still
		// report them removed.
		return out, fmt.Errorf("tenant: erase org_id %q: audit directory %s is a symlink; "+
			"remove what it points to, then the link", t.OrgID, dir)
	default:
		if err := os.RemoveAll(dir); err != nil {
			return out, fmt.Errorf("tenant: erase org_id %q: remove audit directory: %w", t.OrgID, err)
		}
		out.AuditRemoved = true
	}
	return out, nil
}
