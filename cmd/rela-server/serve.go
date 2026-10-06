package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	"github.com/Sourcehaven-BV/rela/internal/jwtauth"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// drainLimit bounds how long a retired generation waits for its requests
// before its services close. Live-update streams end at once (Retire closes
// them); what remains is ordinary requests and command output streams.
const drainLimit = 30 * time.Second

// drainPoll is how often a retiring generation checks its in-flight count.
const drainPoll = 50 * time.Millisecond

// writeWait bounds how long a Configure save waits for record writes that
// are already running before it gives up as busy.
const writeWait = 10 * time.Second

// frozenRetry is the Retry-After a held write is answered with.
const frozenRetry = "5"

// generation is one built server: the services, the app over them, and the
// scheduler that runs against them. A Configure save replaces the whole
// generation rather than mutating the running one.
type generation struct {
	svc     *appbuild.Services
	app     *dataentry.App
	handler http.Handler

	// The scheduler is started by [server.startScheduler] and stopped by
	// [generation.haltScheduler], which a pause and a retirement call from
	// different goroutines; schedMu guards the fields. held keeps a paused
	// or retired generation's scheduler from being started behind its back.
	schedMu   sync.Mutex
	schedStop context.CancelFunc
	schedDone <-chan struct{}
	held      bool

	inflight atomic.Int64
	retired  atomic.Bool

	// writes counts requests that may write records; frozen refuses new
	// ones while a Configure save migrates the records (see server.pause).
	writes atomic.Int64
	frozen atomic.Bool
}

// server is the process: what is built once at startup, and the generation
// currently serving. It is the http.Handler the listener serves.
type server struct {
	f    *serverFlags
	addr string
	mode identityMode
	// idv and webhook are built once: the verifier fetches the JWKS, and the
	// webhook verifier holds the replay set, which must outlive a rebuild.
	idv     *jwtauth.Verifier
	webhook *jwtauth.WebhookVerifier
	// configure is the Configure API, nil unless --config-editing is set.
	// One handler for the process, so its save mutex spans rebuilds.
	configure http.Handler
	// accessLog is the --access-log destination, nil when off. Every
	// generation's router logs to it.
	accessLog *slog.Logger

	cur atomic.Pointer[generation]
	// retiring closes when the latest retirement has finished. Each
	// retirement waits for the one before it, so they run in activation
	// order: a generation's scheduler has started before it is stopped, and
	// two schedulers never run at once.
	mu       sync.Mutex
	retiring <-chan struct{}
}

// newServer builds the process-wide pieces and the first generation over
// svc, and starts its scheduler.
func newServer(f *serverFlags, svc *appbuild.Services, accessLog *slog.Logger) (*server, error) {
	// Identity sources are mutually exclusive — validate BEFORE building
	// anything, so a conflicting config never reaches a running server.
	mode, err := validateIdentityFlags(f, os.Getenv(dataentry.EnvDataEntryUserVar))
	if err != nil {
		return nil, fmt.Errorf("invalid identity configuration: %w", err)
	}
	s := &server{f: f, addr: net.JoinHostPort(f.bind, f.port), mode: mode, accessLog: accessLog}
	// One verifier for the identity gate and the webhook receiver, so the
	// JWKS is fetched a single time (nil when JWT identity is disabled).
	s.idv = buildIdentityVerifier(context.Background(), f)
	if s.webhook, err = buildWebhookVerifier(f, s.idv); err != nil {
		return nil, err
	}
	if f.configEditing {
		if s.configure, err = newConfigure(s, svc); err != nil {
			return nil, fmt.Errorf("--config-editing: %w", err)
		}
	}
	g, err := s.build(svc)
	if err != nil {
		return nil, err
	}
	s.startScheduler(g)
	s.cur.Store(g)
	return s, nil
}

// ServeHTTP serves r on the current generation and counts it in flight, so
// a retiring generation knows when its last request has finished.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for {
		g := s.cur.Load()
		if g.serve(w, r) {
			return
		}
	}
}

// serve runs r on g unless g was retired, reporting whether it did.
//
// The count goes up BEFORE retired is read. A retiring generation sets
// retired before it reads the count, so either this request sees retired
// and moves to the new generation, or the drain sees it in flight.
//
// Writes follow the same protocol against frozen: the count goes up before
// frozen is read, and pause sets frozen before it waits for the count.
func (g *generation) serve(w http.ResponseWriter, r *http.Request) bool {
	g.inflight.Add(1)
	defer g.inflight.Add(-1)
	if g.retired.Load() {
		return false
	}
	if mayWriteRecords(r) {
		g.writes.Add(1)
		defer g.writes.Add(-1)
		if g.frozen.Load() {
			writeFrozen(w)
			return true
		}
	}
	g.handler.ServeHTTP(w, r)
	return true
}

// mayWriteRecords reports whether r may change records. The Configure API is
// left out: its own lock serializes it, and the save doing the pausing is one
// of its requests.
func mayWriteRecords(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return !strings.HasPrefix(r.URL.Path, dataentry.ConfigurePath)
}

func writeFrozen(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Retry-After", frozenRetry)
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, `{"title":"Service Unavailable","status":503,`+
		`"detail":"The configuration is being saved. Try again in a few seconds."}`)
}

