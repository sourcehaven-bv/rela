//go:build postgres

package pgstore_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// The along walk's plan, measured with EXPLAIN (ANALYZE, BUFFERS) on a
// fixed page of four actions under projects and folders, padded with
// unrelated actions, groups, projects and folders that carry the walked
// relations. The gates carry a containment-inheritance policy: alice owns
// the top folders, and every row inherits through child_of and in_folder.

// alongPlanNode is the part of an EXPLAIN (FORMAT JSON) node the checks read.
type alongPlanNode struct {
	NodeType    string          `json:"Node Type"`
	Relation    string          `json:"Relation Name"`
	Index       string          `json:"Index Name"`
	IndexCond   string          `json:"Index Cond"`
	Subplan     string          `json:"Subplan Name"`
	ActualRows  float64         `json:"Actual Rows"`
	ActualLoops float64         `json:"Actual Loops"`
	SharedHit   float64         `json:"Shared Hit Blocks"`
	SharedRead  float64         `json:"Shared Read Blocks"`
	Plans       []alongPlanNode `json:"Plans"`
}

// alongPlan is one EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) result.
type alongPlan struct {
	Plan     alongPlanNode `json:"Plan"`
	Planning float64       `json:"Planning Time"`
}

// alongScan is a scan node with the CTE it runs under.
type alongScan struct {
	cte  string
	node alongPlanNode
}

func (p alongPlan) rows() float64 {
	var sum float64
	var walk func(n alongPlanNode)
	walk = func(n alongPlanNode) {
		sum += n.ActualRows * n.ActualLoops
		for _, c := range n.Plans {
			walk(c)
		}
	}
	walk(p.Plan)
	return sum
}

func (p alongPlan) buffers() float64 { return p.Plan.SharedHit + p.Plan.SharedRead }

// scans lists every node reading table, under the CTE it belongs to.
func (p alongPlan) scans(table string) []alongScan {
	var out []alongScan
	var walk func(n alongPlanNode, cte string)
	walk = func(n alongPlanNode, cte string) {
		if name, ok := strings.CutPrefix(n.Subplan, "CTE "); ok {
			cte = name
		}
		if n.Relation == table {
			out = append(out, alongScan{cte: cte, node: n})
		}
		for _, c := range n.Plans {
			walk(c, cte)
		}
	}
	walk(p.Plan, "main query")
	return out
}

// alongPolicy is the reader's gate for typ: rows alice owns, directly or
// through a containing project, group or folder.
func alongPolicy(typ string) store.GraphQuery {
	return store.GraphQuery{
		EntityType: typ, Faces: store.InWorld(store.TrivialScope()),
		Related: []store.DirectedRelation{{Incoming: true, Pred: store.RelationPredicate{
			Endpoints: []string{"alice"}, OfTypes: []string{"owner-of"},
			EntityInheritThrough: []string{"child_of", "in_folder"}, EntityDepth: 10,
		}}},
	}
}

func alongPolicyStatus(v string) store.Cond {
	return store.PropLeaf(store.PropPredicate{Property: "status", Op: store.PropEqual, Value: v, Scalar: true})
}

// alongPolicyQuery is the probe's held walk over the page, every node gated
// by [alongPolicy].
func alongPolicyQuery(depth int, srcs ...string) store.RollupQuery {
	node := func(typ string, c store.Cond) store.AlongNode {
		return store.AlongNode{Gate: alongPolicy(typ), Counts: &c}
	}
	return store.RollupQuery{Sources: srcs, Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: depth,
		Segments: []store.AlongSegment{
			{Relation: "child_of", Repeat: true, Through: map[string]store.AlongNode{
				"action":  node("action", alongPolicyStatus("open")),
				"project": node("project", store.OrCond(alongPolicyStatus("on_hold"), alongPolicyStatus("dropped"))),
			}},
			{Relation: "in_folder", Repeat: true, Through: map[string]store.AlongNode{
				"folder": node("folder", alongPolicyStatus("dropped")),
			}},
		},
	}}
}

var alongPage = []string{"ACT-P1", "ACT-P2", "ACT-P3", "ACT-P4"}

