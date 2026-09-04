package acl

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PrincipalLookup resolves a raw principal string to entity ids by a
// natural-key property (`principal_property` on `user_entity_type`).
// Returned ids are in store order; the caller treats more than one match
// as an ambiguous key.
type PrincipalLookup interface {
	LookupEntityByProperty(ctx context.Context, entityType, property, value string) ([]string, error)
}

// PrincipalStore is the store capability the lookup needs: a graph query
// with the property equality PUSHED DOWN. Consumer-side on purpose — the
// lookup must never fall back to scanning a whole entity type in Go, which
// is what it did before TKT-1U8XYN: every request began by loading every
// user entity with its body to compare one string, a ~25 ms floor under
// each API call on the postgres backend regardless of what the request
// did. store.Store satisfies this.
type PrincipalStore interface {
	store.GraphQueryer
}

type storePrincipalLookup struct {
	s PrincipalStore
}

// NewStorePrincipalLookup adapts a store to [PrincipalLookup].
func NewStorePrincipalLookup(s PrincipalStore) PrincipalLookup {
	return &storePrincipalLookup{s: s}
}

// LookupEntityByProperty runs one scalar-equality graph query. The
// predicate is marked Scalar so the postgres backend emits the
// `(properties ->> $p) = $v` shape its derived unique index (the
// `unique: true` the policy loader requires on this property) serves
// directly; fs/mem evaluate the same predicate over their in-memory rows.
func (l *storePrincipalLookup) LookupEntityByProperty(
	ctx context.Context, entityType, property, value string,
) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	var ids []string
	// Headers, not rows: the lookup wants ids and nothing else, and the user
	// entity may carry a body the resolver has no business reading.
	for h, err := range store.GraphQueryHeaders(ctx, l.s, store.GraphQuery{
		EntityType: entityType,
		Props:      []store.PropPredicate{{Property: property, Op: store.PropEqual, Value: value, Scalar: true}},
	}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, h.ID)
	}
	return ids, nil
}
