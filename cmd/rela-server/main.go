// rela-server runs the data entry web application as a standalone HTTP server.
//
// Usage:
//
//	rela-server [-project .] [-port 8080]
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	port := flag.String("port", "8080", "HTTP port to listen on")
	flag.Parse()

	repo, err := createRepo(*projectDir)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Create workspace with NopScriptExecutor initially, then set real one
	ws, err := workspace.New(repo, workspace.NopScriptExecutor)
	if err != nil {
		log.Fatalf("Failed to initialize workspace: %v", err)
	}
	// Now set the real script executor (needs workspace to be created first for meta/paths)
	ws.SetScriptExecutor(script.New(ws, ws.Meta(), ws.Paths().Root))

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
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Start file watcher for live-reload.
	// The watcher goroutine is cleaned up on process exit; no explicit stop
	// is needed since log.Fatal calls os.Exit.
	if err := app.StartWatching(); err != nil {
		log.Printf("Warning: file watcher not started: %v", err)
	} else {
		log.Println("File watcher started for live-reload")
	}

	handler := app.NewRouter()

	srv := &http.Server{
		Addr:              ":" + *port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Starting %s on http://localhost:%s", app.Cfg.App.Name, *port)
	log.Fatal(srv.ListenAndServe())
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
