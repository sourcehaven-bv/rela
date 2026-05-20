// rela-server runs the data entry web application as a standalone HTTP server.
//
// Usage:
//
//	rela-server [-project .] [-port 8080] [-bind 127.0.0.1] [-allowed-origin URL]...
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// stringSliceFlag collects repeated -allowed-origin values.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

// serverFlags collects every command-line flag for rela-server.
// Extracting this lets main() stay under the funlen budget while
// keeping flag definitions in one readable block.
type serverFlags struct {
	projectDir      string
	port            string
	bind            string
	allowedOrigins  stringSliceFlag
	verbose         bool
	quiet           bool
	debugPprof      string
	principalHeader string
	readOnly        bool
}

// coverage-ignore: flag wiring — exercised at startup, not in tests
func parseFlags() *serverFlags {
	f := &serverFlags{}
	flag.StringVar(&f.projectDir, "project", ".", "Path to the rela project directory")
	flag.StringVar(&f.port, "port", "8080", "HTTP port to listen on")
	flag.StringVar(&f.bind, "bind", "127.0.0.1",
		"Network interface to bind to. Defaults to loopback. Use 0.0.0.0 to expose on the LAN (see docs/security.md).")
	flag.Var(&f.allowedOrigins, "allowed-origin",
		"Extra origin permitted to call the API (repeatable). Used for dev servers like Vite on http://localhost:5173.")
	flag.BoolVar(&f.verbose, "verbose", false, "Verbose (debug) logging")
	flag.BoolVar(&f.quiet, "quiet", false, "Quiet (warn-only) logging")
	flag.StringVar(&f.debugPprof, "debug-pprof", "",
		"If set, serve net/http/pprof on this loopback address (e.g. 127.0.0.1:6060). "+
			"Diagnostic only. Refuses to bind to non-loopback addresses.")
	flag.StringVar(&f.principalHeader, "principal-header", "",
		"HTTP header to read for audit Principal.User (e.g. X-Forwarded-User). "+
			"Default empty: do not read any header. Operators can override per-process via "+
			"$RELA_DATAENTRY_USER (wins over the header). "+
			"WARNING: the header is only as trustworthy as the upstream proxy that sets it. "+
			"See docs/security.md.")
	flag.BoolVar(&f.readOnly, "read-only", false,
		"Refuse all writes. Useful for demos, maintenance windows, "+
			"observe-only deployments, and post-incident forensic mode. "+
			"Also enabled by RELA_READ_ONLY=1.")
	flag.Parse()
	if os.Getenv("RELA_READ_ONLY") == "1" {
		f.readOnly = true
	}
	return f
}

