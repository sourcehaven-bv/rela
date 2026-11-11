package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

// remoteMapping loads remotes.yaml and returns the mapping of remote name.
func remoteMapping(ctx context.Context, ws WorkspaceProvider, name string) (*remotesync.Mapping, error) {
	var meta *metamodel.Metamodel
	if mp, ok := ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	remotes, err := LoadRemotes(ctx, ws.Config(), meta)
	if err != nil {
		return nil, wrapRemotes(err)
	}
	m := remotes.Mappings[name]
	if m == nil {
		return nil, fmt.Errorf("no remote %q in %s", name, remotesync.FileName)
	}
	return m, nil
}

// RemoteConflicts returns the conflict list of remote name, for
// `rela remote conflicts`. The list is operator state outside the graph,
// like the ledger: it is read from the state store, not through acl.yaml.
//
// Nil: rejected for ws; the list lives in its state store.
func RemoteConflicts(ctx context.Context, ws WorkspaceProvider, name string) ([]remotesync.Conflict, error) {
	if ws == nil {
		return nil, errors.New("scheduler: a workspace is required")
	}
	if _, err := remoteMapping(ctx, ws, name); err != nil {
		return nil, err
	}
	return remotesync.LoadConflicts(ctx, ws.State(), name)
}

// ResolveRemote resolves the conflict list entries of one local row of
// remote name, all of them or the one of field, to side take, and then
// syncs the remote once as [PullRemote] does, so the winning value is sent
// right away. It takes the remote's run lock for both, so no scheduled sync
// rewrites the ledger in between, and checks the entries first, so a
// resolve that names nothing does not count as a run.
//
// Nil: rejected for ws, bind and logger, as for [PullRemote].
func ResolveRemote(
	ctx context.Context, ws WorkspaceProvider, bind IdentityBinder, name, local, field, take string,
	logger *slog.Logger,
) (int, remotesync.Report, error) {
	switch {
	case ws == nil:
		return 0, remotesync.Report{}, errors.New("scheduler: a workspace is required")
	case bind == nil:
		return 0, remotesync.Report{}, errors.New("scheduler: an identity binder is required")
	case logger == nil:
		return 0, remotesync.Report{}, errors.New("scheduler: a logger is required")
	}
	m, err := remoteMapping(ctx, ws, name)
	if err != nil {
		return 0, remotesync.Report{}, err
	}
	list, err := remotesync.LoadConflicts(ctx, ws.State(), name)
	if err != nil {
		return 0, remotesync.Report{}, err
	}
	if _, err = remotesync.CheckResolve(list, local, field, take); err != nil {
		return 0, remotesync.Report{}, err
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return 0, remotesync.Report{}, errors.New("scheduler: the workspace has no scheduler state")
	}
	runID := uuid.NewString()
	ctx = audit.WithTriggeredBy(ctx, RemoteTaskPrefix+name)
	run, err := startManualRun(ctx, runs, RemoteTaskPrefix+name, runID)
	if err != nil {
		return 0, remotesync.Report{}, err
	}
	n, err := remotesync.Resolve(ctx, ws.State(), m, local, field, take)
	var report remotesync.Report
	if err == nil {
		logger.Info("remote conflict resolved", "remote", name, "run_id", runID, "local", local,
			"field", field, "take", take, "entries", n)
		report, err = pull(ctx, ws, bind, m, runID, remotesync.Options{}, nil, logger)
	}
	out := schedulerstate.Outcome{At: time.Now()}
	if err != nil {
		out.Error = err.Error()
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	if _, fErr := runs.FinishRun(fctx, run.ID, out, retryDelay); fErr != nil {
		logger.Error("could not record the sync's outcome", "task", run.Task, "run_id", run.ID, "error", fErr)
	}
	return n, report, err
}
