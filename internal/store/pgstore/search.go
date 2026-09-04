package pgstore

import (
	"context"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

// EntityRenamed is a no-op: RenameEntity rewrites the row's id and
// recomputes search_text in the same transaction, so the new ID is
// searchable and the old one gone without this backend touching anything.
func (b *SearchBackend) EntityRenamed(string, *entity.Entity) error { return nil }

// Search returns entity IDs whose search_text contains the query (case-
// insensitive substring), ordered by trigram similarity to the query (best
// first) then by ID for stable ties. limit <= 0 means no limit.
//
// An empty query matches every entity — but in practice the Service never
// calls Search with empty text (it uses listAll), so this just stays
// consistent with substring semantics ("" is a substring of everything).
func (b *SearchBackend) Search(text string, limit int, w store.WorldScope) ([]search.Face, error) {
	needle := strings.ToLower(text)
	sql, args := buildSearchSQL(needle, limit, w)

	// context.Background(): the search.Backend.Search interface carries no
	// context (see internal/search/types.go), so a search query can't inherit a
	// request deadline/cancellation. Threading ctx through is a search-package
	// interface change tracked separately.
	rows, err := b.db.Query(context.Background(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []search.Face
	for rows.Next() {
		var (
			id   string
			ptr  string
			typ  string
			rank int
		)
		if err := rows.Scan(&id, &ptr, &typ, &rank); err != nil {
			return nil, err
		}
		out = append(out, faceFor(id, typ, entity.Face(ptr), rank, w))
	}
	return out, rows.Err()
}

// faceFor labels a resolved row with the rule that produced it, from the
// same rank [worldSQL] computed — no second chain walk, and no second
// implementation of the semantics to drift from the SQL.
//
// The mapping is total over what the query can return, because worldSQL
// emits exactly three shapes:
//
//   - the world is default, or the type is unscoped: rank 0 on the default
//     face, which is rule 1;
//   - a chain coordinate at position i: rank i, which is rule 2 — and i IS
//     the chain position a caller needs to tell the world's first choice
//     from a later candidate standing in for it;
//   - the `otherwise: default` last resort: rank len(chain) on the default
//     face, which is rule 3.
//
// Rules 1 and 3 both carry the DEFAULT face and are separated by whether
// the type is scoped at all — the same distinction [store.WorldScope.For]
// makes with its ok result.
func faceFor(id, entityType string, p entity.Face, rank int, w store.WorldScope) search.Face {
	f := search.Face{ID: id, Face: p}
	if w.IsDefaultWorld() {
		f.Via = search.RuleUnscoped
		return f
	}
	res, scoped := w.For(entityType)
	if !scoped {
		f.Via = search.RuleUnscoped
		return f
	}
	if p.IsDefault() && rank >= len(res.Chain) {
		f.Via = search.RuleFallbackDefault
		return f
	}
	f.Via = search.RuleChain
	f.ChainPosition = rank
	return f
}

// buildSearchSQL renders the text search, scoped to world w.
//
// # Two shapes, chosen by world
//
// The DEFAULT world keeps the historical query verbatim — `face = ”`, no
// join, no window — so a project that never declares a face pays nothing
// for this feature. That is the [store.WorldScope.IsDefaultWorld] fast-path
// contract.
//
// A non-default world resolves per FAMILY, not per row. `face IN (a, b)`
// would return two rows for an entity holding both coordinates and break the
// at-most-one-hit-per-entity invariant that makes `limit` count entities, so
// the shape is `DISTINCT ON (id) ... ORDER BY id, rank` — the same shape the
// entity listings use, via the same [worldSQL] builder. Sharing that builder
// is what keeps search and listing from disagreeing about which face a world
// serves.
//
// # Resolve first, filter second
//
// The text predicate is applied AFTER the prime is chosen, in the outer
// query. Filtering first would let a non-prime face's text decide whether its
// entity appears: a `published`-world search would hit on a term present only
// in the draft while displaying published bytes that lack it. Selecting the
// prime and then testing ITS text is the whole point of world-scoped search.
//
// # Lockstep
//
// visiblesearch.go's buildVisibleSearchSQL holds the gated and ungated
// streams to an ordered-subsequence contract, so the ORDER BY here and there
// must agree.
func buildSearchSQL(needle string, limit int, w store.WorldScope) (sqlText string, args []any) {
	// search_text is already lowercased by the store, so a plain LIKE with
	// the lowercased needle is case-insensitive without per-row lower()
	// calls. '%' and '_' in the needle are escaped so they match literally.
	textPred := func(col string) string {
		args = append(args, escapeLike(needle))
		return col + ` LIKE '%' || $` + strconv.Itoa(len(args)) + ` || '%' ESCAPE '\'`
	}

	if w.IsDefaultWorld() {
		sqlText = `SELECT id, face, type, 0 FROM entities WHERE ` +
			textPred("search_text") + ` AND face = ''`
	} else {
		rank, candidate := worldSQL(w, "", &args)
		inner := `SELECT DISTINCT ON (id) id, face, type, search_text, (` + rank + `) AS wrank` +
			` FROM entities WHERE ` + candidate +
			` ORDER BY id ASC, (` + rank + `) ASC, face ASC`
		sqlText = `SELECT id, face, type, wrank FROM (` + inner + `) p WHERE ` + textPred("p.search_text")
	}

	if needle == "" {
		// Avoid similarity() on an empty string; just order by id.
		sqlText += ` ORDER BY id ASC`
	} else {
		args = append(args, needle)
		sqlText += ` ORDER BY similarity(search_text, $` + strconv.Itoa(len(args)) + `) DESC, id ASC`
	}
	if limit > 0 {
		args = append(args, limit)
		sqlText += ` LIMIT $` + strconv.Itoa(len(args))
	}
	return sqlText, args
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
