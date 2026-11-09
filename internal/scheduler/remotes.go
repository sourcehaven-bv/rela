package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// RemoteKind is the job kind for one scheduled pull from a sync remote.
var RemoteKind = jobs.NewKind("scheduler", "run-remote-pull")

// RemoteTaskPrefix starts the task name of every remote pull. The scheduled pull
// and `rela remote pull` share the task's run state, which is what keeps two
// pulls of one remote from running at once.
const RemoteTaskPrefix = "remote:"

// minRemoteInterval is the shortest interval a remote may be pulled at. A pull
// diffs every mapped type, so a short interval mostly loads the source.
const minRemoteInterval = 5 * time.Minute

// remoteRunTimeout bounds one pull. It stays under the queue's per-handler cap
// (15m) and the scheduler's running lease, so the run ends, and records its
// outcome, before either reclaims it.
const remoteRunTimeout = 14 * time.Minute

// ErrRemotes marks an invalid remotes.yaml in [LoadConfig]'s error, for a caller
// that still runs the other tasks it returned.
var ErrRemotes = errors.New("remotes.yaml is invalid")

// ErrRemoteBusy reports that a pull of the remote is already running.
var ErrRemoteBusy = errors.New("a pull of this remote is already running")

func wrapRemotes(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrRemotes, err)
}

// LoadRemotes reads and checks remotes.yaml. A missing file is an empty config.
//
// Nil: rejected for meta when the file exists; a mapping cannot be checked
// without the schema.
func LoadRemotes(ctx context.Context, loader config.Loader, meta *metamodel.Metamodel) (*remotesync.Config, error) {
	data, err := loadOptional(ctx, loader, remotesync.FileName)
	if err != nil || data == nil {
		return &remotesync.Config{}, err
	}
	return remotesync.Parse(data, meta)
}

// AddRemotes appends one task per remote with an `every` schedule to cfg. A remote
// without one is pulled only by `rela remote pull`. Tasks added before an error
// are kept out: cfg is changed only on success.
func AddRemotes(cfg *Config, remotes *remotesync.Config) error {
	names := make(map[string]bool, len(cfg.Tasks))
	for _, t := range cfg.Tasks {
		names[t.Name] = true
	}
	var tasks []TaskConfig
	var errs []error
	for _, name := range remotes.Names() {
		m := remotes.Mappings[name]
		if m == nil || m.Remote.Every == "" {
			continue
		}
		taskName := RemoteTaskPrefix + name
		every, err := parseSchedule(m.Remote.Every)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("remotes: %q every: %w", name, err))
			continue
		case every.kind == intervalKind && every.interval < minRemoteInterval:
			errs = append(errs, fmt.Errorf("remotes: %q every %s is shorter than the %s minimum",
				name, m.Remote.Every, minRemoteInterval))
			continue
		case names[taskName]:
			errs = append(errs, fmt.Errorf("remotes: %q: task name %q is taken", name, taskName))
			continue
		}
		tasks = append(tasks, TaskConfig{Name: taskName, Every: every, Remote: m})
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	cfg.Tasks = append(cfg.Tasks, tasks...)
	return nil
}

// remoteJob is the job for one scheduled pull. The payload names the task
// only: the handler reads the mapping from its own config.
func remoteJob(task TaskConfig, runID string) jobs.Job {
	return jobs.Job{
		Kind:           RemoteKind,
		Payload:        map[string]any{payloadTaskName: task.Name, payloadRunID: runID},
		Retry:          jobs.RetryNever,
		IdempotencyKey: runID,
	}
}

// runRemoteJob pulls one remote and records the run's outcome.
func (s *Scheduler) runRemoteJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	run, ok := s.beginRun(ctx, job, runID, name)
	if !ok {
		return nil
	}
	err := s.pullRemoteTask(ctx, name, runID)
	s.finish(ctx, run, err)
	return err
}

func (s *Scheduler) pullRemoteTask(ctx context.Context, taskName, runID string) error {
	task, ok := s.taskNamed(taskName)
	if !ok || task.Remote == nil {
		return fmt.Errorf("scheduler: remote task %q is not configured", taskName)
	}
	// A scheduled pull runs as the remote's run_as, or the scheduler's own
	// identity, as a scheduled task does. Its trigger is the task name,
	// remote:<name>, as for `rela remote pull`, not schedule:<task>.
	user := task.Remote.Remote.RunAs
	if user == "" {
		user = principal.UserScheduler
	}
	ctx = stampContext(ctx, user, task.Name)
	_, err := pull(ctx, s.ws, s.bind, task.Remote, runID, remotesync.Options{}, s.remoteTransport, s.logger)
	return err
}

