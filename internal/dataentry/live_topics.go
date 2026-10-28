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
// changed since the last call, or resync when it lost count. TakeReads
// returns the channels on which each reader's marker was written: such a
// change replays only the subscriptions that show that reader's marker,
// and that reader's inbox. TakeDurable returns the channels a stored write
// changed, and TakeAddressed the principal keys a post or edit addressed;
// both select inbox subscriptions (see live_inbox.go).
type channelChanges interface {
	Wake() <-chan struct{}
	Take() (keys []channels.Key, resync bool)
	TakeReads() map[channels.Reader][]channels.Key
	TakeDurable() []channels.Key
	TakeAddressed() []string
	Close()
}

// roundFilter selects the subscriptions a topic round replays. A nil
// filter selects every subscription (a full round).
type roundFilter struct {
	topics map[string]bool
	// allChannels selects every channel subscription that has a topic: the
	// channel subscription lost count of what changed. A subscription
	// without a topic is not selected: its last replay was refused, so a
	// channel change cannot move its response, and replaying it, or telling
	// it to refetch when the round overflows, would frame it at the time of
	// a message it cannot read.
	allChannels bool
	// readTopics selects, per reader, the channel subscriptions of that
	// reader on the topics of the channels where its marker moved, and
	// that reader's inbox subscriptions.
	readTopics map[channels.Reader]map[string]bool
	// durable holds the head topics of the channels a stored write
	// changed, and types their channel types; addressed holds the
	// principal keys a post or edit addressed. They select inbox
	// subscriptions ([roundFilter.selectsInbox]).
	durable   map[string]bool
	types     map[string]bool
	addressed map[string]bool
}

// addReads adds the topics of the channels where each reader's marker
// moved, for that reader's subscriptions only.
func (f *roundFilter) addReads(reads map[channels.Reader][]channels.Key) {
	for r, keys := range reads {
		if f.readTopics == nil {
			f.readTopics = map[channels.Reader]map[string]bool{}
		}
		set := f.readTopics[r]
		if set == nil {
			set = map[string]bool{}
			f.readTopics[r] = set
		}
		for _, k := range keys {
			for _, t := range channelKeyTopics(k) {
				set[t] = true
			}
		}
	}
}

// newRoundFilter is the filter of a round over the presence faces, the
// changed channels keys and, when resync is set, every channel subscription
// that has a topic.
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
	if s.inbox {
		return f.selectsInbox(s)
	}
	if !s.channel || s.topic == "" {
		return false
	}
	return f.allChannels || f.topics[s.topic] || s.reader.User != "" && f.readTopics[s.reader][s.topic]
}

// topicProbe is where a replay of a channel route reports the topic it
// resolved. [replayHash] allocates one per call.
type topicProbe struct {
	topic  string
	reader channels.Reader
	// inbox is the wake set an inbox route recorded (see recordInbox).
	inbox *inboxWake
}

type topicProbeKey struct{}

func withTopicProbe(ctx context.Context) (context.Context, *topicProbe) {
	p := &topicProbe{}
	return context.WithValue(ctx, topicProbeKey{}, p), p
}

// recordTopic fills the request's probe, when it has one. The channel
// handler calls it only on a successful read, with the key the service
// resolved, and the reader whose marker the answer shows.
func recordTopic(ctx context.Context, topic string, reader channels.Reader) {
	if p, _ := ctx.Value(topicProbeKey{}).(*topicProbe); p != nil {
		p.topic, p.reader = topic, reader
	}
}

// applyTopic sets the topic of the channel subscription s, or the wake
// set of the inbox subscription s, after a replay that ran, with result
// res. A 200 takes what its probe recorded; any other status clears it. A
// replay that did not finish keeps it, so the next channel change still
// replays s. Callers hold the stream's mu.
func applyTopic(s *liveSub, res replayResult) {
	if !res.ok {
		return
	}
	if s.inbox {
		s.wake = nil
		if res.status == http.StatusOK {
			s.wake = res.inbox
		}
		return
	}
	if !s.channel {
		return
	}
	if res.status == http.StatusOK {
		s.topic, s.reader = res.topic, res.reader
		return
	}
	s.topic, s.reader = "", channels.Reader{}
}
