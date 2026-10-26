package dataentry

import (
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// Conversions from the channel service's views to the wire types. The
// service has already decided what the reader may see; these only reshape.

// channelTime renders t as RFC 3339 in UTC, or "" for the zero time.
func channelTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func wireChannelAuthor(a *realtime.AuthorView) *v1.ChannelAuthor {
	if a == nil {
		return nil
	}
	return &v1.ChannelAuthor{User: a.User, Tool: a.Tool, PrincipalType: a.PrincipalType, System: a.System}
}

func wireChannel(c realtime.ChannelView) v1.Channel {
	return v1.Channel{
		Anchor: c.Anchor, Type: c.Type, Name: c.Name, State: string(c.State), Title: c.Title,
		CreatedBy: wireChannelAuthor(c.CreatedBy), CreatedAt: channelTime(c.CreatedAt),
		ClosedAt: channelTime(c.ClosedAt), ArchivedAt: channelTime(c.ArchivedAt),
		LastSeq: c.LastSeq, Rev: c.Rev, Session: wireChannelSession(c.Session),
	}
}

func wireChannelSession(s *realtime.SessionView) *v1.ChannelSession {
	if s == nil {
		return nil
	}
	out := &v1.ChannelSession{
		State: s.State, Label: s.Label, Generation: s.Generation, EnteredAt: channelTime(s.EnteredAt),
		Final: s.Final, Stranded: s.Stranded,
	}
	for _, t := range s.Transitions {
		out.Transitions = append(out.Transitions, v1.ChannelSessionTransition{
			MessageType: t.MessageType, To: t.To, By: t.By, Guarded: t.Guarded, Outputs: t.Outputs,
		})
	}
	if s.DueAt != nil {
		out.DueAt = channelTime(*s.DueAt)
	}
	for _, t := range s.Timers {
		w := v1.ChannelSessionTimer{Name: t.Name, Label: t.Label, To: t.To, Since: t.Since, Outputs: t.Outputs}
		if t.DueAt != nil {
			w.DueAt = channelTime(*t.DueAt)
		}
		out.Timers = append(out.Timers, w)
	}
	for _, r := range s.Reminders {
		w := v1.ChannelSessionReminder{Name: r.Name, Label: r.Label, Every: r.Every, Count: r.Count, Max: r.Max}
		if r.DueAt != nil {
			w.DueAt = channelTime(*r.DueAt)
		}
		out.Reminders = append(out.Reminders, w)
	}
	return out
}

func wireChannelList(l realtime.AnchorChannels) v1.ChannelList {
	out := v1.ChannelList{Anchor: l.Anchor, Types: make([]v1.ChannelTypeList, 0, len(l.Types))}
	for _, tc := range l.Types {
		t := v1.ChannelTypeList{
			Type: tc.Type, Label: tc.Label, Cardinality: string(tc.Cardinality), Create: tc.Create,
			Channels: make([]v1.Channel, 0, len(tc.Channels)),
		}
		for _, c := range tc.Channels {
			t.Channels = append(t.Channels, wireChannel(c))
		}
		out.Types = append(out.Types, t)
	}
	return out
}

func wireRef(r *realtime.RefValue) *v1.RefProp {
	if r == nil {
		return nil
	}
	return &v1.RefProp{ID: r.ID, Type: r.Type, Title: r.Title, Restricted: r.Restricted}
}

// wireProps copies props, rendering each ref as a [v1.RefProp].
func wireProps(props map[string]any) map[string]any {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]any, len(props))
	for k, v := range props {
		if r, ok := v.(*realtime.RefValue); ok {
			out[k] = wireRef(r)
			continue
		}
		out[k] = v
	}
	return out
}

