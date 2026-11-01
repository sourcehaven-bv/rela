package acl

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
)

// edgeBatcher is the batch read a [Graph] may offer so [PrimeLocalRoles] can
// answer a whole page of local-role probes in a fixed number of queries.
// aclstore.Graph implements it; a Graph without it is never primed and keeps
// the per-entity probes.
type edgeBatcher interface {
	// EdgesBetween returns the zero-tail relType edges from one of fromIDs
	// to one of toIDs, as from → set of to. Zero-tail is what
	// [Graph.HasEdge] reads.
	EdgesBetween(ctx context.Context, relType string, fromIDs, toIDs []string) (map[string]map[string]bool, error)
	// OutgoingFrom returns, per id in fromIDs, the targets of its relType
	// edges in the order [Graph.OutgoingRelations] returns them.
	OutgoingFrom(ctx context.Context, relType string, fromIDs []string) (map[string][]string, error)
}

// localRolePrime holds the local roles of a page of entities, computed in one
// batch for one [Request].
type localRolePrime struct {
	req    *Request
	byID   map[string][]RoleAttribution
	parent *localRolePrime
}

type localRolePrimeKey struct{}

// PrimeLocalRoles computes the local roles of every entity in ids in one
// batch and returns a ctx carrying them, so the per-entity checks a page
// makes (its `_actions` verbs, its field verdicts, its included neighbors)
// read the batch instead of probing the graph once per (role relation ×
// member × ancestor) per entity per check. Without it a list page of 100
// rows under a principal with five roles cost 1,800 single-edge lookups
// (BUG-E2W1LS).
//
// The prime belongs to the ctx, not to the [Request]. The Request lives for a
// whole HTTP request, writes included, and a write may create a conferring
// edge; a read path primes its own ctx, so a later write never sees an answer
// computed before it. Only read paths should call this.
//
// It primes the Request bound at the request edge, checked as
// [Declarative.RequestFrom] checks it, so the checks that resolve that
// Request read the prime and nothing else does.
//
// A ctx without a bound Request (none attached, or one bound to another
// principal), a Graph that cannot batch, or a failed batch read returns ctx
// unchanged: the per-entity probes then answer as before, so a
// prime can only make a check cheaper, never different. Priming again on a
// primed ctx adds the new ids; ids already primed are not recomputed.
func PrimeLocalRoles(ctx context.Context, ids []string) context.Context {
	r := FromContext(ctx)
	if r == nil || len(ids) == 0 {
		return ctx
	}
	if _, err := r.d.RequestFrom(ctx); err != nil {
		return ctx
	}
	b, ok := r.d.graph.(edgeBatcher)
	if !ok {
		return ctx
	}
	parent, _ := ctx.Value(localRolePrimeKey{}).(*localRolePrime)
	var todo []string
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, done := parent.lookup(r, id); !done {
			todo = append(todo, id)
		}
	}
	if len(todo) == 0 {
		return ctx
	}
	byID, err := batchLocalRoles(ctx, r, b, todo)
	if err != nil {
		// A canceled request is not a fault: the per-entity checks after it
		// fail on the same ctx and nobody reads the answer (TKT-T2I7P2).
		if !errors.Is(ctx.Err(), context.Canceled) {
			slog.WarnContext(ctx, "acl: batching local roles failed; checking per entity", "entities", len(todo), "err", err)
		}
		return ctx
	}
	return context.WithValue(ctx, localRolePrimeKey{}, &localRolePrime{req: r, byID: byID, parent: parent})
}

// WithoutLocalRolePrime returns ctx with any [PrimeLocalRoles] answers
// hidden, so a check on it walks the graph as it is now. A write
// authorizes on it: a prime is a snapshot taken before the write, and a
// write path that inherited a primed ctx must not decide on it.
func WithoutLocalRolePrime(ctx context.Context) context.Context {
	if ctx.Value(localRolePrimeKey{}) == nil {
		return ctx
	}
	return context.WithValue(ctx, localRolePrimeKey{}, (*localRolePrime)(nil))
}

// primedLocalRoles returns the primed local roles of entityID, if ctx holds a
// prime computed for r that covers it.
func primedLocalRoles(ctx context.Context, r *Request, entityID string) ([]RoleAttribution, bool) {
	p, _ := ctx.Value(localRolePrimeKey{}).(*localRolePrime)
	return p.lookup(r, entityID)
}

