package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

const (
	inboxPath = "/api/v1/_inbox"
	// inboxSummaryPath answers the counts of the inbox's first page.
	inboxSummaryPath = "/api/v1/_inbox/summary"
	// realtimeOffTitle is the title of the inbox's 404 with realtime off.
	// The inbox names no entity, so it need not look like an entity miss.
	realtimeOffTitle = "Realtime is not enabled"
)

// inboxReader serves the inbox route. *realtime.Service satisfies it.
type inboxReader interface {
	Inbox(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error)
}

// serveInbox answers GET /api/v1/_inbox?limit=&cursor=&kinds=: the
// sessions that wait on the caller now, the channels with unread messages,
// and the unread messages that mention the caller or reply to the caller.
// kinds is a comma list of waiting, unread, mention and reply; all by
// default. The service gates and redacts every row before it counts, pages
// or truncates, so this only parses and reshapes.
//
// Realtime off, or channels not wired, is a 404. A nil *channelsHandler is
// accepted, for channels not wired.
//
//archplan:auth=required
func (h *channelsHandler) serveInbox(w http.ResponseWriter, r *http.Request, meta *metamodel.Metamodel) {
	if h == nil || h.inbox == nil || !h.on.Enabled() {
		writeV1Error(w, r, http.StatusNotFound, "not_found", realtimeOffTitle, "")
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	req := realtime.InboxRequest{Cursor: q.Get("cursor")}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			writeV1Error(w, r, http.StatusBadRequest, realtime.TypeInvalid, "Invalid limit",
				"limit must be a whole number")
			return
		}
		if err := realtime.CheckInboxLimit(n); err != nil {
			writeChannelError(w, r, err)
			return
		}
		req.Limit = n
	}
	if q.Has("kinds") {
		req.Kinds = strings.Split(q.Get("kinds"), ",")
		if err := realtime.CheckInboxKinds(req.Kinds); err != nil {
			writeChannelError(w, r, err)
			return
		}
	}
	got, ok := h.readInbox(w, r, req)
	if !ok {
		return
	}
	writeV1JSON(w, http.StatusOK, wireInbox(got, meta))
}

// serveInboxSummary answers GET /api/v1/_inbox/summary: the counts of the
// default first page of the caller's inbox, for a badge. It is a
// projection of the same [realtime.Service.Inbox] call the inbox route
// makes with no query, so it reveals nothing that route does not, and
// answers the same refusals. Its query string is ignored; the live path
// refuses any query on it.
//
// A nil *channelsHandler is accepted, for channels not wired.
//
//archplan:auth=required
func (h *channelsHandler) serveInboxSummary(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.inbox == nil || !h.on.Enabled() {
		writeV1Error(w, r, http.StatusNotFound, "not_found", realtimeOffTitle, "")
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	got, ok := h.readInbox(w, r, realtime.InboxRequest{})
	if !ok {
		return
	}
	writeV1JSON(w, http.StatusOK, wireInboxSummary(got.Summary()))
}

// readInbox reads the inbox for req, through the round's memo during a
// live replay, and reports the wake set of a 200 to the replay's probe.
// It writes the refusal and reports false on an error.
func (h *channelsHandler) readInbox(
	w http.ResponseWriter, r *http.Request, req realtime.InboxRequest,
) (realtime.InboxView, bool) {
	got, err := h.inboxFor(r.Context(), req)
	if errors.Is(err, realtime.ErrRealtimeOff) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", realtimeOffTitle, "")
		return realtime.InboxView{}, false
	}
	if err != nil {
		writeChannelError(w, r, err)
		return realtime.InboxView{}, false
	}
	recordInbox(r.Context(), got, req)
	return got, true
}

func wireInboxSummary(s realtime.InboxSummary) v1.InboxSummary {
	return v1.InboxSummary{
		Waiting: s.Waiting, WaitingTruncated: s.WaitingTruncated, UnreadChannels: s.UnreadChannels,
		Mentions: s.Mentions, Degraded: s.Degraded,
	}
}