func wireMessage(m realtime.MessageView) v1.Message {
	out := v1.Message{
		Seq: m.Seq, Rev: m.Rev, Type: m.Type, Author: wireChannelAuthor(m.Author), At: channelTime(m.At),
		Props: wireProps(m.Props), ReplyTo: m.ReplyTo, ReplyCount: m.ReplyCount, EditedAt: channelTime(m.EditedAt),
		TriggeredBy: m.TriggeredBy, Actions: m.Actions,
		EditUntil: channelTime(m.EditUntil), ReleaseAfter: channelTime(m.ReleaseAfter),
	}
	if m.Removed != nil {
		out.Removed = &v1.MessageRemoval{
			At: channelTime(m.Removed.At), Kind: string(m.Removed.Kind),
			By: wireChannelAuthor(m.Removed.By), Reason: m.Removed.Reason,
		}
	}
	if m.Action != nil {
		out.Action = &v1.MessageAction{
			Name: m.Action.Name, Status: string(m.Action.Status), At: channelTime(m.Action.At),
			Promoted: wireRef(m.Action.Promoted),
		}
	}
	out.Outputs = wireOutputs(m.Outputs)
	return out
}

// wireOutputs renders a notice's outputs mark.
func wireOutputs(o *realtime.OutputsView) *v1.ChannelMessageOutputs {
	if o == nil {
		return nil
	}
	out := &v1.ChannelMessageOutputs{Status: o.Status, Writer: *wireChannelAuthor(&o.Writer)}
	for _, r := range o.Results {
		out.Results = append(out.Results, v1.ChannelMessageOutputResult{
			Kind: r.Kind, Status: r.Status, Code: r.Code, Props: r.Props, Face: r.Face, Entity: wireRef(r.Entity),
			Warnings: r.Warnings,
		})
	}
	return out
}

func wireActionResult(r realtime.ActionResult) v1.MessageActionResponse {
	return v1.MessageActionResponse{Message: wireMessage(r.Message), Warnings: r.Warnings}
}

func wireHead(h realtime.HeadView) v1.ChannelHead {
	out := v1.ChannelHead{
		Channel:   wireChannel(h.Channel),
		Ephemeral: make([]v1.EphemeralMessage, 0, len(h.Ephemeral)),
		TypingNow: make([]v1.TypingEntry, 0, len(h.TypingNow)),
		Actions: v1.ChannelActions{
			Post: h.Actions.Post, EditOwn: h.Actions.EditOwn, DeleteOwn: h.Actions.DeleteOwn,
			Remove: h.Actions.Remove, Lifecycle: h.Actions.Lifecycle, Typing: h.Actions.Typing,
			Transitions: h.Actions.Transitions,
		},
	}
	if out.Actions.Transitions == nil {
		out.Actions.Transitions = []string{}
	}
	if out.Actions.Post == nil {
		out.Actions.Post = []string{}
	}
	if out.Actions.Lifecycle == nil {
		out.Actions.Lifecycle = []string{}
	}
	if h.Last != nil {
		m := wireMessage(*h.Last)
		out.Last = &m
	}
	for _, e := range h.Ephemeral {
		out.Ephemeral = append(out.Ephemeral, v1.EphemeralMessage{
			ID: e.ID, Type: e.Type,
			Author: v1.ChannelAuthor{User: e.Author.User, Tool: e.Author.Tool, PrincipalType: e.Author.PrincipalType},
			At:     channelTime(e.At), ExpiresAt: channelTime(e.ExpiresAt), Props: wireProps(e.Props),
		})
	}
	for _, e := range h.TypingNow {
		out.TypingNow = append(out.TypingNow, v1.TypingEntry{
			Author: v1.ChannelAuthor{User: e.Author.User, Tool: e.Author.Tool, PrincipalType: e.Author.PrincipalType},
			Since:  channelTime(e.Since),
		})
	}
	return out
}

// wirePage renders p with the cursors the service named (see
// [realtime.Cursors]). has_more is set when the page is full.
func wirePage(p realtime.ReadView) v1.MessagePage {
	out := v1.MessagePage{
		Channel: wireChannel(p.Channel), Messages: make([]v1.Message, 0, len(p.Messages)), HasMore: p.HasMore,
		NextAfter: p.NextAfter, NextBefore: p.NextBefore, NextAfterRev: p.NextAfterRev,
	}
	for _, m := range p.Messages {
		out.Messages = append(out.Messages, wireMessage(m))
	}
	return out
}
