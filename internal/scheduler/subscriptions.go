package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/ingest"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/subscription"
)

// SubscriptionKind is the job kind for one poll of a feed subscription.
var SubscriptionKind = jobs.NewKind("scheduler", "run-subscription")

// SubscriptionTaskPrefix starts the task name of every subscription, so a
// subscription cannot share run state with a schedules.yaml task of the same
// name.
const SubscriptionTaskPrefix = "subscription:"

// minSubscriptionInterval is the shortest interval a subscription may poll at.
// Feed publishers rate-limit aggressive pollers, and a feed rarely changes
// faster.
const minSubscriptionInterval = 15 * time.Minute

// subscriptionMaxAttempts bounds the conflict retries of one item's write.
const subscriptionMaxAttempts = 3

// ErrSubscriptions marks an invalid subscriptions.yaml in [LoadConfig]'s error,
// for a caller that still runs the schedules.yaml tasks it returned.
var ErrSubscriptions = errors.New("subscriptions.yaml is invalid")

// LoadConfig reads schedules.yaml and subscriptions.yaml and returns their
// tasks as one config. A missing file contributes no tasks.
//
// An invalid schedules.yaml is an error with a nil config. An invalid
// subscriptions.yaml returns the schedules.yaml tasks AND an error wrapping
// [ErrSubscriptions], so one broken feed definition does not stop unrelated
// scheduled scripts.
//
// Every entry point that runs the scheduler, and the validation that checks
// a project, goes through here; two loaders drifting apart is how a task
// that validates fails to start, or the reverse.
//
// Nil: accepted for meta only without a subscriptions.yaml; a subscription's
// mapping cannot be checked without the schema.
func LoadConfig(ctx context.Context, loader config.Loader, meta *metamodel.Metamodel) (*Config, error) {
	cfg := &Config{}
	data, err := loadOptional(ctx, loader, ConfigFile)
	if err != nil {
		return nil, err
	}
	if data != nil {
		if cfg, err = ParseConfig(data); err != nil {
			return nil, err
		}
	}
	data, err = loadOptional(ctx, loader, subscription.FileName)
	if err != nil || data == nil {
		return cfg, wrapSubscriptions(err)
	}
	if meta == nil {
		return cfg, wrapSubscriptions(errors.New("no metamodel to check them against"))
	}
	subs, err := subscription.Parse(data, meta)
	if err != nil {
		return cfg, wrapSubscriptions(err)
	}
	if err := AddSubscriptions(cfg, subs); err != nil {
		return cfg, wrapSubscriptions(err)
	}
	return cfg, nil
}

func wrapSubscriptions(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrSubscriptions, err)
}

// loadOptional reads name, returning nil data when the file is absent.
func loadOptional(ctx context.Context, loader config.Loader, name string) ([]byte, error) {
	data, err := loader.Load(ctx, name)
	switch {
	case errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

// AddSubscriptions appends one task per subscription to cfg. Tasks added
// before an error are kept out: cfg is changed only on success.
func AddSubscriptions(cfg *Config, subs *subscription.Config) error {
	names := make(map[string]bool, len(cfg.Tasks))
	for _, t := range cfg.Tasks {
		names[t.Name] = true
	}
	tasks := make([]TaskConfig, 0, len(subs.Subscriptions))
	var errs []error
	for _, name := range subs.Names() {
		sub := subs.Subscriptions[name]
		taskName := SubscriptionTaskPrefix + name
		every, err := parseSchedule(sub.Every)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("subscriptions: %q every: %w", name, err))
			continue
		case every.kind == intervalKind && every.interval < minSubscriptionInterval:
			errs = append(errs, fmt.Errorf("subscriptions: %q every %s is shorter than the %s minimum",
				name, sub.Every, minSubscriptionInterval))
			continue
		case names[taskName]:
			errs = append(errs, fmt.Errorf("subscriptions: %q: task name %q is taken in %s",
				name, taskName, ConfigFile))
			continue
		}
		tasks = append(tasks, TaskConfig{Name: taskName, Every: every, Subscription: &sub})
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	cfg.Tasks = append(cfg.Tasks, tasks...)
	return nil
}

// subscriptionJob is the job for one poll. The payload names the task only:
// the handler reads the subscription from its own config, which every process
// sharing the queue loads from the same project.
func subscriptionJob(task TaskConfig, runID string) jobs.Job {
	return jobs.Job{
		Kind:           SubscriptionKind,
		Payload:        map[string]any{payloadTaskName: task.Name, payloadRunID: runID},
		Retry:          jobs.RetryNever,
		IdempotencyKey: runID,
	}
}

// rawStoreProvider is the capability that gives ingest its write-prep reads.
// Type-asserted rather than added to WorkspaceProvider so the existing test
// doubles keep compiling; a workspace without it cannot run subscriptions.
type rawStoreProvider interface {
	Store() store.Store
}

// runSubscriptionJob polls one subscription and records the run's outcome.
func (s *Scheduler) runSubscriptionJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	run, ok := s.beginRun(ctx, job, runID, name)
	if !ok {
		return nil
	}
	err := s.pollSubscription(ctx, name)
	s.finish(ctx, run, err)
	return err
}

