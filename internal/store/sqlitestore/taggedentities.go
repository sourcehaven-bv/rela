package sqlitestore

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// taggedEntitiesSQL is the shared query (see storeutil.TaggedEntitiesSQL) in
// SQLite bind syntax. Numbered binds let the type filter appear twice.
var taggedEntitiesSQL = storeutil.TaggedEntitiesSQL("?1", "?2")

// TaggedEntities implements [store.VersionTagIndex] (TKT-7LRUPB).
func (v *VersionStore) TaggedEntities(
	ctx context.Context, name store.VersionTagName, entityType string,
) ([]store.TaggedEntity, error) {
	if name.IsZero() {
		return nil, fmt.Errorf("%w: missing tag name", store.ErrInvalidVersionTag)
	}
	rows, err := v.db.QueryContext(ctx, taggedEntitiesSQL, name.String(), entityType)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list tagged entities: %w", err)
	}
	defer rows.Close()
	var out []store.TaggedEntity
	for rows.Next() {
		var (
			id, face, typ string
			version       int
			deleted       bool
		)
		if err := rows.Scan(&id, &face, &typ, &version, &deleted); err != nil {
			return nil, fmt.Errorf("sqlitestore: list tagged entities: %w", err)
		}
		out = append(out, storeutil.TaggedEntityRow(id, face, typ, version, deleted))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: list tagged entities: %w", err)
	}
	return out, nil
}

var _ store.VersionTagIndex = (*VersionStore)(nil)
