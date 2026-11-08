package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/peersync"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// PeerKind is the job kind for one scheduled pull from a sync peer.
var PeerKind = jobs.NewKind("scheduler", "run-peer-pull")

// PeerTaskPrefix starts the task name of every peer pull. The scheduled pull
// and `rela peer pull` share the task's run state, which is what keeps two
// pulls of one peer from running at once.
const PeerTaskPrefix = "peer:"

// minPeerInterval is the shortest interval a peer may be pulled at. A pull
// diffs every mapped type, so a short interval mostly loads the source.
const minPeerInterval = 5 * time.Minute

// peerRunTimeout bounds one pull. It stays under the queue's per-handler cap
// (15m) and the scheduler's running lease, so the run ends, and records its
// outcome, before either reclaims it.
const peerRunTimeout = 14 * time.Minute

// ErrPeers marks an invalid peers.yaml in [LoadConfig]'s error, for a caller
// that still runs the other tasks it returned.
var ErrPeers = errors.New("peers.yaml is invalid")

// ErrPeerBusy reports that a pull of the peer is already running.
var ErrPeerBusy = errors.New("a pull of this peer is already running")

func wrapPeers(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrPeers, err)
}

// LoadPeers reads and checks peers.yaml. A missing file is an empty config.
//
// Nil: rejected for meta when the file exists; a mapping cannot be checked
// without the schema.
func LoadPeers(ctx context.Context, loader config.Loader, meta *metamodel.Metamodel) (*peersync.Config, error) {
	data, err := loadOptional(ctx, loader, peersync.FileName)
	if err != nil || data == nil {
		return &peersync.Config{}, err
	}
	return peersync.Parse(data, meta)
}

// AddPeers appends one task per peer with an `every` schedule to cfg. A peer
// without one is pulled only by `rela peer pull`. Tasks added before an error
// are kept out: cfg is changed only on success.
func AddPeers(cfg *Config, peers *peersync.Config) error {
	names := make(map[string]bool, len(cfg.Tasks))
	for _, t := range cfg.Tasks {
		names[t.Name] = true
	}
	var tasks []TaskConfig
	var errs []error
	for _, name := range peers.Names() {
		m := peers.Mappings[name]
		if m == nil || m.Peer.Every == "" {
			continue
		}
		taskName := PeerTaskPrefix + name
		every, err := parseSchedule(m.Peer.Every)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("peers: %q every: %w", name, err))
			continue
		case every.kind == intervalKind && every.interval < minPeerInterval:
			errs = append(errs, fmt.Errorf("peers: %q every %s is shorter than the %s minimum",
				name, m.Peer.Every, minPeerInterval))
			continue
		case names[taskName]:
			errs = append(errs, fmt.Errorf("peers: %q: task name %q is taken", name, taskName))
			continue
		}
		tasks = append(tasks, TaskConfig{Name: taskName, Every: every, Peer: m})
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	cfg.Tasks = append(cfg.Tasks, tasks...)
	return nil
}

// peerJob is the job for one scheduled pull. The payload names the task
// only: the handler reads the mapping from its own config.
func peerJob(task TaskConfig, runID string) jobs.Job {
	return jobs.Job{
		Kind:           PeerKind,
		Payload:        map[string]any{payloadTaskName: task.Name, payloadRunID: runID},
		Retry:          jobs.RetryNever,
		IdempotencyKey: runID,
	}
}

// runPeerJob pulls one peer and records the run's outcome.
func (s *Scheduler) runPeerJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	run, ok := s.beginRun(ctx, job, runID, name)
	if !ok {
		return nil
	}
	err := s.pullPeerTask(ctx, name, runID)
	s.finish(ctx, run, err)
	return err
}

func (s *Scheduler) pullPeerTask(ctx context.Context, taskName, runID string) error {
	task, ok := s.taskNamed(taskName)
	if !ok || task.Peer == nil {
		return fmt.Errorf("scheduler: peer task %q is not configured", taskName)
	}
	_, err := pull(ctx, s.ws, s.bind, task.Peer, runID, peersync.Options{}, s.peerTransport, s.logger)
	return err
}

