package acl

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StoreGraph adapts a store.Store to the acl.Graph interface. Used by
// appbuild (and any other wiring site that already holds a Store) to
// give the resolver its read-side access to the relation index.
//
// Errors from the store iterator are surfaced via the (*StoreGraph)
// methods' error returns. The resolver decides what to do — abort the
// walk loud, in practice (see resolver.go).
type StoreGraph struct {
	S store.Store
}

// NewStoreGraph constructs a StoreGraph backed by s.
func NewStoreGraph(s store.Store) *StoreGraph { return &StoreGraph{S: s} }

// HasEdge reports whether the specific (from, relType, to) edge exists.
// A missing edge ([store.ErrNotFound]) reports false silently. Any
// other error (transient backend failure, context cancelled, etc.)
// also reports false BUT is logged at [slog.Warn] with operator-facing
// context (RR-K3OO) — without the log a pgx hiccup during ACL
// evaluation silently undercounts a member's role-relations and
// produces spurious denies the operator can't trace.
//
// Implementation uses the store's single-key GetRelation rather than
// scanning ListRelations (RR-L3VO): in the ACL resolver this is
// called O(|RoleRelations| × |Members| × |ancestors|) times per
// request, and scanning the full outgoing-by-relType list per call
// is a quadratic foot-cannon on densely-connected nodes.
func (g *StoreGraph) HasEdge(ctx context.Context, from, relType, to string) bool {
	_, err := g.S.GetRelation(ctx, from, relType, to)
	if err == nil {
		return true
	}
	if !errors.Is(err, store.ErrNotFound) {
		slog.Warn("acl: StoreGraph.HasEdge: backend error treated as no-edge",
			"from", from, "rel_type", relType, "to", to, "error", err)
	}
	return false
}

// OutgoingRelations returns the toIDs reachable from `fromID` via
// `relType`. Errors propagate; callers (the resolver) abort the
// surrounding walk rather than silently undercount.
func (g *StoreGraph) OutgoingRelations(ctx context.Context, from, relType string) ([]string, error) {
	var out []string
	for r, err := range g.S.ListRelations(ctx, store.RelationQuery{
		EntityID:  from,
		Direction: store.DirectionOutgoing,
		Type:      relType,
	}) {
		if err != nil {
			return out, err
		}
		out = append(out, r.To)
	}
	return out, nil
}
