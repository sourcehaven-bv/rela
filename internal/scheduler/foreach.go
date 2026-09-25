package scheduler

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

// ForEachProvider is the graph-facing capability needed by fan-out. The
// application wiring owns ACL/store mechanics; scheduler owns only job shape.
type ForEachProvider interface {
	ScheduledForEachEntities(
		ctx context.Context, entityType string, where []string, limit int,
	) (ids []string, dropped int, err error)
	ScheduledForEachPrincipal(ctx context.Context, entityID string) (string, error)
}

// TemplateRunner executes a declarative action under the recipient context
// installed by the child handler. It is optional for script-only workspaces.
type TemplateRunner interface {
	RunScheduledTemplate(ctx context.Context, template, subjectEntityID string) error
}

var (
	// ExpandKind expands one scheduled occurrence into scoped child jobs.
	ExpandKind = jobs.NewKind("scheduler", "expand-task")
	// ChildKind executes one selected subject independently of its peers.
	ChildKind = jobs.NewKind("scheduler", "run-task-subject")
)

const (
	payloadOccurrence = "occurrence"
	payloadSubject    = "subject"
)

func (s *Scheduler) taskNamed(name string) (TaskConfig, bool) {
	for _, task := range s.config.Tasks {
		if task.Name == name {
			return task, true
		}
	}
	return TaskConfig{}, false
}

// runExpandJob selects a for_each run's subjects and enqueues one child job
// per subject.
//
// The run does NOT finish here. It finishes when its last child settles, and
// fails if any child failed (BUG-1YMHIS): "expansion succeeded" is not "the
// task ran". Subjects an earlier run of the same occurrence already delivered
// are left out, so retrying a partly failed fan-out repeats only the failures.
func (s *Scheduler) runExpandJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	occurrence, _ := job.Payload[payloadOccurrence].(string)
	if runID == "" {
		s.logger.Warn("expansion job without a run id dropped", "task", name)
		return nil
	}
	run, ok := s.beginRun(ctx, job, runID, name)
	if !ok {
		return nil
	}

	children, err := s.expand(ctx, run, occurrence)
	if err != nil {
		s.finish(ctx, run, err)
		return err
	}
	if children == 0 {
		// Nothing to wait for, so nothing else would ever end the run.
		s.finish(ctx, run, nil)
	}
	return nil
}

// expand does the work of runExpandJob and returns how many children it
// expects to settle.
func (s *Scheduler) expand(ctx context.Context, run schedulerstate.Run, occurrence string) (int, error) {
	name := run.Task
	if occurrence == "" {
		return 0, errors.New("scheduler: expansion payload requires an occurrence")
	}
	task, ok := s.taskNamed(name)
	if !ok || task.ForEach == nil {
		return 0, fmt.Errorf("scheduler: expansion task %q is no longer configured for_each", name)
	}
	provider, ok := s.ws.(ForEachProvider)
	if !ok {
		return 0, errors.New("scheduler: workspace provides no for_each graph capabilities")
	}

	selectionCtx := stampTaskAuditContext(ctx, name, "")
	ids, dropped, err := provider.ScheduledForEachEntities(
		selectionCtx, task.ForEach.EntityType, task.ForEach.Where, task.ForEach.EffectiveLimit())
	if err != nil {
		return 0, fmt.Errorf("scheduler: expand task %q: %w", name, err)
	}
	if dropped > 0 {
		s.logger.Warn("for_each selection exceeded limit",
			"task", name, "limit", task.ForEach.EffectiveLimit(), "dropped", dropped)
	}

	delivered, err := s.runs.SucceededSubjects(ctx, name, occurrence)
	if err != nil {
		return 0, fmt.Errorf("scheduler: read delivered subjects of %q: %w", name, err)
	}
	slices.Sort(ids)
	pending := slices.DeleteFunc(slices.Compact(ids), func(id string) bool {
		_, found := slices.BinarySearch(delivered, id)
		return found
	})

	// Recorded before any child is enqueued, so no child can settle against a
	// run that does not yet know about it.
	if err := s.runs.ExpectChildren(ctx, run.ID, pending); err != nil {
		return 0, fmt.Errorf("scheduler: record subjects of %q: %w", name, err)
	}
	for _, subject := range pending {
		err := s.queue.Enqueue(selectionCtx, jobs.Job{
			Kind: ChildKind,
			Payload: map[string]any{
				payloadTaskName:   name,
				payloadRunID:      run.ID,
				payloadOccurrence: occurrence,
				payloadSubject:    subject,
			},
			Retry:          jobs.RetryBounded,
			IdempotencyKey: run.ID + "/" + subject,
		})
		if err != nil {
			// The child will never run, so settle it here as failed; the run
			// then ends once its enqueued peers settle.
			s.settleChild(ctx, run.ID, name, subject, fmt.Errorf("enqueue: %w", err))
		}
	}
	s.logger.Info("for_each expansion complete",
		"task", name, "run_id", run.ID, "occurrence", occurrence,
		"children", len(pending), "already_delivered", len(ids)-len(pending), "dropped", dropped)
	return len(pending), nil
}

