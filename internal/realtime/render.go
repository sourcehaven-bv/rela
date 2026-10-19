package realtime

import (
	"slices"
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
	Removed    *RemovedView   `json:"removed,omitempty"`
	Action     *ActionView    `json:"action,omitempty"`
	// Actions lists the actions the reader may run on the message now,
	// sorted. Nil when there are none.
	Actions []string `json:"_actions,omitempty"`
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
}

// channelView renders h, or the empty open channel at key when h is nil.
func channelView(key channels.Key, h *channels.Channel) ChannelView {
	v := ChannelView{
		Anchor: entity.FormatStateRef(key.ID, key.Face), Type: key.Type, Name: key.Name,
		State: channels.StateOpen,
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
}

// refIDs returns the entity ids msgs and sigs name in ref properties and
// promoted actions, deduplicated and sorted.
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
		if m.Action != nil && m.Action.Promoted != nil {
			ids = append(ids, m.Action.Promoted.ID)
		}
	}
	for _, s := range sigs {
		collect(s.typ, s.props)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
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
	v.Type, v.Author, v.At, v.EditedAt = m.Type, &a, m.At, m.EditedAt
	v.Props = r.props(m.Type, m.Props)
	if m.Action != nil {
		av := &ActionView{Name: m.Action.Name, Status: m.Action.Status, At: m.Action.At}
		if m.Action.Promoted != nil {
			av.Promoted = r.ref(m.Action.Promoted.ID)
		}
		v.Action = av
	}
	if r.call != nil {
		v.Actions = r.call.offeredActions(m)
	}
	return v
}

func (r *renderer) signal(s signal) EphemeralView {
	return EphemeralView{
		ID: s.id, Type: s.typ, Author: authorView(s.author), At: s.at, ExpiresAt: s.expires,
		Props: r.props(s.typ, s.props),
	}
}
