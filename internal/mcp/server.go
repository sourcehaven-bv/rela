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
//   - A file watcher over entities/, relations/, and the schema file with
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
	"iter"
	"log/slog"
	"net/http"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// Deps is the focused bundle of backend services the MCP server needs.
// Every field is a domain type — the server holds no reference to any
// composition-root aggregate, so `internal/mcp` does not import
// `internal/appbuild` (enforced by arch-lint). The wiring site
// (`internal/cli`) constructs a Deps from focused services and supplies
// it to [NewServer]; tests build a Deps literal directly.
//
// ProjectRoot is the absolute project root, used by the lua tools to
// resolve relative script paths. It is the only piece of the project
// context MCP consumes — passing the string instead of a
// `*project.Context` keeps that type from leaking into MCP test stubs.
type Deps struct {
	// Store is the READ handle for every MCP read surface — tools,
	// resources, prompts, export and analyze alike. It is deliberately
	// the narrow [GraphReader], not `store.Store`: writes go through
	// EntityManager, so MCP never needs the wide composite, and typing
	// the field this way makes an ungated raw read *unavailable* rather
	// than merely discouraged (the TKT-80EWGM "make the mistake
	// impossible" pattern, applied to reads).
	//
	// The wiring site decides what this is. `rela mcp` (stdio) passes the
	// raw store — the filesystem is the trust boundary there, so a gate
	// would defend nothing. A networked wiring passes a
	// visibility-wrapped reader that resolves the ctx principal per call.
	// Either way the handlers are identical; gating is entirely a wiring
	// decision (DEC-ZBI39P).
	Store         GraphReader
	Meta          *metamodel.Metamodel
	Tracer        tracer.Tracer
	Searcher      search.Searcher
	Validator     validator.Validator
	EntityManager entitymanager.EntityManager
	Config        config.Loader
	LuaWriteDeps  lua.WriteDeps
	LuaCache      *lua.Cache
	Watcher       Watcher
	ProjectRoot   string
}

// GraphReader is the read capability MCP requires of its store — the exact
// set the handlers call, declared here at the CALL SITE rather than reused
// from `store.Store`, which is a ten-interface composite (CRUD, attachments,
// watching, transactions) MCP has no business holding.
//
// It is split deliberately. The three ENTITY/RELATION reads are the gated
// surface: they return rows, so a wiring may substitute a decorator that
// hides some. The two COUNTS are [GraphCounter], kept separate because a
// count is structural — it discloses how many rows of a declared type exist,
// not which ones — and `internal/dataentry` already draws this exact line
// (`analyzeService.relCounts` is "raw (ungated) on purpose").
//
// `store.Store` satisfies the whole thing structurally, so the stdio wiring
// passes one unchanged; a visibility decorator satisfies the gated half,
// which is the point.
type GraphReader interface {
	GraphCounter

	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error)
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// GraphCounter is the structural half of [GraphReader]: type-level tallies
// that name no individual row. Kept as its own interface so a wiring site can
// compose a gated row-reader with a raw counter without either pretending to
// be the other.
type GraphCounter interface {
	CountEntities(ctx context.Context, q store.EntityQuery) (int, error)
	CountRelations(ctx context.Context, q store.RelationQuery) (int, error)
}

