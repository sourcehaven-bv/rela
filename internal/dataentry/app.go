package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// ConfigFile is the conventional filename for data-entry configuration within a rela project.
const ConfigFile = dataentryconfig.ConfigFile

// uiStateFile is the filename for persisted UI state within the .rela directory.
const uiStateFile = "ui-state.json"

// userDefaultsFile is the filename for user-specific default values within the .rela directory.
const userDefaultsFile = "user-defaults.yaml"

// userPaletteFile is the filename for user-specific palette overrides within the .rela directory.
const userPaletteFile = "palette.yaml"

// App is the central application struct for the data-entry server.
//
// # Concurrency model
//
// The co-derived reload core (config, metamodel, style map, OpenAPI
// generator) lives in an immutable [Schema] published via [schemaProvider]'s
// atomic.Pointer. Handlers call a.State() once at entry and work against a
// coherent snapshot for the duration of the request — no lock acquisition, no
// risk of observing a half-reloaded world. Independently-owned reloadable
// state (logo, palette, user defaults) lives in its own self-synchronized
// service, not the snapshot.
//
// Reloads (triggered by the file watcher or by Reload) derive a new Schema
// and publish it atomically via a.schema.Reload. The previous snapshot is
// garbage-collected once no reader holds it.
//
// Mutations (CreateEntity, UpdateEntity, DeleteEntity, CreateRelation,
// UpdateRelation, DeleteRelation, SetProperty, action scripts) serialize
// via writeMu. writeMu excludes concurrent mutations but does NOT block
// readers — readers go through a.State(). The workspace's internal
// reloadMu coordinates the reload itself with the mutation path.
//
// TODO(TKT-R68TV8): App is a god-object. Decompose toward the
// 40-method load line — extract the API/serialization/relation services into
// their own types. Ratchet this number DOWN as methods move out; never up
// EXCEPT for a new required route handler (App owns one method per registered
// HTTP route by the router's design). The sync route cluster (16 methods) moved
// to syncHandler (170 → 154); the command cluster (11 methods) moved to
// commandHandler (154 → 143); the attachment cluster (12 methods) moved to
// attachmentHandler / package functions (143 → 131); the write nucleus —
// entity/relation CRUD, clone, conflict-resolve, and the modern relations
// reconciler (18 methods) — moved to writeHandler (131 → 114).
//
//plimsoll:max-methods=115
type App struct {
	// Primitives — immutable after NewApp.
	fs    storage.FS
	paths *project.Context

	// Core services. Some are passed in (store, entityManager,
	// searcher); the rest are constructed from primitives inside
	// NewApp.
	store store.Store
	// versions is the content-versioning service (entity + relation history
	// reads), a pgstore-only injected concern — nil on fs/mem builds, where the
	// history endpoints return 501. The history handlers bind the narrow
	// sub-interface they need rather than type-asserting the store.
	versions      store.VersionService
	entityManager entitymanager.EntityManager
	searcher      search.Searcher
	// visibleSearcher is the ACL-scoped search seam (TKT-BA8BSX):
	// executeQuery routes free-text searches through it so /_search
	// and the _position search scope only ever see hits the request
	// principal may read. Per-backend wiring: search.NewVisible over
	// the regular searcher on fs/memory builds, pgstore-native on the
	// postgres build.
	visibleSearcher search.VisibleSearcher
	// visibleReader is the ACL-bounded entity-read seam (TKT-N26KLB): the
	// entity-read analog of visibleSearcher. Read handlers gate single-GET
	// and include-filtering through it so the read gate is applied
	// structurally rather than by per-call-site convention. Wraps the same
	// `store` handle; the gate is resolved per-request from the context.
	visibleReader visibleReader
	// reader is the ungated entity/relation read seam over the store. Extracted
	// from App (TKT-N26KLB); a single-dep leaf shared by read/write/affordance
	// paths. ACL scoping lives in visibleReader, not here.
	reader    entityReader
	tracer    tracer.Tracer
	validator validator.Validator
	// analyze runs the read-only graph-analysis checks. Extracted from App
	// (TKT-N26KLB M5.1); holds its own {store, tracer, validator} and takes
	// the metamodel snapshot per call.
	analyze analyzeService
	// affordances computes the _actions/field/relation affordance maps and runs
	// write-time affordance validation. Extracted from App (TKT-N26KLB M5.2);
	// shares the same acl.ACL as the write path (contract-test invariant).
	affordances affordanceService
	// serializer renders an entity into its v1.Entity wire shape. Extracted from
	// App (TKT-N26KLB); pure transform — handlers pass the entity's already-
	// loaded outgoing relations, the serializer does no loading.
	serializer entitySerializer
	// userState persists per-user UI state (UI state, defaults, palette)
	// to the .rela/ KV store. Extracted from App (TKT-N26KLB M5.3).
	userState userStateStore
	// logo owns the user-uploaded sidebar logo — persistence AND the served
	// in-memory cache — self-synchronized. Extracted from the schema snapshot so the
	// logo no longer rides the App-wide snapshot + writeMu.
	logo *logoStore
	// palette owns the user palette override and the resolved palette (derived
	// from Cfg.Palette + the override). Self-synchronized; extracted from
	// Schema. The reload/save paths hand it the current Cfg.Palette so it can
	// recompute — see paletteService.Reresolve.
	palette *paletteService
	// settings owns the per-user default values (create-form/relation defaults).
	// Self-synchronized; extracted from the schema snapshot.
	settings *settingsService
	// sync owns the /api/sync/ route cluster (fs-client ↔ pg-server
	// replication). Extracted from App (TKT-R68TV8); holds narrow store/deleter
	// surfaces plus a pointer to writeMu so its writes serialize with the other
	// mutation handlers.
	sync *syncHandler
	// commands owns the user-configured command surface (SSE shell-exec,
	// file/URL launchers, command resolution). Extracted from App (TKT-R68TV8);
	// holds narrow closures over the schema snapshot, Services bundle, project
	// root, and the view executor.
	commands *commandHandler
	// attachments owns the entity-attachment routes (upload/download/detach).
	// Extracted from App (TKT-R68TV8); closures over the swappable acl/audit/
	// field-resolver collaborators, a pointer to writeMu for the write paths.
	attachments *attachmentHandler
	// export owns the view-export routes (transform list, entity/list export).
	// Extracted from App (TKT-JF5JI8) to keep App under its plimsoll method cap.
	export *exportHandler

	// write owns the entity/relation CRUD + clone + conflict-resolve write
	// nucleus (TKT-R68TV8 M5.4); shares writeMu by pointer.
	write     *writeHandler
	templater templating.Templater
	cfgLoader config.Loader
	kv        state.KV
	acl       acl.ACL

	// attachmentRunner drives external scan/transform commands for uploads.
	// nil out-of-box → uploads get native MIME validation only (Phase 2 wires
	// the cmd: harness). See internal/attachment.PolicyProcessor.
	attachmentRunner attachment.CommandRunner

	// documents renders and caches documents. Created once in NewApp so
	// singleflight deduplication is stable across requests.
	documents *documentService

	// scriptEngine is the long-lived Lua script engine used for action
	// execution. Holding one per App (rather than per-request) means
	// every request shares the same rela.cache state, which is the
	// whole point of having a cache in a long-lived server.
	scriptEngine *script.Engine

	// schema publishes the current reloadable co-derived core (config,
	// metamodel, style map, OpenAPI generator). Readers: a.State(). Writers:
	// the watcher's reload path (rebuildState → schema.Reload). Initial
	// snapshot is published in NewApp.
	schema schemaProvider

	// writeMu serializes mutation handlers (CreateEntity, UpdateEntity,
	// etc.) against each other. Readers never take it.
	writeMu sync.Mutex

	// gitOps provides git operations when git is enabled. Set once in
	// NewApp; never reloaded.
	gitOps *git.Ops

	// broker delivers SSE events to connected browsers for live-reload.
	broker *eventBroker

	// stopConfigWatch releases the data-entry.yaml subscription. Set by
	// StartWatching; nil when watching is not active.
	stopConfigWatch func()

	// stopStoreWatch cancels the store-event -> SSE bridge subscription. Set by
	// StartWatching; nil when watching is not active.
	stopStoreWatch func()

	// security holds the configured Host/Origin allowlists. Set via
	// SetSecurityConfig before NewRouter; nil disables the middlewares
	// (only sensible in unit tests where no HTTP layer is exercised).
	security *security

	// principalResolver is the per-request audit Principal resolver.
	// Set via SetPrincipalResolver before NewRouter; nil falls back
	// to defaultPrincipalResolver (Tool=data-entry, User=unknown).
	// cmd/rela-server chains an env resolver + a header resolver
	// here when --principal-header is set.
	principalResolver PrincipalResolver

	// jwtGate, when non-nil, enforces fail-closed verified-JWT identity on
	// the data API: every /api/ request must carry a valid assertion or is
	// denied 401. Set via SetJWTGate before NewRouter. Mutually exclusive
	// with a header/env principal chain — cmd/rela-server refuses to start
	// with both, so a JWT failure can never downgrade to a spoofable header.
	jwtGate *JWTGateConfig

	// principalHeader is the name of the HTTP header that carries the
	// principal identity (the --principal-header flag value), or ""
	// when no header is configured. Used by noCacheMiddleware to emit
	// `Vary: <header>` on /api/ responses — under ACL those responses
	// are per-principal, and a shared cache keyed only on the URL
	// would serve principal A's filtered list to principal B
	// (TKT-VMD8 AC10, RR-VDTW). Set via SetPrincipalHeader before
	// NewRouter.
	principalHeader string

	// fieldResolver decides per-entity field, option, and
	// relation-meta affordances surfaced as `_fields` / `_relations`
	// on the wire and enforced on writes. Required (never nil) —
	// callers that don't want affordances pass NopFieldVerdictResolver{}.
	// The eventual predicate-engine ticket replaces the stub
	// implementations with a policy-driven resolver via the same
	// interface.
	fieldResolver FieldVerdictResolver

	// webhook holds the inbound-IdP webhook receiver (verifier + target action +
	// dedup). nil until SetWebhookReceiver is called, in which case the
	// /webhooks/idp route is not mounted. Optional wiring, like security and
	// principalResolver above.
	webhook *webhookReceiver

	// auditSink records short-circuit rejections (affordance gates)
	// that never reach the entitymanager. ACL denials already get a
	// `denied-write` row from the manager; affordance denials emit
	// the same op via this sink so log readers see a unified stream.
	// Required (never nil) — callers pass [audit.Nop] to opt out.
	auditSink audit.Audit
}

