package sqlitestore

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Pagination is keyset-based, not OFFSET-based: the cursor carries the last
// sort key seen and the next page resumes strictly after it. That keeps a page
// stable under concurrent inserts and avoids OFFSET's linear scan.
//
// The sort key is the primary key in both cases (entity id; relation triple),
// so the ordering the cursor assumes is the ordering the index already
// provides.

// ListEntitiesPage implements [store.EntityReader].
func (s *Store) ListEntitiesPage(
	ctx context.Context, q store.EntityQuery,
) (store.Page[*entity.Entity], error) {
	return queryEntityPage(ctx, s.q(), q, entityColumns, scanEntity, func(e *entity.Entity) entity.Ref {
		return entity.Ref{ID: e.ID, Face: e.Face}
	})
}

var _ store.HeaderPager = (*Store)(nil)

// ListEntityHeadersPage implements [store.HeaderPager]: the page
// ListEntitiesPage reads, with the content column left out of the SELECT.
func (s *Store) ListEntityHeadersPage(
	ctx context.Context, q store.EntityQuery,
) (store.Page[store.EntityHeader], error) {
	return queryEntityPage(ctx, s.q(), q, entityHeaderColumns, scanEntityHeader,
		func(h store.EntityHeader) entity.Ref { return entity.Ref{ID: h.ID, Face: h.Face} })
}

// queryEntityPage reads q's keyset page of columns, scanning each row with
// scan. key names a row's state, from which the next cursor is made.
func queryEntityPage[T any](
	ctx context.Context, qr querier, q store.EntityQuery, columns string,
	scan func(scanner) (T, error), key func(T) entity.Ref,
) (store.Page[T], error) {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return store.Page[T]{}, err
	}
	sqlText, args, err := entityPageSQL(q, columns)
	if err != nil {
		return store.Page[T]{}, err
	}

	rows, err := qr.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return store.Page[T]{}, fmt.Errorf("sqlitestore: list entities page: %w", err)
	}
	defer rows.Close()

	items := make([]T, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return store.Page[T]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return store.Page[T]{}, fmt.Errorf("sqlitestore: list entities page: %w", err)
	}

	var next string
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[:q.Limit]
		last := key(items[q.Limit-1])
		// The cursor is the STATE key so AllFaces pagination resumes
		// mid-family; for default-only queries it degenerates to the
		// historical bare id.
		next = storeutil.EncodeCursor(entity.FormatStateRef(last.ID, last.Face))
	}
	return store.Page[T]{Items: items, NextCursor: next}, nil
}

// ListRelationsPage implements [store.RelationReader].
//
// The cursor encodes the full triple, because no single column is unique — two
// relations can share a from_id, and the ordering is (from_id, rel_type,
// to_id). Resuming needs the whole key, compared as a row value so the
// comparison matches the ORDER BY exactly.
func (s *Store) ListRelationsPage(
	ctx context.Context, q store.RelationQuery,
) (store.Page[*entity.Relation], error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Relation]{}, err
	}

	if cursorKey != "" {
		if _, _, _, _, ok := splitRelationKey(cursorKey); !ok {
			return store.Page[*entity.Relation]{},
				fmt.Errorf("sqlitestore: invalid relation cursor %q", q.Cursor)
		}
	}
	sqlText, args := buildRelationQueryFrom(q, cursorKey)
	if q.Limit > 0 {
		sqlText += limitClause(q.Limit + 1)
	}

	rows, err := s.q().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return store.Page[*entity.Relation]{}, fmt.Errorf("sqlitestore: list relations page: %w", err)
	}
	defer rows.Close()

	items := make([]*entity.Relation, 0)
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return store.Page[*entity.Relation]{}, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return store.Page[*entity.Relation]{}, fmt.Errorf("sqlitestore: list relations page: %w", err)
	}

	var next string
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[:q.Limit]
		last := items[q.Limit-1]
		next = storeutil.EncodeCursor(relationKey(last.From, last.FromFace, last.Type, last.To))
	}
	return store.Page[*entity.Relation]{Items: items, NextCursor: next}, nil
}

// relationKey and splitRelationKey encode a relation's full identity as one
// cursor key. The tail face joins it because the tail is part of that identity
// (TKT-DOFYR1): a triple can carry one edge per face, and the ordering is
// (from_id, from_face, rel_type, to_id), so resuming needs all four.
//
// The separator is a unit separator (0x1F), which storeutil.ValidateID,
// ValidateRelationType and the entity.Face grammar all reject in their inputs —
// so it cannot occur inside a component and the split is unambiguous.
const relationKeySep = "\x1f"

// relationKeyParts is how many components relationKey joins: from, tail face,
// type, to.
const relationKeyParts = 4

func relationKey(from string, face entity.Face, relType, to string) string {
	return from + relationKeySep + string(face) + relationKeySep + relType + relationKeySep + to
}

func splitRelationKey(key string) (from, face, relType, to string, ok bool) {
	parts := strings.Split(key, relationKeySep)
	if len(parts) != relationKeyParts {
		return "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], parts[3], true
}

// entityPageSQL is the query [Store.ListEntitiesPage] and
// [Store.ListEntityHeadersPage] send for q, selecting columns, cursor
// decoded.
//
// It fetches one extra row to learn whether a further page exists, without
// a second COUNT query. Under a world the rows are already PRIMES — the
// window resolves each family before LIMIT applies — so limit+1 counts
// entities rather than candidate state rows.
func entityPageSQL(q store.EntityQuery, columns string) (sqlText string, args []any, err error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return "", nil, err
	}
	sqlText, args = buildEntitySelectSQL(q, cursorKey, columns)
	if q.Limit > 0 {
		sqlText += limitClause(q.Limit + 1)
	}
	return sqlText, args, nil
}
