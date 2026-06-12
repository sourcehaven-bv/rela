package acl

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Graph is the narrow contract the resolver needs from the store.
// Declared at the consumer per CLAUDE.md's "interfaces at the call
// site" rule; the wiring site (appbuild) supplies a store-backed
// implementation.
//
// Two operations:
//
//   - HasEdge reports whether a specific edge fromID --relType--> toID
//     exists. Used in per-entity local-role probes.
//   - OutgoingRelations returns the toIDs reachable from `fromID` via
//     edges of `relType`. Used in transitive walks (member-of for
//     groups, inherit_roles_through for containment).
//
// Both methods are read-only and snapshot-coherent for the duration
// of one Request; cross-request consistency is not guaranteed (and is
// not load-bearing — see the TKT-SVXL "Trust boundary" section).
//
// TKT-SVXL note: an earlier iteration of this interface returned a
// `[]string` slice with no error. The architect review flagged that
// silent error-drop in production was a security-regression mechanism
// (members disappearing = grants disappearing). The current signature
// adds an explicit error return; consumers propagate or abort.
//
// Production wiring uses [StoreGraph] (a store.Store adapter). Tests
// that need a real graph use the same adapter over a memstore; tests
// that only exercise the legacy-Subject AuthorizeWrite path can use
// [NullGraph] — it answers "no edges" for every probe.
type Graph interface {
	// HasEdge reports whether the specific (from, relType, to) edge
	// exists. Returns false on backend error rather than propagating —
	// a missing edge and an error look the same to authz (deny by
	// absence). Operators see backend errors in the store's own logs.
	HasEdge(ctx context.Context, from, relType, to string) bool

	// OutgoingRelations returns the toIDs reachable from `fromID` via
	// `relType`, or an error if the backend lookup failed. Resolvers
	// abort the surrounding walk on error rather than under-counting —
	// under-counting members is safer than over-granting, but a
	// principal-resolution that proceeds with partial data is worse
	// than failing the request loud.
	OutgoingRelations(ctx context.Context, from, relType string) ([]string, error)
}

// NullGraph implements [Graph] with no edges. Intended for tests that
// construct a [*Declarative] but don't exercise group expansion,
// containment, or local-role probes — typically tests of the legacy
// AuthorizeWrite path that uses the EntityType/RelationType string
// shape instead of the [Subject] sum.
//
// Production wiring never uses NullGraph; it always supplies a real
// store-backed [StoreGraph].
type NullGraph struct{}

// HasEdge always returns false.
func (NullGraph) HasEdge(context.Context, string, string, string) bool { return false }

// OutgoingRelations always returns nil, nil.
func (NullGraph) OutgoingRelations(context.Context, string, string) ([]string, error) {
	return nil, nil
}

// NullGraphQueryer implements [store.GraphQueryer] with empty results.
// Intended for tests that construct a [*Declarative] but don't
// exercise [Request.PermitsRead] / [Request.PermitsReadMany] or the
// list-side ReadQuery path. Returns (matched=0, total=0), an empty
// iterator, and a "no match" verdict for every id probe.
//
// Production wiring never uses NullGraphQueryer; it always passes the
// store itself (which implements GraphQueryer).
type NullGraphQueryer struct{}

// GraphQuery returns an empty iterator.
func (NullGraphQueryer) GraphQuery(context.Context, store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return func(_ func(*entity.Entity, error) bool) {}
}

// GraphCount returns (0, 0, nil).
func (NullGraphQueryer) GraphCount(context.Context, store.GraphQuery) (matched, total int, err error) {
	return 0, 0, nil
}

// MatchingIDs returns a map with every input id mapped to false.
func (NullGraphQueryer) MatchingIDs(_ context.Context, _ store.GraphQuery, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = false
	}
	return out, nil
}
