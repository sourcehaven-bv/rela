package pgstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestSearchAfterMixedCaseRename guards a bug the shared conformance suite can't
// catch: it uses same-case IDs, so it never exercises search_text after renaming
// to a mixed-case ID. The store maintains search_text as all-lowercase; a rename
// must keep it lowercase or the renamed entity becomes unfindable by its new ID
// (regression: RR-YXFYK / go-architect C1).
func TestSearchAfterMixedCaseRename(t *testing.T) {
	pool := newScopedPool(t)
	backend := pgstore.NewSearchBackend(pool)
	st, err := pgstore.New(pool, pgstore.WithObserver(backend))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("Old-ID", "ticket")))

	_, err = st.RenameEntity(ctx, "Old-ID", "New-MixedCase")
	require.NoError(t, err)

	// The backend matches case-insensitively, so a lowercased query for the new
	// ID must find the renamed entity.
	ids := searchIDs(t, backend, "new-mixedcase")
	require.Contains(t, ids, "New-MixedCase",
		"renamed entity must be findable by its new ID (search_text stayed lowercase)")

	// The old ID must no longer match.
	old := searchIDs(t, backend, "old-id")
	require.NotContains(t, old, "New-MixedCase")
	require.NotContains(t, old, "Old-ID")
}

// searchIDs runs a DEFAULT-WORLD search and returns just the ids, which is
// what this file's pre-worlds assertions are written against. World-scoped
// behavior is covered by the store conformance suite, which holds every
// backend to one contract.
func searchIDs(t *testing.T, b *pgstore.SearchBackend, text string) []string {
	t.Helper()
	faces, err := b.Search(text, 0, store.DefaultWorld())
	require.NoError(t, err)
	ids := make([]string, 0, len(faces))
	for _, f := range faces {
		ids = append(ids, f.ID)
	}
	return ids
}

// Ranking looks at the identity/title prefix of search_text, not the body:
// an entity whose TITLE carries the word outranks one that only mentions it
// deep in a long body, and the body-only entity still matches (TKT-1U8XYN).
func TestSearch_RanksTitleMatchAboveBodyMention(t *testing.T) {
	pool := newScopedPool(t)
	backend := pgstore.NewSearchBackend(pool)
	st, err := pgstore.New(pool, pgstore.WithObserver(backend))
	require.NoError(t, err)
	ctx := context.Background()

	body := entity.New("N-1", "note")
	body.SetString("title", "Quarterly review")
	body.Content = strings.Repeat("filler words about nothing in particular. ", 60) + "telemetry appears here at the end"
	require.NoError(t, st.CreateEntity(ctx, body))
	title := entity.New("N-2", "note")
	title.SetString("title", "Telemetry rollout")
	title.Content = strings.Repeat("unrelated prose. ", 80)
	require.NoError(t, st.CreateEntity(ctx, title))

	faces, err := backend.Search("telemetry", 10, store.DefaultWorld())
	require.NoError(t, err)
	ids := make([]string, 0, len(faces))
	for _, f := range faces {
		ids = append(ids, f.ID)
	}
	require.Equal(t, []string{"N-2", "N-1"}, ids)
}

// With a title map, ranking reads the title property alone: case must not
// matter (the needle is lowercased, a raw property is not), a type without an
// entry ranks by id, and the gated search orders exactly as the ungated one.
func TestSearch_RanksByConfiguredTitle(t *testing.T) {
	pool := newScopedPool(t)
	titles := pgstore.SearchTitles{"note": "title", "person": "name"}
	backend := pgstore.NewSearchBackend(pool).RankByTitles(titles)
	st, err := pgstore.New(pool, pgstore.WithObserver(backend), pgstore.WithSearchTitles(titles))
	require.NoError(t, err)
	ctx := context.Background()

	mk := func(id, typ, prop, value, content string) {
		e := entity.New(id, typ)
		if prop != "" {
			e.SetString(prop, value)
		}
		e.Content = content
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	mk("N-1", "note", "title", "Quarterly review", "telemetry is mentioned in the body only")
	mk("N-2", "note", "title", "TELEMETRY", "")
	mk("N-3", "note", "title", "Telemetry rollout plan for the platform", "")
	mk("P-1", "person", "name", "Telemetry Tom", "")
	mk("X-1", "misc", "label", "telemetry", "") // type absent from the map: ranks by id

	faces, err := backend.Search("Telemetry", 10, store.DefaultWorld())
	require.NoError(t, err)
	ids := make([]string, 0, len(faces))
	for _, f := range faces {
		ids = append(ids, f.ID)
	}
	require.Equal(t, []string{"N-2", "P-1", "N-3", "N-1", "X-1"}, ids)

	var gated []string
	scope := map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
	for hit, err := range st.SearchVisible(ctx, search.Query{Text: "Telemetry", Limit: 10}, scope) {
		require.NoError(t, err)
		gated = append(gated, hit.ID)
	}
	require.Equal(t, ids, gated, "gated and ungated searches must rank identically")
}

// Migration 0014 rebuilds search_text in SQL for rows written before the
// column's composition changed; it must produce what the store now writes.
func TestMigration0014_SearchTextRebuildMatchesStore(t *testing.T) {
	pool := newScopedPool(t)
	st, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	e := entity.New("Mixed-Case-9", "note")
	e.SetString("title", "Zebra Crossing")
	e.SetString("owner", "Alice")
	e.Properties["count"] = 3
	e.Content = "Body Text\nsecond line"
	require.NoError(t, st.CreateEntity(ctx, e))

	var written string
	require.NoError(t, pool.QueryRow(ctx, `SELECT search_text FROM entities WHERE id = 'Mixed-Case-9'`).Scan(&written))
	_, err = pool.Exec(ctx, `UPDATE entities SET search_text =
		lower(id) || E'\n' ||
		COALESCE((SELECT string_agg(lower(p.value), E'\n' ORDER BY p.key)
		          FROM jsonb_each_text(properties) p
		          WHERE jsonb_typeof(properties -> p.key) = 'string'), '') || E'\n' ||
		lower(content)`)
	require.NoError(t, err)
	var rebuilt string
	require.NoError(t, pool.QueryRow(ctx, `SELECT search_text FROM entities WHERE id = 'Mixed-Case-9'`).Scan(&rebuilt))
	require.Equal(t, written, rebuilt)
}
