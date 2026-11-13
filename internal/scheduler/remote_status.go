package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
)

// RemoteStatus is what a status page shows about one remote: its settings,
// the types it maps with the conflict rule of each field, how its pulls
// went, and the rows its ledger links and its conflict list holds.
//
// Linked and Conflicts are operator state outside the graph and are not
// filtered by acl.yaml; a caller that shows them to a principal gates each
// row and field first, and counts after that.
type RemoteStatus struct {
	Remote    remotestatus.Remote
	Linked    []remotesync.Linked
	Conflicts []remotesync.Conflict
}

// RemoteStatuses returns the status of every remote in remotes.yaml, sorted
// by name. A project without remotes.yaml has none.
//
// Nil: rejected for ws; the status lives in its state stores.
func RemoteStatuses(ctx context.Context, ws WorkspaceProvider) ([]RemoteStatus, error) {
	if ws == nil {
		return nil, errors.New("scheduler: a workspace is required")
	}
	var meta *metamodel.Metamodel
	if mp, ok := ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	remotes, err := LoadRemotes(ctx, ws.Config(), meta)
	if err != nil {
		return nil, wrapRemotes(err)
	}
	names := remotes.Names()
	if len(names) == 0 {
		return nil, nil
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return nil, errors.New("scheduler: the workspace has no scheduler state")
	}
	tasks := make([]string, len(names))
	for i, name := range names {
		tasks[i] = RemoteTaskPrefix + name
	}
	states, err := runs.Load(ctx, tasks)
	if err != nil {
		return nil, fmt.Errorf("scheduler: read run state: %w", err)
	}
	out := make([]RemoteStatus, 0, len(names))
	for _, name := range names {
		m := remotes.Mappings[name]
		ts := states[RemoteTaskPrefix+name]
		st := RemoteStatus{Remote: m.Remote.Status(name)}
		st.Remote.Types = m.MappedTypes()
		st.Remote.Run = remotestatus.Run{
			LastRun: ts.LastRun, Running: ts.Active != nil, Failures: ts.Failures, NextRetry: ts.NextRetry,
		}
		if every, perr := parseSchedule(m.Remote.Every); perr == nil && m.Remote.Every != "" && !ts.LastRun.IsZero() {
			st.Remote.Run.NextRun = every.Next(ts.LastRun)
		}
		if st.Linked, err = remotesync.LoadLinked(ctx, ws.State(), m); err != nil {
			return nil, err
		}
		if st.Conflicts, err = remotesync.LoadConflicts(ctx, ws.State(), name); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}
