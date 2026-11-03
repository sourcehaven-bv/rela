package realtime

import (
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// AuthorView is who wrote a message, as a reader sees it.
type AuthorView struct {
	User          string `json:"user"`
	Tool          string `json:"tool,omitempty"`
	PrincipalType string `json:"principal_type,omitempty"`
	// System is set on a message a trigger posted, so a client tells it
	// from a person's without reading the user name.
	System bool `json:"system,omitempty"`
}

func authorView(a channels.Author) AuthorView {
	return AuthorView{User: a.User, Tool: a.Tool, PrincipalType: a.PrincipalType}
}

// RefValue renders a ref property. A readable entity shows its id, type
// and display title. An entity the reader cannot read, or that no longer
// exists, shows only Restricted: not even its id, which the reader may not
// have known.
type RefValue struct {
	ID         string `json:"id,omitempty"`
	Type       string `json:"type,omitempty"`
	Title      string `json:"title,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`
}

// MentionView is one principal a message addresses, as a reader sees it.
// Text is the span text the author wrote, so a client can match the span
// in the rendered markdown. Kind is the stored form: "user", "job" or
// "login". Ref is set for a user the reader may read; a user the reader
// cannot read has no MentionView at all.
type MentionView struct {
	Text string    `json:"text"`
	Kind string    `json:"kind"`
	Ref  *RefValue `json:"ref,omitempty"`
}

// RemovedView is the removal record of a removed message. By and Reason
// are set only for a reader who passes the remove gate.
type RemovedView struct {
	At     time.Time            `json:"at"`
	Kind   channels.RemovalKind `json:"kind"`
	By     *AuthorView          `json:"by,omitempty"`
	Reason string               `json:"reason,omitempty"`
}

// ActionView is the action taken on a message. Promoted is a [RefValue],
// so a promoted entity the reader cannot read is restricted.
type ActionView struct {
	Name     string                `json:"name"`
	Status   channels.ActionStatus `json:"status"`
	At       time.Time             `json:"at"`
	Promoted *RefValue             `json:"promoted,omitempty"`
}

// MessageView is one message as a reader sees it. A removed message keeps
// only Seq, Rev, ReplyTo, ReplyCount and Removed: the store erases its
// type, author, time and properties.
//
// Props holds only the properties the message's type declares now. A
// message whose type the configuration no longer declares shows none.
// A session notice is the exception: its type is never declared, and it
// shows its props from, to, generation and cause.
type MessageView struct {
	Seq        int64          `json:"seq"`
	Rev        int64          `json:"rev"`
	Type       string         `json:"type,omitempty"`
	Author     *AuthorView    `json:"author,omitempty"`
	At         time.Time      `json:"at,omitzero"`
	Props      map[string]any `json:"props,omitempty"`
	ReplyTo    int64          `json:"reply_to,omitempty"`
	ReplyCount int            `json:"reply_count,omitempty"`
	EditedAt   time.Time      `json:"edited_at,omitzero"`
	// Mentions lists the principals the message addresses that the reader
	// may see (see [MentionView]). Nil when there are none.
	Mentions []MentionView `json:"mentions,omitempty"`
	Removed  *RemovedView  `json:"removed,omitempty"`
	Action   *ActionView   `json:"action,omitempty"`
	// Outputs is a session notice's outputs mark, in place of Action.
	Outputs *OutputsView `json:"outputs,omitempty"`
	// TriggeredBy names the trigger that posted the message, as
	// "trigger:<name>"; empty for a message a person or agent posted.
	TriggeredBy string `json:"triggered_by,omitempty"`
	// Actions lists what the reader may do with the message now, sorted:
	// the message's actions and the operations [OpEdit], [OpDelete],
	// [OpRemove] and [OpRelease]. Nil when there is nothing.
	Actions []string `json:"_actions,omitempty"`
	// EditUntil and ReleaseAfter are when Actions changes by itself: edit
	// is offered until EditUntil, and release is offered from ReleaseAfter.
	// Each is set only when that change is the next one for this reader
	// (see [call.messageDeadlines]); zero otherwise.
	EditUntil    time.Time `json:"edit_until,omitzero"`
	ReleaseAfter time.Time `json:"release_after,omitzero"`
}

// IsSessionNotice reports whether m is a session notice.
func (m MessageView) IsSessionNotice() bool { return isNotice(m.Type, m.TriggeredBy) }

// IsReminder reports whether m was posted by a session reminder.
func (m MessageView) IsReminder() bool {
	return strings.HasPrefix(m.TriggeredBy, channels.ReminderMarker)
}

// EphemeralView is one live signal.
type EphemeralView struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Author    AuthorView     `json:"author"`
	At        time.Time      `json:"at"`
	ExpiresAt time.Time      `json:"expires_at"`
	Props     map[string]any `json:"props,omitempty"`
}