// PullPeer runs one pull of peer name now, for `rela peer pull`. It takes the
// peer's run lock in the scheduler state, so it never runs alongside a
// scheduled pull of the same peer; that one returns [ErrPeerBusy]. A dry run
// takes no lock and leaves the task's schedule alone, because it writes
// nothing.
//
// Nil: rejected for ws and bind; a pull cannot write without the workspace
// or act without an identity.
func PullPeer(
	ctx context.Context, ws WorkspaceProvider, bind IdentityBinder, name string, o peersync.Options,
	logger *slog.Logger,
) (peersync.Report, error) {
	switch {
	case ws == nil:
		return peersync.Report{}, errors.New("scheduler: a workspace is required")
	case bind == nil:
		return peersync.Report{}, errors.New("scheduler: an identity binder is required")
	case logger == nil:
		return peersync.Report{}, errors.New("scheduler: a logger is required")
	}
	var meta *metamodel.Metamodel
	if mp, ok := ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	peers, err := LoadPeers(ctx, ws.Config(), meta)
	if err != nil {
		return peersync.Report{}, wrapPeers(err)
	}
	m := peers.Mappings[name]
	if m == nil {
		return peersync.Report{}, fmt.Errorf("no peer %q in %s", name, peersync.FileName)
	}
	runID := uuid.NewString()
	ctx = audit.WithTriggeredBy(ctx, manualTrigger(ctx, name))
	if o.DryRun {
		return pull(ctx, ws, bind, m, runID, o, nil, logger)
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return peersync.Report{}, errors.New("scheduler: the workspace has no scheduler state")
	}
	run, err := startManualRun(ctx, runs, PeerTaskPrefix+name, runID)
	if err != nil {
		return peersync.Report{}, err
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

// manualTrigger is the triggered-by label of a pull started by hand. It
// names the caller, because the writes are attributed to the peer's
// principal and would otherwise not say who started them.
func manualTrigger(ctx context.Context, name string) string {
	return PeerTaskPrefix + name + " by " + principal.From(ctx).User
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
		return schedulerstate.Run{}, fmt.Errorf("%s: %w", task, ErrPeerBusy)
	case err != nil:
		return schedulerstate.Run{}, fmt.Errorf("scheduler: create run: %w", err)
	}
	if run, err = runs.StartRun(ctx, id, "cli:"+nodeID(), now, now.Add(runningLease)); err != nil {
		return schedulerstate.Run{}, fmt.Errorf("scheduler: start run: %w", err)
	}
	return run, nil
}

// pull runs one pull of m as the peer's local writer, `system:peer:<name>`.
// The principal is stamped and bound here, the pull's request edge, so the
// manager authorizes and audits each write against it.
func pull(
	ctx context.Context, ws WorkspaceProvider, bind IdentityBinder, m *peersync.Mapping, runID string,
	o peersync.Options, transport http.RoundTripper, logger *slog.Logger,
) (peersync.Report, error) {
	ctx, cancel := context.WithTimeout(ctx, peerRunTimeout)
	defer cancel()
	ctx = principal.With(ctx, principal.Principal{
		User: principal.PeerPullUserPrefix + m.Name, Tool: principal.ToolPeerPull,
	})
	if audit.TriggeredByFrom(ctx) == "" {
		ctx = audit.WithTriggeredBy(ctx, PeerTaskPrefix+m.Name)
	}
	ctx, err := bind(ctx)
	if err != nil {
		return peersync.Report{}, fmt.Errorf("scheduler: resolve identity for peer %q: %w", m.Name, err)
	}
	deps, err := peerDeps(ws, m, runID, transport, logger)
	if err != nil {
		return peersync.Report{}, err
	}
	// The mapping was checked when its file loaded; compile it again against
	// the schema this pull writes with, so the run reads one schema.
	if m, err = peersync.Compile(m.Name, m.Peer, deps.Meta); err != nil {
		return peersync.Report{}, wrapPeers(err)
	}
	report, err := peersync.Run(ctx, m, deps, o)
	logger.Info("peer pulled",
		"peer", m.Name, "run_id", runID, "dry_run", o.DryRun, "refused", report.Refused, "gone", report.Gone,
		"entities", report.Entities, "relations", report.Relations, "issues", len(report.Issues))
	warnPull(logger, m.Name, runID, report)
	return report, err
}

// maxWarnedIssues caps the issues one pull logs; the report holds them all.
const maxWarnedIssues = 20

// warnPull logs at Warn what an operator must look at after a pull: refused
// guards, skipped relation types, and rows that were not applied.
func warnPull(logger *slog.Logger, name, runID string, rep peersync.Report) {
	if rep.Refused != "" {
		logger.Warn("peer pull refused", "peer", name, "run_id", runID, "reason", rep.Refused, "gone", rep.Gone)
	}
	if len(rep.Skipped) > 0 {
		logger.Warn("peer pull skipped relation types", "peer", name, "run_id", runID, "types", rep.Skipped)
	}
	for i, is := range rep.Issues {
		if i == maxWarnedIssues {
			logger.Warn("peer pull has more issues", "peer", name, "run_id", runID, "more", len(rep.Issues)-i)
			break
		}
		logger.Warn("peer pull row not applied",
			"peer", name, "run_id", runID, "kind", is.Kind, "address", is.Address, "local", is.Local, "reason", is.Reason, "fields", is.Fields)
	}
}

// peerDeps assembles the collaborators of one pull: the manager every write
// goes through, the raw store for the write-prep reads, the state store for
// the ledger, and the source dialed with the peer's credential.
//
// The writer is the field-gated manager of the scheduled Lua deps, which
// also updates relations. It is asserted rather than declared on the
// workspace because the workspace type is at its plimsoll method cap; the
// scheduler's other optional capabilities are asserted the same way.
func peerDeps(
	ws WorkspaceProvider, m *peersync.Mapping, runID string, transport http.RoundTripper, logger *slog.Logger,
) (peersync.Deps, error) {
	raw, ok := ws.(rawStoreProvider)
	if !ok || raw.Store() == nil {
		return peersync.Deps{}, errors.New("scheduler: the workspace provides no store for peer pulls")
	}
	lua := ws.ScheduledLuaWriteDeps()
	writer, ok := lua.EntityManager.(peersync.Writer)
	if !ok || isNilPointer(writer) {
		return peersync.Deps{}, fmt.Errorf("scheduler: the entity manager %T cannot write peer rows", lua.EntityManager)
	}
	if lua.Meta == nil {
		return peersync.Deps{}, errors.New("scheduler: the workspace provides no schema for peer pulls")
	}
	if lua.Host == nil {
		return peersync.Deps{}, errors.New("scheduler: the workspace provides no host config for secrets")
	}
	all, err := lua.Host.Secrets("")
	if err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return peersync.Deps{}, fmt.Errorf("peer %q: read credential %s: %w", m.Name, m.Peer.Credential, err)
	}
	credential := strings.TrimSpace(all[m.Peer.Credential])
	if credential == "" {
		return peersync.Deps{}, fmt.Errorf("peer %q: no secret %q in the global section", m.Name, m.Peer.Credential)
	}
	src, err := peersync.Dial(m.Peer, credential, transport)
	if err != nil {
		return peersync.Deps{}, err
	}
	return peersync.Deps{
		Source: src, Meta: lua.Meta, Writer: writer, Store: raw.Store(), State: ws.State(),
		Logger: logger, RunID: runID,
	}, nil
}

// isNilPointer reports a non-nil interface holding a nil pointer, such as
// the workspace's manager when the workspace has none.
func isNilPointer(v any) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}