// seedAlongFixture builds the page and pad padding entities around it, in
// trees of one folder pair, one project, one group and actions.
func seedAlongFixture(t *testing.T, pool *pgxpool.Pool, pad int) {
	t.Helper()
	ctx := context.Background()
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	put := func(id, typ, status string) {
		e := entity.New(id, typ)
		e.Properties["status"] = status
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	link := func(from, rel, to string) {
		_, lerr := s.CreateRelation(ctx, entity.RelationKey{From: from, Type: rel, To: to}, nil)
		require.NoError(t, lerr)
	}
	put("alice", "person", "")
	put("FLD-P2", "folder", "dropped")
	put("FLD-P1", "folder", "active")
	put("PRJ-P1", "project", "on_hold")
	put("GRP-P1", "action", "done")
	link("alice", "owner-of", "FLD-P2")
	link("FLD-P1", "in_folder", "FLD-P2")
	link("PRJ-P1", "in_folder", "FLD-P1")
	link("GRP-P1", "child_of", "PRJ-P1")
	for _, id := range alongPage {
		put(id, "action", "open")
		link(id, "child_of", "GRP-P1")
	}
	for tree := 0; pad > 0; tree++ {
		top, mid := fmt.Sprintf("FLD-X%04d", tree), fmt.Sprintf("FLD-Y%04d", tree)
		prj, grp := fmt.Sprintf("PRJ-X%04d", tree), fmt.Sprintf("GRP-X%04d", tree)
		put(top, "folder", "active")
		put(mid, "folder", "dropped")
		put(prj, "project", "active")
		put(grp, "action", "open")
		link("alice", "owner-of", top)
		link(mid, "in_folder", top)
		link(prj, "in_folder", mid)
		link(grp, "child_of", prj)
		pad -= 4
		for a := 0; a < 6 && pad > 0; a++ {
			id := fmt.Sprintf("ACT-X%04d-%d", tree, a)
			put(id, "action", "open")
			link(id, "child_of", grp)
			pad--
		}
	}
	_, err = pool.Exec(ctx, "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)
}

// explainAlong runs q's statement under EXPLAIN (ANALYZE, BUFFERS) and
// checks it answers the page as the store does.
func explainAlong(t *testing.T, pool *pgxpool.Pool, q store.RollupQuery) alongPlan {
	t.Helper()
	sqlText, args, err := pgstore.BuildAlongSQLForTest(q)
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, pool.QueryRow(context.Background(),
		"EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+sqlText, args...).Scan(&raw))
	var plans []alongPlan
	require.NoError(t, json.Unmarshal(raw, &plans))
	require.Len(t, plans, 1)
	return plans[0]
}

// TestRollupAlongExplainBudget pins checks (a) to (d) of design gap 1:
// the page's work does not grow with the padding, no gate scans entities
// sequentially or by type alone, the upward hops read relations by
// (from_id, ...) and every closure reads them through an index. The
// padding is 1,000 and 3,000 rows: at 100 the tables fit in a page or two
// and the planner rightly prefers sequential scans.
func TestRollupAlongExplainBudget(t *testing.T) {
	plans := map[int]alongPlan{}
	for _, pad := range []int{1000, 3000} {
		pool := newScopedPool(t)
		seedAlongFixture(t, pool, pad)
		s, err := pgstore.New(pool)
		require.NoError(t, err)
		got, err := store.Rollup(context.Background(), s, alongPolicyQuery(12, alongPage...))
		require.NoError(t, err)
		require.Equal(t, map[string]store.RollupResult{
			"ACT-P1": {Count: 2}, "ACT-P2": {Count: 2}, "ACT-P3": {Count: 2}, "ACT-P4": {Count: 2},
		}, got)
		plans[pad] = explainAlong(t, pool, alongPolicyQuery(12, alongPage...))
	}
	small, large := plans[1000], plans[3000]
	t.Logf("rows %v -> %v, buffers %v -> %v", small.rows(), large.rows(), small.buffers(), large.buffers())
	require.InDelta(t, small.rows(), large.rows(), small.rows()*0.05, "(a) actual rows grow with the padding")
	require.InDelta(t, small.buffers(), large.buffers(), small.buffers()*0.1, "(a) buffers grow with the padding")
	for _, sc := range large.scans("entities") {
		require.NotEqual(t, "Seq Scan", sc.node.NodeType, "(b) %s scans entities sequentially", sc.cte)
		require.Regexp(t, `\bid = `, sc.node.IndexCond,
			"(c) %s scans entities by %s without the level's ids", sc.cte, sc.node.IndexCond)
	}
	hop := regexp.MustCompile(`^l\d+_s\d+$`)
	upward := map[string]bool{"relations_type_from_idx": true, "relations_elementless_idx": true, "relations_pkey": true}
	indexed := map[string]bool{"relations_from_idx": true, "relations_to_idx": true, "relations_type_to_idx": true}
	for _, sc := range large.scans("relations") {
		ok := upward[sc.node.Index] || !hop.MatchString(sc.cte) && indexed[sc.node.Index]
		require.True(t, ok, "(d) %s reads relations by %s %q", sc.cte, sc.node.NodeType, sc.node.Index)
	}
}

// TestRollupAlongExplainPlanningTime pins check (e): a 32-level statement
// plans within the bound.
func TestRollupAlongExplainPlanningTime(t *testing.T) {
	const bound = time.Second
	pool := newScopedPool(t)
	seedAlongFixture(t, pool, 100)
	q := alongPolicyQuery(store.MaxAlongDepth, alongPage...)
	require.Len(t, store.AlongLevels(q.Along), store.MaxAlongDepth)
	plan := explainAlong(t, pool, q)
	t.Logf("planning %.1f ms", plan.Planning)
	require.Less(t, plan.Planning, float64(bound.Milliseconds()), "(e) a 32-level statement plans too slowly")
}

// TestRollupStatementTimeout pins that a roll-up statement runs under
// [pgstore.RollupStatementTimeout] and that the setting ends with it.
func TestRollupStatementTimeout(t *testing.T) {
	pool := newScopedPoolSized(t, 1)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	got, err := pgstore.RollupStatementTimeoutSettingForTest(ctx, s)
	require.NoError(t, err)
	require.Equal(t, "5s", got)
	require.Equal(t, 5*time.Second, pgstore.RollupStatementTimeout)
	var after string
	require.NoError(t, pool.QueryRow(ctx, "SELECT current_setting('statement_timeout')").Scan(&after))
	require.NotEqual(t, got, after, "the timeout leaked to the pool")
}
