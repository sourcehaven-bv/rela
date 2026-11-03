package pgstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration0036_AttachmentSHA256 pins migration 0036 (TKT-1TS55R): a
// schema at version 35 gains attachments.sha256, the rows already there get
// the hash of their bytes, and a second run is harmless.
func TestMigration0036_AttachmentSHA256(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	require.NoError(t, s.CreateEntity(ctx, entity.New("T-1", "ticket")))
	require.NoError(t, s.AttachFamilyFile(ctx, "T-1", "spec", "a.txt", strings.NewReader("abc")))

	for _, stmt := range []string{
		`ALTER TABLE attachments DROP COLUMN sha256`,
		`UPDATE schema_version SET version = 35`,
	} {
		_, execErr := pool.Exec(ctx, stmt)
		require.NoError(t, execErr, stmt)
	}

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	infos, err := s.ListFamilyAttachments(ctx, "T-1")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", infos[0].SHA256)
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, target, current)
}
