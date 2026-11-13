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
		case remotesync.ReasonSuperseded:
			writeSuperseded(&b, e)
		case remotesync.ReasonBothChanged:
			b.WriteString(": both sides changed it")
			if e.Fallback != "" {
				fmt.Fprintf(&b, "; rule %s held it (%s)", e.Rule, e.Fallback)
			}
			if e.RemoteValueOmitted {
				b.WriteString("; the remote value is too large to show")
			} else if len(e.RemoteValue) > 0 {
				fmt.Fprintf(&b, "; remote value %s", reconclient.Sanitize(string(e.RemoteValue)))
			}
		case remotesync.ReasonUncertain:
			fmt.Fprintf(&b, ": %s; --take local sends it again, rela remote link links it to a remote row",
				reconclient.Sanitize(e.Detail))
		default:
			fmt.Fprintf(&b, ": the remote refused the push: %s", reconclient.Sanitize(e.Detail))
		}
		fmt.Fprintf(&b, " (since %s)\n", e.Since.UTC().Format(time.RFC3339))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeSuperseded describes an entry the latest rule decided.
func writeSuperseded(b *strings.Builder, e remotesync.Conflict) {
	s := e.Superseded
	fmt.Fprintf(b, ": the latest rule took the %s value (local changed %s, remote %s)", s.Winner,
		s.LocalUpdated.UTC().Format(time.RFC3339), s.RemoteUpdated.UTC().Format(time.RFC3339))
	lost, omitted, side := s.LocalValue, s.LocalValueOmitted, "local"
	if s.Winner == remotesync.TakeLocal {
		lost, omitted, side = e.RemoteValue, e.RemoteValueOmitted, "remote"
	}
	switch {
	case omitted:
		fmt.Fprintf(b, "; the overwritten %s value is too large to show", side)
	case len(lost) > 0:
		fmt.Fprintf(b, "; overwritten %s value %s", side, reconclient.Sanitize(string(lost)))
	default:
		fmt.Fprintf(b, "; the overwritten %s value was not kept", side)
	}
}

// RemoteResolveCmd resolves held conflicts of one local row (TKT-UH2Q9MN),
// or dismisses the entries the latest rule left.
//
// It writes neither side itself: it moves the merge base so the sync that
// follows sends the chosen value, through the normal write path on each
// side, as the person who runs it. A dismiss only removes the entries.
type RemoteResolveCmd struct {
	Name    string `arg:"" help:"Remote name, as in remotes.yaml."`
	Local   string `arg:"" help:"Local entity id, or local relation key, of the row."`
	Field   string `arg:"" optional:"" help:"Local field to resolve; all of the row's entries when left out."`
	Take    string `name:"take" enum:",local,remote" default:"" help:"Side whose value wins: local or remote."`
	Dismiss bool   `name:"dismiss" help:"Remove the entries the latest rule left; changes no data."`
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
	switch {
	case c.Dismiss && c.Take != "":
		return errors.New("use either --take or --dismiss, not both")
	case c.Dismiss:
		return c.dismiss(ctx, ws, logger, w, asJSON)
	case c.Take == "":
		return errors.New("use --take local, --take remote or --dismiss")
	}
	n, report, err := scheduler.ResolveRemote(ctx, ws, bind, c.Name, c.target(), c.Take, logger)
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

// target names the entries the command acts on: the field's, or all of the
// row's when no field is given.
func (c *RemoteResolveCmd) target() remotesync.Target {
	return remotesync.Target{Local: c.Local, Field: c.Field}
}

func (c *RemoteResolveCmd) dismiss(
	ctx context.Context, ws scheduler.WorkspaceProvider, logger *slog.Logger, w io.Writer, asJSON bool,
) error {
	n, err := scheduler.DismissRemote(ctx, ws, c.Name, c.target(), logger)
	switch {
	case errors.Is(err, remotesync.ErrNoConflict):
		return fmt.Errorf("remote %s holds no superseded entry for %s", c.Name, shown(c.Local))
	case err != nil:
		return err
	case asJSON:
		return json.NewEncoder(w).Encode(struct {
			Dismissed int `json:"dismissed"`
		}{n})
	}
	_, err = fmt.Fprintf(w, "Dismissed %d entries of %s.\n", n, shown(c.Local))
	return err
}

// RemoteLinkCmd links an uncertain create to the remote row it made
// (TKT-0FMYPEZ). A push holds a create whose outcome it could not tell; when
// a person finds the row on the remote, linking the two settles the entry
// without sending the create again. It writes neither side.
type RemoteLinkCmd struct {
	Name    string `arg:"" help:"Remote name, as in remotes.yaml."`
	Local   string `arg:"" help:"Local entity id of the uncertain create."`
	Address string `arg:"" help:"Remote row the create made, as type/id."`
}

// Run links the two rows.
func (c *RemoteLinkCmd) Run(ctx context.Context, ws scheduler.WorkspaceProvider) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, asJSON := remoteOutput()
	return c.run(ctx, ws, logger, w, asJSON)
}

func (c *RemoteLinkCmd) run(
	ctx context.Context, ws scheduler.WorkspaceProvider, logger *slog.Logger, w io.Writer, asJSON bool,
) error {
	if !principal.ValidIntegrationName(c.Name) {
		return fmt.Errorf("invalid remote name %q", c.Name)
	}
	err := scheduler.LinkRemote(ctx, ws, c.Name, remotesync.Target{Local: c.Local}, c.Address, logger)
	switch {
	case errors.Is(err, remotesync.ErrNoConflict):
		return fmt.Errorf("remote %s holds no uncertain create of %s", c.Name, shown(c.Local))
	case err != nil:
		return err
	case asJSON:
		return json.NewEncoder(w).Encode(struct {
			Linked  string `json:"linked"`
			Address string `json:"address"`
		}{c.Local, c.Address})
	}
	_, err = fmt.Fprintf(w, "Linked %s to remote row %s; the next sync compares the two.\n", shown(c.Local),
		shown(c.Address))
	return err
}
