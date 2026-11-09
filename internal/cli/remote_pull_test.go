package cli

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/remotesync"
)

func TestRemotePull_RejectsBadArguments(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	err := (&RemotePullCmd{Name: "Bad Name"}).run(context.Background(), nil, nil, logger, nil)
	require.ErrorContains(t, err, "invalid remote name")
	err = (&RemotePullCmd{Name: "cat", AllowGone: -1}).run(context.Background(), nil, nil, logger, nil)
	require.ErrorContains(t, err, "--allow-gone")
	// No workspace: the pull refuses before reaching the source.
	err = (&RemotePullCmd{Name: "cat"}).run(context.Background(), nil, nil, logger, nil)
	require.ErrorContains(t, err, "workspace")
}

func TestWriteRemoteReport(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, writeRemoteReport(&b, remotesync.Report{
		Remote: "cat", DryRun: true, Refused: "too many gone", Gone: 12, Waiting: 5,
		Entities: remotesync.Counts{Created: 2, Conflicts: 1},
		Skipped:  []string{"secret"},
		Issues: []remotesync.Issue{{
			Kind: remotesync.IssueConflict, Address: "req/R-1", Local: "REQ-1", Reason: "both changed\x1b[31m",
			Fields: []string{"title"},
		}},
	}))
	out := b.String()
	require.Contains(t, out, "Dry run of a pull from remote cat.")
	require.Contains(t, out, "Refused: too many gone")
	require.Contains(t, out, "Entities: 2 created, 0 adopted, 0 updated, 0 unchanged, 1 conflicts")
	require.Contains(t, out, "Gone from the source: 12")
	require.Contains(t, out, "max_bytes reached): 5")
	require.Contains(t, out, "Not compared: secret")
	require.Contains(t, out, "conflict req/R-1 (local REQ-1): both changed")
	require.NotContains(t, out, "\x1b", "control characters from the source are not printed")
}
