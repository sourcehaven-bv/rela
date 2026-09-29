package store

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// FamilyHeaders returns the header of every stored face of id, raw, in the
// order the backend lists them. An id with no row yields an empty slice and
// no error.
//
// This is the one "read every face of an id" query: an IDs-scoped listing
// over all faces, never a scan. It returns raw storage truth, so a read-out
// path gates the result (visibility.Resolver.Family) before serving any of
// it.
//
// A row whose id is not exactly id is dropped. A backend may match ids
// case-insensitively (fsstore on a case-folding filesystem), and a family is
// the exact id.
func FamilyHeaders(ctx context.Context, r EntityLister, id string) ([]EntityHeader, error) {
	var out []EntityHeader
	for h, err := range ListEntityHeaders(ctx, r, familyQuery(id)) {
		if err != nil {
			return nil, err
		}
		if h.ID == id {
			out = append(out, h)
		}
	}
	return out, nil
}

// Family is [FamilyHeaders] with bodies: every stored face row of id, raw.
// Use it only when the caller needs the rows themselves; a type or face
// lookup wants the headers.
func Family(ctx context.Context, r EntityLister, id string) ([]*entity.Entity, error) {
	var out []*entity.Entity
	for e, err := range r.ListEntities(ctx, familyQuery(id)) {
		if err != nil {
			return nil, err
		}
		if e.ID == id {
			out = append(out, e)
		}
	}
	return out, nil
}

// familyQuery selects every face of one id.
func familyQuery(id string) EntityQuery {
	return EntityQuery{IDs: []string{id}, AllStates: true}
}
