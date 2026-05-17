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
	"context"
	"errors"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// Services is the slice of backend services the MCP server actually
// uses. Tests that need a narrower surface can implement Services
// directly. The wiring site (internal/cli) constructs a concrete
// implementation from focused services and supplies it to NewServer.
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
	LuaCache() *lua.Cache
	Watcher() Watcher
}

// Watcher is the narrow file-watching capability MCP requires from
// its wiring site. Start arms the watcher with an opaque "something
// changed" callback; Pause / Resume temporarily suppress callbacks
// while in-process writes happen (e.g. entity rename). The wiring
// site supplies an adapter that translates these calls into the
// underlying filesystem watcher.
type Watcher interface {
	Start(onChange func()) error
	Stop()
	Pause()
	Resume()
}

// Server wraps the MCP server with rela-specific state.
type Server struct {
	mcp       *server.MCPServer
	ws        Services
	logger    *slog.Logger
	principal audit.Principal
}

// Option configures a [Server] at construction.
type Option func(*Server)

// WithPrincipal stamps p onto every tool-handler ctx via a server
// middleware so downstream audit records are correctly attributed.
// Applies to every registered tool — including lua_eval / lua_run /
// any future write tool — because the middleware runs ahead of all
// handlers (registration-time wrapping, not per-handler opt-in).
func WithPrincipal(p audit.Principal) Option {
	return func(s *Server) { s.principal = p }
}

// principalMiddleware is the mcp-go ToolHandlerMiddleware that
// stamps the server's Principal on every tool ctx. Registered once
// in NewServer via server.WithToolHandlerMiddleware so no per-handler
// opt-in is required (CLAUDE.md: "make the wrong thing impossible to
// write" — a new write tool added to the server inherits the stamp
// automatically).
//
// NewServer guarantees s.principal is non-zero by the time this
// middleware is registered, so there's no "no Principal" branch here.
func (s *Server) principalMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return next(audit.WithPrincipal(ctx, s.principal), req)
	}
}

// NewServer creates a new MCP server for a rela project. Returns an
// error if [WithPrincipal] was not supplied — silently degrading to
// `unknown/unknown` audit attribution would be an invisible
// production bug (CLAUDE.md "constructors reject nil required
// fields"). Tests must pass a non-zero Principal too — use any
// non-empty `audit.Principal{User: ..., Tool: ...}`.
func NewServer(ws Services, version string, opts ...Option) (*Server, error) {
	s := &Server{
		ws:     ws,
		logger: slog.Default().With("component", "mcp"),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.principal == (audit.Principal{}) {
		return nil, errors.New("mcp.NewServer: Principal is required (use WithPrincipal)")
	}

	mcpServer := server.NewMCPServer(
		"rela",
		version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
		server.WithToolHandlerMiddleware(s.principalMiddleware),
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

	return s, nil
}

// Serve starts the MCP server on stdio.
func (s *Server) Serve() error {
	s.logger.Info("starting rela MCP server on stdio")

	// Start the file watcher; MCP only cares "something changed."
	if err := s.ws.Watcher().Start(func() {
		s.logger.Info("graph re-synced from file changes")
		if s.mcp != nil {
			s.mcp.SendNotificationToAllClients(
				mcpgo.MethodNotificationResourcesListChanged, nil,
			)
		}
	}); err != nil {
		s.logger.Warn("file watcher not started", "error", err)
	}

	defer s.ws.Watcher().Stop()

	// mcp-go's WithErrorLogger expects a stdlib *log.Logger; bridge to slog
	// so all output flows through the configured slog handler.
	bridged := slog.NewLogLogger(s.logger.Handler(), slog.LevelError)
	return server.ServeStdio(s.mcp, server.WithErrorLogger(bridged))
}
