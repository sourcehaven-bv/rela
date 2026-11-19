package acl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// countingAnyEdge counts HasOutgoingEdge reads and can fail them.
type countingAnyEdge struct {
	Graph
	reads int
	err   error
}

func (g *countingAnyEdge) HasOutgoingEdge(
	ctx context.Context, relType string, cond []store.PropPredicate, fromIDs []string,
) (bool, error) {
	g.reads++
	if g.err != nil {
		return false, g.err
	}
	return g.Graph.HasOutgoingEdge(ctx, relType, cond, fromIDs)
}

// anyEntityWorld is the harbourmaster shape (TKT-CBRLZWU):
//
//   - alice --customer-of--> ACC-1
//   - dave --member-of--> G, G --customer-of--> ACC-2
//   - erin --trial-of{until 2026-01-01}--> ACC-3 (expired at the test's read time)
//   - fred --trial-of{until 2026-12-31}--> ACC-3 (live)
//   - gina --reads--> ACC-1 (its role grants no permission)
//   - sam is assigned the global role staff
//   - carol holds nothing
func anyEntityWorld(t *testing.T) (*countingAnyEdge, *Policy) {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "ACC-1", Type: "account"}, {ID: "ACC-2", Type: "account"}, {ID: "ACC-3", Type: "account"},
		{ID: "alice", Type: "user"}, {ID: "carol", Type: "user"}, {ID: "dave", Type: "user"},
		{ID: "erin", Type: "user"}, {ID: "fred", Type: "user"}, {ID: "gina", Type: "user"},
		{ID: "sam", Type: "user"}, {ID: "G", Type: "group"},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	for _, r := range []struct{ from, rel, to, until string }{
		{"alice", "customer-of", "ACC-1", ""},
		{"dave", "member-of", "G", ""},
		{"G", "customer-of", "ACC-2", ""},
		{"erin", "trial-of", "ACC-3", "2026-01-01"},
		{"fred", "trial-of", "ACC-3", "2026-12-31"},
		{"gina", "reads", "ACC-1", ""},
	} {
		var data *store.RelationData
		if r.until != "" {
			data = &store.RelationData{Properties: map[string]any{"until": r.until}}
		}
		_, err := st.CreateRelation(ctx, entity.RelationKey{From: r.from, Type: r.rel, To: r.to}, data)
		require.NoError(t, err)
	}
	until := &GrantCondition{Terms: []GrantTerm{{Property: "until", Op: GrantGE, Operand: OperandToday, Date: true}}}
	p := &Policy{
		Roles: map[string]RoleDef{
			"staff":    {Read: []string{"*"}, Permissions: []string{"staff.manage", "customers.manage"}},
			"customer": {Read: []string{"account"}, Permissions: []string{"portal.use"}},
			"reader":   {Read: []string{"account"}},
		},
		Assignments: map[string]string{"sam": "staff"},
		RoleRelations: map[string]RoleRelationDef{
			"customer-of": {Confers: "customer", RequiresPermission: "customers.manage"},
			"trial-of": {
				Confers: "customer", RequiresPermission: "customers.manage",
				When: "edge.until >= today()", Condition: until,
			},
			"reads":     {Confers: "reader", RequiresPermission: "customers.manage"},
			"member-of": {RequiresPermission: "staff.manage"},
		},
	}
	return &countingAnyEdge{Graph: aclstore.MustNewGraph(st)}, p
}

func TestHoldsPermissionOnAnyEntity(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, user, perm string
		want             bool
		// global is the answer of the global-only HoldsPermission, which
		// must not change.
		global bool
	}{
		{"global role grants it", "sam", "staff.manage", true, true},
		{"role relation on one entity grants it", "alice", "portal.use", true, false},
		{"role relation held by a group grants it", "dave", "portal.use", true, false},
		{"live conditional edge grants it", "fred", "portal.use", true, false},
		{"expired conditional edge does not", "erin", "portal.use", false, false},
		{"no role at all", "carol", "portal.use", false, false},
		{"conferred role without that permission", "alice", "staff.manage", false, false},
		{"conferred role with no permissions", "gina", "portal.use", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, p := anyEntityWorld(t)
			d := newTestDeclarative(t, p, g)
			req := requestAt(t, d, tc.user, now)
			ctx := context.Background()
			require.Equal(t, tc.want, HoldsPermissionOnAnyEntity(ctx, req, tc.perm))
			require.Equal(t, tc.global, req.HoldsPermission(ctx, tc.perm),
				"the global-only check must not count a role relation")
		})
	}
}

