package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

// ErrNoRemote reports a remote name remotes.yaml does not declare.
var ErrNoRemote = errors.New("no remote")

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
		return nil, fmt.Errorf("%w %q in %s", ErrNoRemote, name, remotesync.FileName)
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

// ResolveRemote resolves the conflict list entries target t names on
// remote name to side take, and then syncs the remote once as [PullRemote]
// does, so the winning value is sent right away. It takes the remote's run
// lock for both, so no scheduled sync rewrites the ledger in between, and
// checks the entries first, so a resolve that names nothing does not count
// as a run.
//
// Nil: rejected for ws, bind and logger, as for [PullRemote].
func ResolveRemote(
	ctx context.Context, ws WorkspaceProvider, bind IdentityBinder, name string, t remotesync.Target, take string,
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
	if _, err = remotesync.CheckResolve(list, t, take); err != nil {
		return 0, remotesync.Report{}, err
	}
	subject := remoteSubject(list, t.Local)
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
	n, err := remotesync.Resolve(ctx, ws.State(), m, t, take)
	var report remotesync.Report
	if err == nil {
		logger.Info("remote conflict resolved", "remote", name, "run_id", runID, "local", t.Local,
			"field", t.Field, "take", take, "entries", n)
		recordRemote(ctx, ws, audit.OpRemoteResolve, subject, resolveSummary(name, t, take, n))
		report, err = pull(ctx, ws, bind, m, runID, remotesync.Options{}, nil, logger)
	}
	finishRun(ctx, runs, run, schedulerstate.Outcome{At: time.Now()}, err, logger)
	return n, report, err
}

// ResolveRemoteEntry resolves the conflict list entries target t names on
// remote name to side take, without a sync: the next scheduled or manual
// sync sends the winning value. It holds the remote's run lock, but the
// run does not count as a pull, so the schedule is left alone.
//
// Nil: rejected for ws and logger.
func ResolveRemoteEntry(
	ctx context.Context, ws WorkspaceProvider, name string, t remotesync.Target, take string, logger *slog.Logger,
) (int, error) {
	var n int
	var subject *audit.Subject
	check := func(list []remotesync.Conflict) error {
		subject = remoteSubject(list, t.Local)
		_, err := remotesync.CheckResolve(list, t, take)
		return err
	}
	err := lockedRemote(ctx, ws, name, logger, check, func(m *remotesync.Mapping, runID string) error {
		var err error
		if n, err = remotesync.Resolve(ctx, ws.State(), m, t, take); err == nil {
			logger.Info("remote conflict resolved", "remote", name, "run_id", runID, "local", t.Local,
				"field", t.Field, "take", take, "entries", n)
			recordRemote(ctx, ws, audit.OpRemoteResolve, subject, resolveSummary(name, t, take, n))
		}
		return err
	})
	return n, err
}

// DismissRemote removes the superseded entries target t names on remote
// name. It changes neither side and runs no sync, but it takes the remote's
// run lock, so a sync does not rewrite the conflict list over it.
//
// Nil: rejected for ws and logger.
func DismissRemote(
	ctx context.Context, ws WorkspaceProvider, name string, t remotesync.Target, logger *slog.Logger,
) (int, error) {
	var n int
	var subject *audit.Subject
	check := func(list []remotesync.Conflict) error {
		subject = remoteSubject(list, t.Local)
		_, err := remotesync.CheckDismiss(list, t)
		return err
	}
	err := lockedRemote(ctx, ws, name, logger, check, func(m *remotesync.Mapping, runID string) error {
		var err error
		if n, err = remotesync.Dismiss(ctx, ws.State(), m, t); err == nil {
			logger.Info("remote conflict dismissed", "remote", name, "run_id", runID, "local", t.Local,
				"field", t.Field, "entries", n)
			recordRemote(ctx, ws, audit.OpRemoteDismiss, subject, dismissSummary(name, t, n))
		}
		return err
	})
	return n, err
}

// LinkRemote links the uncertain create of the local entity t.Local on
// remote name to the remote row at address, as [remotesync.Link] does. It
// runs no sync, under the remote's run lock.
//
// Nil: rejected for ws and logger.
func LinkRemote(
	ctx context.Context, ws WorkspaceProvider, name string, t remotesync.Target, address string, logger *slog.Logger,
) error {
	var subject *audit.Subject
	check := func(list []remotesync.Conflict) error {
		subject = remoteSubject(list, t.Local)
		return remotesync.CheckLink(list, t)
	}
	return lockedRemote(ctx, ws, name, logger, check, func(m *remotesync.Mapping, runID string) error {
		err := remotesync.Link(ctx, ws.State(), m, t, address)
		if err == nil {
			logger.Info("remote create linked", "remote", name, "run_id", runID, "local", t.Local, "address", address)
			recordRemote(ctx, ws, audit.OpRemoteLink, subject, "remote="+name+" address="+address)
		}
		return err
	})
}

