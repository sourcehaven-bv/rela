// rela-desktop runs the data entry application as a native desktop app using Wails.
//
// Usage:
//
//	rela-desktop [-project .]
package main

import (
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	flag.Parse()

	app, err := dataentry.NewApp(*projectDir, storage.NewSafeFS(storage.NewOsFS()))
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	handler := app.NewRouter()

	err = wails.Run(&options.App{
		Title:  app.Cfg.App.Name,
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
	})
	if err != nil {
		log.Fatalf("Wails error: %v", err)
	}
}
