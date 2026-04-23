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

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// stringSliceFlag collects repeated -allowed-origin values.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	port := flag.String("port", "8080", "HTTP port to listen on")
	bind := flag.String("bind", "127.0.0.1",
		"Network interface to bind to. Defaults to loopback. Use 0.0.0.0 to expose on the LAN (see docs/security.md).")
	var allowedOrigins stringSliceFlag
	flag.Var(&allowedOrigins, "allowed-origin",
		"Extra origin permitted to call the API (repeatable). Used for dev servers like Vite on http://localhost:5173.")
	verbose := flag.Bool("verbose", false, "Verbose (debug) logging")
	quiet := flag.Bool("quiet", false, "Quiet (warn-only) logging")
	debugPprof := flag.String("debug-pprof", "",
		"If set, serve net/http/pprof on this loopback address (e.g. 127.0.0.1:6060). "+
			"Diagnostic only. Refuses to bind to non-loopback addresses.")
	flag.Parse()

	configureLogging(*verbose, *quiet)

	if err := dataentry.CheckEmbeddedSPA(); err != nil {
		slog.Error("embedded SPA check failed", "error", err)
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*projectDir)
	if err != nil {
		slog.Error("invalid project dir", "error", err)
		os.Exit(1)
	}
	// The workspace's engine runs scheduler/flow/validation scripts — none
	// of which need rela.url (they have no frontend to target). The
	// catalog is wired into dataentry.NewApp below, where it scopes to
	// document renders only.
	ws, err := workspace.Discover(absDir, script.NewEngine())
	if err != nil {
		slog.Error("failed to initialize workspace", "error", err)
		os.Exit(1)
	}

	app, err := dataentry.NewApp(
		ws.FS(), ws.Paths(), ws.Meta(), ws.Store(),
		ws.EntityManager(), ws.Searcher(),
		ws.StartWatching,
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

	addr := net.JoinHostPort(*bind, *port)
	if err := app.SetSecurityConfig(dataentry.SecurityConfig{
		BindAddress:    addr,
		AllowedOrigins: allowedOrigins,
	}); err != nil {
		slog.Error("invalid security configuration", "error", err)
		os.Exit(1)
	}

	srv := newHTTPServer(addr, app.NewRouter())

	if !isLoopbackHost(*bind) {
		slog.Warn("rela-server bound beyond loopback; see docs/security.md for threat model",
			"bind", *bind)
	}
	// Start background scheduler if schedules.yaml exists.
	// The goroutine is cleaned up on process exit.
	scheduler.StartBackground(context.Background(), ws, slog.Default())

	if err := startPprofIfRequested(*debugPprof); err != nil {
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