// ChannelView is a channel's metadata as a reader sees it.
type ChannelView struct {
	Anchor     string         `json:"anchor"`
	Type       string         `json:"type"`
	Name       string         `json:"name,omitempty"`
	State      channels.State `json:"state"`
	Title      string         `json:"title,omitempty"`
	CreatedBy  *AuthorView    `json:"created_by,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitzero"`
	ClosedAt   time.Time      `json:"closed_at,omitzero"`
	ArchivedAt time.Time      `json:"archived_at,omitzero"`
	LastSeq    int64          `json:"last_seq"`
	Rev        int64          `json:"rev"`
	// Session is the channel's session; nil for a type without one.
	Session *SessionView `json:"session,omitempty"`
	// Read is the caller's read marker; nil when they hold none. Only the
	// channel list and the head fill it.
	Read *ChannelRead `json:"read,omitempty"`
}

// channelView renders h of c's channel type, or the empty open channel at
// key when h is nil, for c's reader.
func (c *call) channelView(key channels.Key, h *channels.Channel) ChannelView {
	v := ChannelView{
		Anchor: entity.FormatStateRef(key.ID, key.Face), Type: key.Type, Name: key.Name,
		State: channels.StateOpen, Session: readerSession(c, h),
	}
	if h == nil {
		return v
	}
	by := authorView(h.CreatedBy)
	v.State, v.Title, v.CreatedBy, v.CreatedAt = h.State, h.Title, &by, h.CreatedAt
	v.ClosedAt, v.ArchivedAt, v.LastSeq, v.Rev = h.ClosedAt, h.ArchivedAt, h.LastSeq, h.Rev
	return v
}

// renderer renders the messages of one channel for one reader. It collects
// every ref id first, so a page costs one Resolve, however many refs.
type renderer struct {
	ct realtimecfg.ChannelType
	// stored holds the refs the store recorded for ct, as
	// "message-type.property". They render as refs whatever ct declares.
	stored []string
	// moderator is set when the reader passes the remove gate: they see who
	// removed a message and why.
	moderator bool
	refs      map[string]RefView
	// call, when set, decides the actions each message offers.
	call *call
	// userType is the mention user type; a user mention that resolves to
	// another type is left out. Empty means no check.
	userType string
}