// The edges are read once per Request, however many gates ask: one read per
// role relation that confers a role. PermitsWorld shares the same read.
func TestHoldsPermissionOnAnyEntity_ReadsOncePerRequest(t *testing.T) {
	g, p := anyEntityWorld(t)
	d := newTestDeclarative(t, p, g)
	req := requestAt(t, d, "carol", time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
	ctx := context.Background()
	for range 20 {
		HoldsPermissionOnAnyEntity(ctx, req, "portal.use")
		HoldsPermissionOnAnyEntity(ctx, req, "staff.manage")
		_, err := req.PermitsWorld(ctx, "")
		require.NoError(t, err)
	}
	require.Equal(t, 3, g.reads, "customer-of, trial-of and reads, once each per Request")

	// No read when no role relation confers a role that could answer yes.
	g.reads = 0
	carol := requestAt(t, d, "carol", time.Time{})
	require.False(t, HoldsPermissionOnAnyEntity(ctx, carol, "staff.manage"))
	ok, err := carol.PermitsWorld(ctx, "world:drafts")
	require.NoError(t, err)
	require.False(t, ok)
	require.Zero(t, g.reads)

	// A global holder never reads.
	sam := requestAt(t, d, "sam", time.Time{})
	require.True(t, HoldsPermissionOnAnyEntity(ctx, sam, "staff.manage"))
	require.Zero(t, g.reads)
}

// A failed read counts no conferred role: the answer falls back to the
// global roles and is not retried within the Request.
func TestHoldsPermissionOnAnyEntity_ReadErrorFailsClosed(t *testing.T) {
	g, p := anyEntityWorld(t)
	d := newTestDeclarative(t, p, g)
	alice := requestAt(t, d, "alice", time.Time{})
	sam := requestAt(t, d, "sam", time.Time{})
	g.err = errors.New("store down")
	ctx := context.Background()
	require.False(t, HoldsPermissionOnAnyEntity(ctx, alice, "portal.use"))
	g.err = nil
	require.False(t, HoldsPermissionOnAnyEntity(ctx, alice, "portal.use"), "the error is kept for the Request")
	require.Equal(t, 1, g.reads)
	_, err := alice.PermitsWorld(ctx, "")
	require.Error(t, err, "PermitsWorld reports the kept error")
	require.True(t, HoldsPermissionOnAnyEntity(ctx, sam, "staff.manage"), "a global role does not need the read")
}

// A read that failed because the caller's ctx ended is not kept: the next
// caller reads again.
func TestHoldsPermissionOnAnyEntity_CanceledReadIsNotKept(t *testing.T) {
	g, p := anyEntityWorld(t)
	d := newTestDeclarative(t, p, g)
	alice := requestAt(t, d, "alice", time.Time{})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	g.err = context.Canceled
	require.False(t, HoldsPermissionOnAnyEntity(canceled, alice, "portal.use"))
	g.err = nil
	require.True(t, HoldsPermissionOnAnyEntity(context.Background(), alice, "portal.use"))
}

// A client ceiling that withholds the permission wins over a role relation,
// as it does for every permission check.
func TestHoldsPermissionOnAnyEntity_CeilingApplies(t *testing.T) {
	g, _ := anyEntityWorld(t)
	p := mustPolicy(t, `
roles:
  customer:
    read: [account]
    permissions: [portal.use]
  staff:
    permissions: [customers.manage]
role_relations:
  customer-of:
    confers: customer
    requires_permission: customers.manage
client_baselines:
  apps:
    applies_to: [app]
    deny_permissions: [portal.use]
`)
	d := newTestDeclarative(t, p, g)
	ctx := context.Background()
	for _, tc := range []struct {
		principalType string
		want          bool
	}{{"user", true}, {"app", false}} {
		req, err := d.ForIdentity(identityFor(ctx, t, d, verifiedClient("alice", tc.principalType)))
		require.NoError(t, err)
		require.Equal(t, tc.want, HoldsPermissionOnAnyEntity(ctx, req, "portal.use"), tc.principalType)
	}
}