// StopWatching releases the data-entry.yaml subscription started by
// [App.StartWatching]. The store-level watcher (when present) has its
// own lifecycle managed by the store and is stopped during store
// close, not here — asymmetric on purpose: dataentry doesn't own the
// store, only its config subscription.
// StopWatching is lifecycle-only and must be called from a single goroutine
// (it is the StartWatching counterpart). The stop fields are not synchronized;
// concurrent Start/Stop is not supported.
func (a *App) StopWatching() {
	if a.stopConfigWatch != nil {
		a.stopConfigWatch()
		a.stopConfigWatch = nil
	}
	if a.stopStoreWatch != nil {
		a.stopStoreWatch()
		a.stopStoreWatch = nil
	}
}

// State returns the current reloadable [Schema] snapshot. Handlers should call
// State() once at entry and use the returned snapshot consistently
// throughout, instead of making multiple calls that could see different
// snapshots after a concurrent reload.
func (a *App) State() *Schema { return a.schema.Current() }

// Cfg returns the current data-entry config (convenience accessor).
// Equivalent to a.State().Cfg.
func (a *App) Cfg() *Config { return a.State().Cfg }

// Meta returns the current metamodel (convenience accessor).
func (a *App) Meta() *metamodel.Metamodel { return a.State().Meta }