// runChildJob runs one for_each subject and settles it on its run.
//
// A failing attempt that has retries left returns its error, and the queue
// retries it; the subject settles only on success or on its final attempt, so
// the run's outcome reflects what each subject finally did.
func (s *Scheduler) runChildJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	subject, _ := job.Payload[payloadSubject].(string)
	if runID == "" || subject == "" {
		s.logger.Warn("child job without a run id or subject dropped", "task", name, "subject", subject)
		return nil
	}

	// Progress: the run is alive as long as its children keep starting.
	if err := s.runs.ExtendLease(ctx, runID, s.now().Add(runningLease)); err != nil {
		s.logger.Warn("could not extend run lease", "task", name, "run_id", runID, "error", err)
	}

	err := s.runChild(ctx, name, subject)
	if err != nil && !job.FinalAttempt() {
		s.logger.Warn("for_each subject failed, will retry",
			"task", name, "run_id", runID, "subject", subject, "attempt", job.Attempt, "error", err)
		return err
	}
	s.settleChild(ctx, runID, name, subject, err)
	return err
}

// runChild executes one subject of task.
func (s *Scheduler) runChild(ctx context.Context, name, subject string) error {
	task, ok := s.taskNamed(name)
	if !ok || task.ForEach == nil {
		return fmt.Errorf("scheduler: task %q is no longer configured for_each", name)
	}
	provider, ok := s.ws.(ForEachProvider)
	if !ok {
		return errors.New("scheduler: workspace provides no for_each graph capabilities")
	}
	user, err := provider.ScheduledForEachPrincipal(ctx, subject)
	if err != nil {
		return fmt.Errorf("scheduler: resolve subject %q: %w", subject, err)
	}
	if user == "" {
		// Nothing to deliver to, which is not a failure of the task.
		s.logger.Warn("for_each subject has no principal; skipping",
			"task", name, "subject", subject)
		return nil
	}
	task.RunAs = user
	childCtx := stampTaskAuditContext(ctx, name, user)
	if task.Template != "" {
		runner, ok := s.ws.(TemplateRunner)
		if !ok {
			return errors.New("scheduler: workspace provides no template runner")
		}
		if err := runner.RunScheduledTemplate(childCtx, task.Template, subject); err != nil {
			return fmt.Errorf("scheduler: run template %q: %w", task.Template, err)
		}
		return nil
	}
	return s.runEngine(childCtx, task)
}

// settleChild records one subject's final outcome, and logs the run's end when
// it was the last.
func (s *Scheduler) settleChild(ctx context.Context, runID, name, subject string, runErr error) {
	out := schedulerstate.Outcome{At: s.now()}
	if runErr != nil {
		out.Error = subject + ": " + runErr.Error()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	settled, done, err := s.runs.SettleChild(ctx, runID, subject, out, retryDelay)
	switch {
	case err != nil:
		s.logger.Error("could not settle for_each subject",
			"task", name, "run_id", runID, "subject", subject, "outcome_error", out.Error, "error", err)
		return
	case !settled:
		s.logger.Warn("duplicate for_each result skipped, subject already settled",
			"task", name, "run_id", runID, "subject", subject)
		return
	}
	if runErr != nil {
		s.logger.Warn("for_each subject failed",
			"task", name, "run_id", runID, "subject", subject, "error", runErr)
	}
	if done != nil {
		s.logFinished(*done)
	}
}
