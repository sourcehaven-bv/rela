package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// postRoom posts a text message in the room of id as user.
func postRoom(t *testing.T, p channelProject, user, id, body string) {
	t.Helper()
	_, err := runChannel(t, p.svc, user, "", "channel", "post", id, "room", "text", "-P", "body="+body)
	require.NoError(t, err)
}

// TestChannelMarkRead_Text pins TKT-2B2AIA AC 16 for mark-read: the text
// and JSON answers, the clamp, the default of the last seq, and the
// refusal of a negative seq. Each reader marks at most twice, under the
// per-channel mark rate.
func TestChannelMarkRead_Text(t *testing.T) {
	p := newChannelProject(t)
	for _, b := range []string{"one", "two", "three"} {
		postRoom(t, p, "system:ci", p.item, b)
	}
	got, err := runChannel(t, p.svc, "alice", "", "channel", "mark-read", p.item, "room", "--seq", "1")
	require.NoError(t, err)
	assert.Equal(t, "marked read up to 1 (2 unread)\n", got)

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "mark-read", p.item, "room", "--seq", "99")
	require.NoError(t, err)
	assert.JSONEq(t, `{"seq":3,"unread":0,"last_seq":3}`, got)

	postRoom(t, p, "alice", p.item, "four")
	got, err = runChannel(t, p.svc, "system:ci", "", "channel", "mark-read", p.item, "room", "--seq", "1")
	require.NoError(t, err)
	assert.Equal(t, "marked read up to 3 (1 unread)\n", got, "a marker never moves back")
	got, err = runChannel(t, p.svc, "system:ci", "", "channel", "mark-read", p.item, "room")
	require.NoError(t, err)
	assert.Equal(t, "marked read up to 4 (0 unread)\n", got)

	_, err = runChannel(t, p.svc, "alice", "", "channel", "mark-read", p.item, "room", "--seq=-1")
	require.EqualError(t, err, "--seq: 0 or more")
}

// TestChannelList_Unread pins TKT-2B2AIA AC 16 for list: a row with unread
// messages ends in "(N unread)", a read row and a row without a marker in
// nothing, and the JSON row carries the read object.
func TestChannelList_Unread(t *testing.T) {
	p := newChannelProject(t)
	postRoom(t, p, "alice", p.item, "mine")
	postRoom(t, p, "system:ci", p.item, "one")
	postRoom(t, p, "system:ci", p.item, "two")

	got, err := runChannel(t, p.svc, "alice", "", "channel", "list", p.item)
	require.NoError(t, err)
	room := rowOf(t, got, "room")
	assert.True(t, strings.HasSuffix(room, " 3 (2 unread)"), room)

	got, err = runChannel(t, p.svc, "system:ci", "", "channel", "list", p.item)
	require.NoError(t, err)
	assert.NotContains(t, got, "unread", "the ci marker is at the last seq")

	var view realtime.AnchorChannels
	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "list", p.item)
	require.NoError(t, err)
	channelJSONOf(t, got, &view)
	for _, typ := range view.Types {
		if typ.Type == "room" {
			require.Len(t, typ.Channels, 1)
			assert.Equal(t, &realtime.ChannelRead{Seq: 1, Unread: 2}, typ.Channels[0].Read)
		}
	}
}

// rowOf returns the line of a list table that starts with typ.
func rowOf(t *testing.T, table, typ string) string {
	t.Helper()
	for line := range strings.SplitSeq(table, "\n") {
		if strings.HasPrefix(line, typ+" ") {
			return strings.TrimRight(line, " ")
		}
	}
	t.Fatalf("no %s row in:\n%s", typ, table)
	return ""
}

// TestUnreadNote pins the cap in the list's unread note.
func TestUnreadNote(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		read *realtime.ChannelRead
		want string
	}{
		{"no marker", nil, ""},
		{"read", &realtime.ChannelRead{Seq: 4}, ""},
		{"unread", &realtime.ChannelRead{Seq: 4, Unread: 2}, " (2 unread)"},
		{"at the cap", &realtime.ChannelRead{Unread: realtime.UnreadCap}, " (100+ unread)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, unreadNote(tc.read))
		})
	}
}