// coverage-ignore: main function - entry point
func main() {
	f := parseFlags()

	configureLogging(f.verbose, f.quiet)

	if err := dataentry.CheckEmbeddedSPA(); err != nil {
		slog.Error("embedded SPA check failed", "error", err)
		os.Exit(1)
	}

	absDir, err := filepath.Abs(f.projectDir)
	if err != nil {
		slog.Error("invalid project dir", "error", err)
		os.Exit(1)
	}
	var discoverOpts []appbuild.Option
	if f.readOnly {
		discoverOpts = append(discoverOpts, appbuild.WithACL(acl.ReadOnlyACL{}))
	}
	svc, err := appbuild.Discover(absDir, script.NewEngine(), discoverOpts...)
	if err != nil {
		slog.Error("failed to initialize project services", "error", err)
		os.Exit(1)
	}
	if f.readOnly {
		slog.Warn("rela-server is read-only; every write request will be refused")
	}
	// No defer svc.Close(): rela-server is a daemon — it runs until
	// the process exits, at which point the OS reclaims file
	// descriptors and goroutines. A defer would be reached only via
	// early os.Exit paths, where defers don't run anyway. Per-project
	// Close() *is* required in long-running hosts that switch
	// projects (see rela-desktop); this is the daemon-lifetime case.

	app, err := dataentry.NewApp(
		svc.FS(), svc.Paths(), svc.Meta(), svc.Store(),
		svc.EntityManager(), svc.Searcher(),
	)
	if err != nil {
		var configErr *dataentry.ConfigValidationError
		if errors.As(err, &configErr) {
			fmt.Fprintln(os.Stderr, "Configuration validation failed:")
			for _, e := range configErr.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
			os.Exit(1)
		}
		slog.Error("failed to initialize", "error", err)
		os.Exit(1)
	}

	// Start file watcher for live-reload.
	// The watcher goroutine is cleaned up on process exit.
	if err := app.StartWatching(); err != nil {
		slog.Warn("file watcher not started", "error", err)
	} else {
		slog.Info("file watcher started for live-reload")
	}

	addr := net.JoinHostPort(f.bind, f.port)
	if err := app.SetSecurityConfig(dataentry.SecurityConfig{
		BindAddress:    addr,
		AllowedOrigins: f.allowedOrigins,
	}); err != nil {
		slog.Error("invalid security configuration", "error", err)
		os.Exit(1)
	}

	// Chain order: $RELA_DATAENTRY_USER (local-dev escape hatch)
	// wins over an incoming header; either falls through to
	// "unknown" when both are absent. Empty --principal-header
	// keeps the legacy default behavior.
	app.SetPrincipalResolver(dataentry.ChainResolvers(
		dataentry.EnvPrincipalResolver(),
		dataentry.HeaderPrincipalResolver(f.principalHeader),
	))

	srv := newHTTPServer(addr, app.NewRouter())

	if !isLoopbackHost(f.bind) {
		slog.Warn("rela-server bound beyond loopback; see docs/security.md for threat model",
			"bind", f.bind)
		if f.principalHeader != "" {
			// The combination — exposed bind + header-trusted principal —
			// is exactly the deployment the security doc warns against.
			// Log a second time so an operator scanning startup output
			// sees the explicit hazard, not just the generic bind warning.
			slog.Warn("--principal-header set on non-loopback bind: "+
				"audit attribution trusts an HTTP header from the network; "+
				"only safe if a reverse proxy strips + replaces the header. "+
				"See docs/security.md.",
				"bind", f.bind, "header", f.principalHeader)
		}
		if shouldWarnNoACL(svc.ACL(), f.readOnly) {
			// Non-loopback bind without `acl.yaml` and without
			// `--read-only` means anyone reaching this server can write.
			// The Origin / Host hardening from FEAT-ESLP still blocks
			// browser-driven cross-origin writes, but a direct API call
			// from inside the network is unauthenticated by design.
			// Operators serving multi-user need either `acl.yaml` or a
			// reverse proxy that enforces access; the warning surfaces
			// the gap at startup rather than at first-incident time.
			slog.Warn("rela-server bound beyond loopback without acl.yaml: "+
				"every reachable client can write. Add an acl.yaml at the project "+
				"root or pass --read-only. See docs/security.md.",
				"bind", f.bind)
		}
	}
	// Start background scheduler if schedules.yaml exists.
	// *appbuild.Services satisfies scheduler.WorkspaceProvider
	// structurally (Paths / Config / State / LuaWriteDeps).
	// The goroutine is cleaned up on process exit.
	scheduler.StartBackground(context.Background(), svc, slog.Default())

	if err := startPprofIfRequested(f.debugPprof); err != nil {
		slog.Error("pprof startup failed", "error", err)
		os.Exit(1)
	}

	slog.Info("starting server", "name", app.Cfg().App.Name, "addr", "http://"+addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// newHTTPServer wraps the data-entry handler with cleartext HTTP/2 (h2c)
// alongside HTTP/1.1. Go's http.Server only negotiates HTTP/2 automatically
// when serving TLS, so for plaintext we opt in via h2c.NewHandler. This
// matters because the data-entry SPA holds a permanent EventSource to
// /api/v1/_events — under HTTP/1.1 that eats one of the browser's per-host
// connection slots (Firefox default 6), and under concurrent navigation the
// pool runs dry. HTTP/2 multiplexes many streams over a single connection
// so the per-host limit becomes irrelevant. The wrapper is transparent to
// HTTP/1.1 clients (curl without --http2) and to all existing middlewares —
// they still see a normal *http.Request with Host/Origin/etc. populated the
// same way.
//
// coverage-ignore: server construction, exercised via integration tests
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	h2s := &http2.Server{}
	return &http.Server{
		Addr:              addr,
		Handler:           h2c.NewHandler(handler, h2s),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout intentionally 0: SSE and command-exec stream
		// long-lived responses and would otherwise be killed mid-flight.
		// Trade-off: a slow-reading client can hold a goroutine open as
		// long as it accepts data slowly. On a loopback bind that risk
		// is limited to local processes; see docs/security.md for the
		// residual exposure when --bind opts into LAN access.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}
}

// shouldWarnNoACL reports whether the operator should be told they
// are running with no access control on a non-loopback bind.
//
// True iff:
//   - The active ACL is [acl.NopACL] (i.e. no `acl.yaml`, and the
//     operator didn't pass `--read-only`).
//   - `--read-only` is NOT set (read-only is a stronger guarantee
//     than `acl.yaml` — no need to nag).
//
// Extracted so the warning logic is unit-testable without spinning
// up a server.
func shouldWarnNoACL(active acl.ACL, readOnly bool) bool {
	if readOnly {
		return false
	}
	_, isNop := active.(acl.NopACL)
	return isNop
}

// configureLogging sets the default slog logger based on verbose/quiet flags.
func configureLogging(verbose, quiet bool) {
	level := slog.LevelInfo
	switch {
	case verbose:
		level = slog.LevelDebug
	case quiet:
		level = slog.LevelWarn
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

// startPprofIfRequested launches a diagnostic net/http/pprof listener on
// a separate loopback-only port if addr is non-empty. Returns nil and
// does nothing if addr is empty (the default; pprof off).
//
// pprof handlers are registered on a private mux rather than via the
// usual `_ "net/http/pprof"` blank import, which would register on
// http.DefaultServeMux and risk leaking the diagnostic surface if any
// other code in the process accidentally serves DefaultServeMux. The
// listener also refuses non-loopback binds so a misconfigured
// --bind 0.0.0.0 cannot accidentally expose goroutine dumps to the LAN.
//
// coverage-ignore: diagnostic-only, off by default
func startPprofIfRequested(addr string) error {
	if addr == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid --debug-pprof address %q: %w", addr, err)
	}
	if !isLoopbackHost(host) {
		return fmt.Errorf("--debug-pprof must bind to loopback (got %q)", host)
	}

	// Build our own mux with pprof handlers explicitly registered. The
	// stdlib net/http/pprof package exposes the handler functions
	// directly so we can wire them onto a private mux.
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("pprof listening", "addr", "http://"+addr+"/debug/pprof/")
		if err := srv.ListenAndServe(); err != nil {
			slog.Warn("pprof server stopped", "error", err)
		}
	}()
	return nil
}

// isLoopbackHost reports whether host is the loopback interface.
func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
