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
			"rela is a traceability CLI that manages entities and their relationships. "+
				"Data is stored as markdown files with YAML frontmatter. "+
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
