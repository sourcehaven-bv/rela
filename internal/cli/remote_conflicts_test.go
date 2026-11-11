package cli

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/remotesync"
)

func TestRemoteConflicts_RejectsBadArguments(t *testing.T) {
	ctx := context.Background()
	require.ErrorContains(t, (&RemoteConflictsCmd{Name: "Bad Name"}).run(ctx, nil, nil, false), "invalid remote name")
	require.ErrorContains(t, (&RemoteConflictsCmd{Name: "cat"}).run(ctx, nil, nil, false), "workspace")
	logger := slog.New(slog.DiscardHandler)
	err := (&RemoteResolveCmd{Name: "Bad Name", Local: "REQ-1", Take: "local"}).run(ctx, nil, nil, logger, nil, false)
	require.ErrorContains(t, err, "invalid remote name")
	err = (&RemoteResolveCmd{Name: "cat", Local: "REQ-1", Take: "local"}).run(ctx, nil, nil, logger, nil, false)
	require.ErrorContains(t, err, "workspace")
}

func TestWriteConflicts(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, writeConflicts(&b, "cat", nil))
	require.Equal(t, "Remote cat holds no conflicts.\n", b.String())

	since := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	b.Reset()
	require.NoError(t, writeConflicts(&b, "cat", []remotesync.Conflict{
		{Local: "REQ-1", Type: "requirement", Address: "req/R1", Field: "title", Reason: remotesync.ReasonBothChanged,
			RemoteValue: "Theirs\x1b[31m", Since: since},
		{Local: "REQ-2", Type: "requirement", Address: "req/R2", Field: "body", Reason: remotesync.ReasonBothChanged,
			RemoteValueOmitted: true, Since: since},
		{Local: "REQ-3", Type: "requirement", Reason: remotesync.ReasonRefused, Detail: "422 title is required",
			Since: since},
		{Local: "REQ-4", Type: "requirement", Reason: remotesync.ReasonUncertain, Detail: "the create may have reached",
			Since: since},
	}))
	out := b.String()
	require.Contains(t, out, "Remote cat holds 4 conflicts:")
	require.Contains(t, out, `requirement REQ-1 field title (remote req/R1): both sides changed it; remote value "Theirs`)
	require.Contains(t, out, "too large to show")
	require.Contains(t, out, "REQ-3: the remote refused the push: 422 title is required (since 2026-11-11T09:00:00Z)")
	require.Contains(t, out, "REQ-4: the create may have reached")
	require.NotContains(t, out, "\x1b")
}

func TestWriteRemoteReport_Push(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, writeRemoteReport(&b, remotesync.Report{
		Remote: "cat", Held: 2, Pushed: &remotesync.PushCounts{Created: 1, Updated: 3, Refused: 1},
	}))
	out := b.String()
	require.Contains(t, out, "Synced with remote cat.")
	require.Contains(t, out, "Pushed: 1 created, 3 updated, 0 conflicts, 1 refused, 0 deferred, 0 failed")
	require.Contains(t, out, "Held on the conflict list: 2 (see rela remote conflicts cat)")
}
