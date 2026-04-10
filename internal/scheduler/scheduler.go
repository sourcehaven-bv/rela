package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// maxLookback is the maximum time to look back for missed scheduled runs.
// Covers schedules up to weekly frequency.
const maxLookback = 8 * 24 * time.Hour

// WorkspaceProvider is the subset of workspace.Workspace the scheduler needs.
type WorkspaceProvider interface {
	Sync() (*model.SyncResult, error)
	Meta() *metamodel.Metamodel
	Paths() *project.Context
	ReadCacheFile(name string) ([]byte, error)
	WriteCacheFile(name string, data []byte) error
}

// Scheduler runs Lua scripts on cron schedules.
type Scheduler struct {
	config *Config
	engine *script.Engine
	ws     WorkspaceProvider
	// wsRaw is the workspace as interface{} for passing to ScriptContext.
	// It must satisfy lua.WorkspaceInterface.
	wsRaw  interface{}
	logger *slog.Logger
	now    func() time.Time // for testing

	stateMu sync.Mutex
	state   *State

	// executeTaskFunc overrides task execution for testing.
	// When nil, doExecuteTask is used.
	executeTaskFunc func(ctx context.Context, task TaskConfig)
}

// New creates a Scheduler. The wsRaw parameter must be the *workspace.Workspace
// value (satisfying lua.WorkspaceInterface) that the script engine expects.
func New(cfg *Config, engine *script.Engine, ws WorkspaceProvider, wsRaw interface{}, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		config: cfg,
		engine: engine,
		ws:     ws,
		wsRaw:  wsRaw,
		logger: logger,
		now:    time.Now,
	}
}

// Run starts the scheduler and blocks until ctx is cancelled.
// It first executes any tasks that missed their scheduled window,
// then starts the cron loop.
func (s *Scheduler) Run(ctx context.Context) error {
	s.loadState()

	// Execute missed runs before starting the cron loop.
	s.executeMissedRuns(ctx)

	if len(s.config.Tasks) == 0 {
		s.logger.Info("no tasks configured, waiting for shutdown")
		<-ctx.Done()
		return nil
	}

	c := cron.New(cron.WithLogger(cron.PrintfLogger(slog.NewLogLogger(s.logger.Handler(), slog.LevelDebug))))

	for _, task := range s.config.Tasks {
		t := task // capture
		skipWrapper := cron.SkipIfStillRunning(cron.VerbosePrintfLogger(slog.NewLogLogger(s.logger.Handler(), slog.LevelWarn)))
		job := cron.FuncJob(func() {
			s.executeTask(ctx, t)
		})
		if _, err := c.AddJob(t.Schedule, skipWrapper(job)); err != nil {
			return fmt.Errorf("task %q: add to cron: %w", t.Name, err)
		}
		s.logger.Info("scheduled task", "name", t.Name, "schedule", t.Schedule, "script", t.Script)
	}

	c.Start()
	s.logger.Info("scheduler started", "tasks", len(s.config.Tasks))

	<-ctx.Done()
	s.logger.Info("shutting down scheduler")

	// Stop accepting new runs and wait for in-flight tasks.
	stopCtx := c.Stop()
	<-stopCtx.Done()

	s.logger.Info("scheduler stopped")
	return nil
}

func (s *Scheduler) executeTask(ctx context.Context, task TaskConfig) {
	if s.executeTaskFunc != nil {
		s.executeTaskFunc(ctx, task)
		return
	}
	s.doExecuteTask(ctx, task)
}

// doExecuteTask runs a single task. The ctx is used for cancellation awareness
// in logging, but note that script.Engine does not propagate context cancellation
// to the Lua VM. The Lua runtime has its own timeout (default 30s) that prevents
// infinite loops.
func (s *Scheduler) doExecuteTask(ctx context.Context, task TaskConfig) {
	if ctx.Err() != nil {
		s.logger.Warn("skipping task, scheduler shutting down", "name", task.Name)
		return
	}

	s.logger.Info("task started", "name", task.Name, "script", task.Script)
	start := s.now()

	// Sync workspace to get fresh graph state.
	if _, err := s.ws.Sync(); err != nil {
		s.logger.Warn("workspace sync failed, executing with stale data", "name", task.Name, "error", err)
	}

	sctx := &schedulerScriptContext{
		ws:          s.wsRaw,
		meta:        s.ws.Meta(),
		projectRoot: s.ws.Paths().Root,
	}

	err := s.engine.ExecuteFile(task.Script, sctx)
	elapsed := s.now().Sub(start)

	if err != nil {
		s.logger.Error("task failed", "name", task.Name, "duration", elapsed, "error", err)
		return
	}

	s.logger.Info("task completed", "name", task.Name, "duration", elapsed)

	// Record successful run.
	s.stateMu.Lock()
	s.state.Tasks[task.Name] = s.now()
	s.saveState()
	s.stateMu.Unlock()
}

func (s *Scheduler) loadState() {
	data, err := s.ws.ReadCacheFile(stateFile)
	if err != nil {
		s.state = newState()
		return
	}
	s.state = parseState(data)
}

func (s *Scheduler) saveState() {
	data, err := s.state.marshal()
	if err != nil {
		s.logger.Error("failed to marshal scheduler state", "error", err)
		return
	}
	if err := s.ws.WriteCacheFile(stateFile, data); err != nil {
		s.logger.Error("failed to save scheduler state", "error", err)
	}
}

func (s *Scheduler) executeMissedRuns(ctx context.Context) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	now := s.now()

	for _, task := range s.config.Tasks {
		if ctx.Err() != nil {
			return
		}

		sched, err := parser.Parse(task.Schedule)
		if err != nil {
			continue // already validated in config
		}

		s.stateMu.Lock()
		lastRun, recorded := s.state.Tasks[task.Name]
		s.stateMu.Unlock()

		if !recorded {
			s.logger.Info("first run, executing immediately", "name", task.Name)
			s.executeTask(ctx, task)
			continue
		}

		prevScheduled := prevScheduleTime(sched, now)
		if !prevScheduled.IsZero() && prevScheduled.After(lastRun) {
			s.logger.Info("missed run detected, executing now",
				"name", task.Name,
				"last_run", lastRun,
				"missed_window", prevScheduled,
			)
			s.executeTask(ctx, task)
		}
	}
}

// prevScheduleTime finds the most recent scheduled time before `before`.
// It walks forward from maxLookback ago using the schedule's Next() method.
// This covers schedules up to weekly frequency.
func prevScheduleTime(sched cron.Schedule, before time.Time) time.Time {
	candidate := before.Add(-maxLookback)
	var prev time.Time
	for {
		next := sched.Next(candidate)
		if next.After(before) || next.Equal(before) {
			break
		}
		prev = next
		candidate = next
	}
	return prev
}

// schedulerScriptContext implements metamodel.ScriptContext for scheduled tasks.
type schedulerScriptContext struct {
	ws          interface{}
	meta        *metamodel.Metamodel
	projectRoot string
}

func (c *schedulerScriptContext) GetWorkspace() interface{}     { return c.ws }
func (c *schedulerScriptContext) GetMeta() *metamodel.Metamodel { return c.meta }
func (c *schedulerScriptContext) GetProjectRoot() string        { return c.projectRoot }
func (c *schedulerScriptContext) GetEntity() *model.Entity      { return nil }
func (c *schedulerScriptContext) GetOldEntity() *model.Entity   { return nil }
