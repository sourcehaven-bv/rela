package pgstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

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
// fields search.MatchText considers. The trgm GIN index accelerates the LIKE.
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
// insensitive substring), ordered by trigram distance between the query and
// the title (nearest first; see [SearchTitles]) then by ID for stable ties.
// limit <= 0 means no limit.
//
// An empty query matches every entity — but in practice the Service never
// calls Search with empty text (it uses listAll), so this just stays
// consistent with substring semantics ("" is a substring of everything).
func (b *SearchBackend) Search(text string, limit int, w store.WorldScope) ([]search.Face, error) {
	if !w.IsSet() {
		return nil, fmt.Errorf("%w: search with an unset world", store.ErrInvalidQuery)
	}
	needle := strings.ToLower(text)
	sql, args := buildSearchSQL(needle, limit, w)

	// context.Background(): the search.Backend.Search interface carries no
	// context (see internal/search/types.go), so a search query can't inherit a
	// request deadline/cancellation. Threading ctx through is a search-package
	// interface change tracked separately.
	rows, err := b.db.Query(context.Background(), sql, append([]any{searchQueryMode}, args...)...)
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
	if w.IsTrivial() {
		f.Via = search.RuleUnscoped
		return f
	}
	res, scoped := w.For(entityType)
	if !scoped {
		f.Via = search.RuleUnscoped
		return f
	}
	if p.IsImplicit() && rank >= len(res.Chain) {
		f.Via = search.RuleFallbackDefault
		return f
	}
	f.Via = search.RuleChain
	f.ChainPosition = rank
	return f
}

// buildSearchSQL renders the text search, scoped to world w.
//
// # Ranking reads an index
//
// Hits are ordered by trigram distance between the needle and the row's
// search_title (see [SearchTitles]), then by id. A GiST index on
// search_title returns rows in that order, so for a word most rows contain
// PostgreSQL reads the nearest rows first, tests each against the text and
// world conditions, and stops at the LIMIT. Ranking every match and then
// sorting grew with the data: the scale run's search word matched 85% of the
// rows (TKT-M58EQ1W). For a rare word the planner keeps the trigram index on
// search_text and sorts the few matches. Which plan wins depends on the
// needle, so the statement is planned per execution ([searchQueryMode]).
//
// # Two shapes, chosen by world
//
// The DEFAULT world tests for the empty face, so a project that never declares a
// face pays nothing for faces. That is the [store.WorldScope.IsTrivial]
// fast-path contract.
//
// A non-default world resolves per FAMILY, not per row: the hit for an
// entity is its prime, the candidate face the world ranks first, and the
// prime's own text must match. Filtering on text first would let a non-prime
// face's text decide whether the entity appears. See [worldPrimeSQL].
//
// # Lockstep
//
// visiblesearch.go's buildVisibleSearchSQL holds the gated and ungated
// streams to an ordered-subsequence contract, so the ORDER BY here and there
// must agree. Both order by [searchDistanceSQL], then id.
func buildSearchSQL(needle string, limit int, w store.WorldScope) (sqlText string, args []any) {
	bind := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	// search_text is already lowercased by the store, so a plain LIKE with
	// the lowercased needle is case-insensitive without per-row lower()
	// calls. '%' and '_' in the needle are escaped so they match literally.
	where := []string{likeSQL(bind(escapeLike(needle)))}
	from, rank := "entities e", "0"
	if w.IsTrivial() {
		where = append(where, "e.face = ''")
	} else {
		var prime []string
		from, rank, prime = worldPrimeSQL(w, "", &args)
		where = append(where, prime...)
	}
	sqlText = "SELECT e.id, e.face, e.type, " + rank + " FROM " + from + " WHERE " + strings.Join(where, " AND ")
	if needle == "" {
		sqlText += " ORDER BY e.id ASC"
	} else {
		sqlText += " ORDER BY " + searchDistanceSQL(bind(needle)) + ", e.id ASC"
	}
	if limit > 0 {
		sqlText += " LIMIT " + bind(limit)
	}
	return sqlText, args
}

