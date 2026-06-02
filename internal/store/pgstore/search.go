package pgstore

import (
	"context"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// SearchBackend is a PostgreSQL-backed search.Backend. It shares the store's
// connection handle and queries the entities.search_text column directly, so
// it holds no derived state of its own.
//
// Because the indexed text lives in the same database as the entities (the
// store maintains search_text on every write), EntityPut/EntityDelete are
// no-ops: there is nothing to mirror. This is the "smart backend" case the
// store package doc anticipates.
//
// Search matches case-insensitive substrings over search_text, which the store
// builds from entity ID + content + string-valued properties — exactly the
// fields search.MatchText considers. The trgm GIN index accelerates the ILIKE.
// The Service layer (search.New) applies type/property filters and the result
// limit on top; this backend only maps text to candidate IDs.
type SearchBackend struct {
	db DBTX
}

// compile-time interface check.
var _ search.Backend = (*SearchBackend)(nil)

// NewSearchBackend builds a search backend over the same handle as the store.
func NewSearchBackend(db DBTX) *SearchBackend {
	return &SearchBackend{db: db}
}

// EntityPut is a no-op: the store already persists search_text on write.
func (b *SearchBackend) EntityPut(*entity.Entity) error { return nil }

// EntityDelete is a no-op: deleting the entity row removes it from search.
func (b *SearchBackend) EntityDelete(string) error { return nil }

// Search returns entity IDs whose search_text contains the query (case-
// insensitive substring), ordered by trigram similarity to the query (best
// first) then by ID for stable ties. limit <= 0 means no limit.
//
// An empty query matches every entity — but in practice the Service never
// calls Search with empty text (it uses listAll), so this just stays
// consistent with substring semantics ("" is a substring of everything).
func (b *SearchBackend) Search(text string, limit int) ([]string, error) {
	needle := strings.ToLower(text)

	// search_text is already lowercased by the store, so a plain LIKE with the
	// lowercased needle is case-insensitive without per-row lower() calls.
	// '%' and '_' in the needle are escaped so they match literally.
	sql := `SELECT id FROM entities WHERE search_text LIKE '%' || $1 || '%' ESCAPE '\'`
	args := []any{escapeLike(needle)}

	if needle == "" {
		// Avoid similarity() on an empty string; just order by id.
		sql += ` ORDER BY id ASC`
	} else {
		sql += ` ORDER BY similarity(search_text, $2) DESC, id ASC`
		args = append(args, needle)
	}
	if limit > 0 {
		sql += ` LIMIT $` + strconv.Itoa(len(args)+1)
		args = append(args, limit)
	}

	rows, err := b.db.Query(context.Background(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Close releases backend resources. The handle is owned by the wiring layer,
// so there is nothing to close here.
func (b *SearchBackend) Close() error { return nil }

// escapeLike escapes LIKE wildcards so the needle matches literally. The query
// uses ESCAPE '\'.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
