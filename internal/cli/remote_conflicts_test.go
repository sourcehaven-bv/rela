package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
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
	err = (&RemoteResolveCmd{Name: "cat", Local: "REQ-1"}).run(ctx, nil, nil, logger, nil, false)
	require.ErrorContains(t, err, "--dismiss")
	err = (&RemoteResolveCmd{Name: "cat", Local: "REQ-1", Take: "local", Dismiss: true}).run(ctx, nil, nil, logger,
		nil, false)
	require.ErrorContains(t, err, "not both")
	err = (&RemoteResolveCmd{Name: "cat", Local: "REQ-1", Dismiss: true}).run(ctx, nil, nil, logger, nil, false)
	require.ErrorContains(t, err, "workspace")
	err = (&RemoteLinkCmd{Name: "Bad Name", Local: "REQ-1", Address: "req/R1"}).run(ctx, nil, logger, nil, false)
	require.ErrorContains(t, err, "invalid remote name")
	err = (&RemoteLinkCmd{Name: "cat", Local: "REQ-1", Address: "req/R1"}).run(ctx, nil, logger, nil, false)
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
			RemoteValue: json.RawMessage("\"Theirs\x1b[31m\""), Since: since},
		{Local: "REQ-2", Type: "requirement", Address: "req/R2", Field: "body", Reason: remotesync.ReasonBothChanged,
			RemoteValueOmitted: true, Since: since},
		{Local: "REQ-3", Type: "requirement", Reason: remotesync.ReasonRefused, Detail: "422 title is required",
			Since: since},
		{Local: "REQ-4", Type: "requirement", Reason: remotesync.ReasonUncertain, Detail: "the create may have reached",
			Since: since},
		{Local: "REQ-5", Type: "requirement", Field: "title", Reason: remotesync.ReasonBothChanged,
			Rule: remotesync.ConflictLatest, Fallback: remotesync.FallbackTimeWithheld, Since: since},
		{Local: "REQ-6", Type: "requirement", Field: "title", Reason: remotesync.ReasonSuperseded, Since: since,
			Superseded: &remotestatus.Superseded{Winner: remotesync.TakeRemote, LocalValue: json.RawMessage(`"Ours"`),
				LocalUpdated: since, RemoteUpdated: since.Add(time.Minute)}},
		{Local: "REQ-7", Type: "requirement", Field: "title", Reason: remotesync.ReasonSuperseded, Since: since,
			RemoteValueOmitted: true,
			Superseded:         &remotestatus.Superseded{Winner: remotesync.TakeLocal, LocalUpdated: since.Add(time.Minute)}},
		{Local: "REQ-8", Type: "requirement", Field: "title", Reason: remotesync.ReasonSuperseded, Since: since,
			Superseded: &remotestatus.Superseded{Winner: remotesync.TakeRemote, LocalUpdated: since}},
	}))
	out := b.String()
	require.Contains(t, out, "Remote cat holds 8 conflicts:")
	require.Contains(t, out, "rule latest held it (time_withheld)")
	require.Contains(t, out, `the latest rule took the remote value (local changed 2026-11-11T09:00:00Z, remote `+
		`2026-11-11T09:01:00Z); overwritten local value "Ours"`)
	require.Contains(t, out, "the overwritten remote value is too large to show")
	require.Contains(t, out, `requirement REQ-1 field title (remote req/R1): both sides changed it; remote value "Theirs`)
	require.Contains(t, out, "too large to show")
	require.Contains(t, out, "REQ-3: the remote refused the push: 422 title is required (since 2026-11-11T09:00:00Z)")
	require.Contains(t, out, "REQ-4: the create may have reached; --take local sends it again, rela remote link")
	require.Contains(t, out, "REQ-8 field title: the latest rule took the remote value")
	require.Contains(t, out, "the overwritten local value was not kept")
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
