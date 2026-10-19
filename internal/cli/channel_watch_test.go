package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// TestChannelWatchCLI_TimeoutExit2 pins that a watch with no change ends
// with exit code 2 and says so, in both outputs.
func TestChannelWatchCLI_TimeoutExit2(t *testing.T) {
	p := newChannelProject(t)
	got, err := runChannel(t, p.svc, "alice", "", "channel", "watch", p.item, "room", "--timeout", "1s")
	var exit *relaerrors.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, watchTimeoutExit, exit.Code)
	assert.Equal(t, "no change\n", got)

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "watch", p.item, "room",
		"--timeout", "1s", "--after-rev", "0")
	require.ErrorAs(t, err, &exit)
	var rec watchTimedOut
	channelJSONLine(t, got, &rec)
	assert.True(t, rec.TimedOut)
	assert.Equal(t, int64(0), rec.AfterRev)
}

// TestChannelWatchCLI_ReturnsOnChange pins that a post made while the watch
// waits ends it with exit 0 and prints the change, and that --after-rev
// reports changes already past it at once.
func TestChannelWatchCLI_ReturnsOnChange(t *testing.T) {
	p := newChannelProject(t)
	_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text", "-P", "body=first")
	require.NoError(t, err)

	got, err := runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "watch", p.item, "room",
		"--after-rev", "0", "--timeout", "5s")
	require.NoError(t, err)
	var rec readJSON
	channelJSONLine(t, got, &rec)
	require.Len(t, rec.Messages, 1)
	assert.Equal(t, "first", rec.Messages[0].Props["body"])

	bctx, svc := appboundChannels(t, p)
	cmd := &ChannelWatchCmd{
		channelAddr: channelAddr{ID: p.item, Type: "room"},
		Timeout:     10 * time.Second, Poll: time.Second, Limit: channelDefaultLimit,
	}
	tick := make(chan time.Time)
	done := make(chan error, 1)
	buf := swapOut(t, "")
	go func() { done <- cmd.watch(bctx, svc, tick) }()
	// Post through the same service: its wake ends the wait, no tick needed.
	require.Eventually(t, func() bool {
		_, err := svc.Post(bctx, cmd.raw(), realtime.PostInput{
			Type: "text", Props: map[string]any{"body": "second"}, LogOnly: true,
		})
		require.NoError(t, err)
		select {
		case err := <-done:
			require.NoError(t, err)
			return true
		case <-time.After(200 * time.Millisecond):
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
	assert.Contains(t, buf.String(), propIndent+"body: second\n")
}

// TestChannelWatchCLI_FollowStopsOnCancel pins that --follow prints every
// change and ends without error when interrupted.
func TestChannelWatchCLI_FollowStopsOnCancel(t *testing.T) {
	p := newChannelProject(t)
	bctx, svc := appboundChannels(t, p)
	ctx, cancel := context.WithCancel(bctx)
	zero := int64(0)
	cmd := &ChannelWatchCmd{
		channelAddr: channelAddr{ID: p.item, Type: "room"},
		AfterRev:    &zero, Follow: true, Poll: time.Second, Limit: channelDefaultLimit,
	}
	for _, body := range []string{"one", "two"} {
		_, err := svc.Post(bctx, cmd.raw(), realtime.PostInput{
			Type: "text", Props: map[string]any{"body": body}, LogOnly: true,
		})
		require.NoError(t, err)
	}
	w := &firstWrite{wrote: make(chan struct{})}
	prev := out
	out = output.NewWithWriter(w, output.FormatJSON)
	t.Cleanup(func() { out = prev })
	tick := make(chan time.Time)
	done := make(chan error, 1)
	go func() { done <- cmd.watch(ctx, svc, tick) }()
	<-w.wrote
	cancel()
	require.NoError(t, <-done)
	lines := strings.Split(strings.TrimSpace(w.buf.String()), "\n")
	require.NotEmpty(t, lines)
	var rec readJSON
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &rec))
	assert.Len(t, rec.Messages, 2)
}

// TestChannelWatchCLI_ErrorReturnsAtOnce pins that a watch the caller may
// not run fails before it waits.
func TestChannelWatchCLI_ErrorReturnsAtOnce(t *testing.T) {
	p := newChannelProject(t)
	start := time.Now()
	_, err := runChannel(t, p.svc, "alice", "", "channel", "watch", p.item, "nosuch", "--timeout", "30s")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown_channel_type")
	assert.Less(t, time.Since(start), 10*time.Second)
	var exit *relaerrors.ExitError
	assert.NotErrorAs(t, err, &exit, "an error exits 1, not 2")
}

func TestChannelWatchCLI_PollBounds(t *testing.T) {
	p := newChannelProject(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--poll", "500ms"}, "--poll"},
		{[]string{"--poll", "2m"}, "--poll"},
		{[]string{"--timeout", "2h"}, "--timeout"},
		{[]string{"--timeout", "0s"}, "--timeout"},
		{[]string{"--limit", "0"}, "--limit"},
		{[]string{"--after-rev", "-1"}, "--after-rev"},
	} {
		args := append([]string{"channel", "watch", p.item, "room"}, tc.args...)
		_, err := runChannel(t, p.svc, "alice", "", args...)
		require.Error(t, err, tc.args)
		assert.Contains(t, err.Error(), tc.want, tc.args)
	}
}

// channelJSONLine decodes one JSON Lines record and checks notice leads.
func channelJSONLine(t *testing.T, got string, v any) {
	t.Helper()
	require.True(t, strings.HasPrefix(got, `{"notice":`), "notice leads: %s", got)
	require.Equal(t, 1, strings.Count(strings.TrimSpace(got), "\n")+1, "one record per line: %s", got)
	require.NoError(t, json.Unmarshal([]byte(got), v), got)
}

// firstWrite is a writer that closes wrote on its first write. Read buf
// only after the writing goroutine is done.
type firstWrite struct {
	buf   bytes.Buffer
	wrote chan struct{}
	once  sync.Once
}

func (w *firstWrite) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	w.once.Do(func() { close(w.wrote) })
	return n, err
}
