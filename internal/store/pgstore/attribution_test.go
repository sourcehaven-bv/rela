package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// attributedCtx returns a ctx carrying the boundary-populated write
// attribution, as the entitymanager does for a real principal.
func attributedCtx(user, tool string) context.Context {
	return store.WithAttribution(context.Background(), store.Attribution{User: user, Tool: tool})
}

// TestAttributionColumnsStamped pins the store side of the Attribution ctx
// contract (RR-2VWA0Q): a write with attribution stamps last_edited_by_*, a
// write without leaves/sets them NULL — never an empty or placeholder string.
func TestAttributionColumnsStamped(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// Entity create WITH attribution.
	require.NoError(t, s.CreateEntity(attributedCtx("alice@example.com", "data-entry"),
		mkEntity("ATTR-1", "ticket", "v1")))
	var user, tool *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_edited_by_user, last_edited_by_tool FROM entities WHERE id = 'ATTR-1'`).
		Scan(&user, &tool))
	require.NotNil(t, user)
	require.Equal(t, "alice@example.com", *user)
	require.NotNil(t, tool)
	require.Equal(t, "data-entry", *tool)

	// Update WITHOUT attribution overwrites to NULL: the last write's
	// authorship is honestly unknown.
	require.NoError(t, s.UpdateEntity(ctx, mkEntity("ATTR-1", "ticket", "v2")))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_edited_by_user, last_edited_by_tool FROM entities WHERE id = 'ATTR-1'`).
		Scan(&user, &tool))
	require.Nil(t, user, "unattributed write must leave NULL, not a placeholder")
	require.Nil(t, tool)

	// Update WITH attribution re-stamps.
	require.NoError(t, s.UpdateEntity(attributedCtx("bob", "cli"), mkEntity("ATTR-1", "ticket", "v3")))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_edited_by_user, last_edited_by_tool FROM entities WHERE id = 'ATTR-1'`).
		Scan(&user, &tool))
	require.Equal(t, "bob", *user)
	require.Equal(t, "cli", *tool)

	// Relations: create + update stamp the same columns.
	require.NoError(t, s.CreateEntity(ctx, mkEntity("ATTR-2", "ticket", "peer")))
	_, err = s.CreateRelation(attributedCtx("carol", "mcp"), "ATTR-1", "blocks", "ATTR-2",
		&store.RelationData{Content: "why"})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_edited_by_user, last_edited_by_tool FROM relations
		 WHERE from_id = 'ATTR-1' AND rel_type = 'blocks' AND to_id = 'ATTR-2'`).
		Scan(&user, &tool))
	require.Equal(t, "carol", *user)
	require.Equal(t, "mcp", *tool)

	_, err = s.UpdateRelation(ctx, "ATTR-1", "blocks", "ATTR-2", store.RelationData{Content: "changed"})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_edited_by_user, last_edited_by_tool FROM relations
		 WHERE from_id = 'ATTR-1' AND rel_type = 'blocks' AND to_id = 'ATTR-2'`).
		Scan(&user, &tool))
	require.Nil(t, user)
	require.Nil(t, tool)
}

// TestSweepAttributesRealEditor is the ticket's headline behavior (TKT-ZIRMGM
// AC1/AC2/AC5): swept create/update versions carry the row's recorded editor,
// not the version-sweep system principal.
func TestSweepAttributesRealEditor(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(attributedCtx("alice@example.com", "data-entry"),
		mkEntity("WHO-1", "ticket", "policy text")))
	require.NoError(t, s.CreateEntity(ctx, mkEntity("WHO-2", "ticket", "peer")))
	_, err = s.CreateRelation(attributedCtx("alice@example.com", "data-entry"),
		"WHO-1", "blocks", "WHO-2", &store.RelationData{Content: "reason"})
	require.NoError(t, err)

	// Backdate so both settle past the idle window.
	_, err = pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE relations SET updated_at = now() - interval '1 hour'`)
	require.NoError(t, err)

	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	require.Eventually(t, func() bool {
		metas, e := s.VersionStore().ListVersions(ctx, "WHO-1")
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond)

	metas, err := s.VersionStore().ListVersions(ctx, "WHO-1")
	require.NoError(t, err)
	require.Equal(t, store.VersionOpCreate, metas[0].Op)
	require.Equal(t, "alice@example.com", metas[0].PrincipalUser)
	require.Equal(t, "data-entry", metas[0].PrincipalTool)

	require.Eventually(t, func() bool {
		rm, e := s.VersionStore().ListRelationVersions(ctx,
			store.RelationHistoryQuery{From: "WHO-1", Type: "blocks", To: "WHO-2"})
		return e == nil && len(rm) == 1
	}, 3*time.Second, 25*time.Millisecond)

	rm, err := s.VersionStore().ListRelationVersions(ctx,
		store.RelationHistoryQuery{From: "WHO-1", Type: "blocks", To: "WHO-2"})
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", rm[0].PrincipalUser)
	require.Equal(t, "data-entry", rm[0].PrincipalTool)

	// WHO-2 was written WITHOUT attribution: its swept version must fall back
	// to the system principal, never a fabricated identity (AC5, RR-U964M0).
	require.Eventually(t, func() bool {
		metas, e := s.VersionStore().ListVersions(ctx, "WHO-2")
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond)
	fallback, err := s.VersionStore().ListVersions(ctx, "WHO-2")
	require.NoError(t, err)
	require.Empty(t, fallback[0].PrincipalUser)
	require.Equal(t, "version-sweep", fallback[0].PrincipalTool)
}

// TestSweepAttributesLastEditorOfBurst pins the accepted v1 semantics (flush-
// on-author-change is TKT-0IGI4V): a burst of edits collapses into ONE version
// attributed to the LAST editor.
func TestSweepAttributesLastEditorOfBurst(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(attributedCtx("alice", "cli"), mkEntity("BURST-1", "ticket", "v1")))
	require.NoError(t, s.UpdateEntity(attributedCtx("bob", "data-entry"), mkEntity("BURST-1", "ticket", "v2")))

	_, err = pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = 'BURST-1'`)
	require.NoError(t, err)

	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	require.Eventually(t, func() bool {
		metas, e := s.VersionStore().ListVersions(ctx, "BURST-1")
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond)

	metas, err := s.VersionStore().ListVersions(ctx, "BURST-1")
	require.NoError(t, err)
	require.Equal(t, "bob", metas[0].PrincipalUser)
	require.Equal(t, "data-entry", metas[0].PrincipalTool)
}
