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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
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
	flag.Parse()

	configureLogging(*verbose, *quiet)

	if err := dataentry.CheckEmbeddedSPA(); err != nil {
		slog.Error("embedded SPA check failed", "error", err)
		os.Exit(1)
	}

	repo, err := createRepo(*projectDir)
	if err != nil {
		slog.Error("failed to initialize repository", "error", err)
		os.Exit(1)
	}

	ws, err := workspace.New(repo, script.NewEngine())
	if err != nil {
		slog.Error("failed to initialize workspace", "error", err)
		os.Exit(1)
	}

	app, err := dataentry.NewApp(ws)
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

	handler := app.NewRouter()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout intentionally 0: SSE and command-exec stream
		// long-lived responses and would otherwise be killed mid-flight.
		//
		// Trade-off: a slow-reading client can hold a goroutine open as
		// long as it accepts data slowly. On a loopback bind that risk is
		// limited to local processes; if you opt into LAN access via
		// `--bind`, see docs/security.md for the residual exposure.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	if !isLoopbackHost(*bind) {
		slog.Warn("rela-server bound beyond loopback; see docs/security.md for threat model",
			"bind", *bind)
	}
	// Start background scheduler if schedules.yaml exists.
	// The goroutine is cleaned up on process exit.
	scheduler.StartBackground(context.Background(), ws, slog.Default())

	slog.Info("starting server", "name", app.Cfg().App.Name, "addr", "http://"+addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
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

// createRepo discovers the project and creates a repository.
func createRepo(projectDir string) (repository.Store, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	projCtx, err := project.Discover(absDir, fs)
	if err != nil {
		return nil, fmt.Errorf("discovering project: %w", err)
	}
	return repository.New(fs, projCtx), nil
}
