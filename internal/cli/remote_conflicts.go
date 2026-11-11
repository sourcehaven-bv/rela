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
	"time"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// RemoteConflictsCmd lists a remote's conflict list (TKT-UH2Q9MN).
type RemoteConflictsCmd struct {
	Name string `arg:"" help:"Remote name, as in remotes.yaml."`
}

// Run prints the conflict list.
func (c *RemoteConflictsCmd) Run(ctx context.Context, ws scheduler.WorkspaceProvider) error {
	w, asJSON := remoteOutput()
	return c.run(ctx, ws, w, asJSON)
}

func (c *RemoteConflictsCmd) run(ctx context.Context, ws scheduler.WorkspaceProvider, w io.Writer, asJSON bool) error {
	if !principal.ValidIntegrationName(c.Name) {
		return fmt.Errorf("invalid remote name %q", c.Name)
	}
	list, err := scheduler.RemoteConflicts(ctx, ws, c.Name)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(w).Encode(list)
	}
	return writeConflicts(w, c.Name, list)
}

func writeConflicts(w io.Writer, name string, list []remotesync.Conflict) error {
	var b strings.Builder
	if len(list) == 0 {
		fmt.Fprintf(&b, "Remote %s holds no conflicts.\n", name)
		_, err := io.WriteString(w, b.String())
		return err
	}
	fmt.Fprintf(&b, "Remote %s holds %d conflicts:\n", name, len(list))
	for _, e := range list {
		fmt.Fprintf(&b, "  %s %s", shown(e.Type), shown(e.Local))
		if e.Field != "" {
			fmt.Fprintf(&b, " field %s", shown(e.Field))
		}
		if e.Address != "" {
			fmt.Fprintf(&b, " (remote %s)", shown(e.Address))
		}
		switch e.Reason {
		case remotesync.ReasonBothChanged:
			b.WriteString(": both sides changed it")
			if e.RemoteValueOmitted {
				b.WriteString("; the remote value is too large to show")
			} else if v, err := json.Marshal(e.RemoteValue); err == nil {
				fmt.Fprintf(&b, "; remote value %s", reconclient.Sanitize(string(v)))
			}
		case remotesync.ReasonUncertain:
			fmt.Fprintf(&b, ": %s", reconclient.Sanitize(e.Detail))
		default:
			fmt.Fprintf(&b, ": the remote refused the push: %s", reconclient.Sanitize(e.Detail))
		}
		fmt.Fprintf(&b, " (since %s)\n", e.Since.UTC().Format(time.RFC3339))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RemoteResolveCmd resolves held conflicts of one local row (TKT-UH2Q9MN).
//
// It writes neither side itself: it moves the merge base so the sync that
// follows sends the chosen value, through the normal write path on each
// side, as the person who runs it.
type RemoteResolveCmd struct {
	Name  string `arg:"" help:"Remote name, as in remotes.yaml."`
	Local string `arg:"" help:"Local entity id of the held row."`
	Field string `arg:"" optional:"" help:"Local field to resolve; all of the row's entries when left out."`
	Take  string `name:"take" required:"" enum:"local,remote" help:"Side whose value wins: local or remote."`
}

// Run resolves, syncs and prints the report.
func (c *RemoteResolveCmd) Run(
	ctx context.Context, ws scheduler.WorkspaceProvider, bind scheduler.IdentityBinder,
) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, asJSON := remoteOutput()
	return c.run(ctx, ws, bind, logger, w, asJSON)
}

func (c *RemoteResolveCmd) run(
	ctx context.Context, ws scheduler.WorkspaceProvider, bind scheduler.IdentityBinder, logger *slog.Logger,
	w io.Writer, asJSON bool,
) error {
	if !principal.ValidIntegrationName(c.Name) {
		return fmt.Errorf("invalid remote name %q", c.Name)
	}
	n, report, err := scheduler.ResolveRemote(ctx, ws, bind, c.Name, c.Local, c.Field, c.Take, logger)
	if errors.Is(err, remotesync.ErrNoConflict) {
		return fmt.Errorf("remote %s holds no conflict for %s", c.Name, shown(c.Local))
	}
	if n == 0 {
		return err
	}
	var wErr error
	if asJSON {
		wErr = json.NewEncoder(w).Encode(struct {
			Resolved int               `json:"resolved"`
			Report   remotesync.Report `json:"report"`
		}{n, report})
	} else {
		_, wErr = fmt.Fprintf(w, "Resolved %d conflicts of %s to the %s side.\n", n, shown(c.Local), c.Take)
		if wErr == nil && report.Remote != "" {
			wErr = writeRemoteReport(w, report)
		}
	}
	if err == nil {
		err = wErr
	}
	return err
}
