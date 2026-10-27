package mcp

import (
	"context"
	"errors"
	"strconv"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// InboxReader backs list_inbox. The wiring site passes *realtime.Service,
// which gates and redacts every row before it counts or pages.
type InboxReader interface {
	Inbox(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error)
}

const (
	toolListInbox = "list_inbox"
	// inboxPollEvery is how often the channel poll may replay a list_inbox
	// target. An inbox replay reads every waiting channel of the caller, so
	// the 2 s channel poll would be too often when nothing changed.
	inboxPollEvery = 15 * time.Second
	// inboxWakeGap is the least time between two list_inbox replays that
	// an in-process channel wake or a store event starts. A wake for a
	// channel type without a waiting state starts none.
	inboxWakeGap = 2 * time.Second
)

func toolListInboxDef() *mcpgo.Tool {
	return newTool(toolListInbox,
		withDescription("List the sessions that wait on you now: each channel whose session waits for a role "+
			"you hold, oldest wait first. An entry names the channel, the session state, your waiting roles, "+
			"since (when the session entered its state) and transitions: the message types your post would "+
			"move it with. Read it with read_session or read_channel and post one of transitions. count is "+
			"the whole list, not this page; page with cursor (pass next_cursor back). Watchable: watch it "+
			"without arguments to wait for a session to start or stop waiting on you; one list_inbox "+
			"target per watch. An entry leaves when the session moves, the role moves to someone else, the "+
			"channel closes or you lose access. "+realtime.UntrustedNotice),
		withNumber("limit", description("Page size, 1 to "+strconv.Itoa(realtime.MaxInboxLimit)+
			" (default "+strconv.Itoa(realtime.DefaultInboxLimit)+")")),
		withString("cursor", description("next_cursor from the previous page")),
		withArray(inboxKindsArg, description("What to list: waiting, unread, or both (the default). "+
			"unread_channels lists the channels you marked or posted in that have messages you have not "+
			"read, newest activity first, at most "+strconv.Itoa(realtime.MaxInboxUnread)+", on the first "+
			"page only; unread_count counts them. With [\"waiting\"] a watch ignores chat; with "+
			"[\"unread\"] the waiting fields are absent and a cursor is refused.")),
	)
}

// inboxKindsArg is list_inbox's kinds argument.
const inboxKindsArg = "kinds"

// inboxKinds reads the kinds argument: absent is nil, both kinds.
func inboxKinds(args toolRequest) ([]string, error) {
	raw, ok := args.GetArguments()[inboxKindsArg]
	if !ok || raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("kinds: an array of waiting and unread")
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("kinds: an array of waiting and unread")
		}
		out = append(out, s)
	}
	if err := realtime.CheckInboxKinds(out); err != nil {
		return nil, errors.New("kinds: an array of waiting and unread")
	}
	return out, nil
}

func (h channelHandler) handleInbox(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	limit, err := wholeArg(args, "limit", 1)
	if err != nil {
		return errorResult(err.Error())
	}
	cursor, err := optionalString(args, "cursor")
	if err != nil {
		return errorResult(err.Error())
	}
	kinds, err := inboxKinds(args)
	if err != nil {
		return errorResult(err.Error())
	}
	req := realtime.InboxRequest{Kinds: kinds}
	if limit != nil {
		req.Limit = int(min(*limit, int64(realtime.MaxInboxLimit)+1))
	}
	if cursor != nil {
		req.Cursor = *cursor
	}
	got, err := h.inbox.Inbox(ctx, req)
	if err != nil {
		return h.failed(ctx, toolListInbox, "", err)
	}
	noteInbox(ctx, got)
	return channelJSON(got)
}

func (u unavailableChannels) Inbox(context.Context, realtime.InboxRequest) (realtime.InboxView, error) {
	return realtime.InboxView{}, u.err
}
