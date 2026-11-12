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

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// RemotePullCmd pulls once from a remote in remotes.yaml (TKT-I8L1EQU).
//
// It runs and writes as the person who runs it (or `--run-as`), so the
// local acl.yaml decides what it may write and the audit log names that
// person. It takes the scheduled pull's run lock, so the two never run at
// once. The credential comes from the source remotes.yaml names; there is
// no flag for it.
type RemotePullCmd struct {
	Name      string `arg:"" help:"Remote name, as in remotes.yaml."`
	DryRun    bool   `name:"dry-run" help:"Report what the pull would do, and write nothing."`
	AllowGone int    `name:"allow-gone" help:"Confirm that up to this many deletes and detaches may apply in this run, in either direction, above the remote's max_gone."`
}

// Run pulls and prints the report.
func (c *RemotePullCmd) Run(ctx context.Context, ws scheduler.WorkspaceProvider, bind scheduler.IdentityBinder) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, asJSON := remoteOutput()
	return c.run(ctx, ws, bind, logger, w, asJSON)
}

func (c *RemotePullCmd) run(
	ctx context.Context, ws scheduler.WorkspaceProvider, bind scheduler.IdentityBinder, logger *slog.Logger,
	w io.Writer, asJSON bool,
) error {
	if !principal.ValidIntegrationName(c.Name) {
		return fmt.Errorf("invalid remote name %q", c.Name)
	}
	if c.AllowGone < 0 {
		return errors.New("--allow-gone must not be negative")
	}
	opts := remotesync.Options{DryRun: c.DryRun, AllowGone: c.AllowGone}
	report, err := scheduler.PullRemote(ctx, ws, bind, c.Name, opts, logger)
	if report.Remote != "" {
		var wErr error
		if asJSON {
			wErr = json.NewEncoder(w).Encode(report)
		} else {
			wErr = writeRemoteReport(w, report)
		}
		if err == nil {
			err = wErr
		}
	}
	return err
}

func writeRemoteReport(w io.Writer, r remotesync.Report) error {
	var b strings.Builder
	verb := "Pulled from"
	switch {
	case r.DryRun && r.Pushed != nil:
		verb = "Dry run of a sync with"
	case r.DryRun:
		verb = "Dry run of a pull from"
	case r.Pushed != nil:
		verb = "Synced with"
	}
	fmt.Fprintf(&b, "%s remote %s.\n", verb, r.Remote)
	if r.Refused != "" {
		fmt.Fprintf(&b, "Refused: %s\n", reconclient.Sanitize(r.Refused))
	}
	counts := func(kind string, c remotesync.Counts) {
		fmt.Fprintf(&b, "%s: %d created, %d adopted, %d updated, %d unchanged, %d conflicts, %d rejected, "+
			"%d dropped, %d deferred, %d detached, %d deleted, %d failed\n", kind, c.Created, c.Adopted, c.Updated,
			c.Unchanged, c.Conflicts, c.Rejected, c.Dropped, c.Deferred, c.Detached, c.Deleted, c.Failed)
	}
	counts("Entities", r.Entities)
	counts("Relations", r.Relations)
	pushed := func(kind string, p *remotesync.PushCounts) {
		if p != nil {
			fmt.Fprintf(&b, "%s: %d created, %d updated, %d deleted, %d conflicts, %d refused, %d deferred, "+
				"%d failed\n", kind, p.Created, p.Updated, p.Deleted, p.Conflicts, p.Refused, p.Deferred, p.Failed)
		}
	}
	pushed("Pushed", r.Pushed)
	pushed("Pushed relations", r.PushedRelations)
	if r.Deleting > 0 {
		fmt.Fprintf(&b, "Deleted here, to delete on the remote: %d\n", r.Deleting)
	}
	if r.Held > 0 {
		fmt.Fprintf(&b, "Held on the conflict list: %d (see rela remote conflicts %s)\n", r.Held, shown(r.Remote))
	}
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
