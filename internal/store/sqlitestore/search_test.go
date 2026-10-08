package sqlitestore_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// openSearch opens a store at path with its FTS5 search backend.
func openSearch(t *testing.T, path string, titles sqlitestore.SearchTitles) (*sqlitestore.Store, *sqlitestore.SearchBackend) {
	t.Helper()
	s, db := openWithDB(t, path)
	backend, err := sqlitestore.NewSearchBackend(db)
	require.NoError(t, err)
	return s, backend.RankByTitles(titles)
}

// searchIDs runs a search and returns the ids in result order.
func searchIDs(t *testing.T, b *sqlitestore.SearchBackend, text string, w store.WorldScope) []string {
	t.Helper()
	faces, err := b.Search(text, 0, w)
	require.NoError(t, err)
	ids := make([]string, 0, len(faces))
	for _, f := range faces {
		ids = append(ids, f.ID)
	}
	return ids
}

func createEntity(t *testing.T, s *sqlitestore.Store, id, typ, prop, value, content string) {
	t.Helper()
	e := entity.New(id, typ)
	if prop != "" {
		e.SetString(prop, value)
	}
	e.Content = content
	require.NoError(t, s.CreateEntity(context.Background(), e))
}

func TestSearch_FollowsEveryWrite(t *testing.T) {
	ctx := context.Background()
	s, b := openSearch(t, filepath.Join(t.TempDir(), "s.db"), nil)
	w := store.TrivialScope()

	createEntity(t, s, "Old-ID", "ticket", "title", "Ünïcode Heading", "first body")
	require.Equal(t, []string{"Old-ID"}, searchIDs(t, b, "first", w))
	require.Equal(t, []string{"Old-ID"}, searchIDs(t, b, "ÜNÏCODE", w), "case folding is not ASCII-only")

	e, err := s.GetEntity(ctx, entity.Ref{ID: "Old-ID"})
	require.NoError(t, err)
	e.Content = "second body"
	require.NoError(t, s.UpdateEntity(ctx, e))
	require.Empty(t, searchIDs(t, b, "first", w), "an update replaces the indexed text")
	require.Equal(t, []string{"Old-ID"}, searchIDs(t, b, "second", w))

	_, err = s.RenameFamily(ctx, "Old-ID", "New-MixedCase")
	require.NoError(t, err)
	require.Equal(t, []string{"New-MixedCase"}, searchIDs(t, b, "new-mixedcase", w))
	require.Empty(t, searchIDs(t, b, "old-id", w))

	_, err = s.DeleteFamily(ctx, "New-MixedCase", true)
	require.NoError(t, err)
	require.Empty(t, searchIDs(t, b, "second", w), "a delete removes the row")

	// A rowid freed by the delete may be reused; the insert must not trip
	// over what the index held for it.
	createEntity(t, s, "Fresh", "ticket", "", "", "third body")
	require.Equal(t, []string{"Fresh"}, searchIDs(t, b, "third", w))
}

