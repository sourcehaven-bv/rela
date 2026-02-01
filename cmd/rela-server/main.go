// rela-server runs the data entry web application as a standalone HTTP server.
//
// Usage:
//
//	rela-server [-project .] [-port 8080]
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	port := flag.String("port", "8080", "HTTP port to listen on")
	flag.Parse()

	app, err := dataentry.NewApp(*projectDir, storage.NewSafeFS(storage.NewOsFS()))
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Start file watcher for live-reload.
	// The watcher goroutine is cleaned up on process exit; no explicit stop
	// is needed since log.Fatal calls os.Exit.
	if _, err := app.StartWatching(); err != nil {
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
