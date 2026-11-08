package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/peersync"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// PeerPullCmd pulls once from a peer in peers.yaml (TKT-I8L1EQU).
//
// It writes as `system:peer:<name>`, as the scheduled pull does, and takes
// the same run lock, so it never runs alongside it. The credential comes from
// the secret peers.yaml names; there is no flag for it.
type PeerPullCmd struct {
	Name      string `arg:"" help:"Peer name, as in peers.yaml."`
	DryRun    bool   `name:"dry-run" help:"Report what the pull would do, and write nothing."`
	AllowGone int    `name:"allow-gone" help:"Confirm that up to this many rows the source no longer lists may be detached or deleted, above the peer's max_gone."`
}

// Run pulls and prints the report.
func (c *PeerPullCmd) Run(ctx context.Context, ws scheduler.WorkspaceProvider, bind scheduler.IdentityBinder) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return c.run(ctx, ws, bind, logger, out)
}

func (c *PeerPullCmd) run(
	ctx context.Context, ws scheduler.WorkspaceProvider, bind scheduler.IdentityBinder, logger *slog.Logger,
	w *output.Writer,
) error {
	if !principal.ValidPeerName(c.Name) {
		return fmt.Errorf("invalid peer name %q", c.Name)
	}
	if c.AllowGone < 0 {
		return errors.New("--allow-gone must not be negative")
	}
	report, err := scheduler.PullPeer(ctx, ws, bind, c.Name, peersync.Options{DryRun: c.DryRun, AllowGone: c.AllowGone},
		logger)
	if report.Peer != "" {
		var wErr error
		if w.Format == output.FormatJSON {
			wErr = json.NewEncoder(w.Out).Encode(report)
		} else {
			wErr = writePeerReport(w.Out, report)
		}
		if err == nil {
			err = wErr
		}
	}
	return err
}

func writePeerReport(w io.Writer, r peersync.Report) error {
	var b strings.Builder
	verb := "Pulled"
	if r.DryRun {
		verb = "Dry run of a pull"
	}
	fmt.Fprintf(&b, "%s from peer %s.\n", verb, r.Peer)
	if r.Refused != "" {
		fmt.Fprintf(&b, "Refused: %s\n", reconclient.Sanitize(r.Refused))
	}
	counts := func(kind string, c peersync.Counts) {
		fmt.Fprintf(&b, "%s: %d created, %d adopted, %d updated, %d unchanged, %d conflicts, %d rejected, "+
			"%d dropped, %d deferred, %d detached, %d deleted, %d failed\n", kind, c.Created, c.Adopted, c.Updated,
			c.Unchanged, c.Conflicts, c.Rejected, c.Dropped, c.Deferred, c.Detached, c.Deleted, c.Failed)
	}
	counts("Entities", r.Entities)
	counts("Relations", r.Relations)
	if r.Gone > 0 {
		fmt.Fprintf(&b, "Gone from the source: %d\n", r.Gone)
	}
	if r.Waiting > 0 {
		fmt.Fprintf(&b, "Waiting for a later run (max_fetch or max_bytes reached): %d\n", r.Waiting)
	}
	for _, t := range r.Skipped {
		fmt.Fprintf(&b, "Not compared: %s\n", shown(t))
	}
	for _, i := range r.Issues {
		fmt.Fprintf(&b, "  %s %s", i.Kind, shown(i.Address))
		if i.Local != "" {
			fmt.Fprintf(&b, " (local %s)", shown(i.Local))
		}
		fmt.Fprintf(&b, ": %s", reconclient.Sanitize(i.Reason))
		if len(i.Fields) > 0 {
			fmt.Fprintf(&b, " [%s]", reconclient.Sanitize(strings.Join(i.Fields, ", ")))
		}
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}
