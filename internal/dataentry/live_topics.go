package dataentry

import (
	"context"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Topics let a round replay only the subscriptions a change can move
// (TKT-ZU7P13). A change of viewers moves the presence subscriptions of one
// face, and a channel change moves the heads of that channel and the
// channel lists of its anchor. Everything else is a full round, as before.
//
// A channel subscription learns its topic from its own replay: the channel
// handler records the key it resolved in a [topicProbe], and the hub keeps
// it only after a 200 (see applyTopic). A topic therefore
// never comes from the client, and a subscription the reader may not read
// has none, so a channel change does not replay it. A full round, which
// any graph write or lease change starts, still replays it, so a reader who
// gains access is framed then.

// Topic keys. A prefix keeps the three kinds apart in one filter set.
const (
	topicFace    = "f:" // a presence face: presenceKey of the entity
	topicAnchor  = "a:" // the channel list of an entity family: its id
	topicChannel = "c:" // a channel head: channels.Key.String()
)

// faceTopic is the topic of a presence face.
func faceTopic(face string) string { return topicFace + face }

// anchorTopic is the topic of the channel list of the entity family id.
// The list route shows every channel of the family's faces the reader may
// read, so it listens on the family, not on one face.
func anchorTopic(id string) string { return topicAnchor + id }

// channelTopic is the topic of the head of one channel. anchor is the
// channel's anchor as [entity.FormatStateRef] renders it, which is how
// [channels.Key.String] renders it too.
func channelTopic(anchor, typ, name string) string {
	t := topicChannel + anchor + "/" + typ
	if name != "" {
		t += "/" + name
	}
	return t
}

// channelKeyTopics are the topics a change of channel k moves: its head
// and the channel list of its anchor's family.
func channelKeyTopics(k channels.Key) []string {
	return []string{
		channelTopic(entity.FormatStateRef(k.ID, k.Face), k.Type, k.Name),
		anchorTopic(k.ID),
	}
}

// channelChanges is the channel change subscription the live worker reads.
// *realtime.Subscription satisfies it: Take returns the channels that
// changed since the last call, or resync when it lost count.
type channelChanges interface {
	Wake() <-chan struct{}
	Take() (keys []channels.Key, resync bool)
	Close()
}

// roundFilter selects the subscriptions a topic round replays. A nil
// filter selects every subscription (a full round).
type roundFilter struct {
	topics map[string]bool
	// allChannels selects every channel subscription, topic or not: the
	// channel subscription lost count of what changed.
	allChannels bool
}

// newRoundFilter is the filter of a round over the presence faces, the
// changed channels keys and, when resync is set, every channel subscription.
func newRoundFilter(faces []string, keys []channels.Key, resync bool) *roundFilter {
	f := &roundFilter{topics: make(map[string]bool, len(faces)+2*len(keys)), allChannels: resync}
	for _, face := range faces {
		f.topics[faceTopic(face)] = true
	}
	for _, k := range keys {
		for _, t := range channelKeyTopics(k) {
			f.topics[t] = true
		}
	}
	return f
}

// selects reports whether the round replays s.
func (f *roundFilter) selects(s *liveSub) bool {
	if s.presenceID != "" {
		if s.watching != "" && f.topics[faceTopic(s.watching)] || s.left != "" && f.topics[faceTopic(s.left)] {
			return true
		}
	}
	return s.channel && (f.allChannels || s.topic != "" && f.topics[s.topic])
}

// topicProbe is where a replay of a channel route reports the topic it
// resolved. [replayHash] allocates one per call.
type topicProbe struct{ topic string }

type topicProbeKey struct{}

func withTopicProbe(ctx context.Context) (context.Context, *topicProbe) {
	p := &topicProbe{}
	return context.WithValue(ctx, topicProbeKey{}, p), p
}

// recordTopic fills the request's probe, when it has one. The channel
// handler calls it only on a successful read, with the key the service
// resolved.
func recordTopic(ctx context.Context, topic string) {
	if p, _ := ctx.Value(topicProbeKey{}).(*topicProbe); p != nil {
		p.topic = topic
	}
}

// applyTopic sets the topic of the channel subscription s after a replay
// that ran, with result res. A 200 takes the topic its probe resolved; any
// other status clears it. A replay that did not finish keeps the topic, so
// the next channel change still replays it. Callers hold the stream's mu.
func applyTopic(s *liveSub, res replayResult) {
	if !s.channel || !res.ok {
		return
	}
	if res.status == http.StatusOK {
		s.topic = res.topic
		return
	}
	s.topic = ""
}
