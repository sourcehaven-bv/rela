package tenant

import (
	"context"
	"fmt"
	"sync"
)

// MapResolver is the in-memory [Resolver] test double: a fixed table of
// tenants, validated as production would refuse them. Production resolves
// through [ClusterResolver].
type MapResolver struct {
	mu    sync.RWMutex
	byOrg map[string]Tenant
}

// NewMapResolver builds a resolver over the given tenants. It refuses an
// invalid tenant, a duplicate org and two orgs on one schema, which would
// share one tenant's data with the other.
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
			return nil, fmt.Errorf("tenant %d: org_id %q and %q both map to schema %q", i, t.OrgID, other, t.Schema)
		}
		byOrg[t.OrgID] = t
		bySchema[t.Schema] = t.OrgID
	}
	return &MapResolver{byOrg: byOrg}, nil
}

// Resolve returns the tenant for orgID, or [ErrUnknownTenant].
func (r *MapResolver) Resolve(_ context.Context, orgID string) (Tenant, error) {
	r.mu.RLock()
	t, ok := r.byOrg[orgID]
	r.mu.RUnlock()
	if !ok {
		return Tenant{}, fmt.Errorf("%w: %q", ErrUnknownTenant, orgID)
	}
	return t, nil
}

// Remove drops orgID, as suspending it would, so a test can revalidate.
func (r *MapResolver) Remove(orgID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byOrg, orgID)
}

// Put serves t again, as resuming it would.
func (r *MapResolver) Put(t Tenant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byOrg[t.OrgID] = t
}
