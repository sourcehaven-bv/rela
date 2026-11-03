package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// ChannelActionCmd runs a message action, such as a promote, or releases
// one that is stuck.
type ChannelActionCmd struct {
	channelAddr
	Seq     int64  `arg:"" help:"Message seq."`
	Action  string `arg:"" optional:"" help:"Action name; rela channel read lists them under each message as may:."`
	Release bool   `help:"Release the message's stuck action so it can run again. Check first whether a failed promote made its entity."`
}

// actionJSON is the -o json form of a run or released action.
type actionJSON struct {
	Notice   string               `json:"notice"`
	Message  realtime.MessageView `json:"message"`
	Warnings []entity.Warning     `json:"warnings,omitempty"`
}

// Run runs or releases the action.
func (c *ChannelActionCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	if c.Seq < 1 {
		return errors.New("seq: at least 1")
	}
	switch {
	case c.Release && c.Action != "":
		return errors.New("give an action name or --release, not both")
	case !c.Release && c.Action == "":
		return errors.New("give an action name, or --release")
	}
	addr := c.raw()
	var got realtime.ActionResult
	op := "action"
	if c.Release {
		op = "release"
		got.Message, err = svc.ReleaseAction(ctx, addr, c.Seq)
	} else {
		got, err = svc.RunAction(ctx, addr, c.Seq, c.Action)
	}
	if err != nil {
		return channelFailed(ctx, op, addr.Anchor, err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(actionJSON{
			Notice: realtime.UntrustedNotice, Message: got.Message, Warnings: got.Warnings,
		})
	}
	line := fmt.Sprintf("#%d released", got.Message.Seq)
	if got.Message.Action != nil {
		line = fmt.Sprintf("#%d %s", got.Message.Seq, actionText(got.Message.Action))
	}
	if _, err := fmt.Fprintln(out.Out, line); err != nil {
		return err
	}
	for _, w := range got.Warnings {
		if _, err := fmt.Fprintln(out.Out, warningText(w)); err != nil {
			return err
		}
	}
	return nil
}

// warningText is one soft validation finding of a promote's write.
func warningText(w entity.Warning) string {
	text := "warning: " + printable(w.Code)
	if w.Path != "" {
		text += " " + printable(w.Path)
	}
	if w.Detail != "" {
		text += ": " + printable(w.Detail)
	}
	return text
}

// ChannelSessionCmd shows a channel's session.
type ChannelSessionCmd struct {
	channelAddr
}

// Run shows the session.
func (c *ChannelSessionCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	addr := c.raw()
	got, err := svc.Session(ctx, addr)
	if err != nil {
		return channelFailed(ctx, "session", addr.Anchor, err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(got)
	}
	_, err = fmt.Fprint(out.Out, sessionReadText(got))
	return err
}

// sessionReadText is the text form of a session read: the channel, the
// session line and the message types the reader may post to move it.
func sessionReadText(got realtime.SessionRead) string {
	var b strings.Builder
	ch := got.Channel
	fmt.Fprintf(&b, "%s %s", printable(ch.Anchor), printable(ch.Type))
	if ch.Name != "" {
		fmt.Fprintf(&b, " %s", printable(ch.Name))
	}
	fmt.Fprintf(&b, " (%s)\n", printable(string(ch.State)))
	if got.Session != nil {
		fmt.Fprintf(&b, "%s\n", sessionText(got.Session))
	}
	if len(got.Transitions) == 0 {
		b.WriteString("may post: nothing that moves the session\n")
		return b.String()
	}
	names := make([]string, len(got.Transitions))
	for i, t := range got.Transitions {
		names[i] = printable(t)
	}
	fmt.Fprintf(&b, "may post: %s\n", strings.Join(names, ", "))
	return b.String()
}
