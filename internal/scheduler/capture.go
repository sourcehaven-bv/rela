package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
)

// This file is the capture task (TKT-6OYQO79): a task that computes one
// dashboard card's figure as its run_as identity and appends it as one
// record row, without a script.
//
// The figure is an aggregate stored where every reader of the owner can read
// it, so it is a value computed for one principal and served to others. That
// is a deliberate exception to the rule that aggregates are per principal.
// The task therefore must name its own run_as identity, and the operator must
// give the owner a read rule no wider than what that identity can read (see
// "Capturing a Metric for Trend Charts" in the scheduled-tasks guide).

// CaptureKind is the job kind for one run of a capture task.
var CaptureKind = jobs.NewKind("scheduler", "run-capture")

// defaultCaptureScale is the decimal places a figure is stored with when the
// record field declares no scale.
const defaultCaptureScale = 2

// CaptureConfig names the card a capture task reads and the record row it
// writes.
type CaptureConfig struct {
	// Dashboard is a name under `dashboards:` in data-entry.yaml, or
	// "default" for the `dashboard:` block. Empty means default.
	Dashboard string `yaml:"dashboard,omitempty"`
	// Card is the title of the card, unique on that dashboard.
	Card string `yaml:"card"`
	// Record is a record type declared under `records:` in the schema.
	Record string `yaml:"record"`
	// Owner is the id of the entity the row attaches to.
	Owner string `yaml:"owner"`
	// Field is the integer or decimal field of the record type that holds the
	// figure.
	Field string `yaml:"field"`
}

// CardEvaluator computes a dashboard card's figure as the principal bound on
// ctx. Defined here at the consumer; rela-server and rela-desktop supply the
// data-entry app's implementation, because card evaluation lives there.
//
// The figure is exact: a count, a percentage of matched rows, or the folded
// value of a sum, avg, min or max card. An error means nothing is stored.
type CardEvaluator interface {
	CardFigure(ctx context.Context, dashboard, card string) (*big.Rat, error)
}

// Option configures a [Scheduler] at construction.
type Option func(*Scheduler)

// WithCards gives the scheduler the card evaluator capture tasks need.
//
// Nil: accepted, and means capture is not supported in this process: the
// capture job kind is not registered, so the queue hands capture runs to a
// process that has an evaluator, and capture tasks are left out of the
// schedule with a log line.
func WithCards(cards CardEvaluator) Option {
	return func(s *Scheduler) { s.cards = cards }
}

// CaptureTasks returns the names of the capture tasks in c.
func (c *Config) CaptureTasks() []string {
	var out []string
	for _, t := range c.Tasks {
		if t.Capture != nil {
			out = append(out, t.Name)
		}
	}
	return out
}

// validateCaptureTask checks the schema-independent part of a capture task.
func validateCaptureTask(t TaskConfig) error {
	c := t.Capture
	switch {
	case t.ForEach != nil:
		return fmt.Errorf("task %q: capture and for_each are mutually exclusive", t.Name)
	case t.RunAs == "":
		// The stored figure is readable by every reader of the owner, so the
		// task names the identity whose view it stores; the shared
		// system:scheduler identity usually reads far more than any reader.
		return fmt.Errorf("task %q: capture requires run_as", t.Name)
	case t.RunAs == principal.UserScheduler:
		return fmt.Errorf("task %q: capture needs a run_as of its own, not the shared %s", t.Name,
			principal.UserScheduler)
	case t.Capabilities.Any():
		return fmt.Errorf("task %q: capture runs no script, so it takes no capabilities", t.Name)
	case c.Card == "":
		return fmt.Errorf("task %q: capture.card is required", t.Name)
	case c.Record == "":
		return fmt.Errorf("task %q: capture.record is required", t.Name)
	case c.Owner == "":
		return fmt.Errorf("task %q: capture.owner is required", t.Name)
	case c.Field == "":
		return fmt.Errorf("task %q: capture.field is required", t.Name)
	}
	return nil
}

// validateCaptures checks every capture task's record type and field against
// meta.
func (c *Config) validateCaptures(meta *metamodel.Metamodel) error {
	for _, t := range c.Tasks {
		if t.Capture == nil {
			continue
		}
		if _, err := captureField(meta, t.Capture); err != nil {
			return fmt.Errorf("task %q: %w", t.Name, err)
		}
	}
	return nil
}

// captureField returns the field definition a capture writes. The record type
// must exist and must not be a ledger, the field must be an integer or a
// decimal, and no other field may be required, since the row carries only
// the figure.
func captureField(meta *metamodel.Metamodel, c *CaptureConfig) (metamodel.PropertyDef, error) {
	if meta == nil {
		return metamodel.PropertyDef{}, errors.New("capture: no schema to check the record type against")
	}
	def, ok := meta.Records[c.Record]
	if !ok {
		return metamodel.PropertyDef{}, fmt.Errorf("capture.record %q is not a record type", c.Record)
	}
	if def.Ledger != nil {
		return metamodel.PropertyDef{}, fmt.Errorf("capture.record %q is a ledger; a ledger takes balanced "+
			"transactions, not single figures", c.Record)
	}
	field, ok := def.Fields[c.Field]
	if !ok {
		return metamodel.PropertyDef{}, fmt.Errorf("capture.field %q is not a field of record type %q", c.Field, c.Record)
	}
	if field.Type != metamodel.PropertyTypeInteger && field.Type != metamodel.PropertyTypeDecimal {
		return metamodel.PropertyDef{}, fmt.Errorf("capture.field %q must be an integer or decimal field", c.Field)
	}
	for _, name := range def.FieldNames() {
		if name != c.Field && def.Fields[name].Required {
			return metamodel.PropertyDef{}, fmt.Errorf("record type %q requires field %q, which a capture "+
				"cannot fill", c.Record, name)
		}
	}
	return field, nil
}

