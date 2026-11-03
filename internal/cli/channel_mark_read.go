package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/output"
)

// ChannelMarkReadCmd moves the caller's read marker on a channel.
type ChannelMarkReadCmd struct {
	channelAddr
	Seq *int64 `help:"Mark read up to this seq; 0 or more. Without it, up to the last message."`
}

// Run marks the channel read.
func (c *ChannelMarkReadCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	if c.Seq != nil && *c.Seq < 0 {
		return errors.New("--seq: 0 or more")
	}
	addr := c.raw()
	got, err := svc.MarkRead(ctx, addr, c.Seq)
	if err != nil {
		return channelFailed(ctx, "mark-read", addr.Anchor, err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(got)
	}
	_, err = fmt.Fprintf(out.Out, "marked read up to %d (%d unread)\n", got.Seq, got.Unread)
	return err
}
