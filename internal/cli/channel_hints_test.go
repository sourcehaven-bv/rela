//go:build !memorybackend && !sqlite

package cli

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// A `rela channel post` in another process reaches this process's live
// subscribers: the command flushes its hint before it exits (AC 5).
func TestChannelCLI_PostReachesOtherProcess(t *testing.T) {
	p := newChannelProject(t)
	// Subscribed before the follower starts, so its start resync is taken
	// here and cannot swallow the post's hint.
	sub := appbuild.ChannelService(p.svc).SubscribeLive()
	defer sub.Close()
	ctx, cancel := context.WithCancel(context.Background())
	followed := appbuild.FollowChannels(ctx, p.svc)
	defer func() {
		cancel()
		<-followed
	}()
	require.Eventually(t, func() bool { return sub.TakeLive().Resync }, 15*time.Second, 10*time.Millisecond)

	_, stderr, code := runCLIProcess(t, []string{principal.RunAsEnv + "=system:ci"},
		"--project", p.root, "channel", "post", p.item, "room", "text", "-P", "body=from another process")
	require.Equal(t, 0, code, stderr)

	// The first post creates the channel, so it may arrive as a moved key,
	// which subsumes the durable one.
	var got []channels.Key
	require.Eventually(t, func() bool {
		c := sub.TakeLive()
		got = append(append(got, c.Durable...), c.Moved...)
		return slices.ContainsFunc(got, func(k channels.Key) bool { return k.ID == p.item && k.Type == "room" })
	}, 15*time.Second, 20*time.Millisecond, "the post never reached this process; got %v", got)
}
