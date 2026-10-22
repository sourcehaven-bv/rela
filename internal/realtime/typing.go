package realtime

import (
	"context"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// TypingResult answers a typing call. TTL is how long the entry stays live
// without a refresh; a client refreshes at half of it.
type TypingResult struct {
	TTL time.Duration
}

// TypingView is one live typing entry as a reader sees it: who is typing
// and since when. Nothing in it moves on a refresh, so a refresh changes no
// head and no read digest.
type TypingView struct {
	Author AuthorView `json:"author"`
	Since  time.Time  `json:"since"`
}

// Typing starts or refreshes the caller's typing entry in the channel at
// addr: "this person is writing a log message here now".
//
// The checks run in this order: read access (an unreadable anchor is the
// uniform not-found), an identity, a channel type with typing: true, a
// post role of at least one log message type, an open channel, room under
// the typing caps, then the rate. A refused call charges no token and keeps
// no entry. Only when another typist takes the last slot between the room
// check and the write is the token charged and the call refused.
//
// The rate is [realtimelimits.TypingRate] per identity and channel, and the
// call also draws on [realtimelimits.IdentityRate], the budget an identity
// shares across all its channel writes. limits.signal_rate does not apply.
//
// The entry is process local, never stored and never audited, and the call
// creates no channel. Readers are woken only when an entry starts, when a
// log post clears it and when it expires on the server; a refresh moves the
// deadline and wakes nobody.
func (s *Service) Typing(ctx context.Context, addr RawAddress) (TypingResult, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return TypingResult{}, err
	}
	err = c.requireCaller()
	if err != nil {
		return TypingResult{}, err
	}
	if !c.ct.Typing {
		return TypingResult{}, forbidden(TypeTypingNotOffered,
			"channel type "+c.ct.Name+" does not offer typing", "channel_types."+c.ct.Name+".typing")
	}
	if !c.mayType() {
		return TypingResult{}, typingDenied(c.ct)
	}
	err = c.requireOpen()
	if err != nil {
		return TypingResult{}, err
	}
	identity := bucketIdentity(c.p)
	err = s.local.typing.admits(c.key, identity, c.now)
	if err != nil {
		return TypingResult{}, err
	}
	err = s.local.limiter.take(c.now, identity, c.key, opTyping, realtimelimits.TypingRate)
	if err != nil {
		return TypingResult{}, err
	}
	started, pruned, err := s.local.typing.touch(c.key, identity, authorOf(c.p), c.now)
	if pruned || started {
		s.local.mark(c.key)
	}
	if err != nil {
		return TypingResult{}, err
	}
	s.local.scheduleExpiry()
	return TypingResult{TTL: realtimelimits.TypingTTL}, nil
}

// typingDenied is the 403 for a reader who holds no post role of any log
// message type. It names the union of those types' post roles.
func typingDenied(ct realtimecfg.ChannelType) *DenialError {
	var roles []string
	for _, name := range sortedKeys(ct.MessageTypes) {
		mt := ct.MessageTypes[name]
		if mt.Persist != realtimecfg.PersistLog {
			continue
		}
		for _, r := range mt.Post {
			if !slices.Contains(roles, r) {
				roles = append(roles, r)
			}
		}
	}
	return gateDenied(TypePostDenied, "typing in "+ct.Name+" channels",
		"channel_types."+ct.Name+".typing", realtimecfg.Gate{Roles: roles}, ct.Roles)
}

// mayType reports whether the caller holds a post role of at least one log
// message type of the channel.
func (c *call) mayType() bool {
	for _, name := range sortedKeys(c.ct.MessageTypes) {
		if mt := c.ct.MessageTypes[name]; mt.Persist == realtimecfg.PersistLog && c.mayPost(mt) {
			return true
		}
	}
	return false
}

// typingAllowed is the affordance of [Service.Typing]: the type offers
// typing, the caller has an identity and may post a log type, and the
// channel is open. The rate limit is outside it, as for posting.
func (c *call) typingAllowed() bool {
	return c.ct.Typing && c.requireCaller() == nil && c.state() == StateOpen && c.mayType()
}

// liveSignals returns what Head and Read show of c's channel beside its
// messages: the operator signals of the ring, and the typing entries
// without the caller's own identity. Typing is empty unless the type
// offers it and the channel is open, so a reload that turns typing off, or
// a close, hides held entries at once.
func (s *Service) liveSignals(c *call) (operator []signal, typing []TypingView) {
	operator = s.local.liveSignals(c.key)
	typing = []TypingView{}
	if !c.ct.Typing || c.state() != StateOpen {
		return operator, typing
	}
	self := bucketIdentity(c.p)
	for _, e := range s.local.typing.live(c.key, c.now) {
		if e.identity == self {
			continue
		}
		typing = append(typing, TypingView{Author: authorView(e.author), Since: e.since})
	}
	return operator, typing
}
