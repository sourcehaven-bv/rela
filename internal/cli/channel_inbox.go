package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// ChannelInboxCmd lists the sessions that wait on the caller, the
// channels the caller has not read, and the unread messages that mention
// the caller or reply to the caller.
type ChannelInboxCmd struct {
	Limit  int      `help:"Page size, 1 to 200." default:"50"`
	Cursor string   `help:"The cursor the previous page printed."`
	Kinds  []string `help:"What to list: waiting, unread, mention, reply; all by default. Comma separated."`
}

// Run lists the inbox.
func (c *ChannelInboxCmd) Run(ctx context.Context, cs *channelServices) error {
	if err := realtime.CheckInboxLimit(c.Limit); err != nil {
		return channelFailed(ctx, "inbox", "", err)
	}
	if err := realtime.CheckInboxKinds(c.Kinds); err != nil {
		return channelFailed(ctx, "inbox", "", err)
	}
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	got, err := svc.Inbox(ctx, realtime.InboxRequest{
		Limit: c.Limit, Cursor: strings.TrimSpace(c.Cursor), Kinds: c.Kinds,
	})
	if err != nil {
		return channelFailed(ctx, "inbox", "", err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(got)
	}
	return renderInbox(out.Out, got)
}

// renderInbox writes one line per entry, such as
//
//	RISK-1 breach-response detected, input_required since 2026-10-27T09:00:00Z, waiting for the lead (you)
//
// then a line for a partial or degraded answer and the next cursor, then
// an "Unread channels" section when the request asked for unread, then a
// "Mentions and replies" section when it asked for mention or reply.
func renderInbox(w io.Writer, v realtime.InboxView) error {
	var b strings.Builder
	if v.Degraded {
		b.WriteString("The inbox is not available while realtime is degraded.\n")
	} else if v.HasWaiting() {
		writeWaiting(&b, v)
	}
	if v.UnreadCount != nil && !v.Degraded {
		writeUnread(&b, v)
	}
	if v.MentionCount != nil && !v.Degraded {
		writeMentions(&b, v)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeWaiting(b *strings.Builder, v realtime.InboxView) {
	if len(v.Entries) == 0 {
		b.WriteString("No session waits on you.\n")
	}
	for _, e := range v.Entries {
		writeInboxLine(b, e)
	}
	if v.Truncated {
		b.WriteString("Only part of the inbox is listed: you hold a waiting role on too many entities.\n")
	}
	if v.NextCursor != "" {
		fmt.Fprintf(b, "%d of %d shown; next page: --cursor %s\n", len(v.Entries), v.Count, v.NextCursor)
	}
}

// writeUnread writes the unread section, one line per channel, such as
//
//	RISK-1 breach-response: 2 unread (read up to 40 of 42)
//
// An empty section says so, so it reads apart from a cursor page, where
// the unread half is not computed and nothing is written.
func writeUnread(b *strings.Builder, v realtime.InboxView) {
	if v.HasWaiting() {
		b.WriteString("\n")
	}
	if len(v.UnreadChannels) == 0 {
		b.WriteString("No unread channels.\n")
		return
	}
	b.WriteString("Unread channels:\n")
	for _, u := range v.UnreadChannels {
		writeChannelName(b, u.Channel)
		n := strconv.Itoa(u.Unread)
		if u.Unread >= realtime.UnreadCap {
			n += "+"
		}
		fmt.Fprintf(b, ": %s unread (read up to %d of %d)\n", n, u.Seq, u.LastSeq)
	}
	if *v.UnreadCount > len(v.UnreadChannels) {
		fmt.Fprintf(b, "%d of %d unread channels shown.\n", len(v.UnreadChannels), *v.UnreadCount)
	}
}

// writeMentions writes the mentions section, one line per message, such
// as
//
//	RISK-1 breach-response #42: bob mentioned you (2026-10-27T09:00:00Z)
//	RISK-1 breach-response #43: carol replied to #40 (2026-10-27T09:05:00Z)
//
// Like the unread section, an empty one says so.
func writeMentions(b *strings.Builder, v realtime.InboxView) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	if len(v.Mentions) == 0 {
		b.WriteString("No mentions or replies.\n")
		return
	}
	b.WriteString("Mentions and replies:\n")
	for _, m := range v.Mentions {
		writeChannelName(b, m.Channel)
		fmt.Fprintf(b, " #%d: %s ", m.Seq, printable(m.Author.User))
		if m.Kind == realtime.InboxKindMention {
			b.WriteString("mentioned you")
		} else {
			fmt.Fprintf(b, "replied to #%d", m.ReplyTo)
		}
		fmt.Fprintf(b, " (%s)\n", m.At.UTC().Format(time.RFC3339))
	}
	if *v.MentionCount > len(v.Mentions) {
		fmt.Fprintf(b, "%d of %d mentions and replies shown.\n", len(v.Mentions), *v.MentionCount)
	}
}

// writeChannelName writes "anchor type" or "anchor type/name".
func writeChannelName(b *strings.Builder, c realtime.InboxChannel) {
	b.WriteString(printable(c.Anchor) + " " + printable(c.Type))
	if c.Name != "" {
		b.WriteString("/" + printable(c.Name))
	}
}

func writeInboxLine(b *strings.Builder, e realtime.InboxEntry) {
	s := e.Session
	writeChannelName(b, e.Channel)
	state := s.State
	if s.Label != "" {
		state = s.Label
	}
	fmt.Fprintf(b, " %s, %s since %s", printable(state), printable(s.Status), e.Since.UTC().Format(time.RFC3339))
	roles := make([]string, 0, len(e.Roles))
	for _, w := range s.WaitingFor {
		if !slices.Contains(e.Roles, w.Role) {
			continue
		}
		label := w.Role
		if w.Label != "" {
			label = w.Label
		}
		roles = append(roles, printable(label))
	}
	if len(roles) > 0 {
		b.WriteString(", waiting for " + strings.Join(roles, " or ") + " (you)")
	}
	b.WriteString("\n")
}