// likeSQL is the text match on alias e; needle is the placeholder of the
// escaped, lowercased needle.
func likeSQL(needle string) string {
	return "e.search_text LIKE '%' || " + needle + ` || '%' ESCAPE '\'`
}

// searchDistanceSQL is the ranking expression both search builders order by,
// ascending; they must agree (see buildVisibleSearchSQL's ordered-subsequence
// contract), which is why one place writes it. needle is the placeholder of
// the lowercased needle. It must stay in the form `column <-> value`, or the
// GiST index cannot return rows in its order.
func searchDistanceSQL(needle string) string {
	return "(e.search_title <-> " + needle + ")"
}

// worldPrimeSQL renders the shape both search builders share for a
// non-default world: the FROM clause, the row's world rank (its rule chain
// position, see [faceFor]) and the WHERE conjuncts that keep a row only when
// it is the prime of its entity.
//
// vis is an extra condition a candidate must meet, written against alias e:
// the gated search's visibility clause, or "" for none. A face the caller may
// not read is then not a candidate, and the next readable face is served
// instead (TKT-7IZHP0).
//
// The prime test is an anti-join: a row is the prime when no candidate face
// of its entity ranks before it, that is, has a lower world rank, or the same
// rank and a smaller face. That is the DISTINCT ON order the entity listings
// use. Unlike DISTINCT ON it is a test on each row, so the distance-ordered
// scan can stop at the limit; DISTINCT ON had to resolve every matching
// family first. Both sides are written against alias e, because the
// visibility clause is: the inner query's e deliberately shadows the outer
// one, so the clause tests the sibling face. Renaming either alias would
// silently test the outer row instead. The row's own values reach the inner
// query through the lateral alias k, which PostgreSQL folds away. A scalar `face = (SELECT
// ...)` would say the same, but PostgreSQL estimates such a condition to
// keep almost no rows and then never picks the ordered scan.
func worldPrimeSQL(w store.WorldScope, vis string, args *[]any) (from, rank string, where []string) {
	rank, candidate := worldSQL(w, "e", args)
	cond := candidate
	if vis != "" {
		cond += " AND (" + vis + ")"
	}
	from = "entities e CROSS JOIN LATERAL (SELECT e.id AS id, e.face AS face, (" + rank + ") AS wrank) k"
	where = []string{cond, "NOT EXISTS (SELECT 1 FROM entities e WHERE e.id = k.id AND " + cond +
		" AND ((" + rank + ") < k.wrank OR ((" + rank + ") = k.wrank AND e.face < k.face)))"}
	return from, "k.wrank", where
}

// searchQueryMode makes pgx run a search without a cached prepared
// statement, so PostgreSQL plans it with the needle's value every time. A
// generic plan cannot tell a word most rows contain from one that no row
// does, and the two need opposite plans (see buildSearchSQL).
const searchQueryMode = pgx.QueryExecModeExec

// SearchTitles maps an entity type to the property holding its display
// title. It is plain configuration handed in by the composition root, which
// derives it from the metamodel; this package must not import that.
//
// Free-text ranking is the trigram distance between the needle and the row's
// lowercased title (TKT-U9DYW4), kept in entities.search_title so an index
// can serve the order (TKT-M58EQ1W). Matching is unchanged, the LIKE over
// all of search_text, so every row that mentions the term is still found. A
// row whose type has no entry, or whose title is unset, ranks by its id.
// [Open] records the map in the database with [SyncSearchTitles]; types and
// property names reach SQL as bound parameters, never interpolated.
type SearchTitles map[string]string

// Close releases backend resources. The handle is owned by the wiring layer,
// so there is nothing to close here.
func (b *SearchBackend) Close() error { return nil }

// escapeLike escapes LIKE wildcards so the needle matches literally. The query
// uses ESCAPE '\'.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