// lockedRemote runs fn on the mapping of remote name under the remote's run
// lock. When check is set, it first checks the conflict list with it, before
// the lock, so a request that names nothing does not count as a run. The
// run is lock-only: it neither counts as a pull nor climbs the
// retry ladder, so a change to the conflict list does not postpone the next
// scheduled pull.
func lockedRemote(
	ctx context.Context, ws WorkspaceProvider, name string, logger *slog.Logger,
	check func([]remotesync.Conflict) error, fn func(m *remotesync.Mapping, runID string) error,
) error {
	switch {
	case ws == nil:
		return errors.New("scheduler: a workspace is required")
	case logger == nil:
		return errors.New("scheduler: a logger is required")
	}
	m, err := remoteMapping(ctx, ws, name)
	if err != nil {
		return err
	}
	if check != nil {
		list, lerr := remotesync.LoadConflicts(ctx, ws.State(), name)
		if lerr != nil {
			return lerr
		}
		if cerr := check(list); cerr != nil {
			return cerr
		}
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return errors.New("scheduler: the workspace has no scheduler state")
	}
	runID := uuid.NewString()
	run, err := startManualRun(ctx, runs, RemoteTaskPrefix+name, runID)
	if err != nil {
		return err
	}
	err = fn(m, runID)
	finishRun(ctx, runs, run, schedulerstate.Outcome{At: time.Now(), LockOnly: true}, err, logger)
	return err
}

// finishRun records run's outcome out, with err as its error, past ctx's
// cancellation. A failure to record is logged: the work is done either way.
func finishRun(
	ctx context.Context, runs schedulerstate.Store, run schedulerstate.Run, out schedulerstate.Outcome, err error,
	logger *slog.Logger,
) {
	if err != nil {
		out.Error = err.Error()
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	if _, fErr := runs.FinishRun(fctx, run.ID, out, retryDelay); fErr != nil {
		logger.Error("could not record the run's outcome", "task", run.Task, "run_id", run.ID, "error", fErr)
	}
}

// PurgeRemoteSuperseded drops the superseded entries of the local entity id,
// with the losing values they keep, from the conflict list of every remote
// in remotes.yaml. `rela history purge` calls it, so a purged value leaves
// no copy on a conflict list. Each remote's purge counts as a run and so
// holds its run lock; a remote that is syncing is skipped with an error,
// and the purge can be run again. It returns how many entries it dropped.
//
// Nil: rejected for ws and logger.
func PurgeRemoteSuperseded(ctx context.Context, ws WorkspaceProvider, id string, logger *slog.Logger) (int, error) {
	switch {
	case ws == nil:
		return 0, errors.New("scheduler: a workspace is required")
	case logger == nil:
		return 0, errors.New("scheduler: a logger is required")
	}
	var meta *metamodel.Metamodel
	if mp, ok := ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	remotes, err := LoadRemotes(ctx, ws.Config(), meta)
	if err != nil {
		return 0, wrapRemotes(err)
	}
	if len(remotes.Mappings) == 0 {
		return 0, nil
	}
	total := 0
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(remotes.Mappings)) {
		n, err := purgeOneRemote(ctx, ws, name, id, logger)
		total += n
		if err != nil {
			errs = append(errs, fmt.Errorf("remote %s: %w", name, err))
		}
	}
	return total, errors.Join(errs...)
}

// purgeOneRemote drops id's superseded entries on one remote under its run
// lock.
func purgeOneRemote(ctx context.Context, ws WorkspaceProvider, name, id string, logger *slog.Logger) (int, error) {
	var n int
	err := lockedRemote(ctx, ws, name, logger, nil, func(_ *remotesync.Mapping, runID string) error {
		var err error
		if n, err = remotesync.PurgeSuperseded(ctx, ws.State(), name, id); err == nil && n > 0 {
			logger.Info("superseded remote conflicts purged", "remote", name, "run_id", runID, "local", id,
				"entries", n)
		}
		return err
	})
	return n, err
}

// auditProvider is the audit sink a workspace may carry. A workspace without
// one records nothing, as for every other optional capability here.
type auditProvider interface {
	Audit() audit.Audit
}

// remoteSubject returns the audit subject of the local row local names on
// list: an entity, or a relation when its entry is a relation entry. It is
// nil when list holds no entry of local.
func remoteSubject(list []remotesync.Conflict, local string) *audit.Subject {
	for i := range list {
		e := &list[i]
		if e.Local != local {
			continue
		}
		if key, ok := e.RelationKey(); ok {
			return &audit.Subject{Kind: "relation", RelationType: key.Type, FromID: key.From, ToID: key.To}
		}
		return &audit.Subject{Kind: "entity", Type: e.Type, ID: e.Local}
	}
	return nil
}

// recordRemote records op on subject in the workspace's audit sink. The
// summary names the remote, field, side or address; never a value.
func recordRemote(ctx context.Context, ws WorkspaceProvider, op string, subject *audit.Subject, summary string) {
	ap, ok := ws.(auditProvider)
	if !ok || subject == nil {
		return
	}
	sink := ap.Audit()
	if sink == nil {
		return
	}
	sink.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          op,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		Summary:     summary,
	})
}

func resolveSummary(name string, t remotesync.Target, take string, n int) string {
	return dismissSummary(name, t, n) + " take=" + take
}

func dismissSummary(name string, t remotesync.Target, n int) string {
	s := "remote=" + name
	if t.Field != "" {
		s += " field=" + t.Field
	}
	return s + " entries=" + strconv.Itoa(n)
}