func TestSearch_Needles(t *testing.T) {
	s, b := openSearch(t, filepath.Join(t.TempDir(), "s.db"), nil)
	w := store.TrivialScope()
	createEntity(t, s, "A-1", "note", "title", "Plan", `say "hi" to 50% of us_ers`)
	createEntity(t, s, "B-2", "note", "title", "Other", "nothing here")
	createEntity(t, s, "C-3", "note", "count", "", "")

	tests := []struct {
		needle string
		want   []string
	}{
		{"hi", []string{"A-1", "B-2"}}, // shorter than a trigram: LIKE scan ("nothing")
		{"a", []string{"A-1"}},         // one character: matches the id and "plan"
		{`"hi"`, []string{"A-1"}},      // quotes are literal
		{"50%", []string{"A-1"}},       // % is literal
		{"s_e", []string{"A-1"}},       // _ is literal: "us_ers"
		{"%", []string{"A-1"}},         // short and literal
		{"a-1 plan", nil},              // fields are separated by newlines, not spaces
		{"zzz", nil},
	}
	for _, tc := range tests {
		t.Run(tc.needle, func(t *testing.T) {
			got := searchIDs(t, b, tc.needle, w)
			if tc.want == nil {
				require.Empty(t, got)
				return
			}
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestSearch_RanksByConfiguredTitle(t *testing.T) {
	titles := sqlitestore.SearchTitles{"note": "title", "person": "name", "odd": `we"ird`}
	s, b := openSearch(t, filepath.Join(t.TempDir(), "s.db"), titles)
	createEntity(t, s, "N-1", "note", "title", "Quarterly review", "telemetry is mentioned in the body only")
	createEntity(t, s, "N-2", "note", "title", "TELEMETRY", "")
	createEntity(t, s, "N-3", "note", "title", "Telemetry rollout plan for the platform", "")
	createEntity(t, s, "P-1", "person", "name", "Telemetry Tom", "")
	createEntity(t, s, "X-1", "misc", "label", "telemetry", "") // type absent from the map: ranks by id
	createEntity(t, s, "O-1", "odd", "label", "telemetry", "")  // unusable title property: ranks by id

	require.Equal(t, []string{"N-2", "P-1", "N-3", "N-1", "O-1", "X-1"},
		searchIDs(t, b, "Telemetry", store.TrivialScope()))

	faces, err := b.Search("telemetry", 2, store.TrivialScope())
	require.NoError(t, err)
	require.Len(t, faces, 2)
	require.Equal(t, search.RuleUnscoped, faces[0].Via)
}

// The world picks each entity's prime face before the text is matched, so a
// non-prime face that mentions the needle is not a hit.
func TestSearch_MatchesThePrimeFaceOnly(t *testing.T) {
	ctx := context.Background()
	s, b := openSearch(t, filepath.Join(t.TempDir(), "s.db"), nil)
	createEntity(t, s, "DOC-1", "doc", "", "", "published words")
	draft := entity.New("DOC-1", "doc")
	draft.Face = "draft"
	draft.Content = "draft words"
	require.NoError(t, s.CreateEntity(ctx, draft))
	createEntity(t, s, "DOC-2", "doc", "", "", "only published words")

	drafts := store.NewWorldScope(map[string]store.TypeResolution{
		"doc": {Chain: []entity.Face{"draft"}, Fallback: store.FallbackDefaultState},
	})
	require.Equal(t, []string{"DOC-1"}, searchIDs(t, b, "draft", drafts))
	require.Equal(t, []string{"DOC-2"}, searchIDs(t, b, "published", drafts),
		"DOC-1's prime is its draft, which does not say published")
	require.ElementsMatch(t, []string{"DOC-1", "DOC-2"}, searchIDs(t, b, "published", store.TrivialScope()))
	require.Empty(t, searchIDs(t, b, "draft", store.TrivialScope()))

	faces, err := b.Search("words", 0, drafts)
	require.NoError(t, err)
	require.Len(t, faces, 2)
	byID := map[string]search.Face{}
	for _, f := range faces {
		byID[f.ID] = f
	}
	require.Equal(t, search.RuleChain, byID["DOC-1"].Via)
	require.Equal(t, entity.Face("draft"), byID["DOC-1"].Face)
	require.Equal(t, search.RuleFallbackDefault, byID["DOC-2"].Via)

	all := searchIDs(t, b, "", drafts)
	require.Equal(t, []string{"DOC-1", "DOC-2"}, all, "an empty needle lists every prime in id order")
}

// dropSearchIndex removes the search index, its key table and its triggers.
const dropSearchIndex = `DROP TRIGGER entity_search_insert;
DROP TRIGGER entity_search_update;
DROP TRIGGER entity_search_delete;
DROP TABLE entity_search;
DROP TABLE entity_search_key;`

// rowidSearchIndex is the v12 index, keyed by the entities rowid, filled from
// the rows already there.
const rowidSearchIndex = `CREATE VIRTUAL TABLE entity_search USING fts5(body, tokenize = 'trigram');
CREATE TRIGGER entity_search_insert AFTER INSERT ON entities BEGIN
	INSERT OR REPLACE INTO entity_search(rowid, body) VALUES (NEW.rowid, NEW.id || char(10) || NEW.content);
END;
CREATE TRIGGER entity_search_update AFTER UPDATE OF id, properties, content ON entities BEGIN
	DELETE FROM entity_search WHERE rowid = OLD.rowid;
	INSERT OR REPLACE INTO entity_search(rowid, body) VALUES (NEW.rowid, NEW.id || char(10) || NEW.content);
END;
CREATE TRIGGER entity_search_delete AFTER DELETE ON entities BEGIN
	DELETE FROM entity_search WHERE rowid = OLD.rowid;
END;
INSERT INTO entity_search(rowid, body) SELECT rowid, id || char(10) || content FROM entities;`

// seedOldDatabase writes rows to a fresh database, then rewrites its search
// index with shape and stamps it with version.
func seedOldDatabase(t *testing.T, path, shape string, version int) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	require.NoError(t, err)
	s, err := sqlitestore.New(db)
	require.NoError(t, err)
	createEntity(t, s, "OLD-0", "note", "title", "Doomed row", "")
	createEntity(t, s, "OLD-1", "note", "title", "Legacy row", "first body")
	createEntity(t, s, "OLD-2", "note", "title", "Other row", "second body")
	_, err = s.DeleteFamily(ctx, "OLD-0", true)
	require.NoError(t, err)
	_, err = db.DB().ExecContext(ctx, dropSearchIndex+shape)
	require.NoError(t, err)
	_, err = db.DB().ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, version))
	require.NoError(t, err)
	require.NoError(t, s.Close())
	require.NoError(t, db.Close())
}

// The v12 and v13 rungs carry an older database to the keyed index, filled
// from its rows. A v11 database had no index; a v12 one had the index keyed
// by the entities rowid, whose triggers the v13 rung must replace.
func TestSearch_MigrationRebuildsIndex(t *testing.T) {
	tests := []struct {
		name    string
		shape   string
		version int
	}{
		{"v11 without an index", "", 11},
		{"v12 keyed by rowid", rowidSearchIndex, 12},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "old.db")
			seedOldDatabase(t, path, tc.shape, tc.version)

			// A second open must find nothing left to do.
			for range 2 {
				db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
				require.NoError(t, err)
				require.NoError(t, db.Close())
			}

			s, db := openWithDB(t, path)
			b, err := sqlitestore.NewSearchBackend(db)
			require.NoError(t, err)
			w := store.TrivialScope()
			var version int
			require.NoError(t, db.DB().QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version))
			require.Equal(t, sqlitedb.SchemaVersion(), version)
			require.Equal(t, []string{"OLD-1"}, searchIDs(t, b, "legacy", w), "properties are indexed again")
			require.Empty(t, searchIDs(t, b, "doomed", w))

			// The rows must be keyed so that moving their rowids changes nothing.
			_, err = db.DB().ExecContext(ctx, `UPDATE entities SET rowid = rowid + 100`)
			require.NoError(t, err)
			e, err := s.GetEntity(ctx, entity.Ref{ID: "OLD-1"})
			require.NoError(t, err)
			e.Content = "rewritten"
			require.NoError(t, s.UpdateEntity(ctx, e))
			require.Equal(t, []string{"OLD-1"}, searchIDs(t, b, "rewritten", w))
			require.Equal(t, []string{"OLD-2"}, searchIDs(t, b, "second", w))
			_, err = s.DeleteFamily(ctx, "OLD-2", true)
			require.NoError(t, err)
			require.Empty(t, searchIDs(t, b, "second", w))
			require.Equal(t, []string{"OLD-1"}, searchIDs(t, b, "row", w))
		})
	}
}

