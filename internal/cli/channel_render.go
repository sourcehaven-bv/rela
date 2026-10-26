package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// The table renderers below print channel content written by other people
// and agents. Every string goes through [printable], and each line of a
// property value is prefixed with propIndent, so content cannot forge a
// message header, move the cursor or reorder a line.

// propIndent prefixes every line of untrusted message content.
const propIndent = "  | "

// renderChannelList prints the channels of an anchor as a table.
func renderChannelList(w io.Writer, got realtime.AnchorChannels) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TYPE\tNAME\tSTATE\tTITLE\tLAST_SEQ")
	rows := 0
	for _, t := range got.Types {
		for _, c := range t.Channels {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n",
				cell(t.Type), cell(c.Name), cell(string(c.State)), cell(c.Title), c.LastSeq)
			rows++
		}
		if len(t.Channels) == 0 {
			_, _ = fmt.Fprintf(tw, "%s\t-\t-\t-\t-\n", cell(t.Type))
			rows++
		}
	}
	if rows == 0 {
		_, err := fmt.Fprintln(w, "No channel types attach to this entity.")
		return err
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	var creatable []string
	for _, t := range got.Types {
		if t.Create {
			creatable = append(creatable, printable(t.Type))
		}
	}
	if len(creatable) > 0 {
		_, err := fmt.Fprintf(w, "You may create: %s\n", strings.Join(creatable, ", "))
		return err
	}
	return nil
}

// cell is one table cell: printable, tabs included, and "-" when empty.
func cell(s string) string {
	if s == "" {
		return "-"
	}
	return printable(s)
}

// renderRead prints a channel page: a header, what the caller may post,
// the message types when asked for, and the messages.
func renderRead(w io.Writer, v realtime.ReadView) error {
	b := new(strings.Builder)
	writeChannelHeader(b, v.Channel)
	if v.Actions != nil {
		post := "nothing"
		if len(v.Actions.Post) > 0 {
			post = printable(strings.Join(v.Actions.Post, ", "))
		}
		fmt.Fprintf(b, "You may post: %s\n", post)
	}
	for _, mt := range v.MessageTypes {
		writeMessageType(b, mt)
	}
	b.WriteString("\n")
	if len(v.Messages) == 0 {
		b.WriteString("No messages.\n")
	}
	for _, m := range v.Messages {
		writeMessageLines(b, m)
	}
	writeCursors(b, v)
	_, err := io.WriteString(w, b.String())
	return err
}

// writeChannelHeader prints the channel's address, state and title.
func writeChannelHeader(b *strings.Builder, c realtime.ChannelView) {
	fmt.Fprintf(b, "%s %s", printable(c.Anchor), printable(c.Type))
	if c.Name != "" {
		fmt.Fprintf(b, " %s", printable(c.Name))
	}
	fmt.Fprintf(b, " (%s, last #%d, rev %d)\n", printable(string(c.State)), c.LastSeq, c.Rev)
	if c.Title != "" {
		fmt.Fprintf(b, "Title: %s\n", printable(c.Title))
	}
	if c.Session != nil {
		fmt.Fprintf(b, "%s\n", sessionText(c.Session))
	}
}

// sessionText is the one-line form of a session: its state, label and
// generation, whether the state is final or stranded, and the timer that
// comes due next, when the session has a due time.
func sessionText(s *realtime.SessionView) string {
	text := "session: " + printable(s.State)
	if s.Label != "" && s.Label != s.State {
		text += " (" + printable(s.Label) + ")"
	}
	text += fmt.Sprintf(", generation %d", s.Generation)
	if s.Final {
		text += ", final"
	}
	if s.Stranded {
		text += ", stranded"
	}
	return text + nextTimerText(s) + remindersText(s)
}

// remindersText is the reminders part of [sessionText]: per reminder of
// the current state ", reminder <name>: <count>[ of <max>]", and ", next at
// <time>" while one is due.
func remindersText(s *realtime.SessionView) string {
	var b strings.Builder
	for _, r := range s.Reminders {
		fmt.Fprintf(&b, ", reminder %s: %d", printable(r.Name), r.Count)
		if r.Max > 0 {
			fmt.Fprintf(&b, " of %d", r.Max)
		}
		if r.DueAt != nil {
			fmt.Fprintf(&b, ", next at %s", r.DueAt.UTC().Format(time.RFC3339))
		}
	}
	return b.String()
}

// nextTimerText is the ", next: ..." part of [sessionText]: the timer that
// comes due at the session's due time, or "" when there is none.
func nextTimerText(s *realtime.SessionView) string {
	if s.DueAt == nil {
		return ""
	}
	for _, t := range s.Timers {
		if t.DueAt != nil && t.DueAt.Equal(*s.DueAt) {
			return fmt.Sprintf(", next: %s to %s at %s", printable(t.Name), printable(t.To),
				s.DueAt.UTC().Format(time.RFC3339))
		}
	}
	return ""
}

// writeMessageType prints a message type and its properties.
func writeMessageType(b *strings.Builder, mt realtime.MessageTypeView) {
	fmt.Fprintf(b, "Type %s (%s", printable(mt.Name), printable(string(mt.Persist)))
	if !mt.CanPost {
		b.WriteString(", you may not post")
	}
	b.WriteString(")\n")
	names := make([]string, 0, len(mt.Props.Properties))
	for name := range mt.Props.Properties {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		p := mt.Props.Properties[name]
		typ := p.Type
		if p.Format != "" {
			typ = p.Format
		}
		if len(p.Enum) > 0 {
			typ = "one of " + strings.Join(p.Enum, ", ")
		}
		req := ""
		if slices.Contains(mt.Props.Required, name) {
			req = ", required"
		}
		fmt.Fprintf(b, "  %s: %s%s\n", printable(name), printable(typ), req)
	}
}