// fieldValue rounds figure once to field: to a whole number for an integer
// field, and to the field's scale (or [defaultCaptureScale]) for a decimal.
// Halves round away from zero.
func fieldValue(figure *big.Rat, field metamodel.PropertyDef) (any, error) {
	if field.Type == metamodel.PropertyTypeInteger {
		n, err := strconv.ParseInt(figure.FloatString(0), 10, 64)
		if err != nil {
			return nil, errors.New("capture: the figure does not fit an integer field")
		}
		return n, nil
	}
	scale := defaultCaptureScale
	if field.Scale != nil {
		scale = *field.Scale
	}
	return figure.FloatString(scale), nil
}

// supportedTasks returns cfg without the tasks this scheduler cannot run,
// and their names. Without a card evaluator that is every capture task.
func (s *Scheduler) supportedTasks(cfg *Config) (supported *Config, skipped []string) {
	if s.cards != nil {
		return cfg, nil
	}
	skipped = cfg.CaptureTasks()
	if len(skipped) == 0 {
		return cfg, nil
	}
	tasks := make([]TaskConfig, 0, len(cfg.Tasks)-len(skipped))
	for _, t := range cfg.Tasks {
		if t.Capture == nil {
			tasks = append(tasks, t)
		}
	}
	return &Config{Tasks: tasks}, skipped
}

// noteSkippedCaptures logs the capture tasks left out of the schedule.
func (s *Scheduler) noteSkippedCaptures(skipped []string) {
	if len(skipped) > 0 {
		s.logger.Warn("capture tasks not scheduled here: this process computes no dashboard cards; "+
			"rela-server or rela-desktop runs them", "tasks", skipped)
	}
}

// captureJob is the job for one capture run. Like a subscription poll, the
// payload names the task only: the handler reads the capture from its own
// config.
func captureJob(task TaskConfig, runID string) jobs.Job {
	return jobs.Job{
		Kind:           CaptureKind,
		Payload:        map[string]any{payloadTaskName: task.Name, payloadRunID: runID},
		Retry:          jobs.RetryNever,
		IdempotencyKey: runID,
	}
}

// runCaptureJob runs one capture and records the run's outcome.
func (s *Scheduler) runCaptureJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	run, ok := s.beginRun(ctx, job, runID, name)
	if !ok {
		return nil
	}
	err := s.capture(ctx, name)
	s.finish(ctx, run, err)
	return err
}

// capture computes the task's card figure as its run_as identity and appends
// it as one row. Every failure stores nothing.
//
// The row's time is the run's read time, the instant the figure describes.
// If the append succeeds but the run's outcome is lost, the retry appends a
// second row for the same figure; the guide says so.
func (s *Scheduler) capture(ctx context.Context, taskName string) error {
	task, ok := s.taskNamed(taskName)
	if !ok || task.Capture == nil {
		return fmt.Errorf("scheduler: capture task %q is not configured", taskName)
	}
	if s.cards == nil {
		return fmt.Errorf("scheduler: capture task %q: this process computes no dashboard cards", taskName)
	}
	c := task.Capture
	var meta *metamodel.Metamodel
	if mp, ok := s.ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	field, err := captureField(meta, c)
	if err != nil {
		return fmt.Errorf("scheduler: capture task %q: %w", taskName, err)
	}
	appender := s.ws.ScheduledLuaWriteDeps().Records
	if appender == nil {
		return fmt.Errorf("scheduler: capture task %q: no record service", taskName)
	}
	taskCtx, err := s.taskContext(ctx, task.Name, task.RunAs)
	if err != nil {
		return err
	}
	figure, err := s.cards.CardFigure(taskCtx, c.Dashboard, c.Card)
	if err != nil {
		return fmt.Errorf("scheduler: capture task %q: card %q: %w", taskName, c.Card, err)
	}
	value, err := fieldValue(figure, field)
	if err != nil {
		return fmt.Errorf("scheduler: capture task %q: %w", taskName, err)
	}
	at, _ := readtime.From(taskCtx)
	row := lua.RecordRow{Owner: c.Owner, TS: at, Fields: map[string]any{c.Field: value}}
	if _, err := appender.AppendRecords(taskCtx, c.Record, []lua.RecordRow{row}); err != nil {
		return fmt.Errorf("scheduler: capture task %q: append to %s: %w", taskName, c.Owner, err)
	}
	// The figure is data run_as reads, so it stays out of the log.
	s.logger.Info("metric captured", "task", taskName, "card", c.Card, "owner", c.Owner, "record", c.Record)
	return nil
}
