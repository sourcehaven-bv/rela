package aclstore

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PrincipalLookup implements acl.PrincipalLookup over a store.
//
// It needs exactly the graph-query surface store.GraphQueryHeaders takes,
// so the property equality is PUSHED DOWN and only headers come back. The
// lookup must never fall back to scanning a whole entity type in Go, which
// is what it did before TKT-1U8XYN: every request began by loading every
// user entity with its body to compare one string, a ~25 ms floor under
// each API call on the postgres backend regardless of what the request
// did.
type PrincipalLookup struct {
	s store.GraphQueryer
}

// NewPrincipalLookup constructs a PrincipalLookup backed by s.
//
// Nil: rejected with an error; a lookup with no store has nothing to query.
func NewPrincipalLookup(s store.GraphQueryer) (*PrincipalLookup, error) {
	if s == nil {
		return nil, errors.New("aclstore: NewPrincipalLookup: store is nil")
	}
	return &PrincipalLookup{s: s}, nil
}

// MustNewPrincipalLookup is [NewPrincipalLookup] for tests and fixtures: it
// panics on nil.
func MustNewPrincipalLookup(s store.GraphQueryer) *PrincipalLookup {
	l, err := NewPrincipalLookup(s)
	if err != nil {
		panic(err)
	}
	return l
}

// LookupEntityByProperty runs one scalar-equality graph query. The
// predicate is marked Scalar so the postgres backend emits the
// `(properties ->> $p) = $v` shape its derived unique index (the
// `unique: true` the policy loader requires on this property) serves
// directly; fs/mem evaluate the same predicate over their in-memory rows.
//
// Returned ids are in store order; the caller treats more than one match
// as an ambiguous key.
func (l *PrincipalLookup) LookupEntityByProperty(
	ctx context.Context, entityType, property, value string,
) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	var ids []string
	// Headers, not rows: the lookup wants ids and nothing else, and the user
	// entity may carry a body the resolver has no business reading.
	//
	// The implicit face, exactly: a faced user type is a load error
	// (acl.Policy.ValidateAgainstMetamodel), so a principal has one row and
	// one identity whatever world a request reads in.
	for h, err := range store.GraphQueryHeaders(ctx, l.s, store.GraphQuery{
		EntityType: entityType,
		Faces:      store.AtFaces(entity.ImplicitFace),
		Props:      []store.PropPredicate{{Property: property, Op: store.PropEqual, Value: value, Scalar: true}},
	}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, h.ID)
	}
	return ids, nil
}
