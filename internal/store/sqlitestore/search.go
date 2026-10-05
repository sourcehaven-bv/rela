package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SearchBackend is the search.Backend over the database's FTS5 index
// (DEC-10Z731). The index lives in the entity_search table, which triggers on
// the entities table keep current inside every write transaction (see
// sqlitedb's searchDDL), so this type holds no state of its own and its
// observer methods do nothing.
//
// It matches case-insensitive substrings of an entity's id, string-valued
// properties and body, the same text and semantics as pgstore's
// SearchBackend, so the two database builds find the same entities.
type SearchBackend struct {
	db     *sql.DB
	titles SearchTitles
}

var (
	_ search.Backend          = (*SearchBackend)(nil)
	_ search.AdmittingBackend = (*SearchBackend)(nil)
)

// minTrigramRunes is the shortest needle the trigram index can answer. A
// shorter one is matched with LIKE over the indexed text, which scans it.
const minTrigramRunes = 3

// NewSearchBackend returns the search backend over db.
//
// Nil: db is rejected.
func NewSearchBackend(db *sqlitedb.DB) (*SearchBackend, error) {
	if db == nil {
		return nil, errors.New("sqlitestore: nil database")
	}
	return &SearchBackend{db: db.DB()}, nil
}

// SearchTitles maps an entity type to the property holding its title. Search
// ranks an entity whose title matches the query above one that only mentions
// it. A type with no entry ranks by its id.
type SearchTitles map[string]string

// RankByTitles sets the title property per type and returns b.
func (b *SearchBackend) RankByTitles(t SearchTitles) *SearchBackend {
	b.titles = t
	return b
}

// EntityPut does nothing: the triggers index the row.
func (b *SearchBackend) EntityPut(*entity.Entity) error { return nil }

// EntityDelete does nothing: the triggers remove the row.
func (b *SearchBackend) EntityDelete(string) error { return nil }

// EntityRenamed does nothing: a rename updates the row's id in place, and the
// update trigger reindexes it.
func (b *SearchBackend) EntityRenamed(string, *entity.Entity) error { return nil }

// Close does nothing: the database belongs to the caller.
func (b *SearchBackend) Close() error { return nil }

// Search returns the faces whose text contains text, best title match first,
// resolved under w. An empty text matches every entity, in id order.
func (b *SearchBackend) Search(text string, limit int, w store.WorldScope) ([]search.Face, error) {
	return b.SearchAdmitted(text, limit, w, nil)
}

