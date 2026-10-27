package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

const (
	inboxPath = "/api/v1/_inbox"
	// realtimeOffTitle is the title of the inbox's 404 with realtime off.
	// The inbox names no entity, so it need not look like an entity miss.
	realtimeOffTitle = "Realtime is not enabled"
)

// inboxReader serves the inbox route. *realtime.Service satisfies it.
type inboxReader interface {
	Inbox(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error)
}

// serveInbox answers GET /api/v1/_inbox?limit=&cursor=: the sessions that
// wait on the caller now. The service gates and redacts every row before
// it counts, pages or truncates, so this only parses and reshapes.
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
	got, err := h.inbox.Inbox(r.Context(), req)
	if errors.Is(err, realtime.ErrRealtimeOff) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", realtimeOffTitle, "")
		return
	}
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, wireInbox(got, meta))
}

func wireInbox(v realtime.InboxView, meta *metamodel.Metamodel) v1.Inbox {
	out := v1.Inbox{
		Notice: v.Notice, Entries: make([]v1.InboxEntry, 0, len(v.Entries)), Count: v.Count,
		Truncated: v.Truncated, Degraded: v.Degraded, NextCursor: v.NextCursor,
	}
	for _, e := range v.Entries {
		out.Entries = append(out.Entries, wireInboxEntry(e, meta))
	}
	return out
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
	return v1.InboxEntry{
		Kind: e.Kind,
		Channel: v1.InboxChannel{
			Anchor: c.Anchor, EntityType: c.EntityType, AnchorTitle: c.AnchorTitle, Type: c.Type, Name: c.Name,
			Title: c.Title, State: string(c.State),
		},
		Session:     ws,
		Roles:       nonNil(e.Roles),
		Since:       channelTime(e.Since),
		Transitions: nonNil(e.Transitions),
		Links:       v1.InboxLinks{Entity: entityLink, Channel: channelLink},
	}
}

// nonNil renders a nil list as [] rather than null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
