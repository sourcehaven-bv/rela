// Package aclstore adapts a store to the read contracts the ACL resolver
// declares: acl.Graph for membership and ancestor walks, and
// acl.PrincipalLookup for resolving a raw principal to a user entity.
//
// acl owns those interfaces; this package owns the store-backed
// implementations so that acl does not depend on a concrete store. It
// imports store but not acl, so acl's own tests can use it without an
// import cycle.
package aclstore

import (
	"context"
	"errors"
	"iter"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// EdgeReader is the store capability [Graph] needs.
type EdgeReader interface {
	GetRelation(ctx context.Context, key entity.RelationKey) (*entity.Relation, error)
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// Graph implements acl.Graph over a store.
//
// Errors from the store iterator are surfaced via the methods' error
// returns. The resolver decides what to do: in practice it aborts the
// walk loudly.
type Graph struct {
	s EdgeReader
}

// NewGraph constructs a Graph backed by s.
//
// Nil: rejected with an error; a Graph with no store has nothing to probe.
func NewGraph(s EdgeReader) (*Graph, error) {
	if s == nil {
		return nil, errors.New("aclstore: NewGraph: store is nil")
	}
	return &Graph{s: s}, nil
}

// MustNewGraph is [NewGraph] for tests and fixtures: it panics on nil.
func MustNewGraph(s EdgeReader) *Graph {
	g, err := NewGraph(s)
	if err != nil {
		panic(err)
	}
	return g
}

// HasEdge reports whether the specific (from, relType, to) edge exists.
// A missing edge ([store.ErrNotFound]) reports false silently. Any
// other error (transient backend failure, context cancelled, etc.)
// also reports false BUT is logged at [slog.LevelWarn] with operator-facing
// context (RR-K3OO) — without the log a transient store error during ACL
// evaluation silently undercounts a member's role-relations and
// produces spurious denies the operator can't trace.
//
// Implementation uses the store's single-key GetRelation rather than
// scanning ListRelations (RR-L3VO): in the ACL resolver this is
// called O(|RoleRelations| × |Members| × |ancestors|) times per
// request, and scanning the full outgoing-by-relType list per call
// is a quadratic foot-cannon on densely-connected nodes.
//
// cond restricts the edge by its own properties (a conditional grant,
// TKT-TKSCSBT). An edge that does not match reports false.
func (g *Graph) HasEdge(ctx context.Context, from, relType, to string, cond []store.PropPredicate) bool {
	// The zero tail on purpose: membership edges are identity-scoped, and
	// this is the default-world seam (TKT-7IZHP0).
	rel, err := g.s.GetRelation(ctx, entity.RelationKey{From: from, Type: relType, To: to})
	if err == nil {
		ok, mErr := matchEdge(rel, cond)
		if mErr != nil {
			slog.WarnContext(ctx, "aclstore: Graph.HasEdge: edge condition refused, treated as no-edge",
				"from", from, "rel_type", relType, "to", to, "error", mErr)
		}
		return ok
	}
	if !errors.Is(err, store.ErrNotFound) {
		// A cancelled request context is routine: the caller hung up and
		// nobody is waiting for the answer. It is not a backend fault, and
		// at this call's fan-out (see the O(...) note above) a single
		// disconnect emits a burst of these — enough to bury the genuine
		// backend errors this log exists to surface. Demote those to Debug
		// and keep Warn for real failures. DeadlineExceeded stays at Warn
		// on purpose: a timeout IS a signal about backend latency.
		level := slog.LevelWarn
		if errors.Is(err, context.Canceled) {
			level = slog.LevelDebug
		}
		slog.Log(ctx, level, "aclstore: Graph.HasEdge: backend error treated as no-edge",
			"from", from, "rel_type", relType, "to", to, "error", err)
	}
	return false
}

// OutgoingRelations returns the toIDs reachable from `from` via
// `relType`. Errors propagate; callers (the resolver) abort the
// surrounding walk rather than silently undercount.
// cond restricts the edges as in [Graph.HasEdge].
func (g *Graph) OutgoingRelations(
	ctx context.Context, from, relType string, cond []store.PropPredicate,
) ([]string, error) {
	var out []string
	for r, err := range g.s.ListRelations(ctx, store.RelationQuery{
		EntityID:  from,
		Direction: store.DirectionOutgoing,
		Type:      relType,
	}) {
		if err != nil {
			return out, err
		}
		if ok, err := matchEdge(r, cond); err != nil {
			return out, err
		} else if !ok {
			continue
		}
		out = append(out, r.To)
	}
	return out, nil
}

// EdgesBetween implements the acl edge batcher with ONE store query. Both ends are
// filtered: a shared ancestor (a project every task is in) may hold edges
// from thousands of non-members, and those must not be read.
//
// cond restricts the edges as in [Graph.HasEdge].
func (g *Graph) EdgesBetween(
	ctx context.Context, relType string, cond []store.PropPredicate, fromIDs, toIDs []string,
) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	if len(fromIDs) == 0 || len(toIDs) == 0 {
		return out, nil
	}
	// The zero tail, as HasEdge reads it: membership edges are identity-scoped.
	zero := entity.Face("")
	for rel, err := range g.s.ListRelations(ctx, store.RelationQuery{
		Type:      relType,
		EntityIDs: toIDs,
		PeerIDs:   fromIDs,
		Direction: store.DirectionIncoming,
		FromFace:  &zero,
	}) {
		if err != nil {
			return nil, err
		}
		if ok, err := matchEdge(rel, cond); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		if out[rel.From] == nil {
			out[rel.From] = map[string]bool{}
		}
		out[rel.From][rel.To] = true
	}
	return out, nil
}

// OutgoingFrom implements the acl edge batcher with ONE store query. Like
// [Graph.OutgoingRelations] it reads every tail.
func (g *Graph) OutgoingFrom(
	ctx context.Context, relType string, fromIDs []string,
) (map[string][]string, error) {
	out := map[string][]string{}
	if len(fromIDs) == 0 {
		return out, nil
	}
	for rel, err := range g.s.ListRelations(ctx, store.RelationQuery{
		Type:      relType,
		EntityIDs: fromIDs,
		Direction: store.DirectionOutgoing,
	}) {
		if err != nil {
			return nil, err
		}
		out[rel.From] = append(out[rel.From], rel.To)
	}
	return out, nil
}

// matchEdge reports whether rel's own properties satisfy cond, with the
// semantics a [store.RelationPredicate.EdgeProps] query has. nil cond
// matches every edge. It is the store's own matcher, so the Go probes and
// the row gate agree on every value, an unquoted YAML date included.
func matchEdge(rel *entity.Relation, cond []store.PropPredicate) (bool, error) {
	if len(cond) == 0 {
		return true, nil
	}
	return graphquerynaive.MatchProps(rel.Properties, cond)
}
