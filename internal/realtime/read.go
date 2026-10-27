package realtime

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// State is a channel's lifecycle state. It is the store's type, exported
// here so a transport can name a state without importing the store.
type State = channels.State

// Key names one channel. It is the store's type, exported here so a
// transport can match the keys a [Subscription] takes without importing
// the store.
type Key = channels.Key

// Channel lifecycle states.
const (
	StateOpen     = channels.StateOpen
	StateClosed   = channels.StateClosed
	StateArchived = channels.StateArchived
)

// ReadRequest asks for a channel and one page of its messages.
//
// It names at most one cursor. AfterSeq and BeforeSeq count when they are
// above 0. AfterRev counts when it is set, so 0 is a valid rev: it follows
// a channel that has no message yet. Without a cursor the page is the
// newest messages.
type ReadRequest struct {
	AfterSeq  int64
	BeforeSeq int64
	AfterRev  *int64
	// Limit is clamped to the configured page limit; 0 means that limit.
	Limit int
	// Signals includes the live ephemeral signals.
	Signals bool
	// Schemas includes a description of each message type.
	Schemas bool
	// Actions includes the caller's affordances. A transport that does not
	// render them leaves it unset, and the post and edit gates are not
	// evaluated.
	Actions bool
	// LogOnly reports a transport that cannot post signals (see
	// [PostInput.LogOnly]). The caller's affordances and CanPost then
	// leave out the ephemeral message types.
	LogOnly bool
}

// Cursors name where the next read continues. Every transport takes them
// from here, so they follow one rule:
//
//   - An after_rev page names NextAfterRev. A full page continues at its
//     last rev. A page that is not full continues at the highest of its
//     last rev, the requested rev and the channel's rev: the channel was
//     read before the page, so nothing below that rev is skipped, and a rev
//     step that changed no message is passed.
//   - An after page names NextAfter: its last seq, or the requested seq
//     when it is empty.
//   - The newest page and a before page name NextBefore, the first seq,
//     when they hold a message. The newest page also names NextAfterRev,
//     the channel's rev, as the start for following the channel. A post
//     that lands between the two reads is then seen twice, never missed;
//     a reader dedupes by seq.
//
// NextAfterRev is a pointer so that rev 0 is sent.
type Cursors struct {
	NextAfter    int64  `json:"next_after,omitempty"`
	NextBefore   int64  `json:"next_before,omitempty"`
	NextAfterRev *int64 `json:"next_after_rev,omitempty"`
}

// MessageTypeView describes a message type of the channel to the caller.
type MessageTypeView struct {
	Name    string              `json:"name"`
	Label   string              `json:"label,omitempty"`
	Persist realtimecfg.Persist `json:"persist"`
	// TTLSeconds is how long a signal lives; 0 for a log type.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
	// CanPost reports whether the caller may post the type here now: they
	// hold a post role, the channel is open and the transport can carry
	// it. The type's `when:` depends on the message and is not evaluated.
	CanPost bool `json:"can_post"`
	// Reply reports whether a message of the type may reply to another.
	// Only log messages can.
	Reply bool                    `json:"reply"`
	Props realtimecfg.PropsSchema `json:"props_schema"`
}

// ReadView is a channel and one page of its messages, oldest first, or by
// rev for an after_rev page. HasMore is set when the page is full: there
// may be more past its cursor.
type ReadView struct {
	Channel      ChannelView       `json:"channel"`
	Actions      *Affordances      `json:"actions,omitempty"`
	MessageTypes []MessageTypeView `json:"message_types,omitempty"`
	Signals      []EphemeralView   `json:"signals,omitempty"`
	// TypingNow lists who else is typing, with Signals; see
	// [Service.Typing]. It holds no operator signal, and Signals holds no
	// typing entry.
	TypingNow []TypingView  `json:"typing_now,omitempty"`
	Messages  []MessageView `json:"messages"`
	HasMore   bool          `json:"has_more"`
	Cursors
}