// pause holds g still for a Configure save: it stops reloading data-entry.yaml,
// refuses new record writes, waits for running ones, and stops g's scheduler
// so no task writes while records migrate. resume undoes all of it.
func (s *server) pause(ctx context.Context, g *generation) (resume func(), err error) {
	resumeReload := dataentry.PauseConfigReload(g.app)
	g.frozen.Store(true)
	deadline := time.Now().Add(writeWait)
	for n := g.writes.Load(); n > 0; n = g.writes.Load() {
		if time.Now().After(deadline) || ctx.Err() != nil {
			g.frozen.Store(false)
			resumeReload()
			return nil, fmt.Errorf("%d record writes still running", n)
		}
		time.Sleep(drainPoll)
	}
	g.haltScheduler()
	return func() { //nolint:contextcheck // the scheduler outlives the save request
		g.frozen.Store(false)
		g.schedMu.Lock()
		g.held = false
		g.schedMu.Unlock()
		s.startScheduler(g)
		resumeReload()
	}, nil
}

// build makes a generation over svc without serving it or starting its
// scheduler. Every failure is returned: at startup the caller exits, and
// during a Configure save the old generation keeps serving.
func (s *server) build(svc *appbuild.Services) (*generation, error) {
	app, err := s.buildApp(svc)
	if err != nil {
		return nil, err
	}
	if err := app.StartWatching(); err != nil {
		slog.Warn("file watcher not started", "error", err)
	}
	return &generation{svc: svc, app: app, handler: app.NewRouter(dataentry.WithAccessLog(s.accessLog))}, nil
}

// buildApp builds the data-entry app over svc with the process's identity,
// security and Configure settings.
func (s *server) buildApp(svc *appbuild.Services) (*dataentry.App, error) {
	fieldResolver, err := dataentry.ResolverFromServices(svc)
	if err != nil {
		return nil, fmt.Errorf("build affordance resolver: %w", err)
	}
	commandAuthz, err := buildCommandAuthorizer(s.f, svc)
	if err != nil {
		return nil, err
	}
	app, err := dataentry.NewApp(
		svc.FS(), svc.Paths(), svc.ProjectFiles(), svc.Templater(), svc.Meta(), svc.Store(), svc.Versions(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		fieldResolver,
		svc.Audit(),
		svc.State(),
		commandAuthz,
		appbuild.CompiledWorlds(svc),
	)
	if err != nil {
		return nil, err
	}

	if err := dataentrywire.Services(app, svc); err != nil {
		return nil, fmt.Errorf("wire the data-entry app: %w", err)
	}
	if err := app.SetSecurityConfig(dataentry.SecurityConfig{
		BindAddress:    s.addr,
		AllowedOrigins: s.f.allowedOrigins,
	}); err != nil {
		return nil, fmt.Errorf("invalid security configuration: %w", err)
	}

	// The order is load-bearing: wireRemoteMCP refuses to enable MCP unless
	// the JWT gate is installed.
	if err := wirePrincipalResolvers(app, s.f, s.idv, s.mode); err != nil {
		return nil, err
	}
	if s.webhook != nil {
		app.SetWebhookReceiver(webhookVerifierAdapter{s.webhook}, s.f.webhookAction)
	}
	if err := wireRemoteMCP(app, svc, s.f); err != nil {
		return nil, fmt.Errorf("enable remote MCP: %w", err)
	}
	if s.configure != nil {
		dataentry.SetConfigure(app, s.configure)
	}
	return app, nil
}

// startScheduler starts g's background scheduler, unless it is running or g
// is held.
func (s *server) startScheduler(g *generation) {
	g.schedMu.Lock()
	defer g.schedMu.Unlock()
	if g.held || g.schedStop != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.schedStop = cancel
	// *appbuild.Services satisfies scheduler.WorkspaceProvider structurally.
	g.schedDone = scheduler.StartBackground(ctx, g.svc, slog.Default())
}

// haltScheduler stops g's scheduler, waits for it, and holds g so it is not
// started again until a resume clears held.
func (g *generation) haltScheduler() {
	g.schedMu.Lock()
	stop, done := g.schedStop, g.schedDone
	g.schedStop, g.schedDone, g.held = nil, nil, true
	g.schedMu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
}

// activate makes next the serving generation and retires the one it
// replaces. It returns once next serves; the old generation is taken down
// in the background, because the request that triggered the switch is
// itself one of the old generation's requests.
func (s *server) activate(next *generation) {
	done := make(chan struct{})
	s.mu.Lock()
	old := s.cur.Swap(next)
	prev := s.retiring
	s.retiring = done
	s.mu.Unlock()

	old.retired.Store(true)
	dataentry.Retire(old.app)
	go func() {
		defer close(done)
		if prev != nil {
			<-prev
		}
		s.retire(old, next)
	}()
}

// retire hands the scheduler from old to next, waits for old's requests and
// closes its services. The old scheduler stops before the new one starts,
// so a task never runs twice at once.
func (s *server) retire(old, next *generation) {
	old.haltScheduler()
	// A no-op when a later save has paused next already; that save resumes
	// it on failure, or retires it after this retirement has finished.
	s.startScheduler(next)

	deadline := time.Now().Add(drainLimit)
	for old.inflight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(drainPoll)
	}
	if n := old.inflight.Load(); n > 0 {
		slog.Warn("closing the replaced configuration with requests still running", "requests", n)
	}
	if err := old.svc.Close(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("closing the replaced configuration", "error", err)
	}
	// The old store saved its index on close, missing what next wrote.
	if err := appbuild.DropStoreIndex(old.svc); err != nil {
		slog.Warn("removing the replaced configuration's store index", "error", err)
	}
}
