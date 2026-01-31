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

	app, err := dataentry.NewAppFS(*projectDir, storage.NewSafeFS(storage.NewOsFS()))
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
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