// luaWriteDeps builds a lua.WriteDeps bundle using the current Schema
// metamodel. Called per action-script invocation so that metamodel reloads
// propagate to scripts without requiring app reconstruction. All other
// fields are immutable for the App's lifetime.
// Reads are ACL-BOUND to the request principal (DEC-O59WM4): an action
// script, an export_render override, or an MCP-invoked script sees exactly
// the caller's view — hidden entities absent, hidden properties redacted.
// Identity resolves per call from the ctx, so one bundle serves every
// request. WritePrepStore stays RAW so update_entity's read-before-write
// cannot erase hidden properties (see lua.ReadDeps.WritePrepStore).
func (a *App) luaWriteDeps() lua.WriteDeps {
	redactor := affRedactor{aff: func() affordanceService { return a.affordances }}
	return lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			VisibleReader:  a.scriptReader(redactor),
			WritePrepStore: a.store,
			Tracer:         a.scriptTracer(redactor),
			Searcher:       a.searcher,
			Meta:           a.Meta(),
			ProjectRoot:    a.paths.Root,
		},
		EntityManager: a.entityManager,
	}
}

// scriptReader returns the ACL-bound read-out handle for script runtimes,
// or the raw store when no Declarative policy is configured (the NopACL
// path — byte-identical to pre-ACL behavior). A construction fault
// degrades to the raw store with a warning rather than breaking every
// script; a genuine DENY is still a deny.
func (a *App) scriptReader(redactor visibility.FieldRedactor) lua.EntityReader {
	d, ok := a.acl.(*acl.Declarative)
	if !ok || d == nil {
		return a.store
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		slog.Warn("dataentry: ACL gate unavailable; script reads stay unrestricted", "err", err)
		return a.store
	}
	reader, err := visibility.NewPolicyReader(gate, redactor, a.store)
	if err != nil {
		slog.Warn("dataentry: policy reader unavailable; script reads stay unrestricted", "err", err)
		return a.store
	}
	sr, err := visibility.NewScriptReader(reader, a.store, gate)
	if err != nil {
		slog.Warn("dataentry: script reader unavailable; script reads stay unrestricted", "err", err)
		return a.store
	}
	return sr
}

// scriptTracer wraps the tracer in the visibility decorator when a
// Declarative policy is configured. Trace bindings are unchanged either
// way — pruning happens inside the decorator.
func (a *App) scriptTracer(redactor visibility.FieldRedactor) tracer.Tracer {
	d, ok := a.acl.(*acl.Declarative)
	if !ok || d == nil {
		return a.tracer
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		slog.Warn("dataentry: ACL gate unavailable; traversal stays unrestricted", "err", err)
		return a.tracer
	}
	vt, err := visibility.NewVisibleTracer(a.tracer, gate, redactor, a.store)
	if err != nil {
		slog.Warn("dataentry: visible tracer unavailable; traversal stays unrestricted", "err", err)
		return a.tracer
	}
	return vt
}

