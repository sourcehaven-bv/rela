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
	"sync/atomic"

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
)

// AppState bundles the reloadable fields of App into an immutable snapshot.
//
// During this PR, App still holds the same fields as plain struct fields
// (Cfg, meta, g, styleMap, styledTypes, userDefaults, palette, userPalette,
// openAPIGen) and the AppState is published in parallel via atomic.Pointer.
// Handlers will be migrated to read from the snapshot in a subsequent PR.
//
// The fields here mirror the workspace.workspaceState pattern: callers
// Load() once and work against a coherent snapshot, instead of holding a
// read lock around the entire request.
type AppState struct {
	Cfg          *Config
	Meta         *metamodel.Metamodel
	StyleMap     map[string]map[string]string
	StyledTypes  map[string]bool
	UserDefaults *UserDefaults
	Palette      *ResolvedPalette
	UserPalette  *PaletteConfig
	OpenAPIGen   *openapi.Generator

	// User-uploaded sidebar logo. Empty UserLogoExt means "no logo
	// configured"; UserLogoBytes/UserLogoHash are populated together.
	// Bytes live in-memory (≤256 KiB) so GET /_theme/logo doesn't hit
	// disk on every request.
	UserLogoBytes []byte
	UserLogoExt   string
	UserLogoHash  string
}

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
// All reloadable state (config, metamodel, graph, style map, palette,
// user defaults, OpenAPI generator) lives in an immutable AppState struct
// held via atomic.Pointer. Handlers call a.State() once at entry and work
// against a coherent snapshot for the duration of the request — no lock
// acquisition, no risk of observing a half-reloaded world.
//
// Reloads (triggered by the file watcher or by Reload) build a new
// AppState and publish it atomically via a.state.Store. The previous
// state is garbage-collected once no reader holds it.
//
// Mutations (CreateEntity, UpdateEntity, DeleteEntity, CreateRelation,
// UpdateRelation, DeleteRelation, SetProperty, action scripts) serialize
// via writeMu. writeMu excludes concurrent mutations but does NOT block
// readers — readers go through state.Load(). The workspace's internal
// reloadMu coordinates the reload itself with the mutation path.
//
// TODO(TKT-N26KLB): App is a god-object (167 methods). Decompose toward the
// 40-method load line — extract the API/serialization/relation services into
// their own types. Ratchet this number DOWN as methods move out; never up.
//
//plimsoll:max-methods=167
type App struct {
	// Primitives — immutable after NewApp.
	fs    storage.FS
	paths *project.Context

	// Core services. Some are passed in (store, entityManager,
	// searcher); the rest are constructed from primitives inside
	// NewApp.
	store         store.Store
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
	// userState persists per-user UI state (logo, UI state, defaults, palette)
	// to the .rela/ KV store. Extracted from App (TKT-N26KLB M5.3).
	userState userStateStore
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

	// state holds the current reloadable snapshot. Readers: a.State().
	// Writers: onReload rebuilds and publishes a new state after file
	// changes. Initial state is published in NewApp.
	state atomic.Pointer[AppState]

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

// State returns the current reloadable snapshot. Handlers should call
// State() once at entry and use the returned snapshot consistently
// throughout, instead of making multiple calls that could see different
// snapshots after a concurrent reload.
func (a *App) State() *AppState { return a.state.Load() }

// Cfg returns the current data-entry config (convenience accessor).
// Equivalent to a.State().Cfg.
func (a *App) Cfg() *Config { return a.State().Cfg }

// Meta returns the current metamodel (convenience accessor).
func (a *App) Meta() *metamodel.Metamodel { return a.State().Meta }

// luaWriteDeps builds a lua.WriteDeps bundle using the current AppState
// metamodel. Called per action-script invocation so that metamodel reloads
// propagate to scripts without requiring app reconstruction. All other
// fields are immutable for the App's lifetime.
func (a *App) luaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			Store:       a.store,
			Tracer:      a.tracer,
			Searcher:    a.searcher,
			Meta:        a.Meta(),
			ProjectRoot: a.paths.Root,
		},
		EntityManager: a.entityManager,
	}
}

// mutateState atomically updates the published AppState. It takes
// writeMu, builds a shallow copy of the current snapshot, runs the
// caller's mutator on the copy, and publishes the copy via state.Store.
//
// This is the canonical way for mutation handlers to change reloadable
// fields like UserDefaults or UserPalette. Reaching through a.State()
// to assign field values directly is a bug — it scribbles on the shared
// snapshot pointer that lock-free readers also hold.
func (a *App) mutateState(fn func(*AppState)) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	cur := a.state.Load()
	next := *cur // shallow copy of the snapshot
	fn(&next)
	a.state.Store(&next)
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
func NewApp(
	fs storage.FS,
	paths *project.Context,
	meta *metamodel.Metamodel,
	st store.Store,
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
	readDeps := lua.ReadDeps{
		Store:       st,
		Tracer:      trc,
		Searcher:    searcher,
		Meta:        meta,
		ProjectRoot: paths.Root,
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

	userDefaults := app.userState.loadUserDefaults()
	userPalette, paletteErr := app.userState.loadUserPalette()
	if paletteErr != nil {
		// Surface the error so users notice their palette is broken
		// rather than silently falling back to defaults (which would
		// then be persisted on the next save, destroying their data).
		return nil, fmt.Errorf("load user palette: %w", paletteErr)
	}

	logoBytes, logoExt, logoErr := app.userState.loadUserLogo()
	if logoErr != nil {
		// Same policy as palette: surface read errors so a corrupt
		// .rela/theme/ doesn't get silently overwritten on next save.
		return nil, fmt.Errorf("load user logo: %w", logoErr)
	}
	var logoHash string
	if logoExt != "" {
		logoHash = hashLogoBytes(logoBytes)
	}

	// Build and publish the initial AppState snapshot. All reloadable
	// state lives here; there are no convenience aliases on App to keep
	// in sync.
	app.state.Store(&AppState{
		Cfg:           &cfg,
		Meta:          meta,
		StyleMap:      styleMap,
		StyledTypes:   styledTypes,
		UserDefaults:  userDefaults,
		Palette:       ResolvePalette(cfg.Palette, userPalette),
		UserPalette:   userPalette,
		UserLogoBytes: logoBytes,
		UserLogoExt:   logoExt,
		UserLogoHash:  logoHash,
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
	if runner, rerr := attachment.NewCmdRunner(attachmentCmdTimeout, store.MaxAttachmentBytes); rerr == nil {
		app.attachmentRunner = runner
		app.probeAttachmentCommands(meta, runner)
	} else {
		slog.Warn("attachments: command runner unavailable; scan/transform disabled", "err", rerr)
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

func (a *App) findListByEntityType(s *AppState, entries []NavigationEntry, entityType string) string {
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

func buildStyleMap(cfg *Config, meta *metamodel.Metamodel) (styleMap map[string]map[string]string, styledTypes map[string]bool) {
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
