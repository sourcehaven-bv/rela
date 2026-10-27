package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// ChannelInboxCmd lists the sessions that wait on the caller.
type ChannelInboxCmd struct {
	Limit  int    `help:"Page size, 1 to 200." default:"50"`
	Cursor string `help:"The cursor the previous page printed."`
}

// Run lists the inbox.
func (c *ChannelInboxCmd) Run(ctx context.Context, cs *channelServices) error {
	if err := realtime.CheckInboxLimit(c.Limit); err != nil {
		return channelFailed(ctx, "inbox", "", err)
	}
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	got, err := svc.Inbox(ctx, realtime.InboxRequest{Limit: c.Limit, Cursor: strings.TrimSpace(c.Cursor)})
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
// then a line for a partial or degraded answer and the next cursor.
func renderInbox(w io.Writer, v realtime.InboxView) error {
	var b strings.Builder
	if v.Degraded {
		b.WriteString("The inbox is not available while realtime is degraded.\n")
	} else if len(v.Entries) == 0 {
		b.WriteString("No session waits on you.\n")
	}
	for _, e := range v.Entries {
		writeInboxLine(&b, e)
	}
	if v.Truncated {
		b.WriteString("Only part of the inbox is listed: you hold a waiting role on too many entities.\n")
	}
	if v.NextCursor != "" {
		fmt.Fprintf(&b, "%d of %d shown; next page: --cursor %s\n", len(v.Entries), v.Count, v.NextCursor)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeInboxLine(b *strings.Builder, e realtime.InboxEntry) {
	c, s := e.Channel, e.Session
	b.WriteString(printable(c.Anchor) + " " + printable(c.Type))
	if c.Name != "" {
		b.WriteString("/" + printable(c.Name))
	}
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