// SetSecurityConfig configures the HTTP security middlewares applied by
// NewRouter. It must be called before NewRouter.
func (a *App) SetSecurityConfig(cfg SecurityConfig) error {
	s, err := newSecurity(cfg)
	if err != nil {
		return err
	}
	a.security = s
	return nil
}

// SetPrincipalResolver installs a custom [PrincipalResolver] used by
// the router's audit-stamp middleware. Must be called before
// [App.NewRouter]; subsequent changes have no effect on already-built
// routers.
//
// The typical wiring (in cmd/rela-server) chains
// [EnvPrincipalResolver] and [HeaderPrincipalResolver] so a
// `$RELA_DATAENTRY_USER` env var overrides any incoming header and
// the header itself overrides the default. Passing nil restores
// [defaultPrincipalResolver] behavior.
func (a *App) SetPrincipalResolver(r PrincipalResolver) {
	a.principalResolver = r
}

// SetPrincipalHeader records the name of the HTTP header that carries
// the principal identity so API responses can declare `Vary` on it.
// Call alongside [App.SetPrincipalResolver] (before [App.NewRouter])
// when wiring a [HeaderPrincipalResolver]; leave unset otherwise.
func (a *App) SetPrincipalHeader(name string) {
	a.principalHeader = name
}

// SetJWTGate enables fail-closed verified-JWT identity. Must be called before
// [App.NewRouter]; subsequent changes have no effect on already-built routers.
//
// When set, every [isAPIPath] request must carry an assertion that verifies, or
// it is denied 401 — see [requireVerifiedJWT]. This REPLACES the principal
// resolver for API requests rather than layering over it: JWT identity is
// exclusive, so there is no header or env source to fall back to. Callers must
// not also install a header/env chain via [App.SetPrincipalResolver];
// cmd/rela-server enforces that at startup.
//
// Returns an error when a required field is missing rather than accepting a
// config that cannot work. An empty HeaderName is fatal in a quiet way — every
// assertion would read as absent, so the server would boot clean and then 401
// every API request.
//
// Note the interface-nil caveat: a TYPED nil (e.g. a (*jwtauth.Verifier)(nil)
// stored in the interface) is not == nil and cannot be caught here without
// reflection. Callers must not construct one; cmd/rela-server checks the
// concrete pointer before it ever reaches this interface.
func (a *App) SetJWTGate(cfg JWTGateConfig) error {
	if cfg.Verifier == nil {
		return errors.New("dataentry: jwt gate requires a non-nil Verifier")
	}
	if cfg.HeaderName == "" {
		return errors.New("dataentry: jwt gate requires a non-empty HeaderName")
	}
	a.jwtGate = &cfg
	return nil
}