func (p *localRolePrime) lookup(r *Request, id string) ([]RoleAttribution, bool) {
	for ; p != nil; p = p.parent {
		if p.req != r {
			continue
		}
		if local, ok := p.byID[id]; ok {
			return local, true
		}
	}
	return nil, false
}

// batchLocalRoles is r's localRoles walk for many entities: the ancestor
// chains and the conferring edges are read in batches, then each entity runs
// the same walk over the loaded edges. It is a function rather than a
// method so Request stays under its method budget.
func batchLocalRoles(
	ctx context.Context, r *Request, b edgeBatcher, ids []string,
) (map[string][]RoleAttribution, error) {
	globals := r.Globals()
	relTypes := r.conferringRelations()
	byID := make(map[string][]RoleAttribution, len(ids))
	if len(relTypes) == 0 || len(globals.Members) == 0 {
		for _, id := range ids {
			byID[id] = nil
		}
		return byID, nil
	}
	chains, err := batchAncestors(ctx, r, b, ids)
	if err != nil {
		return nil, err
	}
	targetSet := map[string]bool{}
	for _, chain := range chains {
		for _, t := range chain {
			targetSet[t] = true
		}
	}
	targets := slices.Sorted(maps.Keys(targetSet))
	edges := make(map[string]map[string]map[string]bool, len(relTypes))
	for _, relType := range relTypes {
		into, err := b.EdgesBetween(ctx, relType, globals.Members, targets)
		if err != nil {
			return nil, err
		}
		edges[relType] = into
	}
	hasEdge := func(member, relType, target string) bool {
		return edges[relType][member][target]
	}
	for _, id := range ids {
		byID[id] = r.localRoles(id, chains[id], globals.Members, hasEdge)
	}
	return byID, nil
}

// batchAncestors returns r's ancestor chain for every id, reading each BFS
// level of all chains together: one query per inherit_roles_through type per
// level instead of one per node. A read error fails the batch rather than
// truncating a chain, so the caller falls back to the per-entity walk.
func batchAncestors(ctx context.Context, r *Request, b edgeBatcher, ids []string) (map[string][]string, error) {
	chains := make(map[string][]string, len(ids))
	inherit := r.d.policy.InheritRolesThrough
	walks := make(map[string]*ancestorWalk, len(ids))
	for _, id := range ids {
		chains[id] = []string{id}
		walks[id] = &ancestorWalk{visited: map[string]bool{id: true}, frontier: []string{id}}
	}
	if len(inherit) == 0 {
		return chains, nil
	}
	// out[node][relType] caches each node's targets across all chains.
	out := map[string]map[string][]string{}
	for range depthCap {
		if err := loadOutgoing(ctx, b, inherit, walks, out); err != nil {
			return nil, err
		}
		active := false
		for _, id := range ids {
			if walks[id].step(inherit, out, func(to string) { chains[id] = append(chains[id], to) }) {
				active = true
			}
		}
		if !active {
			break
		}
	}
	return chains, nil
}

// ancestorWalk is one entity's breadth-first walk up its containment.
type ancestorWalk struct {
	visited  map[string]bool
	frontier []string
}

// step advances w one level over the loaded edges, passing each newly
// reached node to add in the order the per-entity ancestor walk would. It reports
// whether the walk reached any node.
func (w *ancestorWalk) step(inherit []string, out map[string]map[string][]string, add func(string)) bool {
	var next []string
	for _, n := range w.frontier {
		for _, relType := range inherit {
			for _, to := range out[n][relType] {
				if w.visited[to] {
					continue
				}
				w.visited[to] = true
				add(to)
				next = append(next, to)
			}
		}
	}
	w.frontier = next
	return len(next) > 0
}

// loadOutgoing reads the inherit edges of every frontier node not yet in
// out, one query per relation type.
func loadOutgoing(
	ctx context.Context, b edgeBatcher, inherit []string, walks map[string]*ancestorWalk,
	out map[string]map[string][]string,
) error {
	need := map[string]bool{}
	for _, w := range walks {
		for _, n := range w.frontier {
			if _, loaded := out[n]; !loaded {
				need[n] = true
			}
		}
	}
	if len(need) == 0 {
		return nil
	}
	nodes := slices.Sorted(maps.Keys(need))
	for _, n := range nodes {
		out[n] = map[string][]string{}
	}
	for _, relType := range inherit {
		tos, err := b.OutgoingFrom(ctx, relType, nodes)
		if err != nil {
			return err
		}
		for from, t := range tos {
			if m, ok := out[from]; ok {
				m[relType] = t
			}
		}
	}
	return nil
}