// writeMessageLines prints one message: a header line, then its props, each
// line prefixed with propIndent.
func writeMessageLines(b *strings.Builder, m realtime.MessageView) {
	fmt.Fprintf(b, "#%d", m.Seq)
	if m.ReplyTo > 0 {
		fmt.Fprintf(b, " (reply to #%d)", m.ReplyTo)
	}
	if m.Removed != nil {
		writeRemoved(b, m.Removed)
		return
	}
	fmt.Fprintf(b, "  %s  %s  %s", m.At.UTC().Format(time.RFC3339), authorText(m.Author), printable(m.Type))
	if !m.EditedAt.IsZero() {
		b.WriteString("  (edited)")
	}
	if m.IsReminder() {
		b.WriteString("  (reminder)")
	}
	switch {
	case m.ReplyCount == 1:
		b.WriteString("  1 reply")
	case m.ReplyCount > 1:
		fmt.Fprintf(b, "  %d replies", m.ReplyCount)
	}
	b.WriteString("\n")
	if m.IsSessionNotice() {
		fmt.Fprintf(b, "  moved from %s to %s",
			printable(propText(m.Props["from"])), printable(propText(m.Props["to"])))
		if timer, ok := m.Props["timer"]; ok {
			fmt.Fprintf(b, " (timer %s)", printable(propText(timer)))
		}
		b.WriteString("\n")
		return
	}
	names := make([]string, 0, len(m.Props))
	for name := range m.Props {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		writeProp(b, name, m.Props[name])
	}
	if m.Action != nil {
		fmt.Fprintf(b, "  action %s: %s", printable(m.Action.Name), printable(string(m.Action.Status)))
		if m.Action.Promoted != nil {
			fmt.Fprintf(b, " -> %s", refText(m.Action.Promoted))
		}
		b.WriteString("\n")
	}
}

// writeRemoved prints the placeholder of a removed message. Who removed it
// and why are set only for a moderator.
func writeRemoved(b *strings.Builder, r *realtime.RemovedView) {
	if r.Kind == "author" {
		b.WriteString("  [removed by author]")
	} else {
		b.WriteString("  [removed]")
	}
	if r.By != nil {
		fmt.Fprintf(b, " by %s", authorText(r.By))
	}
	b.WriteString("\n")
	if r.Reason != "" {
		writeProp(b, "reason", r.Reason)
	}
}

// authorText is "user (tool)", or "user" without a tool.
func authorText(a *realtime.AuthorView) string {
	if a == nil {
		return "-"
	}
	if a.Tool == "" {
		return printable(a.User)
	}
	return printable(a.User) + " (" + printable(a.Tool) + ")"
}

// writeProp prints one property. Each line of its value is made printable
// and prefixed with propIndent, so a value cannot start a line of its own.
func writeProp(b *strings.Builder, name string, value any) {
	lines := strings.Split(propText(value), "\n")
	fmt.Fprintf(b, "%s%s: %s\n", propIndent, printable(name), printable(lines[0]))
	for _, line := range lines[1:] {
		fmt.Fprintf(b, "%s%s\n", propIndent, printable(line))
	}
}

// propText is a property value as text. A ref is `ID "title"`, or
// "(restricted)" for one the reader cannot read.
func propText(value any) string {
	switch v := value.(type) {
	case string:
		return strings.ReplaceAll(v, "\r\n", "\n")
	case *realtime.RefValue:
		return refText(v)
	case realtime.RefValue:
		return refText(&v)
	case int64:
		return strconv.FormatInt(v, 10)
	case nil:
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil { // coverage-ignore: defensive: rendered props are JSON values
		return fmt.Sprint(value)
	}
	return string(data)
}

// refText renders a ref.
func refText(r *realtime.RefValue) string {
	if r == nil || r.Restricted {
		return "(restricted)"
	}
	if r.Title == "" || r.Title == r.ID {
		return printable(r.ID)
	}
	return printable(r.ID) + " " + strconv.Quote(printable(r.Title))
}

// writeCursors prints the flags that continue from this page.
func writeCursors(b *strings.Builder, v realtime.ReadView) {
	var next []string
	if v.HasMore && v.NextBefore > 0 {
		next = append(next, fmt.Sprintf("older: --before %d", v.NextBefore))
	}
	if v.HasMore && v.NextAfter > 0 {
		next = append(next, fmt.Sprintf("more: --after %d", v.NextAfter))
	}
	if v.NextAfterRev != nil {
		next = append(next, fmt.Sprintf("follow: --after-rev %d", *v.NextAfterRev))
	}
	if len(next) > 0 {
		fmt.Fprintf(b, "\n%s\n", strings.Join(next, "; "))
	}
}

// renderCreated prints a created channel.
func renderCreated(w io.Writer, c realtime.ChannelView) error {
	b := new(strings.Builder)
	fmt.Fprintf(b, "created %s %s %s", printable(c.Anchor), printable(c.Type), printable(c.Name))
	if c.Title != "" {
		fmt.Fprintf(b, " %q", printable(c.Title))
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}