// NewApp creates and initializes an App. Callers pass in the
// primitives (fs, paths, meta, store) plus the services that depend
// on workspace assembly: entityManager (the production write path)
// and searcher (the live Bleve index). Everything else — state.KV,
// config.Loader, tracer, templater, validator — is constructed
// locally.
//
// The store-level file watcher (live-reload of external entity /
// relation edits) is feature-detected on `st` inside
// [App.StartWatching] via the [storeWatcher] interface; callers do
// not wire it.
//
//nolint:gocognit,funlen // composition root: validates and wires many optional collaborators, each guarded independently; the branches are wiring steps, not shared logic to extract.
func NewApp(
	fs storage.FS,
	paths *project.Context,
	meta *metamodel.Metamodel,
	st store.Store,
	versions store.VersionService,
	em entitymanager.EntityManager,
	searcher search.Searcher,
	visibleSearcher search.VisibleSearcher,
	aclImpl acl.ACL,
	fieldResolver FieldVerdictResolver,
	auditSink audit.Audit,
) (*App, error) {
	// Reject nil required collaborators up front rather than letting a
	// downstream handler panic on the first request that exercises them.
	// fs and paths can also be nil in tests that take a different code path
	// (newAppFromParts wires them post-construction), so they're checked
	// only when they participate in the construction below.
	if meta == nil {
		return nil, errors.New("dataentry.NewApp: meta is required")
	}
	if st == nil {
		return nil, errors.New("dataentry.NewApp: store is required")
	}
	if em == nil {
		return nil, errors.New("dataentry.NewApp: entityManager is required")
	}
	if searcher == nil {
		return nil, errors.New("dataentry.NewApp: searcher is required")
	}
	if visibleSearcher == nil {
		return nil, errors.New("dataentry.NewApp: visibleSearcher is required (wire appbuild's Services.VisibleSearcher)")
	}
	if aclImpl == nil {
		return nil, errors.New("dataentry.NewApp: acl is required (use acl.NopACL{} to opt out)")
	}
	if fieldResolver == nil {
		return nil, errors.New("dataentry.NewApp: fieldResolver is required (pass NopFieldVerdictResolver{} for permissive default)")
	}
	if auditSink == nil {
		return nil, errors.New("dataentry.NewApp: auditSink is required (pass audit.Nop{} to opt out)")
	}
	// Construct reconstructible services from the primitives.
	cfgLoader := config.NewFSLoader(fs, paths.Root)
	kvRoot, err := storage.NewRootedFS(fs, paths.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("dataentry: rooted fs for state kv: %w", err)
	}
	kv := state.NewFSKV(kvRoot)
	trc := tracer.New(st)
	templater := templating.NewFSTemplater(fs, paths)
	// VALIDATOR: unrestricted reads, deliberately (DEC-O59WM4).
	//
	// The entity being validated does NOT come through this bundle — the
	// validator loads it itself and passes it in as the `entity` global, so
	// redacting here would not protect it anyway. What this bundle serves
	// is a rule body's incidental cross-entity lookups, and redacting THOSE
	// manufactures false violations: a rule asserting "every ticket links
	// to a project" would fire on tickets whose project the current
	// principal cannot see. Same reasoning the validator already applies to
	// locked/unreadable entities, which it skips rather than mis-validates.
	readDeps := lua.ReadDeps{
		VisibleReader:  st,
		WritePrepStore: st,
		Tracer:         trc,
		Searcher:       searcher,
		Meta:           meta,
		ProjectRoot:    paths.Root,
	}
	val := validator.New(st, meta, readDeps)

	// Load data-entry config from project root
	cfgData, err := cfgLoader.Load(context.Background(), ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	// Check for deprecated syntax that needs migration
	configPath := filepath.Join(paths.Root, ConfigFile)
	detections := migration.DetectBytes(cfgData, migration.FileTypeDataEntry)
	if len(detections) > 0 {
		return nil, &migration.Error{
			FilePath:   configPath,
			Detections: detections,
		}
	}

	var cfg Config
	if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigFile, unmarshalErr)
	}

	// Validate config against metamodel
	if validationErr := ValidateConfig(cfgData, &cfg, meta); validationErr != nil {
		return nil, fmt.Errorf("invalid %s: %w", ConfigFile, validationErr)
	}

	// Non-fatal configuration warnings (e.g. a relation filter control whose
	// incoming direction targets a type the relation never points to). Logged,
	// not fatal — the app still serves, the filter just returns no rows.
	for _, w := range CollectConfigWarnings(&cfg, meta) {
		slog.Warn("data-entry config warning", "detail", w)
	}

	// Verify action scripts exist on disk (catches typos at startup).
	// Skip set-only actions which have no script.
	for id, action := range cfg.Actions {
		if action.Script == "" {
			continue
		}
		if err := script.CheckActionScriptExists(paths.Root, action.Script); err != nil {
			return nil, fmt.Errorf("invalid %s: action %q: %w", ConfigFile, id, err)
		}
	}

	// Verify document scripts exist on disk. Shell-command documents are
	// not checkable this way (the binary may be on PATH at render time
	// but unavailable now); Lua scripts live in scripts/ under the
	// project root so existence can be verified upfront.
	for id, doc := range cfg.Documents {
		if doc.Script == "" {
			continue
		}
		if err := script.CheckDocumentScriptExists(paths.Root, doc.Script); err != nil {
			return nil, fmt.Errorf("invalid %s: document %q: %w", ConfigFile, id, err)
		}
	}

	entCount, _ := st.CountEntities(context.Background(), store.EntityQuery{})
	relCount, _ := st.CountRelations(context.Background(), store.RelationQuery{})
	slog.Info("loaded project", "entities", entCount, "relations", relCount)

	// Build style map from config styles
	styleMap, styledTypes := buildStyleMap(&cfg, meta)

	scriptEngine := script.NewEngine()
	app := &App{
		fs:              fs,
		paths:           paths,
		store:           st,
		versions:        versions,
		entityManager:   em,
		searcher:        searcher,
		visibleSearcher: visibleSearcher,
		visibleReader:   newVisibleReader(st),
		reader:          entityReader{store: st},
		tracer:          trc,
		validator:       val,
		analyze:         analyzeService{store: st, tracer: trc, validator: val},
		templater:       templater,
		cfgLoader:       cfgLoader,
		kv:              kv,
		userState:       userStateStore{kv: kv},
		acl:             aclImpl,
		broker:          newEventBroker(),
		scriptEngine:    scriptEngine,
		fieldResolver:   fieldResolver,
		auditSink:       auditSink,
	}
	// documentService needs scriptEngine (for Lua renders) and a closure
	// that yields fresh lua.WriteDeps (so metamodel reloads propagate).
	// Constructed after app because luaWriteDeps is a method on App.
	app.documents = newDocumentService(st, kv, paths.Root, scriptEngine, app.luaWriteDeps)

	// affordanceService shares the App's acl/fieldResolver/store and takes the
	// metamodel per-request via app.State(). The two relation-graph reads are
	// App methods, so it's wired after the struct literal. It MUST share the
	// same acl instance as the write path (contract-test invariant).
	app.affordances = affordanceService{
		acl:                func() acl.ACL { return app.acl },
		resolver:           func() FieldVerdictResolver { return app.fieldResolver },
		store:              st,
		meta:               func() *metamodel.Metamodel { return app.State().Meta },
		getEntity:          app.reader.getEntity,
		currentEdgesByPeer: app.currentEdgesByPeer,
	}

	app.serializer = entitySerializer{affordances: app.affordances}

	app.settings = newSettingsService(kv)

	// palette owns the user override + resolved palette. Surface a broken
	// palette rather than silently falling back to defaults (which the next
	// save would then persist, destroying the user's data).
	palette, paletteErr := newPaletteService(kv, cfg.Palette)
	if paletteErr != nil {
		return nil, fmt.Errorf("load user palette: %w", paletteErr)
	}
	app.palette = palette

	// logo owns its own persistence + served cache. Same read-error policy as
	// palette: surface a corrupt .rela/theme/ rather than silently overwriting
	// it on the next save.
	logo, logoErr := newLogoStore(kv)
	if logoErr != nil {
		return nil, fmt.Errorf("load user logo: %w", logoErr)
	}
	app.logo = logo

	// syncHandler owns the /api/sync/ route cluster (fs-client ↔ pg-server
	// replication). It shares App's store (reads), entityManager (deletes), and
	// — crucially — a POINTER to App's writeMu so sync pushes/deletes serialize
	// against every other data-entry mutation. The manifest/applier capabilities
	// are resolved once from the concrete store/manager (nil on fs/memory builds,
	// where the sync endpoints degrade to 501).
	app.sync = newSyncHandler(st, app.entityManager, &app.writeMu)

	// commandHandler owns the user-configured command surface. Its
	// collaborators are narrow closures over App: the schema snapshot (command/
	// list/view config), the Services read bundle, the project root (exec cwd +
	// env), and the view executor for view-context commands.
	app.commands = &commandHandler{
		schema:      app.State,
		services:    app.Services,
		projectRoot: app.ProjectRoot,
		executeView: app.executeView,
		// Late-bound: tests reassign app.acl after construction.
		aclImpl: func() acl.ACL { return app.acl },
	}

	// Build and publish the initial Schema snapshot. All reloadable
	// state lives here; there are no convenience aliases on App to keep
	// in sync.
	app.schema.Publish(&Schema{
		Cfg:         &cfg,
		Meta:        meta,
		StyleMap:    styleMap,
		StyledTypes: styledTypes,
		OpenAPIGen: openapi.New(meta, openapi.Config{
			Title:       cfg.App.Name + " API",
			Description: cfg.App.Description,
			Version:     "1.0.0",
		}),
	})

	// Initialize git ops if enabled and repo is a git repository
	if cfg.Git != nil && cfg.Git.Enabled && git.IsRepo(paths.Root) {
		app.gitOps = git.NewOps(paths.Root, *cfg.Git)
		slog.Info("git sync enabled", "mode", cfg.Git.Mode)
	}

	// Wire the external-command runner for attachment scan/transform. It is
	// always available; the PolicyProcessor only invokes it when a property's
	// scan/transform config references a command. A nil runner (constructor
	// failure) leaves uploads with native MIME validation only.
	var runnerOpts []attachment.CmdRunnerOption
	if socks := metamodel.NewAttachmentPolicy(meta).ScanSockets(); len(socks) > 0 {
		runnerOpts = append(runnerOpts, attachment.WithScannerSockets(socks...))
	}
	runner, rerr := attachment.NewCmdRunner(attachmentCmdTimeout, store.MaxAttachmentBytes, runnerOpts...)
	if rerr == nil {
		app.attachmentRunner = runner
		// Tell the operator the confinement posture at boot, so an unsandboxable
		// host is discovered now rather than on the first upload that needs a
		// scan/transform (which will fail closed).
		slog.Info("external command confinement", "detail", runner.Describe())
		probeAttachmentCommands(meta, runner)
	} else {
		slog.Warn("attachments: command runner unavailable; scan/transform disabled", "err", rerr)
	}

	// exportHandler owns the view-export routes (transform list, entity/list
	// export). Extracted from App to keep App under its method cap. Probe the
	// transforms at startup so a missing converter (e.g. pandoc not installed)
	// surfaces as a boot warning rather than a 500 on the first export.
	export, exportErr := newExportHandler(app)
	if exportErr != nil {
		return nil, exportErr
	}
	app.export = export
	app.export.probeTransforms()

	// attachmentHandler owns the entity-attachment routes. Constructed after
	// the runner wiring above so it captures the resolved runner. The acl/
	// audit/field-resolver deps are closures because tests swap those fields
	// on App after construction (same rationale as affordanceService); the
	// store/manager handles are fixed for App's lifetime. writeMu is shared by
	// pointer so attachment writes serialize with every other mutation handler.
	app.attachments = &attachmentHandler{
		schema:     app.State,
		store:      st,
		manager:    app.entityManager,
		runner:     func() attachment.CommandRunner { return app.attachmentRunner },
		reader:     app.reader,
		serializer: app.serializer,
		acl:        func() acl.ACL { return app.acl },
		audit:      func() audit.Audit { return app.auditSink },
		fields:     func() FieldVerdictResolver { return app.fieldResolver },
		gateRead:   app.gateReadOrNotFound,
		writeMu:    &app.writeMu,
	}

	// writeHandler owns the entity/relation CRUD + clone + conflict-resolve
	// nucleus. Same collaborator rationale as attachmentHandler above: fixed
	// services by value, test-swappable deps as closures over App, and the
	// shared read/write helpers (gateRead/denyAfford/computeETag) as closures
	// so both paths stay behaviorally identical. writeMu is shared by pointer
	// so these writes serialize with every other mutation handler.
	app.write = &writeHandler{
		schema:             app.State,
		store:              st,
		manager:            app.entityManager,
		reader:             app.reader,
		serializer:         app.serializer,
		affordances:        app.affordances,
		acl:                func() acl.ACL { return app.acl },
		audit:              func() audit.Audit { return app.auditSink },
		gateRead:           app.gateReadOrNotFound,
		denyAfford:         app.denyAffordance,
		computeETag:        app.computeEntityETag,
		currentEdgesByPeer: app.currentEdgesByPeer,
		paths:              paths,
		writeMu:            &app.writeMu,
	}

	// Nudge the operator to make a conscious virus-scan choice: if the
	// metamodel has file properties with no scan command configured (and no
	// explicit `scan: off`), warn once. Configuring a scan_cmd or setting
	// `scan: off` silences this. The warning never blocks startup or uploads.
	if metamodel.NewAttachmentPolicy(meta).HasUnconfiguredScan() {
		slog.Warn("attachments: no virus scanner configured for file properties; "+
			"set attachments.scan_cmd to enable scanning, or `scan: off` on the property to silence this",
			"docs", "docs/attachment-security.md")
	}

	return app, nil
}