// refIDs returns the entity ids msgs and sigs name in ref properties,
// promoted actions and outputs results, deduplicated and sorted.
func (r *renderer) refIDs(msgs []channels.Message, sigs []signal) []string {
	var ids []string
	collect := func(typ string, props map[string]any) {
		mt := r.ct.MessageTypes[typ]
		if mt == nil {
			return
		}
		for name, p := range mt.Properties {
			if !r.isRef(typ, name, p) {
				continue
			}
			if id, ok := props[name].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
	}
	for _, m := range msgs {
		if m.Removed != nil {
			continue
		}
		collect(m.Type, m.Props)
		ids = append(ids, messageEntityIDs(m)...)
	}
	for _, s := range sigs {
		collect(s.typ, s.props)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// messageEntityIDs returns the entity ids m names outside its properties:
// its user mention targets, its promoted entity and its outputs results.
//
// Nil: returned when m names none.
func messageEntityIDs(m channels.Message) []string {
	var ids []string
	for _, t := range m.Mentions {
		if t.Form == channels.FormUser {
			ids = append(ids, t.Target)
		}
	}
	if m.Action != nil && m.Action.Promoted != nil {
		ids = append(ids, m.Action.Promoted.ID)
	}
	if isOutputs(m.Action) {
		for _, res := range m.Action.Outputs.Results {
			if res.Entity != nil {
				ids = append(ids, res.Entity.ID)
			}
		}
	}
	return ids
}

// isRef reports whether property name of message type typ renders as a
// ref: the configuration declares it one, or the store recorded it as one.
func (r *renderer) isRef(typ, name string, p realtimecfg.Property) bool {
	return p.Type == realtimecfg.PropertyRef || slices.Contains(r.stored, typ+"."+name)
}

func (r *renderer) ref(id string) *RefValue {
	if v, ok := r.refs[id]; ok {
		return &RefValue{ID: v.ID, Type: v.Type, Title: v.Title}
	}
	return &RefValue{Restricted: true}
}

// props renders the declared properties of a message of type typ.
func (r *renderer) props(typ string, in map[string]any) map[string]any {
	mt := r.ct.MessageTypes[typ]
	if mt == nil || len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for name, v := range in {
		p, ok := mt.Properties[name]
		if !ok || v == nil {
			continue
		}
		if r.isRef(typ, name, p) {
			id, _ := v.(string)
			out[name] = r.ref(id)
			continue
		}
		out[name] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *renderer) message(m channels.Message) MessageView {
	v := MessageView{Seq: m.Seq, Rev: m.Rev, ReplyTo: m.ReplyTo, ReplyCount: m.ReplyCount}
	if m.Removed != nil {
		kind := m.Removed.Kind
		if kind == "" {
			kind = channels.RemovalModerator
		}
		rv := &RemovedView{At: m.Removed.At, Kind: kind}
		if r.moderator {
			by := authorView(m.Removed.By)
			rv.By, rv.Reason = &by, m.Removed.Reason
		}
		v.Removed = rv
		return v
	}
	a := authorView(m.Author)
	a.System = m.TriggeredBy != ""
	v.Type, v.Author, v.At, v.EditedAt, v.TriggeredBy = m.Type, &a, m.At, m.EditedAt, m.TriggeredBy
	v.Props = r.props(m.Type, m.Props)
	v.Mentions = r.mentions(m.Mentions)
	if isNotice(m.Type, m.TriggeredBy) {
		// A notice's type is not declared, but its props are the record of
		// the transition: from, to, generation and cause.
		v.Props = m.Props
	}
	if isOutputs(m.Action) {
		v.Outputs = r.outputsView(m.Action)
	} else if m.Action != nil {
		av := &ActionView{Name: m.Action.Name, Status: m.Action.Status, At: m.Action.At}
		if m.Action.Promoted != nil {
			av.Promoted = r.ref(m.Action.Promoted.ID)
		}
		v.Action = av
	}
	if r.call != nil {
		v.Actions = r.call.messageActions(m)
		v.EditUntil, v.ReleaseAfter = r.call.messageDeadlines(m, v.Actions)
	}
	return v
}

// mentions renders the stored targets of a message by their stored form,
// never re-deriving it from the current configuration. A user target is
// looked up by its id, then by the span the author wrote (a post's own
// answer resolved the span). A user the reader cannot read, or that no
// longer has the mention user type, is left out.
//
// Nil: returned when no target is visible.
func (r *renderer) mentions(ms []channels.Mention) []MentionView {
	var out []MentionView
	for _, m := range ms {
		text := m.Target
		if m.Span != "" {
			text = m.Span
		}
		switch m.Form {
		case channels.FormUser:
			ref, ok := r.refs[m.Target]
			if !ok && m.Span != "" {
				ref, ok = r.refs[m.Span]
			}
			if !ok || (r.userType != "" && ref.Type != r.userType) {
				continue
			}
			out = append(out, MentionView{Text: text, Kind: string(m.Form),
				Ref: &RefValue{ID: ref.ID, Type: ref.Type, Title: ref.Title}})
		case channels.FormLogin:
			if m.Span == "" {
				text = loginPrefix + m.Target
			}
			out = append(out, MentionView{Text: text, Kind: string(m.Form)})
		case channels.FormJob:
			out = append(out, MentionView{Text: text, Kind: string(m.Form)})
		}
	}
	return out
}

func (r *renderer) signal(s signal) EphemeralView {
	return EphemeralView{
		ID: s.id, Type: s.typ, Author: authorView(s.author), At: s.at, ExpiresAt: s.expires,
		Props: r.props(s.typ, s.props),
	}
}
