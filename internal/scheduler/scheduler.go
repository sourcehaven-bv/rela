package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// tickInterval is how often the scheduler wakes to check for due tasks.
const tickInterval = 60 * time.Second

// WorkspaceProvider is the subset of workspace.Workspace the scheduler needs.
type WorkspaceProvider interface {
	Sync() (*model.SyncResult, error)
	Paths() *project.Context
	Config() config.Loader
	State() state.KV
}

// StartBackground starts the scheduler in a background goroutine if
// schedules.yaml exists. It is a no-op if the file is missing. The scheduler
// runs until ctx is cancelled. Errors are logged, not returned.
func StartBackground(
	ctx context.Context,
	ws WorkspaceProvider,
	wsRaw interface{},
	metaFn func() *metamodel.Metamodel,
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
	s := New(cfg, engine, ws, wsRaw, metaFn, logger)

	go func() {
		logger.Info("background scheduler starting", "tasks", len(cfg.Tasks))
		if runErr := s.Run(ctx); runErr != nil {
			logger.Error("scheduler stopped with error", "error", runErr)
		}
	}()
}

// Scheduler runs Lua scripts sequentially on simple recurring schedules.
type Scheduler struct {
	config *Config
	engine *script.Engine
	ws     WorkspaceProvider
	metaFn func() *metamodel.Metamodel
	// wsRaw is the workspace as interface{} for passing to ScriptContext.
	// ScriptContext.GetWorkspace() consumers type-assert to lua.Services.
	wsRaw  interface{}
	state  *State
	logger *slog.Logger
	now    func() time.Time // for testing

	// executeTaskFunc overrides task execution for testing.
	// When nil, doExecuteTask is used.
	executeTaskFunc func(ctx context.Context, task TaskConfig)
}

// New creates a Scheduler. The metaFn returns the current metamodel; callers
// typically pass ws.Meta from a workspace.Workspace.
func New(
	cfg *Config,
	engine *script.Engine,
	ws WorkspaceProvider,
	wsRaw interface{},
	metaFn func() *metamodel.Metamodel,
	logger *slog.Logger,
) *Scheduler {
	return &Scheduler{
		config: cfg,
		engine: engine,
		ws:     ws,
		metaFn: metaFn,
		wsRaw:  wsRaw,
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

	// Sync workspace to get fresh graph state.
	if _, err := s.ws.Sync(); err != nil {
		s.logger.Warn("workspace sync failed, executing with stale data",
			"name", task.Name, "error", err)
	}

	sctx := &schedulerScriptContext{
		ws:          s.wsRaw,
		meta:        s.metaFn(),
		projectRoot: s.ws.Paths().Root,
	}

	err := s.engine.ExecuteFile(task.Script, sctx)
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

// schedulerScriptContext implements metamodel.ScriptContext for scheduled tasks.
type schedulerScriptContext struct {
	ws          interface{}
	meta        *metamodel.Metamodel
	projectRoot string
}

func (c *schedulerScriptContext) GetWorkspace() interface{}     { return c.ws }
func (c *schedulerScriptContext) GetMeta() *metamodel.Metamodel { return c.meta }
func (c *schedulerScriptContext) GetProjectRoot() string        { return c.projectRoot }
func (c *schedulerScriptContext) GetEntity() *entity.Entity     { return nil }
func (c *schedulerScriptContext) GetOldEntity() *entity.Entity  { return nil }