// Read returns the channel at addr and one page of its messages in one
// operation: one config snapshot, one access, one channel read before one
// page read, and one ref batch for the page and the signals.
func (s *Service) Read(ctx context.Context, addr RawAddress, req ReadRequest) (ReadView, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return ReadView{}, err
	}
	page, err := readPage(req, c.snap.Config.Limits().PageLimit)
	if err != nil {
		return ReadView{}, err
	}
	out := ReadView{Channel: c.channelView(c.key, c.head), Messages: []MessageView{}}
	if req.Actions || req.Schemas {
		post := c.postable()
		if req.LogOnly {
			maps.DeleteFunc(post, func(name string, _ bool) bool {
				return c.ct.MessageTypes[name].Persist == realtimecfg.PersistEphemeral
			})
		}
		if req.Actions {
			a := c.affordancesWith(post)
			out.Actions = &a
		}
		if req.Schemas {
			out.MessageTypes = c.messageTypes(post)
		}
	}
	var msgs []channels.Message
	if c.head != nil {
		msgs, err = s.store.Messages(c.ctx, c.key, page)
		if err != nil && !errors.Is(err, channels.ErrNotFound) {
			return ReadView{}, storeError(err)
		}
	}
	var sigs []signal
	if req.Signals {
		sigs, out.TypingNow = s.liveSignals(c)
	}
	r, err := s.renderer(c, msgs, sigs)
	if err != nil {
		return ReadView{}, err
	}
	for _, m := range msgs {
		out.Messages = append(out.Messages, r.message(m))
	}
	for _, sg := range sigs {
		out.Signals = append(out.Signals, r.signal(sg))
	}
	out.HasMore = len(msgs) >= page.Limit
	out.Cursors = cursors(req, out)
	return out, nil
}

// readPage is the store page of req, its limit clamped to pageLimit.
func readPage(req ReadRequest, pageLimit int) (channels.Page, error) {
	page := channels.Page{AfterSeq: req.AfterSeq, BeforeSeq: req.BeforeSeq, Limit: req.Limit}
	if req.AfterRev != nil {
		page.AfterRev, page.ByRev = *req.AfterRev, true
	}
	if page.Limit <= 0 || page.Limit > pageLimit {
		page.Limit = pageLimit
	}
	page, err := page.Normalize()
	if err != nil {
		return channels.Page{}, storeError(err)
	}
	return page, nil
}

// cursors applies the cursor rule of [Cursors] to the page in v.
func cursors(req ReadRequest, v ReadView) Cursors {
	var out Cursors
	msgs := v.Messages
	switch {
	case req.AfterRev != nil:
		next := max(*req.AfterRev, v.Channel.Rev)
		if len(msgs) > 0 {
			last := msgs[len(msgs)-1].Rev
			next = max(next, last)
			if v.HasMore {
				next = last
			}
		}
		out.NextAfterRev = &next
	case req.AfterSeq > 0:
		out.NextAfter = req.AfterSeq
		if len(msgs) > 0 {
			out.NextAfter = msgs[len(msgs)-1].Seq
		}
	default:
		if len(msgs) > 0 {
			out.NextBefore = msgs[0].Seq
		}
		if req.BeforeSeq == 0 {
			rev := v.Channel.Rev
			out.NextAfterRev = &rev
		}
	}
	return out
}

// messageTypes describes c's message types, sorted by name. post is the
// set from [call.postable], less what the transport cannot carry.
func (c *call) messageTypes(post map[string]bool) []MessageTypeView {
	out := make([]MessageTypeView, 0, len(c.ct.MessageTypes))
	for _, name := range sortedKeys(c.ct.MessageTypes) {
		mt := c.ct.MessageTypes[name]
		ttl, reply := messageTypeTiming(mt)
		out = append(out, MessageTypeView{
			Name: mt.Name, Label: mt.Label, Persist: mt.Persist, TTLSeconds: ttl, Reply: reply,
			CanPost: post[name],
			Props:   mt.PropsSchema(),
		})
	}
	return out
}

// signalTTL is how long a signal of mt lives.
func signalTTL(mt *realtimecfg.MessageType) time.Duration {
	ttl := realtimelimits.MaxEphemeralTTL
	if mt.TTL > 0 {
		ttl = min(mt.TTL, ttl)
	}
	return ttl
}
