package acl

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// conditionalWorld is a store with an expiring auditor grant (TKT-TKSCSBT):
//
//   - aud --grants{until 2026-12-31}--> TKT-1 (live until year end)
//   - aud --grants{until 2026-01-01}--> TKT-2 (expired)
//   - aud --grants{}--> TKT-4 (no date: never counts)
//   - aud --owns--> TKT-2 (unconditional, confers the same role)
//   - TKT-3 --partOf--> TKT-1 (inherits TKT-1's grant)
//   - ali --member-of{until 2026-01-01}--> G (expired membership)
//   - bob --member-of{until 2026-12-31}--> G
//   - G is assigned role `staff`, which reads people
func conditionalWorld(t *testing.T) (*Declarative, store.Store) {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket"}, {ID: "TKT-2", Type: "ticket"},
		{ID: "TKT-3", Type: "ticket"}, {ID: "TKT-4", Type: "ticket"},
		{ID: "aud", Type: "user"}, {ID: "ali", Type: "user"}, {ID: "bob", Type: "user"},
		{ID: "G", Type: "group"},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	for _, r := range []struct {
		from, rel, to, until string
	}{
		{"aud", "grants", "TKT-1", "2026-12-31"},
		{"aud", "grants", "TKT-2", "2026-01-01"},
		{"aud", "grants", "TKT-4", ""},
		{"aud", "owns", "TKT-2", ""},
		{"TKT-3", "partOf", "TKT-1", ""},
		{"ali", "member-of", "G", "2026-01-01"},
		{"bob", "member-of", "G", "2026-12-31"},
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
			"auditor": {Read: []string{"ticket"}},
			"staff":   {Read: []string{"user"}},
		},
		Assignments:         map[string]string{"G": "staff"},
		InheritRolesThrough: []string{"partOf"},
		RoleRelations: map[string]RoleRelationDef{
			"grants": {
				Confers: "auditor", RequiresPermission: "delegate", When: "edge.valid_until >= today()", Condition: until,
			},
			"member-of": {RequiresPermission: "delegate", When: "edge.valid_until >= today()", Condition: until},
			"owns":      {Confers: "auditor"},
		},
	}
	d, err := NewDeclarative(p, aclstore.MustNewGraph(st), st)
	require.NoError(t, err)
	return d, st
}

func requestAt(t *testing.T, d *Declarative, user string, at time.Time) *Request {
	t.Helper()
	ctx := context.Background()
	if !at.IsZero() {
		ctx = readtime.With(ctx, at)
	}
	req, err := d.ForIdentity(identityFor(ctx, t, d, principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	require.NoError(t, err)
	return req
}

var tickets = []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"}

// readable answers the row gate in the store: the ids among tickets the
// request's ReadQuery admits.
func readable(t *testing.T, st store.Store, req *Request) []string {
	t.Helper()
	ctx := context.Background()
	rq := req.ReadQuery(ctx, "ticket")
	if rq.DenyAll {
		return nil
	}
	require.NotNil(t, rq.Query)
	q := *rq.Query
	q.Faces = store.AllFaces()
	m, err := store.MatchingIDs(ctx, st, q, tickets)
	require.NoError(t, err)
	var out []string
	for _, id := range tickets {
		if m[id] {
			out = append(out, id)
		}
	}
	return out
}

// localAuditor answers the Go probe path: the ids among tickets on which
// ForEntity attributes the auditor role.
func localAuditor(ctx context.Context, req *Request) []string {
	var out []string
	for _, id := range tickets {
		for _, a := range req.ForEntity(ctx, "ticket", id) {
			if a.Role == "auditor" {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

func TestConditionalGrant_GoAndStoreAgree(t *testing.T) {
	d, st := conditionalWorld(t)
	tests := []struct {
		name string
		at   time.Time
		want []string
	}{
		{"live grant and its inheritor", time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC), []string{"TKT-1", "TKT-2", "TKT-3"}},
		{"last day still counts", time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), []string{"TKT-1", "TKT-2", "TKT-3"}},
		{"the day after, only the unconditional grant", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), []string{"TKT-2"}},
		{"no read time: only the unconditional grant", time.Time{}, []string{"TKT-2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := requestAt(t, d, "aud", tc.at)
			require.Equal(t, tc.want, readable(t, st, req), "store row gate")
			require.Equal(t, tc.want, localAuditor(context.Background(), req), "Go probes")
			primed := PrimeLocalRoles(WithRequest(context.Background(), req), tickets)
			require.Equal(t, tc.want, localAuditor(primed, req), "primed batch")

			preds, ok := req.LocalRolePredicates("auditor")
			require.True(t, ok, "the unconditional relation keeps the role type live")
			var held []string
			for i := range preds {
				q := store.GraphQuery{EntityType: "ticket", HasInbound: &preds[i], Faces: store.AllFaces()}
				m, err := store.MatchingIDs(context.Background(), st, q, tickets)
				require.NoError(t, err)
				for _, id := range tickets {
					if m[id] {
						held = append(held, id)
					}
				}
			}
			slices.Sort(held)
			require.Equal(t, tc.want, held, "LocalRolePredicates")
		})
	}
}

func TestConditionalMembership(t *testing.T) {
	d, _ := conditionalWorld(t)
	at := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	require.Equal(t, []string{"ali"}, requestAt(t, d, "ali", at).Globals().Members,
		"an expired membership edge is not walked")
	require.Equal(t, []string{"bob", "G"}, requestAt(t, d, "bob", at).Globals().Members)
	require.Equal(t, []string{"bob"}, requestAt(t, d, "bob", time.Time{}).Globals().Members,
		"no read time: the conditional membership fails closed")
}

func TestConditionalGrant_RetoolKeepsTheBoundTime(t *testing.T) {
	d, st := conditionalWorld(t)
	req := requestAt(t, d, "aud", time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC))
	require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3"}, readable(t, st, RetoolRequest(req, "realtime")))
}

func TestNewDeclarative_RefusesUncompiledWhen(t *testing.T) {
	st := memstore.New()
	_, err := NewDeclarative(&Policy{RoleRelations: map[string]RoleRelationDef{
		"grants": {Confers: "auditor", RequiresPermission: "delegate", When: "edge.valid_until >= today()"},
	}}, aclstore.MustNewGraph(st), st)
	require.ErrorContains(t, err, "was not compiled")
}
