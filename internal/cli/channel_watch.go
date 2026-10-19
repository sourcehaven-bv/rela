package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// Watch bounds. The poll is how another process's change reaches this one;
// a change made in this process wakes the watch at once.
const (
	watchMinPoll    = time.Second
	watchMaxPoll    = time.Minute
	watchMaxTimeout = time.Hour
	// watchTimeoutExit is the exit code of a watch that saw no change.
	watchTimeoutExit = 2
	// watchInterruptExit is the exit code of a watch interrupted before a
	// change, the code a shell gives a process ended by SIGINT.
	watchInterruptExit = 130
)

// ChannelWatchCmd waits for changes in a channel.
//
// Each tick is a fresh read after the last rev, with its own access check,
// so a watch on a channel the caller cannot read fails at once and never
// learns of activity there.
type ChannelWatchCmd struct {
	channelAddr
	AfterRev *int64        `name:"after-rev" help:"Report changes after this rev (default: the channel's rev at start)."`
	Timeout  time.Duration `help:"Give up after this long, at most 1h; exit code 2." default:"30s"`
	Follow   bool          `help:"Keep watching and print every change until interrupted; ignores --timeout."`
	Poll     time.Duration `help:"How often to look for changes made by other processes, 1s to 60s." default:"2s"`
	Limit    int           `help:"Messages per change, 1 to 200." default:"20"`
}

// watchTimedOut is the JSON record of a watch that saw no change.
type watchTimedOut struct {
	Notice   string `json:"notice"`
	TimedOut bool   `json:"timed_out"`
	AfterRev int64  `json:"after_rev"`
}

// Run watches the channel. It exits 0 after a change, 2 on timeout, 130
// when interrupted and 1 on an error. With --follow it prints every change
// until interrupted, and then exits 0.
func (c *ChannelWatchCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	if err := c.check(); err != nil {
		return err
	}
	ticker := time.NewTicker(c.Poll)
	defer ticker.Stop()
	return c.watch(ctx, svc, ticker.C)
}

// check validates the bounds.
func (c *ChannelWatchCmd) check() error {
	if c.Poll < watchMinPoll || c.Poll > watchMaxPoll {
		return fmt.Errorf("--poll: %s to %s", watchMinPoll, watchMaxPoll)
	}
	if !c.Follow && (c.Timeout <= 0 || c.Timeout > watchMaxTimeout) {
		return fmt.Errorf("--timeout: more than 0, at most %s", watchMaxTimeout)
	}
	if c.Limit < 1 || c.Limit > channelMaxLimit {
		return fmt.Errorf("--limit: 1 to %d", channelMaxLimit)
	}
	if c.AfterRev != nil && *c.AfterRev < 0 {
		return errors.New("--after-rev: a rev of at least 0")
	}
	return nil
}

// watchService is what the watch loop uses: reads and the wake-up, no
// writes.
type watchService interface {
	channelReader
	channelWaker
}

// watch runs the loop; tick drives the poll, so a test can step it.
func (c *ChannelWatchCmd) watch(ctx context.Context, svc watchService, tick <-chan time.Time) error {
	addr := c.raw()
	sub := svc.Subscribe()
	defer sub.Close()

	rev, err := c.startRev(ctx, svc, addr)
	if err != nil {
		return err
	}
	var deadline <-chan time.Time
	if !c.Follow {
		timer := time.NewTimer(c.Timeout)
		defer timer.Stop()
		deadline = timer.C
	}
	for {
		view, err := svc.Read(ctx, addr, realtime.ReadRequest{AfterRev: &rev, Limit: c.Limit, LogOnly: true})
		if err != nil {
			return c.failed(ctx, addr, err)
		}
		if next := nextRev(view, rev); next != rev || len(view.Messages) > 0 {
			if err := writeChange(view); err != nil {
				return err
			}
			if !c.Follow {
				return nil
			}
			rev = next
			continue
		}
		select {
		case <-ctx.Done():
			return c.interrupted()
		case <-deadline:
			return writeTimedOut(rev)
		case <-sub.Wake():
		case <-tick:
		}
	}
}

// startRev is --after-rev, or the channel's rev now.
func (c *ChannelWatchCmd) startRev(ctx context.Context, svc channelReader, addr realtime.RawAddress) (int64, error) {
	if c.AfterRev != nil {
		return *c.AfterRev, nil
	}
	view, err := svc.Read(ctx, addr, realtime.ReadRequest{Limit: 1, LogOnly: true})
	if err != nil {
		return 0, c.failed(ctx, addr, err)
	}
	return nextRev(view, 0), nil
}

// failed answers a failed read. An interrupt that lands during the read is
// not a channel failure, so it is not logged as one.
func (c *ChannelWatchCmd) failed(ctx context.Context, addr realtime.RawAddress, err error) error {
	if ctx.Err() != nil {
		return c.interrupted()
	}
	return channelFailed(ctx, "watch", addr.Anchor, err)
}

// interrupted is the outcome of an interrupt: the normal end of --follow,
// else exit code 130 with no message.
func (c *ChannelWatchCmd) interrupted() error {
	if c.Follow {
		return nil
	}
	return relaerrors.NewExitError(watchInterruptExit)
}

// nextRev is the rev to follow from after view: its next_after_rev, else
// the channel's rev, never below rev. A lifecycle change moves the rev
// without a message, and counts as a change.
func nextRev(view realtime.ReadView, rev int64) int64 {
	next := view.Channel.Rev
	if view.NextAfterRev != nil {
		next = *view.NextAfterRev
	}
	return max(next, rev)
}

// writeChange prints one change: the after_rev page as a table, or as one
// JSON Lines record.
func writeChange(view realtime.ReadView) error {
	if out.Format == output.FormatJSON {
		data, err := json.Marshal(readJSON{Notice: realtime.UntrustedNotice, ReadView: view})
		if err != nil {
			return err
		}
		return writeJSONLine(data)
	}
	return renderRead(out.Out, view)
}

// writeTimedOut reports a watch that saw no change and ends it with exit
// code 2.
func writeTimedOut(rev int64) error {
	var err error
	if out.Format == output.FormatJSON {
		err = json.NewEncoder(out.Out).Encode(watchTimedOut{
			Notice: realtime.UntrustedNotice, TimedOut: true, AfterRev: rev,
		})
	} else {
		_, err = fmt.Fprintln(out.Out, "no change")
	}
	if err != nil {
		return err
	}
	return relaerrors.NewExitError(watchTimeoutExit)
}
