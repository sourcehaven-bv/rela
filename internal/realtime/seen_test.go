package realtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

func TestShardsFor(t *testing.T) {
	for _, tc := range []struct{ n, want int }{
		{0, 1},
		{realtimelimits.SeenPerShard, 1},
		{realtimelimits.SeenPerShard + 1, 2},
		{4*realtimelimits.SeenPerShard + 1, 8},
		{1 << 40, realtimelimits.MaxSeenShards},
	} {
		require.Equal(t, tc.want, shardsFor(tc.n), "n=%d", tc.n)
	}
}

func TestShardOf_FacesShareAShard(t *testing.T) {
	for _, shards := range []int{1, 2, 64} {
		n := shardOf("PAGE-1", shards)
		require.Equal(t, n, shardOf(idOf("PAGE-1@draft"), shards))
		require.Less(t, n, shards)
	}
}

func TestFingerprint(t *testing.T) {
	fields := []string{"status", "title"}
	a := fingerprint(fields, map[string]any{"status": "open", "title": "A", "owner": "x"})
	require.Equal(t, a, fingerprint(fields, map[string]any{"title": "A", "status": "open"}),
		"an unread field and key order do not count")
	require.NotEqual(t, a, fingerprint(fields, map[string]any{"status": "open", "title": "B"}))
	require.NotContains(t, a, "open", "the seen-set never holds a value")
}

func TestSeenStore_CorruptIndex(t *testing.T) {
	kv := newMemKV()
	s := seenStore{kv: kv}
	ctx := context.Background()
	idx, err := s.loadIndex(ctx)
	require.NoError(t, err)
	require.Empty(t, idx.Rules)
	require.NoError(t, kv.Put(ctx, seenIndexKey, []byte("{")))
	_, err = s.loadIndex(ctx)
	require.Error(t, err)
	require.NoError(t, kv.Put(ctx, shardKey("r", 1, 0), []byte("[")))
	_, err = s.loadShard(ctx, shardKey("r", 1, 0))
	require.Error(t, err)
}

func TestSystemWriter_ChecksProps(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	snap := th.cfg.Load()
	ct, ok := snap.Config.ChannelType("room")
	require.True(t, ok)
	_, err := th.trg.w.post(context.Background(), snap, th.clock.Now(), roomKey(), ct, ct.MessageTypes["note"],
		map[string]any{"level": 0}, "trigger:x", true)
	require.ErrorContains(t, err, "when: condition")
	_, err = th.trg.w.post(context.Background(), snap, th.clock.Now(), roomKey(), ct, ct.MessageTypes["text"],
		map[string]any{}, "trigger:x", true)
	require.Error(t, err)
	require.Zero(t, th.store.appends.Load())
}
