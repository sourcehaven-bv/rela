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
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Entity]{}, err
	}

	var (
		conds []string
		args  []any
	)
	if q.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, q.Type)
	}
	if len(q.IDs) > 0 {
		conds = append(conds, "id IN ("+placeholders(len(q.IDs))+")")
		for _, id := range q.IDs {
			args = append(args, id)
		}
	}
	if cursorKey != "" {
		conds = append(conds, "id > ?")
		args = append(args, cursorKey)
	}

	sqlText := `SELECT id, type, properties, content, updated_at FROM entities`
	if len(conds) > 0 {
		sqlText += ` WHERE ` + strings.Join(conds, " AND ")
	}
	sqlText += ` ORDER BY id`
	// Fetch one extra row to learn whether a further page exists, without a
	// second COUNT query.
	if q.Limit > 0 {
		sqlText += fmt.Sprintf(" LIMIT %d", q.Limit+1)
	}

	rows, err := s.q().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return store.Page[*entity.Entity]{}, fmt.Errorf("sqlitestore: list entities page: %w", err)
	}
	defer rows.Close()

	items := make([]*entity.Entity, 0)
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return store.Page[*entity.Entity]{}, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return store.Page[*entity.Entity]{}, fmt.Errorf("sqlitestore: list entities page: %w", err)
	}

	var next string
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[:q.Limit]
		next = storeutil.EncodeCursor(items[q.Limit-1].ID)
	}
	return store.Page[*entity.Entity]{Items: items, NextCursor: next}, nil
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
		if _, _, _, ok := splitRelationKey(cursorKey); !ok {
			return store.Page[*entity.Relation]{},
				fmt.Errorf("sqlitestore: invalid relation cursor %q", q.Cursor)
		}
	}
	sqlText, args := buildRelationQueryFrom(q, cursorKey)
	if q.Limit > 0 {
		sqlText += fmt.Sprintf(" LIMIT %d", q.Limit+1)
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
		next = storeutil.EncodeCursor(relationKey(last.From, last.Type, last.To))
	}
	return store.Page[*entity.Relation]{Items: items, NextCursor: next}, nil
}

// relationKey and splitRelationKey encode a relation triple as one cursor key.
// The separator is a unit separator (0x1F), which storeutil.ValidateID and
// ValidateRelationType both reject in their inputs — so it cannot occur inside
// a component and the split is unambiguous.
const relationKeySep = "\x1f"

func relationKey(from, relType, to string) string {
	return from + relationKeySep + relType + relationKeySep + to
}

func splitRelationKey(key string) (from, relType, to string, ok bool) {
	parts := strings.Split(key, relationKeySep)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// placeholders returns "?, ?, ..." for an IN clause of n values.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