// PullRemote runs one pull of remote name now, for `rela remote pull`, as the
// principal on ctx: the person who runs the command. It takes the
// remote's run lock in the scheduler state, so it never runs alongside a
// scheduled pull of the same remote; that one returns [ErrRemoteBusy]. A dry run
// takes no lock and leaves the task's schedule alone, because it writes
// nothing.
//
// Nil: rejected for ws, bind and logger; a pull cannot write without the
// workspace, act without an identity, or report without a logger.
func PullRemote(
	ctx context.Context, ws WorkspaceProvider, bind IdentityBinder, name string, o remotesync.Options,
	logger *slog.Logger,
) (remotesync.Report, error) {
	switch {
	case ws == nil:
		return remotesync.Report{}, errors.New("scheduler: a workspace is required")
	case bind == nil:
		return remotesync.Report{}, errors.New("scheduler: an identity binder is required")
	case logger == nil:
		return remotesync.Report{}, errors.New("scheduler: a logger is required")
	}
	var meta *metamodel.Metamodel
	if mp, ok := ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	remotes, err := LoadRemotes(ctx, ws.Config(), meta)
	if err != nil {
		return remotesync.Report{}, wrapRemotes(err)
	}
	m := remotes.Mappings[name]
	if m == nil {
		return remotesync.Report{}, fmt.Errorf("no remote %q in %s", name, remotesync.FileName)
	}
	runID := uuid.NewString()
	ctx = audit.WithTriggeredBy(ctx, RemoteTaskPrefix+name)
	if !o.DryRun {
		pruneLedgers(ctx, ws.State(), remotes, logger)
	}
	if o.DryRun {
		return pull(ctx, ws, bind, m, runID, o, nil, logger)
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return remotesync.Report{}, errors.New("scheduler: the workspace has no scheduler state")
	}
	run, err := startManualRun(ctx, runs, RemoteTaskPrefix+name, runID)
	if err != nil {
		return remotesync.Report{}, err
	}
	report, err := pull(ctx, ws, bind, m, runID, o, nil, logger)
	out := schedulerstate.Outcome{At: time.Now()}
	if err != nil {
		out.Error = err.Error()
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	if _, fErr := runs.FinishRun(fctx, run.ID, out, retryDelay); fErr != nil {
		logger.Error("could not record the pull's outcome", "task", run.Task, "run_id", run.ID, "error", fErr)
	}
	return report, err
}

// pruneLedgers deletes the ledgers of remotes that remotes.yaml no longer
// names. remotes must be a file that loaded; a pull prunes before it runs,
// and the scheduler at start. A failure is logged: a stale ledger does not
// stop a pull.
func pruneLedgers(ctx context.Context, kv state.KV, remotes *remotesync.Config, logger *slog.Logger) {
	if kv == nil || remotes == nil {
		return
	}
	pruned, err := remotesync.Prune(ctx, kv, remotes.Names())
	if err != nil {
		logger.Warn("could not delete the ledgers of removed remotes", "error", err)
	}
	if len(pruned) > 0 {
		logger.Info("deleted the ledgers of removed remotes", "remotes", pruned)
	}
}

// pruneRemoteLedgers prunes at start, when remotes.yaml loads, so a ledger
// of a removed remote goes even when no remote is pulled any more.
func (s *Scheduler) pruneRemoteLedgers(ctx context.Context) {
	var meta *metamodel.Metamodel
	if mp, ok := s.ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	remotes, err := LoadRemotes(ctx, s.ws.Config(), meta)
	if err != nil {
		return
	}
	pruneLedgers(ctx, s.ws.State(), remotes, s.logger)
}

// startManualRun records and starts a run of task outside the tick, as the
// lock that keeps it from overlapping a scheduled run.
func startManualRun(ctx context.Context, runs schedulerstate.Store, task, id string) (schedulerstate.Run, error) {
	now := time.Now()
	run := schedulerstate.Run{ID: id, Task: task, CreatedAt: now, LeaseUntil: now.Add(runningLease)}
	// A stale version means a tick updated the task's state between the
	// read and the create; that is not a running pull, so read again once.
	var err error
	for range 2 {
		var states map[string]schedulerstate.TaskState
		if states, err = runs.Load(ctx, []string{task}); err != nil {
			return schedulerstate.Run{}, fmt.Errorf("scheduler: read run state: %w", err)
		}
		if err = runs.CreateRun(ctx, run, states[task].Version); !errors.Is(err, schedulerstate.ErrStale) {
			break
		}
	}
	switch {
	case errors.Is(err, schedulerstate.ErrRunActive):
		return schedulerstate.Run{}, fmt.Errorf("%s: %w", task, ErrRemoteBusy)
	case err != nil:
		return schedulerstate.Run{}, fmt.Errorf("scheduler: create run: %w", err)
	}
	if run, err = runs.StartRun(ctx, id, "cli:"+nodeID(), now, now.Add(runningLease)); err != nil {
		return schedulerstate.Run{}, fmt.Errorf("scheduler: start run: %w", err)
	}
	return run, nil
}

// pull runs one pull of m as the principal on ctx. It binds that principal
// here, the pull's request edge, so the manager authorizes and audits each
// write against it. The caller stamps the principal and the triggered-by
// label.
func pull(
	ctx context.Context, ws WorkspaceProvider, bind IdentityBinder, m *remotesync.Mapping, runID string,
	o remotesync.Options, transport http.RoundTripper, logger *slog.Logger,
) (remotesync.Report, error) {
	ctx, cancel := context.WithTimeout(ctx, remoteRunTimeout)
	defer cancel()
	ctx, err := bind(ctx)
	if err != nil {
		return remotesync.Report{}, fmt.Errorf("scheduler: resolve identity for remote %q: %w", m.Name, err)
	}
	deps, err := remoteDeps(ws, m, runID, transport, logger, os.Getenv)
	if err != nil {
		return remotesync.Report{}, err
	}
	// The mapping was checked when its file loaded; compile it again against
	// the schema this pull writes with, so the run reads one schema.
	if m, err = remotesync.Compile(m.Name, m.Remote, deps.Meta); err != nil {
		return remotesync.Report{}, wrapRemotes(err)
	}
	report, err := remotesync.Run(ctx, m, deps, o)
	logger.Info("remote pulled",
		"remote", m.Name, "run_id", runID, "dry_run", o.DryRun, "refused", report.Refused, "gone", report.Gone,
		"entities", report.Entities, "relations", report.Relations, "issues", len(report.Issues))
	warnPull(logger, m.Name, runID, report)
	return report, err
}

// maxWarnedIssues caps the issues one pull logs; the report holds them all.
const maxWarnedIssues = 20

// warnPull logs at Warn what an operator must look at after a pull: refused
// guards, skipped relation types, and rows that were not applied.
func warnPull(logger *slog.Logger, name, runID string, rep remotesync.Report) {
	if rep.Refused != "" {
		logger.Warn("remote pull refused", "remote", name, "run_id", runID, "reason", rep.Refused, "gone", rep.Gone)
	}
	if len(rep.Skipped) > 0 {
		logger.Warn("remote pull skipped relation types", "remote", name, "run_id", runID, "types", rep.Skipped)
	}
	for i, is := range rep.Issues {
		if i == maxWarnedIssues {
			logger.Warn("remote pull has more issues", "remote", name, "run_id", runID, "more", len(rep.Issues)-i)
			break
		}
		logger.Warn("remote pull row not applied",
			"remote", name, "run_id", runID, "kind", is.Kind, "address", is.Address, "local", is.Local, "reason", is.Reason, "fields", is.Fields)
	}
}

// remoteDeps assembles the collaborators of one pull: the manager every write
// goes through, the raw store for the write-prep reads, the state store for
// the ledger, and the source dialed with the remote's credential.
//
// The writer is the field-gated manager of the scheduled Lua deps, which
// also updates relations. It is asserted rather than declared on the
// workspace because the workspace type is at its plimsoll method cap; the
// scheduler's other optional capabilities are asserted the same way.
func remoteDeps(
	ws WorkspaceProvider, m *remotesync.Mapping, runID string, transport http.RoundTripper, logger *slog.Logger,
	getenv func(string) string,
) (remotesync.Deps, error) {
	raw, ok := ws.(rawStoreProvider)
	if !ok || raw.Store() == nil {
		return remotesync.Deps{}, errors.New("scheduler: the workspace provides no store for remote pulls")
	}
	lua := ws.ScheduledLuaWriteDeps()
	writer, ok := lua.EntityManager.(remotesync.Writer)
	if !ok || isNilPointer(writer) {
		return remotesync.Deps{}, fmt.Errorf("scheduler: the entity manager %T cannot write remote rows", lua.EntityManager)
	}
	if lua.Meta == nil {
		return remotesync.Deps{}, errors.New("scheduler: the workspace provides no schema for remote pulls")
	}
	credential, err := m.Remote.ReadCredential(getenv, globalSecret(lua.Host))
	if err != nil {
		return remotesync.Deps{}, fmt.Errorf("remote %q: %w", m.Name, err)
	}
	src, err := remotesync.Dial(m.Remote, credential, transport)
	if err != nil {
		return remotesync.Deps{}, err
	}
	return remotesync.Deps{
		Source: src, Meta: lua.Meta, Writer: writer, Store: raw.Store(), State: ws.State(),
		Logger: logger, RunID: runID,
	}, nil
}

// globalSecret reads one secret from the global section of host's secrets,
// for a remote_credential. name. A missing secret reads as empty, which
// [remotesync.Remote.ReadCredential] reports as no credential.
//
// Nil: host is accepted; the returned reader then fails.
func globalSecret(host hostSecrets) func(string) (string, error) {
	return func(name string) (string, error) {
		if host == nil {
			return "", errors.New("the workspace provides no secrets")
		}
		all, err := host.Secrets("")
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return "", err
		}
		return all[name], nil
	}
}

// hostSecrets reads the secrets visible to a script; "" is the global
// section.
type hostSecrets interface {
	Secrets(script string) (map[string]string, error)
}

// isNilPointer reports a non-nil interface holding a nil pointer, such as
// the workspace's manager when the workspace has none.
func isNilPointer(v any) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}
