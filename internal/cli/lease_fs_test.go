//go:build !postgres && !memorybackend && !sqlite

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/kvlease"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// The test in this file writes the lease document of the default
// filesystem build directly, so it runs only on that build.

// TestLeaseCLI_AbandonedLeaseExpires claims through the lease files the way
// an instance that then died would, with a lease that has already run out,
// and claims again through the CLI (AC2). The abandoned instance writes
// with its own clock, so no test has to wait out a TTL.
func TestLeaseCLI_AbandonedLeaseExpires(t *testing.T) {
	root, ids := newLeaseProject(t)
	cache := filepath.Join(root, ".rela")
	rfs, err := storage.NewRootedFS(storage.NewSafeFS(storage.NewOsFS()), cache)
	require.NoError(t, err)
	locker, err := filelock.New(filepath.Join(cache, "leases.lock"))
	require.NoError(t, err)
	abandoned, err := kvlease.New(state.NewFSKV(rfs), locker)
	require.NoError(t, err)
	sum := sha256.Sum256([]byte("abandoned"))
	then := time.Now().Add(-time.Hour)
	_, err = abandoned.Claim(context.Background(), ids[0],
		lease.Holder{Identity: "ghost-identity", User: "ghost", Tool: "mcp", Label: "ghost"},
		hex.EncodeToString(sum[:]), then, then.Add(lease.MinTTL), "gone")
	require.NoError(t, err)

	svc := openLeaseServices(t, root)
	got, err := runCLI(t, svc, "", "-o", "json", "lease", "claim", ids[0], "--label", "next")
	require.NoError(t, err, "an expired lease does not hold the entity")
	claimJSONOf(t, got)

	// The same claim with a lease still running is refused.
	now := time.Now()
	_, err = abandoned.Claim(context.Background(), ids[0],
		lease.Holder{Identity: "ghost-identity", User: "ghost", Tool: "mcp", Label: "ghost"},
		hex.EncodeToString(sum[:]), now, now.Add(lease.MinTTL), "")
	var held *lease.HeldError
	require.ErrorAs(t, err, &held, "the live CLI lease holds the entity for the other instance too")
	require.Equal(t, "next", held.Current.Holder.Label)
}