func (s *Scheduler) pollSubscription(ctx context.Context, taskName string) error {
	task, ok := s.task(taskName)
	if !ok || task.Subscription == nil {
		return fmt.Errorf("scheduler: subscription task %q is not configured", taskName)
	}
	name := taskName[len(SubscriptionTaskPrefix):]
	ctx = stampContext(ctx, principal.ReservedPrefix+"subscription:"+name, "subscription:"+name)
	if s.subscriptionRunner != nil {
		return s.subscriptionRunner(ctx, name, *task.Subscription)
	}
	deps, err := s.subscriptionDeps(name)
	if err != nil {
		return err
	}
	report, err := subscription.Run(ctx, name, *task.Subscription, deps)
	s.logger.Info("subscription polled",
		"subscription", name,
		"not_modified", report.NotModified,
		"items", report.Items,
		"dropped", report.Dropped,
		"created", report.Created,
		"updated", report.Updated,
		"unchanged", report.Unchanged,
		"skipped", report.Skipped,
		"rejected", report.Rejected,
		"conflicted", report.Conflicted)
	return err
}

// subscriptionDeps assembles the collaborators of one poll from the
// workspace: the gated reader and field-gated writer of a scheduled task,
// which resolve against the principal on the ctx, plus the raw store for the
// write-prep re-read, the state store and the global secrets.
func (s *Scheduler) subscriptionDeps(name string) (subscription.Deps, error) {
	raw, ok := s.ws.(rawStoreProvider)
	if !ok || raw.Store() == nil {
		return subscription.Deps{}, errors.New("scheduler: the workspace provides no store for subscriptions")
	}
	lua := s.ws.ScheduledLuaWriteDeps()
	if lua.Host == nil {
		return subscription.Deps{}, errors.New("scheduler: the workspace provides no host config for secrets")
	}
	return subscription.Deps{
		Ingest: ingest.Deps{
			Reader:      lua.VisibleReader,
			World:       lua.World,
			Writer:      lua.EntityManager,
			WritePrep:   raw.Store(),
			MaxAttempts: subscriptionMaxAttempts,
			Label:       SubscriptionTaskPrefix + name,
		},
		State:   s.ws.State(),
		Secrets: func() (map[string]string, error) { return lua.Host.Secrets("") },
		Fetcher: &subscription.Fetcher{Transport: s.subscriptionTransport},
		Logger:  s.logger,
	}, nil
}

// task returns the configured task named name.
func (s *Scheduler) task(name string) (TaskConfig, bool) {
	for _, t := range s.config.Tasks {
		if t.Name == name {
			return t, true
		}
	}
	return TaskConfig{}, false
}
