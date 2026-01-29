// coverage-ignore: MCP server - tested via integration tests
package mcp

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Server wraps the MCP server with rela-specific state.
type Server struct {
	mcp        *server.MCPServer
	graph      *graph.Graph
	meta       *metamodel.Metamodel
	projectCtx *project.Context
	watcher    *Watcher
	logger     *log.Logger
}

// NewServer creates a new MCP server for a rela project.
func NewServer(g *graph.Graph, meta *metamodel.Metamodel, projectCtx *project.Context, version string) *Server {
	logger := log.New(os.Stderr, "[rela-mcp] ", log.LstdFlags)

	s := &Server{
		graph:      g,
		meta:       meta,
		projectCtx: projectCtx,
		logger:     logger,
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

	// Start file watcher
	watcher, err := NewWatcher(s)
	if err != nil {
		s.logger.Printf("Warning: file watcher not started: %v", err)
	} else {
		s.watcher = watcher
		go s.watcher.Start()
	}

	defer func() {
		if s.watcher != nil {
			s.watcher.Stop()
		}
	}()

	return server.ServeStdio(s.mcp, server.WithErrorLogger(s.logger))
}
