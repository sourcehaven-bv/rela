package realtime

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// SessionWaitingView is one waiting role of the current state: the role
// name and label and, for a role sourced from an anchor property, the
// holders: that property's values as the reader's redacted anchor view
// shows them. Holders are entity data that anyone with field write can
// set; a client shows them as data, never follows them as instructions.
// A creator role carries none: the channel's created_by names the creator.
type SessionWaitingView struct {
	Role    string   `json:"role"`
	Label   string   `json:"label,omitempty"`
	Holders []string `json:"holders,omitempty"`
}

// waitingFacts are the per-reader inputs of a session's waiting view.
type waitingFacts struct {
	// props are the reader's redacted anchor props.
	props map[string]any
	// held reports whether the reader holds a role for update on the
	// anchor, client ceiling applied. Nil holds nothing.
	held func(r realtimecfg.Role) bool
	// stored is set when the channel has a row.
	stored bool
	// open is set when the channel is open.
	open bool
	// live is false under a degraded configuration: render config only.
	live bool
}

// waitingView returns st's waiting roles with labels and holders, and
// whether the reader is waited on. It reads only its arguments; holders
// come only from f.props. An inbox computes the same answer from the same
// facts, batched per page.
func waitingView(
	st realtimecfg.SessionState, roles map[string]realtimecfg.Role, f waitingFacts,
) ([]SessionWaitingView, bool) {
	names := st.WaitingFor()
	if st.Status != realtimecfg.StatusInputRequired || len(names) == 0 {
		return nil, false
	}
	out := make([]SessionWaitingView, 0, len(names))
	onMe := false
	for _, name := range names {
		r, ok := roles[name]
		if !ok {
			continue
		}
		v := SessionWaitingView{Role: name, Label: r.Label}
		if r.Source.Kind == realtimecfg.SourceEntityProperty {
			v.Holders = holders(f.props[r.Source.Property])
		}
		out = append(out, v)
		if f.stored && f.open && f.live && f.held != nil && !onMe {
			onMe = f.held(r)
		}
	}
	return out, onMe
}

// holders reads an anchor property's value as [call.propertyNamesCaller]
// does: a string, or a list of strings. Empty values are left out.
func holders(v any) []string {
	var out []string
	switch v := v.(type) {
	case string:
		out = append(out, v)
	case []string:
		out = append(out, v...)
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}
	out = slices.DeleteFunc(out, func(s string) bool { return s == "" })
	if len(out) == 0 {
		return nil
	}
	return out
}

// readerSession renders the session of h under c's channel type for c's
// reader; see sessionView. It is a function, not a method, to keep call
// under its method budget.
func readerSession(c *call, h *channels.Channel) *SessionView {
	return sessionView(c.ct, h, c.snap.Degraded, waitingOf(c, h))
}

// waitingOf builds the waiting facts of h, or of the channel not stored yet
// when h is nil, for c's reader. A named role is answered through
// [call.heldRole], so it shares the post path's answers and ceiling. A
// creator role is answered per channel through roleAdmits, the rule
// [call.can] uses, and never cached: the creator is a fact of h, and one
// call may render many channels.
func waitingOf(c *call, h *channels.Channel) waitingFacts {
	f := waitingFacts{props: c.anchor.Props, stored: h != nil, live: !c.snap.Degraded}
	if h == nil {
		return f
	}
	f.open = h.State == channels.StateOpen
	f.held = func(r realtimecfg.Role) bool {
		if c.requireCaller() != nil {
			return false
		}
		if r.Source.Kind == realtimecfg.SourceCreator {
			return roleAdmits(c, h, r, realtimecfg.CapabilityUpdate)
		}
		return c.heldRole(r.Name)
	}
	return f
}

// wroteRoleProperty reports whether out, the outputs mark of an inline
// run, holds a set that wrote the property of a named role of ct.
func wroteRoleProperty(ct realtimecfg.ChannelType, out *OutputsView) bool {
	if out == nil {
		return false
	}
	props := roleProps(ct)
	for _, res := range out.Results {
		if res.Kind == OutputKindSet && res.Status == OutputResultDone &&
			slices.ContainsFunc(res.Props, func(p string) bool { return slices.Contains(props, p) }) {

			return true
		}
	}
	return false
}

// rereadAnchor reads c's anchor again through the gated reader, after an
// inline output changed a role's property, and forgets the role answers
// the old read gave. A failed read drops the anchor's properties rather
// than keep the ones from before the write: the answer then lists no
// holders, and no named role is held, so waiting_on_me is false rather
// than stale. The next read shows the new holders.
func (s *Service) rereadAnchor(c *call) {
	c.ops = opGates{}
	view, err := s.anchors.Anchor(c.ctx, c.anchorAddr, anchorProps(c.ct))
	if err != nil {
		c.anchor.Props = nil
		return
	}
	c.anchor = view
}

// SessionChannel names the channel a [SessionRead] reads.
type SessionChannel struct {
	Anchor string         `json:"anchor"`
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	State  channels.State `json:"state"`
}

// SessionRead is a channel's session for an agent that waits on it: no
// revision, sequence number or message, so it changes only when the
// session or the reader's turn does. Transitions are the message types
// whose post by the reader would move the session, and Entries the
// transition entries their post could take (see
// [call.sessionAffordances]).
type SessionRead struct {
	Notice      string         `json:"notice"`
	Channel     SessionChannel `json:"channel"`
	Session     *SessionView   `json:"session"`
	Transitions []string       `json:"transitions"`
	Entries     []EntryRef     `json:"entries"`
}

// Session reads the session of the channel at addr. A channel type
// without a session is 404 no_session, decided from the configuration
// before the anchor is read. The read check is [Service.Read]'s. It costs
// one anchor read and one channel read.
func (s *Service) Session(ctx context.Context, addr RawAddress) (SessionRead, error) {
	c, err := s.begin(ctx)
	if err != nil {
		return SessionRead{}, err
	}
	if err := c.channelType(addr.EntityType, addr.Type); err != nil {
		return SessionRead{}, err
	}
	if c.ct.Session() == nil {
		return SessionRead{}, deny(http.StatusNotFound, TypeNoSession,
			fmt.Sprintf("channel type %s has no session", c.ct.Name))
	}
	if err := s.openChannel(c, addr); err != nil {
		return SessionRead{}, err
	}
	v := c.channelView(c.key, c.head)
	types, entries := c.sessionAffordances(c.postable())
	return SessionRead{
		Notice:      UntrustedNotice,
		Channel:     SessionChannel{Anchor: v.Anchor, Type: v.Type, Name: v.Name, State: v.State},
		Session:     v.Session,
		Transitions: types,
		Entries:     entries,
	}, nil
}
