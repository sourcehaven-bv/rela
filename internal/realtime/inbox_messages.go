package realtime

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// One inbox page names at most MaxInboxLimit waiting messages, and one
// Store.GetMessages call reads that many; this fails to compile when the
// two drift apart.
var _ [channels.MaxGetMessages - MaxInboxLimit]struct{}

var _ [MaxInboxLimit - channels.MaxGetMessages]struct{}

// ErrInboxReader is the refusal of [WaitingMessages] for a view answered
// for another reader. It is a programming error: the view must come from
// [Service.Inbox] for the same principal.
var ErrInboxReader = errors.New("realtime: the inbox view was answered for another reader")

// WaitingMessages returns v with the waiting message of each waiting
// entry (see [InboxEntry.Message]). It is a separate step from
// [Service.Inbox], so a caller that needs only the counts (the badge)
// never pays for it.
//
// It works only on the entries v already holds, which [Service.Inbox]
// gated for the caller, so it gates nothing new and reads nothing for any
// other row. It costs one [channels.Store.GetMessages] call and one ref
// resolve batch, whatever the number of entries. An entry whose session
// has no cause, whose message is gone, or whose channel type the current
// configuration no longer declares keeps no message.
//
// v must be the caller's own answer: a view whose reader is not the
// caller's is refused with [ErrInboxReader].
func WaitingMessages(ctx context.Context, s *Service, v InboxView) (InboxView, error) {
	if len(v.page.refs) == 0 {
		return v, nil
	}
	c, err := s.begin(ctx)
	if err != nil {
		return InboxView{}, err
	}
	if denied := c.requireCaller(); denied != nil {
		return InboxView{}, denied
	}
	if reader, _ := callReader(c); reader != v.Reader {
		return InboxView{}, ErrInboxReader
	}
	// Under another configuration than the one that gated the entries, a
	// reload may have revoked the read or remove role they were judged by;
	// the entries then keep no message, as if the reload came first. Each
	// entry pairs with its waitingRef by index; a view whose two lists
	// differ in length was not built by Service.Inbox.
	if c.snap.Degraded || c.snap.Config != v.page.config || len(v.page.refs) != len(v.Entries) {
		return v, nil
	}
	cfg := c.snap.Config
	var refs []channels.MessageRef
	for _, w := range v.page.refs {
		if _, ok := cfg.ChannelType(w.key.Type); ok && w.cause > 0 {
			refs = append(refs, channels.MessageRef{Key: w.key, Seq: w.cause})
		}
	}
	if len(refs) == 0 {
		return v, nil
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	got, err := s.store.GetMessages(c.ctx, refs)
	if err != nil {
		return InboxView{}, fmt.Errorf("realtime: read waiting messages: %w", err)
	}
	rs := make([]*renderer, len(v.page.refs))
	var ids []string
	for i, w := range v.page.refs {
		m, ok := got[channels.MessageRef{Key: w.key, Seq: w.cause}]
		ct, declared := cfg.ChannelType(w.key.Type)
		if !ok || !declared {
			continue
		}
		//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
		stored, refErr := s.local.storedRefs(c.ctx, s.store, cfg, ct.Name)
		if refErr != nil {
			return InboxView{}, refErr
		}
		rs[i] = messageRenderer(ct, w.moderator, cfg.MentionUserType(), stored)
		ids = append(ids, rs[i].refIDs([]channels.Message{m}, nil)...)
	}
	resolved := map[string]RefView{}
	if len(ids) > 0 {
		slices.Sort(ids)
		//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
		if resolved, err = s.refs.Resolve(c.ctx, slices.Compact(ids)); err != nil {
			return InboxView{}, fmt.Errorf("realtime: resolve refs: %w", err)
		}
	}
	v.Entries = slices.Clone(v.Entries)
	for i, w := range v.page.refs {
		if rs[i] == nil {
			continue
		}
		rs[i].refs = resolved
		mv := rs[i].message(got[channels.MessageRef{Key: w.key, Seq: w.cause}])
		v.Entries[i].Message = &mv
	}
	return v, nil
}

// messageRenderer is a renderer of ct's messages without a call: it
// renders no per-message actions or deadlines. moderator decides whether
// a removal shows who removed the message and why; stored are the refs
// the store recorded for ct (see Local.storedRefs).
func messageRenderer(ct realtimecfg.ChannelType, moderator bool, userType string, stored []string) *renderer {
	return &renderer{ct: ct, moderator: moderator, userType: userType, stored: stored}
}