// NavItem is an enriched navigation entry that includes the entity type for client-side matching.
type NavItem struct {
	Label      string
	List       string
	Dashboard  bool
	Kanban     string
	EntityType string
	Count      int
}

// NavGroup is an enriched navigation group containing resolved nav items.
type NavGroup struct {
	Group     string
	Collapsed bool
	Items     []NavItem
}

// NavElement is a union of either a direct NavItem or a NavGroup.
// Exactly one of Item or Group is non-nil.
type NavElement struct {
	Item  *NavItem
	Group *NavGroup
}

// enrichNavEntry resolves a single NavigationEntry into a NavItem with entity type and count.
func (a *App) enrichNavEntry(ctx context.Context, nav NavigationEntry) NavItem {
	item := NavItem{Label: nav.Label, List: nav.List, Dashboard: nav.Dashboard, Kanban: nav.Kanban}
	if nav.Dashboard || nav.Kanban != "" {
		return item
	}
	s := a.State()
	if list, ok := s.Cfg.Lists[nav.List]; ok {
		item.EntityType = list.EntityType
		entities := listFromStoreByTypes(ctx, a.Services(), []string{list.EntityType})
		entities = applyFilters(entities, list.Filters)
		item.Count = len(entities)
	}
	return item
}

// navElements returns the navigation structure with groups and items resolved.
// The activeList parameter is used to auto-expand the group containing the active item.
func (a *App) navElements(ctx context.Context, activeList string) []NavElement {
	uiState := a.userState.loadUIState(ctx)
	cfgNav := a.State().Cfg.Navigation
	elements := make([]NavElement, 0, len(cfgNav))
	for _, nav := range cfgNav {
		if nav.IsGroup() {
			grp := NavGroup{Group: nav.Group}
			// Determine collapsed state: UIState overrides config default
			if override, ok := uiState.CollapsedGroups[nav.Group]; ok {
				grp.Collapsed = override
			} else {
				grp.Collapsed = nav.Collapsed
			}
			grp.Items = make([]NavItem, len(nav.Items))
			for i, child := range nav.Items {
				grp.Items[i] = a.enrichNavEntry(ctx, child)
				// Auto-expand group if it contains the active list
				if child.List == activeList && activeList != "" {
					grp.Collapsed = false
				}
			}
			elements = append(elements, NavElement{Group: &grp})
		} else {
			item := a.enrichNavEntry(ctx, nav)
			elements = append(elements, NavElement{Item: &item})
		}
	}
	return elements
}