// entityRowids maps each default-face entity id to its entities rowid.
func entityRowids(t *testing.T, db *sqlitedb.DB) map[string]int64 {
	t.Helper()
	rows, err := db.DB().Query(`SELECT id, rowid FROM entities WHERE face = ''`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var rowid int64
		require.NoError(t, rows.Scan(&id, &rowid))
		out[id] = rowid
	}
	require.NoError(t, rows.Err())
	return out
}

// VACUUM may renumber the entities rowids (RR-44YDTU). The index is keyed by
// entity_search_key, so search, and every trigger after it, must still find
// the right rows.
//
// The SQLite this build links keeps the rowids of a table that has an index,
// as entities does, so VACUUM alone does not renumber them here. SQLite only
// documents that it MAY, and a dump and reload does. So after the VACUUM the
// test closes the gaps the deletes left itself, by rewriting the rowids the
// way a renumbering VACUUM would: without firing a trigger.
func TestSearch_SurvivesVacuumRenumbering(t *testing.T) {
	ctx := context.Background()
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "s.db"))
	backend, err := sqlitestore.NewSearchBackend(db)
	require.NoError(t, err)
	w := store.TrivialScope()

	for i := range 20 {
		createEntity(t, s, fmt.Sprintf("E-%02d", i), "note", "title", fmt.Sprintf("title%02d", i),
			fmt.Sprintf("body%02d", i))
	}
	for i := range 10 {
		_, err = s.DeleteFamily(ctx, fmt.Sprintf("E-%02d", i), true)
		require.NoError(t, err)
	}
	before := entityRowids(t, db)
	_, err = db.DB().ExecContext(ctx, `VACUUM`)
	require.NoError(t, err)
	_, err = db.DB().ExecContext(ctx, `UPDATE entities SET rowid = rowid - 10`)
	require.NoError(t, err)
	require.NotEqual(t, before, entityRowids(t, db), "the entities rowids must have moved")

	for i := 10; i < 20; i++ {
		id := fmt.Sprintf("E-%02d", i)
		require.Equal(t, []string{id}, searchIDs(t, backend, fmt.Sprintf("body%02d", i), w))
		require.Equal(t, []string{id}, searchIDs(t, backend, fmt.Sprintf("title%02d", i), w))
	}
	require.Empty(t, searchIDs(t, backend, "body05", w), "a deleted entity stays out of the index")

	e, err := s.GetEntity(ctx, entity.Ref{ID: "E-12"})
	require.NoError(t, err)
	e.Content = "rewritten"
	require.NoError(t, s.UpdateEntity(ctx, e))
	require.Equal(t, []string{"E-12"}, searchIDs(t, backend, "rewritten", w))
	require.Empty(t, searchIDs(t, backend, "body12", w), "an update replaces the indexed text")
	require.Equal(t, []string{"E-13"}, searchIDs(t, backend, "body13", w), "an update touches no other row")

	_, err = s.RenameFamily(ctx, "E-14", "Renamed-14")
	require.NoError(t, err)
	require.Equal(t, []string{"Renamed-14"}, searchIDs(t, backend, "body14", w))
	require.Empty(t, searchIDs(t, backend, "e-14", w))

	_, err = s.DeleteFamily(ctx, "E-15", true)
	require.NoError(t, err)
	require.Empty(t, searchIDs(t, backend, "body15", w))
	require.Equal(t, []string{"E-16"}, searchIDs(t, backend, "body16", w), "a delete removes no other row")

	createEntity(t, s, "E-NEW", "note", "", "", "fresh body")
	require.Equal(t, []string{"E-NEW"}, searchIDs(t, backend, "fresh", w))
	require.ElementsMatch(t,
		[]string{"E-10", "E-11", "E-13", "Renamed-14", "E-16", "E-17", "E-18", "E-19", "E-NEW"},
		searchIDs(t, backend, "body", w))
}

func TestNewSearchBackend_RejectsNil(t *testing.T) {
	_, err := sqlitestore.NewSearchBackend(nil)
	require.Error(t, err)
}
