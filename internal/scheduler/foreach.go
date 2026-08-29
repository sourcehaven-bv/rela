package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
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

func childIdentity(task, occurrence, subject string) string {
	return task + "/" + occurrence + "/" + subject
}

func (s *Scheduler) taskNamed(name string) (TaskConfig, bool) {
	for _, task := range s.config.Tasks {
		if task.Name == name {
			return task, true
		}
	}
	return TaskConfig{}, false
}

func (s *Scheduler) runExpandJob(ctx context.Context, job jobs.Job) (err error) {
	name, _ := job.Payload[payloadTaskName].(string)
	token, _ := job.Payload[payloadRunToken].(string)
	defer func() { s.reportInFlight(name, token, err) }()

	occurrence, _ := job.Payload[payloadOccurrence].(string)
	if name == "" || occurrence == "" {
		return errors.New("scheduler: expansion payload requires task and occurrence")
	}
	task, ok := s.taskNamed(name)
	if !ok || task.ForEach == nil {
		return fmt.Errorf("scheduler: expansion task %q is no longer configured for_each", name)
	}
	provider, ok := s.ws.(ForEachProvider)
	if !ok {
		return errors.New("scheduler: workspace provides no for_each graph capabilities")
	}

	selectionCtx := stampTaskAuditContext(ctx, name, "")
	ids, dropped, err := provider.ScheduledForEachEntities(
		selectionCtx, task.ForEach.EntityType, task.ForEach.Where, task.ForEach.EffectiveLimit())
	if err != nil {
		return fmt.Errorf("scheduler: expand task %q: %w", name, err)
	}
	slices.Sort(ids)
	for _, subject := range ids {
		identity := childIdentity(name, occurrence, subject)
		err = s.queue.Enqueue(selectionCtx, jobs.Job{
			Kind: ChildKind,
			Payload: map[string]any{
				payloadTaskName:   name,
				payloadOccurrence: occurrence,
				payloadSubject:    subject,
			},
			Retry:          jobs.RetryBounded,
			IdempotencyKey: identity,
		})
		if err != nil {
			if errors.Is(err, jobs.ErrDuplicateJob) {
				continue
			}
			return fmt.Errorf("scheduler: enqueue child %q: %w", identity, err)
		}
	}
	if dropped > 0 {
		s.logger.Warn("for_each selection exceeded limit",
			"task", name, "limit", task.ForEach.EffectiveLimit(), "dropped", dropped)
	}
	s.logger.Info("for_each expansion complete", "task", name, "children", len(ids), "dropped", dropped)
	return nil
}

func (s *Scheduler) runChildJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	occurrence, _ := job.Payload[payloadOccurrence].(string)
	subject, _ := job.Payload[payloadSubject].(string)
	if name == "" || occurrence == "" || subject == "" {
		return errors.New("scheduler: child payload requires task, occurrence, and subject")
	}
	task, ok := s.taskNamed(name)
	if !ok || task.ForEach == nil {
		slog.WarnContext(ctx, "scheduled child declaration removed; dropping",
			"task", name, "subject", subject)
		return nil
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
