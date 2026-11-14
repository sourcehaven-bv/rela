package visibility

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// errRelationPageLimit reports a relation page read without a positive
// Limit. A page read never returns every edge, so Limit 0 (unbounded on
// [store.RelationReader.ListRelationsPage]) is refused here.
var errRelationPageLimit = errors.New("visibility: a relation page needs a positive limit")

// ListRelationsPage reads up to q.Limit relations of q whose BOTH endpoints
// the caller may read, as [ScriptReader.ListRelations] gates them
// (TKT-P9QMEVI).
//
// The store has no relation gate to push down (TKT-OGL697S), so the page is
// filled from raw keyset pages, each peer-gated in one strict batch. A gate
// fault is an error, not a thinned page: a short page would read as a
// complete one. The first raw page asks for exactly q.Limit rows, so a
// caller that may read every edge pays one store read. Each refill asks for
// at least headerGateChunk rows, so hidden edges cost one store read per
// chunk, not one per edge.
//
// NextCursor never follows a hidden row: a cursor encodes the key of the
// row it follows. When the page fills before the end of a raw page, the
// cursor comes from re-reading that raw page up to the last row served.
// NextCursor is empty only when the store has no more rows. A non-empty
// one means more raw rows exist, not that a visible one does: the next
// page may be empty, so a caller must not present it as "has more".
//
// q.Limit must be positive. q.Cursor resumes a previous page of the same
// query.
func (s *ScriptReader) ListRelationsPage(
	ctx context.Context, q store.RelationQuery,
) (store.Page[*entity.Relation], error) {
	if q.Limit <= 0 {
		return store.Page[*entity.Relation]{}, errRelationPageLimit
	}
	bound := s.bind(ctx)
	want := q.Limit
	out := make([]*entity.Relation, 0, min(want, headerGateChunk))
	q.Limit = want // the first raw page is exact; refills grow below
	for {
		raw, err := s.raw.ListRelationsPage(bound, q)
		if err != nil {
			return store.Page[*entity.Relation]{}, err
		}
		kept, err := s.reader.FilterRelationsStrict(bound, raw.Items)
		if err != nil {
			return store.Page[*entity.Relation]{}, err
		}
		if need := want - len(out); len(kept) >= need {
			out = append(out, kept[:need]...)
			next, err := s.cursorAfter(bound, q, raw, kept[need-1])
			if err != nil {
				return store.Page[*entity.Relation]{}, err
			}
			return store.Page[*entity.Relation]{Items: out, NextCursor: next}, nil
		}
		out = append(out, kept...)
		switch {
		case raw.NextCursor == "":
			return store.Page[*entity.Relation]{Items: out}, nil
		case raw.NextCursor == q.Cursor || len(raw.Items) == 0:
			// A store that hands back the cursor it was given, or an empty
			// page with a cursor, would loop forever.
			return store.Page[*entity.Relation]{}, fmt.Errorf(
				"visibility: relation page did not advance (cursor %q)", q.Cursor)
		}
		q.Cursor = raw.NextCursor
		q.Limit = max(want-len(out), headerGateChunk)
	}
}

// cursorAfter returns the cursor that resumes q's raw page right after
// last, a row of raw. When last is the final row of raw, that is raw's own
// cursor. Otherwise the raw page is read again up to last, because a cursor
// is opaque and cannot be built here. A page that changed in between, so
// the re-read no longer ends on last, is an error rather than a cursor that
// could follow a hidden row.
func (s *ScriptReader) cursorAfter(
	ctx context.Context, q store.RelationQuery, raw store.Page[*entity.Relation], last *entity.Relation,
) (string, error) {
	i := slices.Index(raw.Items, last)
	if i == len(raw.Items)-1 {
		return raw.NextCursor, nil
	}
	q.Limit = i + 1
	again, err := s.raw.ListRelationsPage(ctx, q)
	if err != nil {
		return "", err
	}
	if len(again.Items) != i+1 || again.Items[i].Key() != last.Key() {
		return "", errors.New("visibility: relations changed while a page was read; read it again")
	}
	return again.NextCursor, nil
}

// ListRelationsPage implements the script read surface: one raw page.
func (r *UnrestrictedReader) ListRelationsPage(
	ctx context.Context, q store.RelationQuery,
) (store.Page[*entity.Relation], error) {
	if q.Limit <= 0 {
		return store.Page[*entity.Relation]{}, errRelationPageLimit
	}
	return r.st.ListRelationsPage(ctx, q)
}

// ListEntitiesPage implements the elevated read surface: one raw page.
// q.Limit must be positive, as on [UnrestrictedReader.ListRelationsPage].
func (r *UnrestrictedReader) ListEntitiesPage(
	ctx context.Context, q store.EntityQuery,
) (store.Page[*entity.Entity], error) {
	if q.Limit <= 0 {
		return store.Page[*entity.Entity]{}, errors.New("visibility: an entity page needs a positive limit")
	}
	return r.st.ListEntitiesPage(ctx, q)
}

// ListRelationsPage implements the script read surface: always refuses.
func (DenyReader) ListRelationsPage(context.Context, store.RelationQuery) (store.Page[*entity.Relation], error) {
	return store.Page[*entity.Relation]{}, ErrReaderUnavailable
}

// ListEntitiesPage implements the elevated read surface: always refuses.
func (DenyReader) ListEntitiesPage(context.Context, store.EntityQuery) (store.Page[*entity.Entity], error) {
	return store.Page[*entity.Entity]{}, ErrReaderUnavailable
}
