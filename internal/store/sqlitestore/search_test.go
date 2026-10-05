package sqlitestore_test

import (
	"context"
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

// A database written before the index existed gets it filled by the v12 rung.
func TestSearch_MigrationFillsIndex(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	require.NoError(t, err)
	s, err := sqlitestore.New(db)
	require.NoError(t, err)
	createEntity(t, s, "OLD-1", "note", "title", "Legacy row", "")
	// Stand in for a v11 database: no index rows, an older version stamp.
	_, err = db.DB().ExecContext(ctx, `DELETE FROM entity_search`)
	require.NoError(t, err)
	_, err = db.DB().ExecContext(ctx, `PRAGMA user_version = 11`)
	require.NoError(t, err)
	require.NoError(t, s.Close())
	require.NoError(t, db.Close())

	_, b := openSearch(t, path, nil)
	require.Equal(t, []string{"OLD-1"}, searchIDs(t, b, "legacy", store.TrivialScope()))
}

func TestNewSearchBackend_RejectsNil(t *testing.T) {
	_, err := sqlitestore.NewSearchBackend(nil)
	require.Error(t, err)
}
