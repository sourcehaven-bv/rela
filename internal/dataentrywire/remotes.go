package dataentrywire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// remotes adapts the scheduler's remote functions to
// [dataentry.RemoteService]. It holds no state: every call reads remotes.yaml,
// the run state, the ledgers and the conflict lists afresh.
type remotes struct {
	ws     auditedWorkspace
	logger *slog.Logger
}

// auditedWorkspace is the workspace the remote routes run on. It must carry
// an audit sink: the scheduler records resolve, dismiss and link only on a
// workspace that has one, so asking for it here makes a wiring without an
// audit sink a compile error rather than a silent gap in the audit log.
type auditedWorkspace interface {
	scheduler.WorkspaceProvider
	Audit() audit.Audit
}

// newRemotes returns the adapter over ws.
//
// Nil: rejected for ws and logger.
func newRemotes(ws auditedWorkspace, logger *slog.Logger) (*remotes, error) {
	if ws == nil || logger == nil {
		return nil, errors.New("dataentrywire: remotes need a workspace and a logger")
	}
	return &remotes{ws: ws, logger: logger}, nil
}

// Remotes implements [dataentry.RemoteService].
func (r *remotes) Remotes(ctx context.Context) ([]dataentry.RemoteStatus, error) {
	list, err := scheduler.RemoteStatuses(ctx, r.ws)
	if err != nil {
		return nil, err
	}
	out := make([]dataentry.RemoteStatus, len(list))
	for i, st := range list {
		out[i] = remoteStatus(st)
	}
	return out, nil
}

// Resolve implements [dataentry.RemoteService]. It names the entry exactly,
// so it settles no other entry of the row than the one the caller was
// gated on.
func (r *remotes) Resolve(ctx context.Context, name string, key dataentry.RemoteEntryKey, take string) (int, error) {
	n, err := scheduler.ResolveRemoteEntry(ctx, r.ws, name, exact(key), take, r.logger)
	return n, remoteError(err)
}

// Dismiss implements [dataentry.RemoteService], naming the entry exactly.
func (r *remotes) Dismiss(ctx context.Context, name string, key dataentry.RemoteEntryKey) (int, error) {
	n, err := scheduler.DismissRemote(ctx, r.ws, name, exact(key), r.logger)
	return n, remoteError(err)
}

// Link implements [dataentry.RemoteService].
func (r *remotes) Link(ctx context.Context, name string, key dataentry.RemoteEntryKey, address string) error {
	return remoteError(scheduler.LinkRemote(ctx, r.ws, name, exact(key), address, r.logger))
}

// exact names the one entry key names, with the fields the handler checked
// on it.
func exact(key dataentry.RemoteEntryKey) remotesync.Target {
	return remotesync.Target{Local: key.Local, Field: key.Field, Exact: true, Fields: key.Fields}
}

// remoteError maps the scheduler's and remotesync's errors to dataentry's.
// A refusal's text stays in the error for the log; the route answers with
// its own words.
func remoteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, scheduler.ErrNoRemote):
		return fmt.Errorf("%w: %w", dataentry.ErrRemoteUnknown, err)
	case errors.Is(err, remotesync.ErrNoConflict):
		return fmt.Errorf("%w: %w", dataentry.ErrRemoteNoEntry, err)
	case errors.Is(err, scheduler.ErrRemoteBusy):
		return fmt.Errorf("%w: %w", dataentry.ErrRemoteBusy, err)
	case errors.Is(err, remotesync.ErrRefused):
		return fmt.Errorf("%w: %w", dataentry.ErrRemoteRefused, err)
	}
	return err
}

func remoteStatus(st scheduler.RemoteStatus) dataentry.RemoteStatus {
	out := dataentry.RemoteStatus{
		Remote:     st.Remote,
		LinkedRows: make([]dataentry.RemoteRow, 0, len(st.Linked)),
		Entries:    make([]dataentry.RemoteEntry, 0, len(st.Conflicts)),
	}
	for _, l := range st.Linked {
		row, ok := remoteRow(l.Kind, l.Type, l.Local)
		if ok {
			out.LinkedRows = append(out.LinkedRows, row)
		}
	}
	for i := range st.Conflicts {
		if e, ok := remoteEntry(&st.Conflicts[i]); ok {
			out.Entries = append(out.Entries, e)
		}
	}
	return out
}

// remoteRow is the dataentry row of a local entity or relation. A relation
// key that does not parse names no row anyone can read, so it is left out.
func remoteRow(kind, typ, local string) (dataentry.RemoteRow, bool) {
	row := dataentry.RemoteRow{Kind: kind, Type: typ, Local: local}
	if kind != remotesync.KindRelation {
		return row, true
	}
	key, ok := (&remotesync.Conflict{Kind: kind, Local: local}).RelationKey()
	if !ok {
		return dataentry.RemoteRow{}, false
	}
	row.Relation = key
	return row, true
}

// remoteEntry is c without its Detail, which quotes the remote. An entry
// written before Cause existed gets its cause from the reason and its
// status from the leading number of its Detail.
func remoteEntry(c *remotesync.Conflict) (dataentry.RemoteEntry, bool) {
	row, ok := remoteRow(c.Kind, c.Type, c.Local)
	if !ok {
		return dataentry.RemoteEntry{}, false
	}
	e := dataentry.RemoteEntry{
		RemoteRow: row, Address: c.Address, Field: c.Field, RemoteField: c.RemoteField,
		Fields: c.RefusedLocalFields(), Reason: c.Reason, Cause: c.Cause, Status: c.Status, Rule: c.Rule,
		Fallback: c.Fallback, Since: c.Since, Seen: c.Seen, RemoteValue: c.RemoteValue,
		RemoteValueOmitted: c.RemoteValueOmitted,
	}
	if e.Cause == "" && c.Reason == remotesync.ReasonRefused {
		e.Cause = remotesync.CauseRefused
	}
	if e.Status == 0 {
		e.Status = leadingStatus(c.Detail)
	}
	if s := c.Superseded; s != nil {
		sup := *s
		e.Superseded = &sup
	}
	return e, true
}

// leadingStatus is the HTTP status an old entry's Detail starts with, as
// "422 title is required", or 0.
func leadingStatus(detail string) int {
	word, _, _ := strings.Cut(detail, " ")
	n, err := strconv.Atoi(word)
	if err != nil || n < 100 || n > 599 {
		return 0
	}
	return n
}
