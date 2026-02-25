// coverage-ignore: MCP server - tested via integration tests
package mcp

import (
	"log"
	"os"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// Server wraps the MCP server with rela-specific state.
type Server struct {
	mcp    *server.MCPServer
	ws     *workspace.Workspace
	logger *log.Logger
}

// NewServer creates a new MCP server for a rela project.
func NewServer(ws *workspace.Workspace, version string) *Server {
	logger := log.New(os.Stderr, "[rela-mcp] ", log.LstdFlags)

	s := &Server{
		ws:     ws,
		logger: logger,
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
	s.logger.Println("Starting rela MCP server on stdio")

	// Start file watcher via workspace
	if err := s.ws.StartWatching(workspace.WatchOptions{
		OnReload: func(_ []repository.ChangeEvent) {
			s.logger.Println("Graph re-synced from file changes")
			if s.mcp != nil {
				s.mcp.SendNotificationToAllClients(
					mcpgo.MethodNotificationResourcesListChanged, nil,
				)
			}
		},
	}); err != nil {
		s.logger.Printf("Warning: file watcher not started: %v", err)
	}

	defer s.ws.StopWatching()

	return server.ServeStdio(s.mcp, server.WithErrorLogger(s.logger))
}
