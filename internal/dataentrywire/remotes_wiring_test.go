package dataentrywire

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

func TestNewRemotes_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := newRemotes(nil, slog.Default())
	require.Error(t, err)
}

func TestRemoteError(t *testing.T) {
	t.Parallel()
	require.NoError(t, remoteError(nil))
	for _, tc := range []struct {
		in, want error
	}{
		{fmt.Errorf("x: %w", scheduler.ErrNoRemote), dataentry.ErrRemoteUnknown},
		{fmt.Errorf("x: %w", remotesync.ErrNoConflict), dataentry.ErrRemoteNoEntry},
		{fmt.Errorf("x: %w", scheduler.ErrRemoteBusy), dataentry.ErrRemoteBusy},
		{fmt.Errorf("x: %w", remotesync.ErrRefused), dataentry.ErrRemoteRefused},
	} {
		got := remoteError(tc.in)
		require.ErrorIs(t, got, tc.want)
		require.ErrorIs(t, got, tc.in)
	}
	other := errors.New("disk full")
	require.Equal(t, other, remoteError(other))
}

func TestLeadingStatus(t *testing.T) {
	t.Parallel()
	for detail, want := range map[string]int{
		"422 title is required": 422, "502": 502, "": 0, "timeout": 0, "42 nope": 0, "700 x": 0,
	} {
		require.Equal(t, want, leadingStatus(detail), detail)
	}
}

func TestRemoteStatus_Mapping(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	st := scheduler.RemoteStatus{
		Remote: remotestatus.Remote{
			Name:                "cat",
			Settings:            remotestatus.Settings{URL: "https://cat.test", Conflict: "latest", Push: true},
			CredentialKind:      "env",
			SupersededRetention: time.Hour,
			Types: []remotestatus.MappedType{{Local: "requirement", Source: "requirement",
				Fields: []remotestatus.MappedField{{Local: "title", Remote: "name", Rule: "latest"}}}},
			Run: remotestatus.Run{LastRun: t0, Failures: 1},
		},
		Linked: []remotesync.Linked{
			{Type: "requirement", Local: "REQ-1"},
			{Kind: remotesync.KindRelation, Type: "traces", Local: "REQ-1--traces--REQ-2"},
			{Kind: remotesync.KindRelation, Type: "traces", Local: "not a key"},
		},
		Conflicts: []remotesync.Conflict{
			// An old refused entry: no Cause, the status in its Detail.
			{Local: "REQ-1", Type: "requirement", Reason: remotesync.ReasonRefused,
				Detail: "422 token=secretvalue rejected", RefusedFields: map[string]string{"title<name": "d"}},
			{Local: "REQ-2", Type: "requirement", Field: "title", Reason: remotesync.ReasonSuperseded,
				RemoteValue: json.RawMessage(`"Theirs"`),
				Superseded:  &remotestatus.Superseded{Winner: "remote", LocalValueOmitted: true}},
			{Kind: remotesync.KindRelation, Local: "broken", Reason: remotesync.ReasonRefused},
		},
	}
	got := remoteStatus(st)
	require.Equal(t, st.Remote, got.Remote)
	require.Len(t, got.LinkedRows, 2, "an unparsable relation key names no row")
	require.Equal(t, entity.RelationKey{From: "REQ-1", Type: "traces", To: "REQ-2"}, got.LinkedRows[1].Relation)
	require.Len(t, got.Entries, 2)

	refused := got.Entries[0]
	require.Equal(t, remotesync.CauseRefused, refused.Cause)
	require.Equal(t, 422, refused.Status)
	require.Equal(t, []string{"title"}, refused.Fields)
	b, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(b), "secretvalue", "Detail quotes the remote and stays off the wire")

	sup := got.Entries[1]
	require.NotNil(t, sup.Superseded)
	require.True(t, sup.Superseded.LocalValueOmitted)
	require.JSONEq(t, `"Theirs"`, string(sup.RemoteValue))
}
