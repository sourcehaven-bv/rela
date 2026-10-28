package realtime

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// ChannelRead is the caller's read marker on a channel: the highest seq
// they marked read, and how many counting messages lie above it, at most
// [channels.UnreadCap]. A marker is the caller's own: no one else sees it.
type ChannelRead struct {
	Seq    int64 `json:"seq"`
	Unread int   `json:"unread"`
}

// UnreadCap is the highest unread count a marker reports; a count at the
// cap means "this many or more".
const UnreadCap = channels.UnreadCap

// MarkReadResult is a marker after a mark: the stored values, which may be
// above the seq asked for when another tab or a post moved it further.
type MarkReadResult struct {
	Seq     int64 `json:"seq"`
	Unread  int   `json:"unread"`
	LastSeq int64 `json:"last_seq"`
}

// callReader returns the reader whose markers c's caller reads and moves,
// and false when the caller has no identity a marker can name.
func callReader(c *call) (channels.Reader, bool) {
	if c.requireCaller() != nil {
		return channels.Reader{}, false
	}
	return channels.ReaderOf(authorOf(c.p)), true
}

// MarkRead moves the caller's read marker on the channel at addr up to seq,
// or to the channel's last seq when seq is nil. It needs read access only.
// The marker never moves back, and a seq above the last one is clamped.
// A cardinality-one channel that does not exist yet answers zeros and
// stores nothing. Marks have a rate limit of their own
// ([realtimelimits.ReadRate] per channel, under
// [realtimelimits.ReadIdentityRate]), apart from posts. Every stored
// mark, moved or not, reaches [Service.SubscribeLive] tagged with the
// caller, and no plain subscription: a first marker at 0 moves nothing
// but adds the channel to the caller's inbox window. The read rate limit
// bounds how often.
func (s *Service) MarkRead(ctx context.Context, addr RawAddress, seq *int64) (MarkReadResult, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return MarkReadResult{}, err
	}
	reader, ok := callReader(c)
	if !ok {
		return MarkReadResult{}, c.requireCaller()
	}
	if seq != nil && *seq < 0 {
		return MarkReadResult{}, invalid("seq must be 0 or more")
	}
	if c.head == nil {
		return MarkReadResult{}, nil
	}
	err = s.local.readLimiter.take(c.now, bucketIdentity(c.p), c.key, opRead, realtimelimits.ReadRate)
	if err != nil {
		return MarkReadResult{}, err
	}
	target := c.head.LastSeq
	if seq != nil {
		target = *seq
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	st, _, err := s.store.SetRead(c.ctx, c.key, reader, target, c.now)
	if err != nil {
		return MarkReadResult{}, storeError(err)
	}
	s.local.markFor(reader, c.key)
	return MarkReadResult{Seq: st.Seq, Unread: st.Unread, LastSeq: st.LastSeq}, nil
}

// readsOf returns c's caller's markers on keys from st, in chunks of
// chunk keys ([channels.MaxReadKeys] unless a test lowers it). A caller
// without an identity, or no keys, makes no store call and gets an empty
// map.
func readsOf(st channels.Store, chunk int, c *call, keys []channels.Key) (map[channels.Key]channels.ReadState, error) {
	out := make(map[channels.Key]channels.ReadState, len(keys))
	reader, ok := callReader(c)
	if len(keys) == 0 || !ok {
		return out, nil
	}
	for part := range slices.Chunk(keys, chunk) {
		got, err := st.Reads(c.ctx, reader, part)
		if err != nil {
			return nil, fmt.Errorf("realtime: read markers: %w", err)
		}
		maps.Copy(out, got)
	}
	return out, nil
}

// readView is the [ChannelRead] of st.
func readView(st channels.ReadState) *ChannelRead {
	return &ChannelRead{Seq: st.Seq, Unread: st.Unread}
}