// coverage-ignore: requires running workspace, tested via e2e

// firstNavTarget returns the first navigable item from the navigation config,
// walking into groups as needed.
func firstNavTarget(nav []NavigationEntry) *NavigationEntry {
	for i := range nav {
		if nav[i].IsGroup() {
			if target := firstNavTarget(nav[i].Items); target != nil {
				return target
			}
			continue
		}
		return &nav[i]
	}
	return nil
}

// editFormForType returns the first edit form ID configured for the given entity type,
// or "" if no edit form is found. Forms with explicit mode="edit" are preferred.
func (a *App) editFormForType(entityType string) string {
	s := a.State()
	ids := make([]string, 0, len(s.Cfg.Forms))
	for id := range s.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	// First pass: look for explicit edit mode
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType == entityType && f.Mode == "edit" {
			return id
		}
	}
	// Second pass: fall back to forms with no mode specified
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType == entityType && f.Mode == "" {
			return id
		}
	}
	return ""
}

// createFormForType returns the first form ID that can be used to create an entity
// of the given type. It prefers forms with mode "create" or unset, but falls back
// to edit-mode forms (which work for creation when no entity ID is provided).
func (a *App) createFormForType(entityType string) string {
	s := a.State()
	ids := make([]string, 0, len(s.Cfg.Forms))
	for id := range s.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	fallback := ""
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType != entityType {
			continue
		}
		if f.Mode != "edit" {
			return id
		}
		if fallback == "" {
			fallback = id
		}
	}
	return fallback
}

