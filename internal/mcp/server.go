// Package mcp implements the Model Context Protocol server exposed by
// `rela mcp` over stdio.
//
// The server exposes rela's capabilities to AI assistants:
//
//   - Tools for entity/relation CRUD, graph trace/path, analysis (orphans,
//     cardinality, properties, validations, schema), schema introspection,
//     export, and Lua execution. Registered in tools.go (grep AddTool).
//   - Resources: rela://metamodel, rela://entity/{type}/{id},
//     rela://relation/{from}/{type}/{to}
//   - Prompts: analyze-traceability, review-orphans, summarize-project,
//     review-entity
//   - A file watcher over entities/, relations/, and metamodel.yaml with
//     a 200ms debounce; tests that exercise the watcher must wait past it
//     (see watcher.go).
//
// The server handles its own project init (discovery, metamodel load,
// store wiring) independently from the standard CLI PersistentPreRunE.

// coverage-ignore: MCP server - tested via integration tests
package mcp

import (
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// Services is the slice of the workspace API the MCP server actually
// uses. *workspace.Workspace satisfies it; tests that need a narrower
// surface can implement Services directly instead of building a full
// workspace.
type Services interface {
	Store() store.Store
	Meta() *metamodel.Metamodel
	Tracer() tracer.Tracer
	Searcher() search.Searcher
	Validator() validator.Validator
	EntityManager() entitymanager.EntityManager
	Config() config.Loader
	Paths() *project.Context
	LuaWriteDeps() lua.WriteDeps
	PauseWatching()
	ResumeWatching()
	StartWatching(workspace.WatchOptions) error
	StopWatching()
}

// compile-time check that *workspace.Workspace satisfies Services.
var _ Services = (*workspace.Workspace)(nil)

// Server wraps the MCP server with rela-specific state.
type Server struct {
	mcp    *server.MCPServer
	ws     Services
	logger *slog.Logger
}

// NewServer creates a new MCP server for a rela project.
func NewServer(ws Services, version string) *Server {
	s := &Server{
		ws:     ws,
		logger: slog.Default().With("component", "mcp"),
	}

	mcpServer := server.NewMCPServer(
		"rela",
		version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
		server.WithInstructions(
			"rela is a schema-driven entity-graph platform. The domain is defined by a "+
				"YAML metamodel (entity types, relation types, properties, validation rules); "+
				"entities and relations are stored as markdown files with YAML frontmatter. "+
				"Traceability is one common use case, not the only one — the graph can model "+
				"requirements, compliance controls, project plans, issue trackers, "+
				"knowledge bases, or any typed-entity-and-relation domain. "+
				"Use tools to query, create, update, and delete entities and relations. "+
				"Use resources to read entity and metamodel data directly.",
		),
	)

	s.mcp = mcpServer

	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

// Serve starts the MCP server on stdio.
func (s *Server) Serve() error {
	s.logger.Info("starting rela MCP server on stdio")

	// Start file watcher via workspace
	if err := s.ws.StartWatching(workspace.WatchOptions{
		OnChange: func(_ []workspace.ChangeEvent) {
			s.logger.Info("graph re-synced from file changes")
			if s.mcp != nil {
				s.mcp.SendNotificationToAllClients(
					mcpgo.MethodNotificationResourcesListChanged, nil,
				)
			}
		},
	}); err != nil {
		s.logger.Warn("file watcher not started", "error", err)
	}

	defer s.ws.StopWatching()

	// mcp-go's WithErrorLogger expects a stdlib *log.Logger; bridge to slog
	// so all output flows through the configured slog handler.
	bridged := slog.NewLogLogger(s.logger.Handler(), slog.LevelError)
	return server.ServeStdio(s.mcp, server.WithErrorLogger(bridged))
}