// SearchAdmitted implements [search.AdmittingBackend]: [SearchBackend.Search]
// with admit trimming the matched entities' families before the world ranks
// them. A nil admit is exactly Search.
//
// With admit, the families of every entity with a matching face are read and
// admitted in ONE call, the world picks each prime among the admitted faces,
// and a hit is kept only when that prime is itself a matching face. The hits
// keep the order of the ranked match query, so the result is an ordered
// subsequence of what the same query ranks without admission.
func (b *SearchBackend) SearchAdmitted(
	text string, limit int, w store.WorldScope, admit search.AdmitFunc,
) ([]search.Face, error) {
	if !w.IsSet() {
		return nil, fmt.Errorf("%w: search with an unset world", store.ErrInvalidQuery)
	}
	if admit != nil {
		return b.searchAdmitted(text, limit, w, admit)
	}
	sqlText, args := buildSearchSQL(text, limit, w, b.titles)
	rows, err := b.db.QueryContext(context.Background(), sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []search.Face
	for rows.Next() {
		var (
			id, face, typ string
			rank          int
		)
		if err := rows.Scan(&id, &face, &typ, &rank); err != nil {
			return nil, err
		}
		out = append(out, faceFor(id, typ, entity.Face(face), rank, w))
	}
	return out, rows.Err()
}

// searchAdmitted is SearchAdmitted with a non-nil admit.
func (b *SearchBackend) searchAdmitted(
	text string, limit int, w store.WorldScope, admit search.AdmitFunc,
) ([]search.Face, error) {
	ctx := context.Background()
	matchText, matchArgs := buildMatchedFacesSQL(text, b.titles)
	matched, err := b.queryCandidates(ctx, matchText, matchArgs)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, nil
	}

	familyText, familyArgs := buildFamiliesSQL(text)
	families, err := b.queryCandidates(ctx, familyText, familyArgs)
	if err != nil {
		return nil, err
	}
	admitted, err := admit(families)
	if err != nil {
		return nil, err
	}
	primes := search.ResolvePrimes(w, admitted)

	var out []search.Face
	for _, c := range matched {
		res, ok := primes[c.ID]
		if !ok || res.Face != c.Face {
			// The world serves another face of this entity, or none the
			// reader may see; a match on a non-prime face is not a hit.
			continue
		}
		out = append(out, search.Face{ID: c.ID, Face: res.Face, Via: res.Via, ChainPosition: res.ChainPosition})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// queryCandidates runs a query returning (id, face, type) rows.
func (b *SearchBackend) queryCandidates(ctx context.Context, sqlText string, args []any) ([]search.Candidate, error) {
	rows, err := b.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []search.Candidate
	for rows.Next() {
		var id, face, typ string
		if err := rows.Scan(&id, &face, &typ); err != nil {
			return nil, err
		}
		out = append(out, search.Candidate{ID: id, Type: typ, Face: entity.Face(face)})
	}
	return out, rows.Err()
}

// buildMatchedFacesSQL selects every face row whose own text contains text,
// in the order [buildSearchSQL] ranks primes. An empty text matches every
// row, in id order.
func buildMatchedFacesSQL(text string, titles SearchTitles) (sqlText string, args []any) {
	b := &sqlBuilder{}
	needle := strings.ToLower(text)
	all := `SELECT rowid AS rid, id, face, type, properties FROM entities`
	if needle == "" {
		return `SELECT id, face, type FROM (` + all + `) ORDER BY id, face`, b.args
	}
	matched := `SELECT id, face, type, ` + titleSQL(b, titles) + ` AS t FROM (` + all + `) p` +
		` WHERE p.rid IN (` + matchSQL(b, needle) + `)`
	ranked := `SELECT id, face, type, t, ` + titleRankSQL(b, needle) + ` AS r FROM (` + matched + `)`
	return `SELECT id, face, type FROM (` + ranked + `)` +
		` ORDER BY r DESC, CASE WHEN r > 0 THEN length(t) ELSE 0 END, id, face`, b.args
}

// buildFamiliesSQL selects every face of every entity that has a face whose
// text contains text: the whole families admission and ranking need.
func buildFamiliesSQL(text string) (sqlText string, args []any) {
	b := &sqlBuilder{}
	needle := strings.ToLower(text)
	if needle == "" {
		return `SELECT id, face, type FROM entities`, b.args
	}
	return `SELECT id, face, type FROM entities WHERE id IN (` +
		`SELECT id FROM entities WHERE rowid IN (` + matchSQL(b, needle) + `))`, b.args
}

// faceFor records which world rule chose a face, as pgstore's does.
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

// buildSearchSQL builds the search query.
//
// The world picks each entity's prime face FIRST and the text is matched
// against that face only, so a draft that mentions the needle does not make
// the published face a hit. That is pgstore's order too.
func buildSearchSQL(text string, limit int, w store.WorldScope, titles SearchTitles) (sqlText string, args []any) {
	b := &sqlBuilder{}
	needle := strings.ToLower(text)

	primes := `SELECT rowid AS rid, id, face, type, properties, 0 AS wrank FROM entities WHERE face = ''`
	if !w.IsTrivial() {
		rank, candidate := worldSQL(b, w, "")
		primes = `SELECT rid, id, face, type, properties, wrank FROM (` +
			`SELECT rowid AS rid, id, face, type, properties, (` + rank + `) AS wrank, ` +
			`ROW_NUMBER() OVER (PARTITION BY id ORDER BY (` + rank + `), face) AS rn ` +
			`FROM entities WHERE ` + candidate + `) WHERE rn = 1`
	}

	if needle == "" {
		sqlText = `SELECT id, face, type, wrank FROM (` + primes + `) ORDER BY id`
	} else {
		matched := `SELECT id, face, type, wrank, ` + titleSQL(b, titles) + ` AS t FROM (` + primes + `) p` +
			` WHERE p.rid IN (` + matchSQL(b, needle) + `)`
		ranked := `SELECT id, face, type, wrank, t, ` + titleRankSQL(b, needle) + ` AS r FROM (` + matched + `)`
		sqlText = `SELECT id, face, type, wrank FROM (` + ranked + `)` +
			` ORDER BY r DESC, CASE WHEN r > 0 THEN length(t) ELSE 0 END, id`
	}
	if limit > 0 {
		sqlText += ` LIMIT ` + b.arg(limit)
	}
	return sqlText, b.args
}

// matchSQL selects the rowids of index rows containing needle.
func matchSQL(b *sqlBuilder, needle string) string {
	if utf8.RuneCountInString(needle) >= minTrigramRunes {
		// A quoted FTS5 phrase is matched literally, as a substring.
		return `SELECT rowid FROM entity_search WHERE entity_search MATCH ` +
			b.arg(`"`+strings.ReplaceAll(needle, `"`, `""`)+`"`)
	}
	return `SELECT rowid FROM entity_search WHERE body LIKE ` +
		b.arg("%"+escapeLike(needle)+"%") + ` ESCAPE '\'`
}

// titleSQL is the lowercased title of row p: its type's title property, else
// its id. lower() folds ASCII only, which affects the ranking of a title in
// another script but never whether an entity matches.
func titleSQL(b *sqlBuilder, titles SearchTitles) string {
	byProp := map[string][]string{}
	for typ, prop := range titles {
		if strings.ContainsAny(prop, `"\`) || !safeLiteral(prop) {
			continue // not expressible as a JSON path; rank by id
		}
		byProp[prop] = append(byProp[prop], typ)
	}
	props := make([]string, 0, len(byProp))
	for prop := range byProp {
		props = append(props, prop)
	}
	sort.Strings(props) // deterministic SQL
	if len(props) == 0 {
		return `lower(p.id)`
	}
	var arms strings.Builder
	for _, prop := range props {
		types := byProp[prop]
		sort.Strings(types)
		in := make([]string, len(types))
		for i, typ := range types {
			in[i] = b.arg(typ)
		}
		arms.WriteString(` WHEN p.type IN (` + strings.Join(in, ", ") + `) THEN json_extract(p.properties, ` +
			b.jsonPath(prop) + `)`)
	}
	return `lower(coalesce(CASE` + arms.String() + ` END, p.id))`
}

// titleRankSQL ranks a matched row by how its title t relates to needle:
// equal, then a prefix, then containing it, then a match elsewhere only.
// Within a title rank the shorter title comes first; then the id decides.
func titleRankSQL(b *sqlBuilder, needle string) string {
	n := b.arg(needle)
	return `CASE WHEN t = ` + n + ` THEN 3` +
		` WHEN substr(t, 1, ` + strconv.Itoa(utf8.RuneCountInString(needle)) + `) = ` + n + ` THEN 2` +
		` WHEN instr(t, ` + n + `) > 0 THEN 1 ELSE 0 END`
}

// escapeLike escapes LIKE's wildcards and its escape character.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