// resolveLinkTarget resolves a link configuration value to a URL.
// Supported values:
//   - "" or empty: no link (returns "")
//   - "detail": link to entity detail view (/entity/{type}/{id})
//   - "document/<name>": link to document preview (/document/<name>/{id})
func (a *App) resolveLinkTarget(link, entityType, entityID string) string {
	switch {
	case link == "":
		return ""
	case link == "detail":
		return "/entity/" + entityType + "/" + entityID
	case strings.HasPrefix(link, "document/"):
		docName := strings.TrimPrefix(link, "document/")
		return "/document/" + docName + "/" + entityID
	default:
		return ""
	}
}

// activeListForEntityType returns the first navigation list ID whose entity type
// matches the given type, or "" if none match. Walks into groups.
func (a *App) activeListForEntityType(entityType string) string {
	s := a.State()
	return a.findListByEntityType(s, s.Cfg.Navigation, entityType)
}

func (a *App) findListByEntityType(s *Schema, entries []NavigationEntry, entityType string) string {
	for _, nav := range entries {
		if nav.IsGroup() {
			if found := a.findListByEntityType(s, nav.Items, entityType); found != "" {
				return found
			}
			continue
		}
		if list, ok := s.Cfg.Lists[nav.List]; ok && list.EntityType == entityType {
			return nav.List
		}
	}
	return ""
}

// activeListFromReferer extracts a list ID from the Referer header path
// (e.g. "/list/tickets" -> "tickets"). Returns "" if the referer doesn't
// point to a known list.
func (a *App) activeListFromReferer(r *http.Request) string {
	ref := r.Header.Get("Referer")
	if ref == "" {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	path := parsed.Path
	if !strings.HasPrefix(path, "/list/") {
		return ""
	}
	listID := strings.TrimPrefix(path, "/list/")
	if _, ok := a.State().Cfg.Lists[listID]; ok {
		return listID
	}
	return ""
}

// resolveActiveList returns the best active list for the sidebar.
// It first checks for an explicit "from" query parameter (set when navigating
// from a list), then tries matching by entity type, then falls back to the
// Referer header.
func (a *App) resolveActiveList(entityType string, r *http.Request) string {
	if from := r.URL.Query().Get("from"); from != "" {
		if _, ok := a.State().Cfg.Lists[from]; ok {
			return from
		}
	}
	if active := a.activeListForEntityType(entityType); active != "" {
		return active
	}
	return a.activeListFromReferer(r)
}

// ProjectName returns the display name of the loaded project.
func (a *App) ProjectName() string {
	return a.State().Cfg.App.Name
}

// ProjectRoot returns the root directory of the loaded project.
func (a *App) ProjectRoot() string {
	return a.paths.Root
}

// colorToCSSClass maps a color name from config to a CSS class.
var colorToCSSClass = map[string]string{
	"blue":   "badge-blue",
	"purple": "badge-purple",
	"green":  "badge-green",
	"gray":   "badge-gray",
	"red":    "badge-red",
	"orange": "badge-orange",
	"yellow": "badge-yellow",
}

// autoColors assigns colors to enum values that have no explicit style.
var autoColors = []string{"blue", "purple", "green", "orange", "yellow", "red", "gray"}

func buildStyleMap(
	cfg *Config, meta *metamodel.Metamodel,
) (styleMap map[string]map[string]string, styledTypes map[string]bool) {
	sm := make(map[string]map[string]string)
	st := make(map[string]bool)

	// Populate from explicit config styles
	for typeName, valueColors := range cfg.Styles {
		sm[typeName] = make(map[string]string)
		st[typeName] = true
		for val, color := range valueColors {
			if cls, ok := colorToCSSClass[color]; ok {
				sm[typeName][val] = cls
			} else {
				sm[typeName][val] = "badge-gray"
			}
		}
	}

	// Auto-assign styles for custom types not already styled
	for typeName, ct := range meta.Types {
		if _, alreadyStyled := sm[typeName]; alreadyStyled {
			continue
		}
		sm[typeName] = make(map[string]string)
		st[typeName] = true
		for i, val := range ct.Values {
			sm[typeName][val] = colorToCSSClass[autoColors[i%len(autoColors)]]
		}
	}

	return sm, st
}