func wireInbox(v realtime.InboxView, meta *metamodel.Metamodel) v1.Inbox {
	out := v1.Inbox{
		Notice: v.Notice, Degraded: v.Degraded, UnreadCount: v.UnreadCount, MentionCount: v.MentionCount,
	}
	if v.HasWaiting() {
		count, truncated := v.Count, v.Truncated
		out.Entries, out.Count, out.Truncated = make([]v1.InboxEntry, 0, len(v.Entries)), &count, &truncated
		out.NextCursor = v.NextCursor
		for _, e := range v.Entries {
			out.Entries = append(out.Entries, wireInboxEntry(e, meta))
		}
	}
	if v.UnreadChannels != nil {
		out.UnreadChannels = make([]v1.InboxUnread, 0, len(v.UnreadChannels))
		for _, u := range v.UnreadChannels {
			ch, links := wireInboxChannel(u.Channel, meta)
			out.UnreadChannels = append(out.UnreadChannels, v1.InboxUnread{
				Channel: ch, Seq: u.Seq, LastSeq: u.LastSeq, Unread: u.Unread, Links: links,
			})
		}
	}
	if v.Mentions != nil {
		out.Mentions = make([]v1.InboxMention, 0, len(v.Mentions))
		for _, m := range v.Mentions {
			out.Mentions = append(out.Mentions, wireInboxMention(m, meta))
		}
	}
	return out
}

// wireInboxMention renders a mention or reply entry with the path of its
// message.
func wireInboxMention(m realtime.InboxMention, meta *metamodel.Metamodel) v1.InboxMention {
	ch, links := wireInboxChannel(m.Channel, meta)
	links.Message = links.Channel + "/" + messagesSegment + "/" + strconv.FormatInt(m.Seq, 10)
	return v1.InboxMention{
		Kind: m.Kind, Channel: ch, Seq: m.Seq, At: channelTime(m.At), MessageType: m.MessageType,
		Author: *wireChannelAuthor(&m.Author), ReplyTo: m.ReplyTo, Links: links,
	}
}

// wireInboxChannel renders an inbox channel and the API paths of its
// anchor and channel.
func wireInboxChannel(c realtime.InboxChannel, meta *metamodel.Metamodel) (v1.InboxChannel, v1.InboxLinks) {
	plural := c.EntityType
	if meta != nil {
		if def, ok := meta.Entities[c.EntityType]; ok {
			plural = def.GetPlural(c.EntityType)
		}
	}
	entityLink := "/api/v1/" + url.PathEscape(plural) + "/" + url.PathEscape(c.Anchor)
	channelLink := entityLink + "/" + channelsSegment + "/" + url.PathEscape(c.Type)
	if c.Name != "" {
		channelLink += "/" + url.PathEscape(c.Name)
	}
	return v1.InboxChannel{
		Anchor: c.Anchor, EntityType: c.EntityType, AnchorTitle: c.AnchorTitle, Type: c.Type, Name: c.Name,
		Title: c.Title, State: string(c.State),
	}, v1.InboxLinks{Entity: entityLink, Channel: channelLink}
}

func wireInboxEntry(e realtime.InboxEntry, meta *metamodel.Metamodel) v1.InboxEntry {
	c := e.Channel
	s := e.Session
	ws := v1.InboxSession{
		State: s.State, Label: s.Label, Status: s.Status, Generation: s.Generation,
		EnteredAt: channelTime(s.EnteredAt),
	}
	for _, w := range s.WaitingFor {
		ws.WaitingFor = append(ws.WaitingFor,
			v1.ChannelSessionWaiting{Role: w.Role, Label: w.Label, Holders: w.Holders})
	}
	ch, links := wireInboxChannel(c, meta)
	return v1.InboxEntry{
		Kind:        e.Kind,
		Channel:     ch,
		Session:     ws,
		Roles:       nonNil(e.Roles),
		Since:       channelTime(e.Since),
		Transitions: nonNil(e.Transitions),
		Links:       links,
	}
}

// nonNil renders a nil list as [] rather than null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