// validate rejects a Deps missing any field whose zero value would
// defer a failure to request time — a nil collaborator panics inside a
// tool handler, and an empty ProjectRoot makes lua_list silently walk
// the process CWD instead of the project's scripts/ dir. Catching these
// at construction keeps the failure where it can be diagnosed.
//
// LuaCache is intentionally absent: a nil cache is a valid "no cache"
// signal that lua.WithCache tolerates.
func (d Deps) validate() error {
	switch {
	case d.Store == nil:
		return errors.New("mcp: Deps.Store is required")
	case d.Meta == nil:
		return errors.New("mcp: Deps.Meta is required")
	case d.Tracer == nil:
		return errors.New("mcp: Deps.Tracer is required")
	case d.Searcher == nil:
		return errors.New("mcp: Deps.Searcher is required")
	case d.Validator == nil:
		return errors.New("mcp: Deps.Validator is required")
	case d.EntityManager == nil:
		return errors.New("mcp: Deps.EntityManager is required")
	case d.Config == nil:
		return errors.New("mcp: Deps.Config is required")
	case d.Watcher == nil:
		return errors.New("mcp: Deps.Watcher is required")
	case d.ProjectRoot == "":
		return errors.New("mcp: Deps.ProjectRoot is required")
	}
	return nil
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
//
// TODO(TKT-N0IKN9): Server started this arc over the 40-method load line;
// it is under it now (25 methods). The directive stays pinned to the actual
// count so extraction gains cannot silently erode; ratchet it further as
// the remaining handler clusters move out.
//
// 48 → 49: [Server.HTTPHandler] (TKT-BDG8U9). It belongs on Server — it
// exposes THIS server over a second transport, the peer of [Server.Serve] —
// and it is the only method the remote endpoint added: the stateless-transport
// choice lives inside it, and the wiring site holds an http.Handler rather
// than reaching for the SDK.
//
// 49 → 38 (TKT-YUETL7): the type-name helpers moved to typeResolver (they
// need only the metamodel) and the trace/export tool handlers moved to
// traceHandler / exportHandler (store + tracer, and store + resolver,
// respectively). Each is a field below, wired from Deps in [NewServer];
// registerTools points the affected AddTool lines at the field's methods.
//
// 38 → 25 (TKT-MGNE5L): the lua tools moved to luaHandler (the sole user of
// LuaWriteDeps / LuaCache / ProjectRoot), the schema tools + resource reads
// to schemaResourceHandler (store + metamodel), and the prompt handlers to
// promptHandler (store + metamodel + tracer + resolver). register* stay on
// Server and point at the fields' methods. The remaining handlers genuinely
// span deps — the next TKT-N0IKN9 slice.
//
//plimsoll:max-methods=25
type Server struct {
	mcp       *mcpgo.Server
	deps      Deps
	logger    *slog.Logger
	principal principal.Principal

	// Extracted handler groups (TKT-YUETL7, TKT-MGNE5L) — built from deps
	// by Deps.handlers. Embedded so the groups stay addressable as
	// s.trace/s.export/... Identity is NOT threaded into them: every
	// handler reads the principal from its ctx, stamped by
	// principalMiddleware.
	handlerSet
}

// handlerSet is the extracted handler groups a [Server] carries, grouped so
// Deps.handlers has one return value rather than one per group.
type handlerSet struct {
	types     typeResolver
	trace     traceHandler
	export    exportHandler
	lua       luaHandler
	schemaRes schemaResourceHandler
	prompts   promptHandler
}

// handlers builds the extracted handler groups a [Server] carries. One
// derivation shared by [NewServer] and the test helpers that construct
// Server literals, so the wiring cannot drift between them.
func (d Deps) handlers() handlerSet {
	types := typeResolver{meta: d.Meta}
	return handlerSet{
		types:     types,
		trace:     traceHandler{store: d.Store, tracer: d.Tracer},
		export:    exportHandler{store: d.Store, types: types},
		lua:       luaHandler{writeDeps: d.LuaWriteDeps, cache: d.LuaCache, projectRoot: d.ProjectRoot},
		schemaRes: schemaResourceHandler{store: d.Store, meta: d.Meta},
		prompts:   promptHandler{store: d.Store, meta: d.Meta, tracer: d.Tracer, types: types},
	}
}

// Option configures a [Server] at construction.
type Option func(*Server)

// WithPrincipal stamps p onto every tool-handler ctx via a server
// middleware so downstream audit records are correctly attributed.
// Applies to every registered tool — including lua_eval / lua_run /
// any future write tool — because the middleware runs ahead of all
// handlers (registration-time wrapping, not per-handler opt-in).
func WithPrincipal(p principal.Principal) Option {
	return func(s *Server) { s.principal = p }
}

// principalMiddleware stamps the server's Principal on every inbound
// request ctx. Registered once in NewServer via AddReceivingMiddleware
// so no per-handler opt-in is required (CLAUDE.md: "make the wrong thing
// impossible to write" — a new write tool added to the server inherits
// the stamp automatically).
//
// The go-sdk's middleware is method-level rather than tool-level, so
// unlike the previous ToolHandlerMiddleware this also covers resource
// and prompt handlers. That is a strict improvement: those surfaces
// read the graph too (see RR-CFFL52 / RR-NSUN49) and previously ran
// with no principal on the ctx at all.
//
// **An identity already on the ctx WINS.** Under stdio there is never
// one, so this is the stdio server's own principal in practice. Over
// HTTP (TKT-BDG8U9) the transport hands the SDK the *http.Request ctx,
// which the middleware chain has already stamped with the JWT-verified
// caller — and overwriting that with a process-wide identity would
// attribute every remote caller's writes to one principal AND hand the
// ACL the wrong subject to gate reads against. The construction-time
// principal is the fallback for a transport that carries no identity,
// not an override of one that does.
//
// NewServer guarantees s.principal is non-zero, so the fallback is
// never the zero Principal.
func (s *Server) principalMiddleware(next mcpgo.MethodHandler) mcpgo.MethodHandler {
	return func(ctx context.Context, method string, req mcpgo.Request) (mcpgo.Result, error) {
		if _, stamped := principal.Stamped(ctx); stamped {
			return next(ctx, method, req)
		}
		return next(principal.With(ctx, s.principal), method, req)
	}
}

// NewServer creates a new MCP server for a rela project. Returns an
// error if [WithPrincipal] was not supplied — silently degrading to
// `unknown/unknown` audit attribution would be an invisible
// production bug (CLAUDE.md "constructors reject nil required
// fields"). Tests must pass a non-zero Principal too — use any
// non-empty `principal.Principal{User: ..., Tool: ...}`.
func NewServer(deps Deps, version string, opts ...Option) (*Server, error) {
	s := &Server{
		deps:   deps,
		logger: slog.Default().With("component", "mcp"),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.principal.IsZero() {
		return nil, errors.New("mcp.NewServer: Principal is required (use WithPrincipal)")
	}
	if err := deps.validate(); err != nil {
		return nil, err
	}
	s.handlerSet = deps.handlers()

	// Capabilities are inferred by the go-sdk from the features actually
	// registered below (tools/resources/prompts each gain listChanged when
	// the first one is added), so there is no explicit With*Capabilities
	// equivalent to carry over.
	mcpServer := mcpgo.NewServer(
		&mcpgo.Implementation{Name: "rela", Version: version},
		&mcpgo.ServerOptions{
			Instructions: "rela is a schema-driven entity-graph platform. The domain is defined by a " +
				"YAML metamodel (entity types, relation types, properties, validation rules); " +
				"entities and relations are stored as markdown files with YAML frontmatter. " +
				"Traceability is one common use case, not the only one — the graph can model " +
				"requirements, compliance controls, project plans, issue trackers, " +
				"knowledge bases, or any typed-entity-and-relation domain. " +
				"Use tools to query, create, update, and delete entities and relations. " +
				"Use resources to read entity and metamodel data directly.",
		},
	)

	s.mcp = mcpServer
	s.mcp.AddReceivingMiddleware(s.principalMiddleware)

	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s, nil
}

// HTTPHandler returns an http.Handler serving this server over Streamable
// HTTP, for mounting inside an existing router (TKT-BDG8U9). The caller owns
// authentication, ACL and routing; this method owns only the MCP transport.
//
// **Stateless is required, not a tuning choice.** Protocol revision
// 2026-07-28 is reachable ONLY on a stateless server in the go-sdk — a
// session-bearing one negotiates down to 2025-11-25, because the newer
// revision removes sessions entirely. Consequences the caller inherits:
//
//   - GET and DELETE get 405; only POST carries messages.
//   - Server→client requests are rejected (there is no channel to answer on).
//   - Notifications reach the client only within an in-flight request.
//
// That last point is why the file watcher is pointless on this transport and
// a caller should pass a no-op [Watcher]: `resources/list_changed` has no
// stateless equivalent. Remote clients re-read on demand and see fresh data,
// because every read goes to the store.
//
// The returned handler serves THIS server for every request, so per-request
// state must travel on the ctx rather than be baked in here. That is exactly
// how identity works: the transport passes the *http.Request ctx through to
// handlers, and Server.principalMiddleware preserves a principal already
// stamped there in preference to the construction-time one.
func (s *Server) HTTPHandler() http.Handler {
	return mcpgo.NewStreamableHTTPHandler(
		func(*http.Request) *mcpgo.Server { return s.mcp },
		&mcpgo.StreamableHTTPOptions{Stateless: true},
	)
}

// Serve starts the MCP server on stdio and blocks until the peer
// disconnects or ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("starting rela MCP server on stdio")

	// Start the file watcher; MCP only cares "something changed."
	//
	// Behavior change vs mark3labs (TKT-UIR41P, documented delta): the
	// previous library exposed SendNotificationToAllClients, which this
	// callback used to push notifications/resources/list_changed on every
	// file change. The go-sdk has no equivalent — it emits list_changed
	// automatically when the resource SET changes (AddResource /
	// RemoveResources), which is a different event from "the contents
	// behind a resource template changed", and offers no exported way to
	// send an ad-hoc one.
	//
	// Resources here are a static list plus two URI templates, so the set
	// never changes at runtime; only contents do. Rather than fake a
	// set-change to trigger the notification, the callback now just logs.
	// Clients re-read on demand and see fresh data, because every read
	// goes to the store. The practical loss is that a client caching a
	// resource list is not proactively invalidated — acceptable, and
	// aligned with the direction of the 2026-07-28 spec, which requires an
	// explicit subscriptions/listen opt-in for these notifications anyway.
	if err := s.deps.Watcher.Start(func() {
		s.logger.Info("graph re-synced from file changes")
	}); err != nil {
		s.logger.Warn("file watcher not started", "error", err)
	}

	defer s.deps.Watcher.Stop()

	return s.mcp.Run(ctx, &mcpgo.StdioTransport{})
}
