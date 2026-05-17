// Package scheduler runs Lua scripts on recurring schedules defined in
// schedules.yaml.
//
// The scheduler is a single-threaded sequential loop: tasks execute one at
// a time in config order. Each task gets a fresh ws.LuaWriteDeps() from the
// workspace; the store is the source of truth, so no explicit sync is
// needed. Last-run timestamps are persisted in .rela/scheduler-state.json.
// Tasks that missed their scheduled window run immediately on startup.
// Shutdown is graceful on SIGINT/SIGTERM.
//
// Schedule values in schedules.yaml:
//
//	day          once per day (after midnight local time)
//	<weekday>    once per week on that weekday (monday, friday, ...)
//	week         alias for monday
//	30m, 2h      fixed interval (any Go duration)
//	15           bare number interpreted as minutes
//
// See Config/TaskConfig for the YAML shape and Schedule.IsDue for the
// due-time logic.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// tickInterval is how often the scheduler wakes to check for due tasks.
const tickInterval = 60 * time.Second

// WorkspaceProvider is the subset of workspace.Workspace the scheduler needs.
type WorkspaceProvider interface {
	Paths() *project.Context
	Config() config.Loader
	State() state.KV
	LuaWriteDeps() lua.WriteDeps
}

// StartBackground starts the scheduler in a background goroutine if
// schedules.yaml exists. It is a no-op if the file is missing. The scheduler
// runs until ctx is cancelled. Errors are logged, not returned.
func StartBackground(
	ctx context.Context,
	ws WorkspaceProvider,
	logger *slog.Logger,
) {
	data, err := ws.Config().Load(ctx, ConfigFile)
	if err != nil {
		// No schedules.yaml — nothing to do.
		return
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		logger.Error("invalid schedules.yaml, scheduler not started", "error", err)
		return
	}

	if len(cfg.Tasks) == 0 {
		return
	}

	engine := script.NewEngine()
	s := New(cfg, engine, ws, logger)

	go func() {
		logger.Info("background scheduler starting", "tasks", len(cfg.Tasks))
		if runErr := s.Run(ctx); runErr != nil {
			logger.Error("scheduler stopped with error", "error", runErr)
		}
	}()
}

// stampTaskAuditContext stamps the scheduler-specific Principal and
// the per-task triggered_by label on a child context so audit records
// produced by the Lua script (directly via rela.create_entity, or
// indirectly via automation cascades) carry the right attribution.
//
// Extracted so the stamping logic can be unit-tested without booting
// the script engine.
func stampTaskAuditContext(ctx context.Context, taskName string) context.Context {
	out := audit.WithPrincipal(ctx, audit.Principal{
		User: audit.SystemUser(),
		Tool: audit.ToolScheduler,
	})
	return audit.WithTriggeredBy(out, "schedule:"+taskName)
}

// Scheduler runs Lua scripts sequentially on simple recurring schedules.
type Scheduler struct {
	config *Config
	engine *script.Engine
	ws     WorkspaceProvider
	state  *State
	logger *slog.Logger
	now    func() time.Time // for testing

	// executeTaskFunc overrides task execution for testing.
	// When nil, doExecuteTask is used.
	executeTaskFunc func(ctx context.Context, task TaskConfig)
}

// New creates a Scheduler.
func New(
	cfg *Config,
	engine *script.Engine,
	ws WorkspaceProvider,
	logger *slog.Logger,
) *Scheduler {
	return &Scheduler{
		config: cfg,
		engine: engine,
		ws:     ws,
		logger: logger,
		now:    time.Now,
	}
}

// Run starts the scheduler and blocks until ctx is cancelled.
// Tasks are executed sequentially in a single goroutine — no concurrent
// script execution, no mutexes needed.
func (s *Scheduler) Run(ctx context.Context) error {
	s.loadState()

	if len(s.config.Tasks) == 0 {
		s.logger.Info("no tasks configured, waiting for shutdown")
		<-ctx.Done()
		return nil
	}

	for _, t := range s.config.Tasks {
		s.logger.Info("scheduled task", "name", t.Name, "every", t.Every, "script", t.Script)
	}

	// Run due tasks immediately (handles first-ever and missed runs).
	s.runDueTasks(ctx)

	s.logger.Info("scheduler started", "tasks", len(s.config.Tasks))

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return nil
		case <-ticker.C:
			s.runDueTasks(ctx)
		}
	}
}

// runDueTasks checks each task and executes it if due. All execution is
// sequential in the caller's goroutine.
func (s *Scheduler) runDueTasks(ctx context.Context) {
	now := s.now()
	for _, task := range s.config.Tasks {
		if ctx.Err() != nil {
			return
		}

		lastRun, recorded := s.state.Tasks[task.Name]
		if !recorded {
			s.logger.Info("first run, executing immediately", "name", task.Name)
			s.executeTask(ctx, task)
			continue
		}

		if task.Every.IsDue(lastRun, now) {
			s.logger.Info("task due", "name", task.Name, "last_run", lastRun)
			s.executeTask(ctx, task)
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task TaskConfig) {
	if s.executeTaskFunc != nil {
		s.executeTaskFunc(ctx, task)
		return
	}
	s.doExecuteTask(ctx, task)
}

func (s *Scheduler) doExecuteTask(ctx context.Context, task TaskConfig) {
	if ctx.Err() != nil {
		s.logger.Warn("skipping task, scheduler shutting down", "name", task.Name)
		return
	}

	s.logger.Info("task started", "name", task.Name, "script", task.Script)
	start := s.now()

	taskCtx := stampTaskAuditContext(ctx, task.Name)
	err := s.engine.ExecuteFile(taskCtx, task.Script, s.ws.LuaWriteDeps(), nil, nil)
	elapsed := s.now().Sub(start)

	if err != nil {
		s.logger.Error("task failed", "name", task.Name, "duration", elapsed, "error", err)
		return
	}

	s.logger.Info("task completed", "name", task.Name, "duration", elapsed)

	// Record successful run — no mutex needed, single goroutine.
	s.state.Tasks[task.Name] = s.now()
	s.saveState()
}

func (s *Scheduler) loadState() {
	data, err := s.ws.State().Get(context.Background(), stateFile)
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
	if err := s.ws.State().Put(context.Background(), stateFile, data); err != nil {
		s.logger.Error("failed to save scheduler state", "error", err)
	}
}
