// Package lua provides a Lua scripting runtime for rela with bindings
// to query entities, relations, and output results.
//
// The runtime is sandboxed: only safe Lua libraries are loaded (base, table,
// string, math, utf8, coroutine). The io, os, and debug libraries are NOT
// available to prevent filesystem access and code execution. File operations
// are only possible through the provided rela.write_file() function which
// validates paths are within the project root.
package lua

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// Default values for Lua API functions.
const (
	defaultSearchLimit = 20
	// argPosCreateEntityID is the position of the optional ID parameter in create_entity
	// (type=1, properties=2, content=3, id=4).
	argPosCreateEntityID = 4
)

// stripShebang removes a shebang line from the beginning of Lua code.
// This allows scripts to be directly executable from the command line
// (e.g., #!/usr/bin/env -S rela script). If the code starts with "#!",
// the first line is replaced with a blank line to preserve line numbers
// in error messages. A leading UTF-8 BOM is also stripped if present.
// Otherwise, the code is returned unchanged.
func stripShebang(code string) string {
	code = strings.TrimPrefix(code, "\xEF\xBB\xBF")
	if !strings.HasPrefix(code, "#!") {
		return code
	}
	idx := strings.Index(code, "\n")
	if idx == -1 {
		return ""
	}
	return code[idx:]
}

// Runtime wraps gopher-lua VM with rela bindings.
//
// The runtime is constructed via NewReader (read-only) or NewWriter (read-
// write). A read-only runtime has no mutation bindings (create/update/delete
// of entities and relations) registered at all; calling those from Lua raises
// a "attempt to call a nil value" error from the VM itself.
type Runtime struct {
	L             *lua.LState
	deps          WriteDeps // EntityManager is nil on a reader runtime.
	stdout        io.Writer
	outputDir     string          // Directory where write_file can write (defaults to "output")
	timeout       time.Duration   // Execution timeout (0 = no timeout)
	parentCtx     context.Context //nolint:containedctx // cached parent ctx for lua-callback child-ctx propagation
	cancelTimeout context.CancelFunc
	params        map[string]string // rela.params values (used by action scripts)
	secrets       map[string]string // rela.secrets values (from .rela/secrets.yaml)
	isAction      bool              // true when running as an action (changes rela.output behavior)
	isDocument    bool              // document mode: rela.document populated, rela.output becomes a warning
	documentID    string            // data-entry.yaml documents: key, exposed as rela.document.id
	documentEntry string            // ID of the entity being rendered, exposed as rela.document.entry_id
	aiProvider    ai.Provider       // nil means AI is not configured
	cache         cacheStore        // nil means rela.cache.* is not registered
	routes        RouteCatalog      // nil means rela.url is not registered
	scriptPath    string            // set by RunFile; empty for RunString/inline
}

// cacheStore is the minimal contract the Lua cache bindings need from
// their backend. It's defined at the consumer (Runtime) so alternative
// implementations (a future disk-backed cache, a test fake) can drop in
// without touching wiring code. Kept unexported: callers still pass
// *Cache via WithCache and we keep the flexibility internally.
type cacheStore interface {
	get(namespacedKey string) ([]interface{}, bool)
	set(namespacedKey string, values []interface{}, ttl time.Duration)
	delete(namespacedKey string)
}

// Option configures a Runtime.
type Option func(*Runtime)

// DefaultTimeout is the default execution timeout for scripts.
// This prevents infinite loops and resource exhaustion.
const DefaultTimeout = 30 * time.Second

// WithTimeout sets the execution timeout for scripts.
// Default is 30 seconds. Set to 0 to disable timeout (not recommended).
func WithTimeout(d time.Duration) Option {
	return func(r *Runtime) {
		r.timeout = d
	}
}

// WithContext sets a parent context for the runtime. Cancellation of this
// context propagates into in-flight Lua operations (e.g. long-running loops
// or blocking calls from bindings). When combined with WithTimeout, the
// timeout is derived from this parent so canceling the parent also cancels
// the timeout-bound context.
//
// Typical usage: pass cmd.Context() from a cobra RunE so that Ctrl+C
// interrupts script execution.
func WithContext(ctx context.Context) Option {
	return func(r *Runtime) {
		r.parentCtx = ctx
	}
}

// WithOutputDir sets the output directory for write_file.
// If the path is absolute, files will be written there directly.
// If relative, it's relative to the project root.
func WithOutputDir(dir string) Option {
	return func(r *Runtime) {
		r.outputDir = dir
	}
}

// WithParams sets the rela.params table contents for action scripts.
// Params are static key-value strings from the data-entry config.
func WithParams(params map[string]string) Option {
	return func(r *Runtime) {
		r.params = params
	}
}

// WithActionMode marks the runtime as running in action mode, which changes
// rela.output behavior (logs a warning instead of writing to stdout).
func WithActionMode() Option {
	return func(r *Runtime) {
		r.isAction = true
	}
}

// WithDocumentMode marks the runtime as running a data-entry document
// renderer. Populates the rela.document.{id, entry_id} table and sets
// rela.mode = "document" so scripts can branch on context; also changes
// rela.output behavior to emit a warning line (the captured stdout is the
// rendered document, so JSON noise in-band is almost certainly a mistake).
// documentID is the key under documents: in data-entry.yaml; entryID is
// the ID of the entity being rendered.
func WithDocumentMode(documentID, entryID string) Option {
	return func(r *Runtime) {
		r.isDocument = true
		r.documentID = documentID
		r.documentEntry = entryID
	}
}

// WithSecrets sets the rela.secrets table contents.
// Secrets are loaded from .rela/secrets.yaml by the caller.
func WithSecrets(secrets map[string]string) Option {
	return func(r *Runtime) {
		r.secrets = secrets
	}
}

// WithAIProvider wires an AI provider into the runtime so the ai.* Lua
// bindings are functional. When omitted, ai.chat and ai.complete return
// a typed not_configured error.
func WithAIProvider(p ai.Provider) Option {
	return func(r *Runtime) {
		r.aiProvider = p
	}
}

// WithCache wires a process-wide cache into the runtime so the
// rela.cache.* Lua bindings are registered. When omitted (or passed
// nil), rela.cache.* is absent from the rela table — calling it from
// Lua raises "attempt to call a nil value" from the VM. The cache is
// namespaced by the runtime's script path (set by RunFile); inline or
// eval contexts that call rela.cache.* receive a fixed Lua error
// rather than sharing a nameless namespace.
func WithCache(c *Cache) Option {
	return func(r *Runtime) {
		r.cache = c
	}
}

// NewReader creates a read-only Runtime with query/trace/search/output bindings.
// Mutation bindings (create/update/delete for entities and relations) are not
// registered; calling them from Lua raises "attempt to call a nil value".
//
// The Lua VM is sandboxed with only safe libraries loaded (no io, os, or debug).
func NewReader(d ReadDeps, stdout io.Writer, opts ...Option) *Runtime {
	return newRuntime(WriteDeps{ReadDeps: d}, stdout, false, opts...)
}

// NewWriter creates a read-write Runtime. All read bindings plus mutation
// bindings (create_entity, update_entity, delete_entity, create_relation,
// delete_relation) are registered.
//
// The Lua VM is sandboxed with only safe libraries loaded (no io, os, or debug).
func NewWriter(d WriteDeps, stdout io.Writer, opts ...Option) *Runtime {
	return newRuntime(d, stdout, true, opts...)
}

func newRuntime(deps WriteDeps, stdout io.Writer, allowWrites bool, opts ...Option) *Runtime {
	// Fail loud at construction: a writer runtime with no EntityManager
	// would register mutation bindings that nil-deref on first call.
	// Catching this here turns a silent runtime surprise into a build-time
	// or start-time panic with a clear message.
	if allowWrites && deps.EntityManager == nil {
		panic("lua.NewWriter: WriteDeps.EntityManager is required for a writer runtime")
	}

	// Create sandboxed Lua state - skip default libraries for security
	L := lua.NewState(lua.Options{
		SkipOpenLibs:  true,
		CallStackSize: 1024,      // Limit call stack depth to prevent stack overflow
		RegistrySize:  1024 * 64, // Limit registry size
	})

	// Load only safe libraries (NOT io, os, or debug)
	openSafeLibraries(L)

	r := &Runtime{
		L:         L,
		deps:      deps,
		stdout:    stdout,
		outputDir: defaultOutputDir,
		timeout:   DefaultTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	// In captured-stdout contexts (document mode, action mode), redirect
	// Lua's base `print` through r.stdout. gopher-lua's default writes to
	// os.Stdout, which silently drops output from runtimes whose stdout
	// is a buffer. We override only for these modes so CLI / scheduler /
	// MCP / validation / automation scripts keep the default behavior —
	// users rely on print() reaching their terminal there.
	if r.isDocument || r.isAction {
		L.SetGlobal("print", L.NewFunction(r.luaPrint))
	}

	r.registerBindings(allowWrites)
	return r
}

// luaPrint replaces gopher-lua's base print so its output lands in
// r.stdout rather than os.Stdout. Matches Lua's stock print: each
// argument is stringified via __tostring, joined with tabs, terminated
// by a newline.
func (r *Runtime) luaPrint(ls *lua.LState) int {
	top := ls.GetTop()
	for i := 1; i <= top; i++ {
		if i > 1 {
			fmt.Fprint(r.stdout, "\t")
		}
		fmt.Fprint(r.stdout, ls.ToStringMeta(ls.Get(i)).String())
	}
	fmt.Fprintln(r.stdout)
	return 0
}

// openSafeLibraries loads only safe Lua standard libraries.
// Excluded for security: io (file access), os (system commands), debug (internals).
func openSafeLibraries(ls *lua.LState) {
	// Libraries to load - order matters, LoadLibName must come first if used
	safeLibs := []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		{lua.CoroutineLibName, lua.OpenCoroutine},
		// NOT included: lua.IoLibName, lua.OsLibName, lua.DebugLibName, lua.ChannelLibName
	}

	for _, lib := range safeLibs {
		ls.Push(ls.NewFunction(lib.fn))
		ls.Push(lua.LString(lib.name))
		ls.Call(1, 0)
	}

	// Remove dangerous base functions that could bypass sandbox
	ls.SetGlobal("loadfile", lua.LNil)
	ls.SetGlobal("dofile", lua.LNil)
	ls.SetGlobal("load", lua.LNil)
	ls.SetGlobal("loadstring", lua.LNil)

	// Remove raw access functions that could bypass metamethod protections
	// and modify the rela module internals
	ls.SetGlobal("rawget", lua.LNil)
	ls.SetGlobal("rawset", lua.LNil)
	ls.SetGlobal("rawequal", lua.LNil)
	ls.SetGlobal("rawlen", lua.LNil)
	ls.SetGlobal("getmetatable", lua.LNil)
	ls.SetGlobal("setmetatable", lua.LNil)
}

// RunFile executes a Lua script file with arguments.
// Shebang lines (starting with #!) are automatically stripped.
//
// RunFile sets the runtime's scriptPath (via filepath.Clean(path)) so
// the rela.cache.* bindings can namespace entries by script. Callers
// using RunString/inline code do not get this identity and any
// rela.cache.* call raises a Lua error.
func (r *Runtime) RunFile(path string, args []string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read script file: %w", err)
	}
	return r.RunFileContent(path, content, args)
}

// RunFileContent executes Lua code loaded from `path`, using the
// already-read `content`. This exists for callers that need traversal-
// resistant file reads (e.g. MCP's lua_run, which uses os.OpenRoot).
// Effects match RunFile: chunk name for errors, scriptPath for
// rela.cache.* namespacing, rela.args for script arguments. Shebangs
// are stripped.
func (r *Runtime) RunFileContent(path string, content []byte, args []string) error {
	// Set rela.args
	argsTable := r.L.NewTable()
	for i, arg := range args {
		argsTable.RawSetInt(i+1, lua.LString(arg))
	}
	relaTable, ok := r.L.GetGlobal("rela").(*lua.LTable)
	if !ok {
		return errors.New("rela module not initialized")
	}
	relaTable.RawSetString("args", argsTable)

	// Record the cleaned script path so rela.cache.* can namespace
	// entries. Cleaning (rather than absolutising) keeps two runs with
	// different CWDs sharing the same namespace when the relative path
	// is identical — the CLI already chdirs to project root, so this is
	// the stable identity.
	r.scriptPath = filepath.Clean(path)

	// Strip shebang if present
	code := stripShebang(string(content))

	r.applyTimeout()

	fn, err := r.L.Load(strings.NewReader(code), path)
	if err != nil {
		return fmt.Errorf("cannot compile script: %w", err)
	}

	r.L.Push(fn)
	return r.L.PCall(0, lua.MultRet, nil)
}

// RunString executes Lua code from a string.
// Shebang lines (starting with #!) are automatically stripped.
func (r *Runtime) RunString(code string) error {
	r.applyTimeout()
	return r.L.DoString(stripShebang(code))
}

// ErrNoReturnValue is returned by RunActionString when the script did not
// return a value. Action handlers can use errors.Is to check for this.
var ErrNoReturnValue = errors.New("script did not return a value")

// RunActionString executes Lua code as an action, returning the script's
// top-of-stack return value as a Go interface{}. Returns ErrNoReturnValue
// if the script did not return any values.
func (r *Runtime) RunActionString(code, name string) (interface{}, error) {
	r.applyTimeout()

	cleaned := stripShebang(code)
	fn, err := r.L.Load(strings.NewReader(cleaned), name)
	if err != nil {
		return nil, fmt.Errorf("cannot compile script: %w", err)
	}

	// Record stack depth before so we can detect if the script returned anything
	topBefore := r.L.GetTop()
	r.L.Push(fn)
	if pcallErr := r.L.PCall(0, lua.MultRet, nil); pcallErr != nil {
		return nil, pcallErr
	}
	topAfter := r.L.GetTop()

	if topAfter <= topBefore {
		// Script did not return a value
		return nil, ErrNoReturnValue
	}

	// Script may have returned multiple values; the first one is the primary return.
	// We read topBefore+1 (stack is 1-indexed; Get returns LNil for invalid indices).
	ret := r.L.Get(topBefore + 1)
	// Pop all returned values to leave the stack as we found it
	r.L.SetTop(topBefore)

	return luaValueToGo(ret), nil
}

// applyTimeout sets the execution timeout on the Lua state.
// Must be called before executing any Lua code.
//
// The derived context is rooted at r.parentCtx (if set) so that canceling
// the caller's context (e.g. Ctrl+C via a cobra command context) interrupts
// in-flight Lua execution. When no timeout is configured but a parent context
// is set, the parent is attached directly so cancellation still propagates.
func (r *Runtime) applyTimeout() {
	r.clearTimeout()
	parent := r.parentCtx
	if parent == nil {
		parent = context.Background()
	}
	if r.timeout > 0 {
		ctx, cancel := context.WithTimeout(parent, r.timeout)
		r.cancelTimeout = cancel
		r.L.SetContext(ctx)
		return
	}
	if r.parentCtx != nil {
		r.L.SetContext(r.parentCtx)
	}
}

// clearTimeout cancels any active timeout and removes the context from the Lua state.
func (r *Runtime) clearTimeout() {
	if r.cancelTimeout != nil {
		r.cancelTimeout()
		r.cancelTimeout = nil
	}
	r.L.RemoveContext()
}

// SetScriptPath records a cache namespace identity for callers that
// run via RunString or RunActionString (not RunFile/RunFileContent,
// which set it automatically). The "path" here is used purely as the
// rela.cache.* namespace — for inline-code callers that need a
// stable cache scope it's the caller's name for the script
// (e.g. "validations/<rule-name>"); for file-backed callers
// RunFileContent is preferred because it sets chunk name, args, and
// namespace in one step. The path is cleaned (filepath.Clean).
//
// SetScriptPath persists across subsequent RunString calls on the
// same Runtime. Call with "" to revert to inline/eval mode (cache
// calls raise). RunFile / RunFileContent override it.
func (r *Runtime) SetScriptPath(path string) {
	if path == "" {
		r.scriptPath = ""
		return
	}
	r.scriptPath = filepath.Clean(path)
}

// SetArgs sets the script arguments (rela.args) before execution.
func (r *Runtime) SetArgs(args []string) {
	argsTable := r.L.NewTable()
	for i, arg := range args {
		argsTable.RawSetInt(i+1, lua.LString(arg))
	}
	relaTable, ok := r.L.GetGlobal("rela").(*lua.LTable)
	if ok {
		relaTable.RawSetString("args", argsTable)
	}
}

// Close releases Lua VM resources.
func (r *Runtime) Close() {
	r.clearTimeout()
	r.L.Close()
}

// LState returns the underlying Lua state for setting globals.
func (r *Runtime) LState() *lua.LState {
	return r.L
}

// registerBindings sets up the rela module. When allowWrites is false, only
// read bindings are registered — mutation functions are absent from the rela.*
// table and calling them from Lua raises "attempt to call a nil value".
func (r *Runtime) registerBindings(allowWrites bool) {
	rela := r.L.NewTable()

	r.registerReadBindings(rela)
	if allowWrites {
		r.registerWriteBindings(rela)
	}
	r.registerContextBindings(rela)

	r.L.SetGlobal("rela", rela)

	// Top-level ai.* module (always registered; functions return a
	// typed not_configured error when no provider is wired).
	r.registerAIModule()
}

// registerReadBindings installs read-only bindings on the rela table: entity
// and relation queries, graph traversal, output, schema introspection.
func (r *Runtime) registerReadBindings(rela *lua.LTable) {
	// Entity query functions
	r.L.SetField(rela, "get_entity", r.L.NewFunction(r.luaGetEntity))
	r.L.SetField(rela, "list_entities", r.L.NewFunction(r.luaListEntities))
	r.L.SetField(rela, "search", r.L.NewFunction(r.luaSearch))

	// Relation query functions
	r.L.SetField(rela, "get_relations", r.L.NewFunction(r.luaGetRelations))

	// Graph traversal
	r.L.SetField(rela, "trace_from", r.L.NewFunction(r.luaTraceFrom))
	r.L.SetField(rela, "trace_to", r.L.NewFunction(r.luaTraceTo))
	r.L.SetField(rela, "find_path", r.L.NewFunction(r.luaFindPath))

	// Output functions
	r.L.SetField(rela, "output", r.L.NewFunction(r.luaOutput))

	// Schema introspection
	r.L.SetField(rela, "get_entity_types", r.L.NewFunction(r.luaGetEntityTypes))
	r.L.SetField(rela, "get_relation_types", r.L.NewFunction(r.luaGetRelationTypes))

	// Utility functions
	r.L.SetField(rela, "sort_entities", r.L.NewFunction(r.luaSortEntities))
	r.L.SetField(rela, "days_since", r.L.NewFunction(luaDaysSince))
	r.L.SetField(rela, "today", lua.LString(time.Now().Format("2006-01-02")))

	// Date and RRULE utility functions
	registerDateHelpers(r.L, rela)

	// Markdown AST and generation helpers module (rela.md.*)
	r.registerMarkdownModule(rela)
}

// registerWriteBindings installs mutation bindings on the rela table.
// Graph mutations (create/update/delete for entities and relations) and
// filesystem writes (write_file) are all restricted to writer runtimes —
// readers (validation rules, etc.) have no way to mutate state of any kind.
func (r *Runtime) registerWriteBindings(rela *lua.LTable) {
	r.L.SetField(rela, "create_entity", r.L.NewFunction(r.luaCreateEntity))
	r.L.SetField(rela, "update_entity", r.L.NewFunction(r.luaUpdateEntity))
	r.L.SetField(rela, "delete_entity", r.L.NewFunction(r.luaDeleteEntity))
	r.L.SetField(rela, "create_relation", r.L.NewFunction(r.luaCreateRelation))
	r.L.SetField(rela, "delete_relation", r.L.NewFunction(r.luaDeleteRelation))
	r.L.SetField(rela, "write_file", r.L.NewFunction(r.luaWriteFile))
}

// registerContextBindings installs per-runtime context tables: args, params,
// secrets. Present on both reader and writer runtimes.
func (r *Runtime) registerContextBindings(rela *lua.LTable) {
	// Context
	r.L.SetField(rela, "args", r.L.NewTable()) // Will be set before running script

	// Params table (populated from WithParams option, used by action scripts)
	paramsTable := r.L.NewTable()
	for k, v := range r.params {
		r.L.SetField(paramsTable, k, lua.LString(v))
	}
	r.L.SetField(rela, "params", paramsTable)

	// Secrets table (populated from WithSecrets option, loaded from .rela/secrets.yaml)
	secretsTable := r.L.NewTable()
	for k, v := range r.secrets {
		r.L.SetField(secretsTable, k, lua.LString(v))
	}
	r.L.SetField(rela, "secrets", secretsTable)

	// rela.cache.{get,set,memoize} when a cache is wired. The binding
	// itself guards against inline/eval contexts (empty scriptPath) by
	// raising a Lua error on any call, so a runtime with a cache but
	// no script path still behaves safely.
	r.registerCacheBindings(rela)

	// rela.url when a route catalog is wired. Absent by default so
	// runtimes that have no business building frontend URLs (validation
	// rules, scheduler scripts, etc.) don't accidentally reference one.
	if r.routes != nil {
		r.L.SetField(rela, "url", r.L.NewFunction(r.luaURL))
	}

	// Document-mode context: rela.mode + rela.document.{id, entry_id}.
	// Only populated when WithDocumentMode was applied. In every other
	// context rela.mode and rela.document are absent (Lua nil), so a
	// script that branches on them sees nil outside document renders.
	if r.isDocument {
		r.L.SetField(rela, "mode", lua.LString("document"))
		docTable := r.L.NewTable()
		r.L.SetField(docTable, "id", lua.LString(r.documentID))
		r.L.SetField(docTable, "entry_id", lua.LString(r.documentEntry))
		r.L.SetField(rela, "document", docTable)
	}
}

// luaGetEntity implements rela.get_entity(id) -> table|nil
func (r *Runtime) luaGetEntity(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}

	e, err := r.deps.Store.GetEntity(context.Background(), id)
	if err != nil {
		ls.Push(lua.LNil)
		return 1
	}

	ls.Push(EntityToTable(ls, e))
	return 1
}

// luaListEntities implements rela.list_entities(type, filter?) -> table
func (r *Runtime) luaListEntities(ls *lua.LState) int {
	entityType := ls.CheckString(1)
	if entityType == "" {
		ls.RaiseError("entity type cannot be empty")
		return 0
	}
	filterExpr := ls.OptString(2, "")

	entities := make([]*entity.Entity, 0)
	for e, err := range r.deps.Store.ListEntities(context.Background(), store.EntityQuery{Type: entityType}) {
		if err != nil {
			break
		}
		entities = append(entities, e)
	}

	// Apply filter if provided
	if filterExpr != "" {
		f, err := filter.Parse(filterExpr)
		if err != nil {
			ls.RaiseError("invalid filter: %s", err.Error())
			return 0
		}

		entityDef, found := r.deps.Meta.GetEntityDef(entityType)
		if !found {
			ls.RaiseError("unknown entity type: %s", entityType)
			return 0
		}

		filters := []*filter.Filter{f}
		filtered := make([]*entity.Entity, 0)
		for _, e := range entities {
			record := filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties}
			match, err := filter.MatchAll(record, filters, entityDef, r.deps.Meta)
			if err != nil {
				ls.RaiseError("filter error: %s", err.Error())
				return 0
			}
			if match {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	result := ls.NewTable()
	for i, e := range entities {
		result.RawSetInt(i+1, EntityToTable(ls, e))
	}
	ls.Push(result)
	return 1
}

// luaGetRelations implements rela.get_relations(opts?) -> table
// opts can have: from, type, to
func (r *Runtime) luaGetRelations(ls *lua.LState) int {
	var fromFilter, typeFilter, toFilter string

	// Parse options table if provided
	if ls.GetTop() >= 1 && ls.Get(1).Type() == lua.LTTable {
		opts := ls.CheckTable(1)
		if v, ok := opts.RawGetString("from").(lua.LString); ok {
			fromFilter = string(v)
		}
		if v, ok := opts.RawGetString("type").(lua.LString); ok {
			typeFilter = string(v)
		}
		if v, ok := opts.RawGetString("to").(lua.LString); ok {
			toFilter = string(v)
		}
	}

	q := store.RelationQuery{From: fromFilter, Type: typeFilter, To: toFilter}
	result := ls.NewTable()
	idx := 1
	for rel, err := range r.deps.Store.ListRelations(context.Background(), q) {
		if err != nil {
			break
		}
		result.RawSetInt(idx, relationToTable(ls, rel))
		idx++
	}
	ls.Push(result)
	return 1
}

// luaTraceFrom implements rela.trace_from(id, depth?) -> table|nil
func (r *Runtime) luaTraceFrom(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}
	maxDepth := ls.OptInt(2, 0)

	trace := r.deps.Tracer.TraceFrom(context.Background(), id, maxDepth)
	if trace == nil {
		ls.Push(lua.LNil)
		return 1
	}
	ls.Push(traceResultToTable(ls, trace))
	return 1
}

// luaTraceTo implements rela.trace_to(id, depth?) -> table|nil
func (r *Runtime) luaTraceTo(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}
	maxDepth := ls.OptInt(2, 0)

	trace := r.deps.Tracer.TraceTo(context.Background(), id, maxDepth)
	if trace == nil {
		ls.Push(lua.LNil)
		return 1
	}
	ls.Push(traceResultToTable(ls, trace))
	return 1
}

// luaOutput implements rela.output(data) - JSON encode to stdout
func (r *Runtime) luaOutput(ls *lua.LState) int {
	// Type-check the arg up front; the Lua → Go conversion is deferred
	// past the mode guards so muted modes (action/document) don't pay
	// for converting a potentially-large nested table.
	data := ls.CheckAny(1)

	if r.isAction {
		// In action mode, rela.output is a no-op. Log a warning so script
		// authors notice that output should use the return statement instead.
		fmt.Fprintln(r.stdout, "warning: rela.output() called in action mode; use 'return' to produce the response")
		return 0
	}

	if r.isDocument {
		// In document mode, captured stdout is the rendered document.
		// Raw JSON in the middle of rendered markdown is almost always a
		// mistake — emit a warning line (visible in the panel) so the
		// script author notices, rather than silently producing garbage.
		fmt.Fprintln(r.stdout, "warning: rela.output() called in document mode; use print() to emit markdown")
		return 0
	}

	goData := luaValueToGo(data)
	encoder := json.NewEncoder(r.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(goData); err != nil {
		ls.RaiseError("JSON encoding error: %s", err.Error())
		return 0
	}
	return 0
}

// defaultOutputDir is the default directory where Lua scripts can write files.
const defaultOutputDir = "output"

// luaWriteFile implements rela.write_file(path, content, opts?)
// Files can ONLY be written to the configured output directory for security.
// Path is relative to output dir (e.g., "report.txt" -> "{output}/report.txt").
// Options:
//   - ensure_newline: boolean - ensure content ends with a newline (default: false)
func (r *Runtime) luaWriteFile(ls *lua.LState) int {
	path := ls.CheckString(1)
	content := ls.CheckString(2)

	if path == "" {
		ls.RaiseError("write_file: path cannot be empty")
		return 0
	}

	// Parse options if provided
	ensureNewline := false
	if ls.GetTop() >= 3 && ls.Get(3).Type() == lua.LTTable {
		opts := ls.CheckTable(3)
		if v := opts.RawGetString("ensure_newline"); v != lua.LNil {
			if b, ok := v.(lua.LBool); ok {
				ensureNewline = bool(b)
			}
		}
	}

	// Ensure content ends with newline if requested
	if ensureNewline && content != "" && content[len(content)-1] != '\n' {
		content += "\n"
	}

	// Validate the path is local (no "..", no absolute paths)
	if !filepath.IsLocal(path) {
		ls.RaiseError("write_file: path must be a local path (no '..' or absolute paths)")
		return 0
	}

	// Build the full path within output directory
	var outputPath string
	if filepath.IsAbs(r.outputDir) {
		outputPath = r.outputDir
	} else {
		outputPath = filepath.Join(r.deps.ProjectRoot, r.outputDir)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		ls.RaiseError("write_file: cannot create output directory: %s", err.Error())
		return 0
	}

	// Ensure parent directories within output/ exist
	fullPath := filepath.Join(outputPath, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		ls.RaiseError("write_file: cannot create directory: %s", err.Error())
		return 0
	}

	// Write the file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		ls.RaiseError("write_file: cannot write file: %s", err.Error())
		return 0
	}

	return 0
}

// EntityToTable converts an entity.Entity to a Lua table.
// The returned table has a prop(name, default) method.
// Exported for use by workspace automation execution.
func EntityToTable(ls *lua.LState, e *entity.Entity) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(e.ID))
	t.RawSetString("type", lua.LString(e.Type))
	t.RawSetString("content", lua.LString(e.Content))

	// Add modification time as ISO 8601 string (empty if zero)
	if !e.UpdatedAt.IsZero() {
		t.RawSetString("mod_time", lua.LString(e.UpdatedAt.Format(time.RFC3339)))
	} else {
		t.RawSetString("mod_time", lua.LString(""))
	}

	props := ls.NewTable()
	for k, v := range e.Properties {
		props.RawSetString(k, GoToLuaValue(ls, v))
	}
	t.RawSetString("properties", props)

	// Add prop(name, default) method via a function field
	t.RawSetString("prop", ls.NewFunction(luaEntityProp))

	// Add strip_prefix() method to get ID without type prefix
	t.RawSetString("strip_prefix", ls.NewFunction(luaEntityStripPrefix))

	return t
}

// luaEntityProp implements entity:prop(name, default) -> value
// Returns the property value or the default if not set/empty.
func luaEntityProp(ls *lua.LState) int {
	// Get self (the entity table) - first argument in method call
	self := ls.CheckTable(1)
	name := ls.CheckString(2)
	defaultVal := ls.Get(3) // optional, can be nil

	// Get properties table
	propsVal := self.RawGetString("properties")
	props, ok := propsVal.(*lua.LTable)
	if !ok {
		ls.Push(defaultVal)
		return 1
	}

	// Get the property value
	val := props.RawGetString(name)

	// Return default if nil or empty string
	if val == lua.LNil {
		ls.Push(defaultVal)
		return 1
	}
	if str, ok := val.(lua.LString); ok && string(str) == "" {
		ls.Push(defaultVal)
		return 1
	}

	ls.Push(val)
	return 1
}

// luaEntityStripPrefix implements entity:strip_prefix() -> string
// Returns the entity ID with the type prefix removed (e.g., "GUIDE-foo" -> "foo").
func luaEntityStripPrefix(ls *lua.LState) int {
	self := ls.CheckTable(1)
	idVal := self.RawGetString("id")

	id, ok := idVal.(lua.LString)
	if !ok {
		ls.Push(lua.LString(""))
		return 1
	}

	// Strip prefix: find first hyphen and return everything after it
	idStr := string(id)
	for i, c := range idStr {
		if c == '-' {
			ls.Push(lua.LString(idStr[i+1:]))
			return 1
		}
	}

	// No hyphen found, return as-is
	ls.Push(id)
	return 1
}

// relationToTable converts an entity.Relation to a Lua table.
func relationToTable(ls *lua.LState, rel *entity.Relation) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("from", lua.LString(rel.From))
	t.RawSetString("type", lua.LString(rel.Type))
	t.RawSetString("to", lua.LString(rel.To))

	if len(rel.Properties) > 0 {
		props := ls.NewTable()
		for k, v := range rel.Properties {
			props.RawSetString(k, GoToLuaValue(ls, v))
		}
		t.RawSetString("properties", props)
	}

	return t
}

// traceResultToTable converts a trace result tree to a Lua table.
func traceResultToTable(ls *lua.LState, trace *tracer.TraceResult) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(trace.ID))
	t.RawSetString("type", lua.LString(trace.Type))
	t.RawSetString("title", lua.LString(trace.Title))
	t.RawSetString("depth", lua.LNumber(trace.Depth))
	t.RawSetString("relation", lua.LString(trace.Relation))
	t.RawSetString("incoming", lua.LBool(trace.Incoming))

	// Convert children recursively
	children := ls.NewTable()
	for i, child := range trace.Children {
		children.RawSetInt(i+1, traceResultToTable(ls, child))
	}
	t.RawSetString("children", children)

	return t
}

// GoToLuaValue converts a Go value to a Lua value.
// Exported for use by workspace automation execution.
func GoToLuaValue(ls *lua.LState, v interface{}) lua.LValue {
	if v == nil {
		return lua.LNil
	}
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	case []interface{}:
		t := ls.NewTable()
		for i, item := range val {
			t.RawSetInt(i+1, GoToLuaValue(ls, item))
		}
		return t
	case []string:
		t := ls.NewTable()
		for i, item := range val {
			t.RawSetInt(i+1, lua.LString(item))
		}
		return t
	case map[string]interface{}:
		t := ls.NewTable()
		for k, item := range val {
			t.RawSetString(k, GoToLuaValue(ls, item))
		}
		return t
	default:
		// Fallback: convert to string
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

// luaValueToGo converts a Lua value to a Go value. Safe against cyclic
// tables: a second visit to a table already on the current ancestry
// chain yields the sentinel string "<cyclic reference>" rather than
// recursing forever. Without that guard, a self-referential Lua value
// (e.g. `t.self = t`) would overflow the Go stack and crash the entire
// process — not catchable from PCall. Every caller (rela.output,
// RunActionString, luaTableToGoMap, and the cache) inherits the guard.
func luaValueToGo(lv lua.LValue) interface{} {
	return luaValueToGoSeen(lv, make(map[*lua.LTable]bool))
}

func luaValueToGoSeen(lv lua.LValue, seen map[*lua.LTable]bool) interface{} {
	switch v := lv.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		return luaTableToGoSeen(v, seen)
	case *lua.LNilType:
		return nil
	default:
		return nil
	}
}

// maxArraySize is the maximum size for arrays converted from Lua tables.
const maxArraySize = 100000

// cyclicReferenceMarker is the string inserted in place of a table that
// appears a second time on its own ancestry chain during conversion.
// Callers that render JSON, compare structures, or use `rela.output`
// see this marker and can investigate, rather than hanging forever.
const cyclicReferenceMarker = "<cyclic reference>"

// luaTableToGoSeen converts a Lua table to a Go map or slice, using
// the ancestry set `seen` to terminate cycles. Callers wanting a fresh
// conversion should go through luaValueToGo, which bootstraps the
// seen-set.
func luaTableToGoSeen(t *lua.LTable, seen map[*lua.LTable]bool) interface{} {
	if seen[t] {
		return cyclicReferenceMarker
	}
	seen[t] = true
	// Unmark on return so siblings that share a non-cyclic inner table
	// are not false-positive rejected. Cycle detection is per-ancestry,
	// not per-occurrence.
	defer delete(seen, t)

	// Check if it's an array (sequential positive integer keys starting at 1)
	isArray := true
	maxN := 0
	t.ForEach(func(k, _ lua.LValue) {
		if kn, ok := k.(lua.LNumber); ok {
			f := float64(kn)
			// Must be a positive integer within bounds
			if f != math.Floor(f) || f < 1 || f > maxArraySize {
				isArray = false
				return
			}
			n := int(f)
			if n > maxN {
				maxN = n
			}
		} else {
			isArray = false
		}
	})

	if isArray && maxN > 0 {
		arr := make([]interface{}, maxN)
		t.ForEach(func(k, v lua.LValue) {
			if kn, ok := k.(lua.LNumber); ok {
				idx := int(kn) - 1
				if idx >= 0 && idx < maxN {
					arr[idx] = luaValueToGoSeen(v, seen)
				}
			}
		})
		return arr
	}

	// It's a map
	m := make(map[string]interface{})
	t.ForEach(func(k, v lua.LValue) {
		var key string
		switch kv := k.(type) {
		case lua.LString:
			key = string(kv)
		case lua.LNumber:
			key = fmt.Sprintf("%v", float64(kv))
		default:
			key = k.String()
		}
		m[key] = luaValueToGoSeen(v, seen)
	})
	return m
}

// luaSearch implements rela.search(query, limit?) -> table
// Performs full-text search across entity titles and properties.
func (r *Runtime) luaSearch(ls *lua.LState) int {
	query := ls.CheckString(1)
	if query == "" {
		ls.RaiseError("search query cannot be empty")
		return 0
	}

	limit := ls.OptInt(2, defaultSearchLimit)

	if r.deps.Searcher == nil {
		ls.RaiseError("search not available")
		return 0
	}
	result := ls.NewTable()
	i := 1
	for hit, err := range r.deps.Searcher.Search(context.Background(), search.Query{Text: query, Limit: limit}) {
		if err != nil {
			ls.RaiseError("search error: %s", err.Error())
			return 0
		}
		// Fetch the full entity for the lua table (search hits are minimal).
		e, err := r.deps.Store.GetEntity(context.Background(), hit.ID)
		if err != nil {
			continue
		}
		result.RawSetInt(i, EntityToTable(ls, e))
		i++
	}
	ls.Push(result)
	return 1
}

// luaCreateEntity implements rela.create_entity(type, properties, content?, id?) -> table
func (r *Runtime) luaCreateEntity(ls *lua.LState) int {
	entityType := ls.CheckString(1)
	if entityType == "" {
		ls.RaiseError("entity type cannot be empty")
		return 0
	}

	propsTable := ls.CheckTable(2)
	props := luaTableToGoMap(propsTable)

	content := ls.OptString(3, "")
	customID := ls.OptString(argPosCreateEntityID, "")

	newE := &entity.Entity{
		ID:         customID,
		Type:       entityType,
		Properties: props,
		Content:    content,
	}
	result, err := r.deps.EntityManager.CreateEntity(
		context.Background(), newE, entitymanager.CreateOptions{ID: customID})
	if err != nil {
		ls.RaiseError("create entity error: %s", err.Error())
		return 0
	}

	ls.Push(EntityToTable(ls, result.Entity))
	return 1
}

// luaUpdateEntity implements rela.update_entity(id, properties, content?) -> table
func (r *Runtime) luaUpdateEntity(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}

	ctx := context.Background()
	existing, err := r.deps.Store.GetEntity(ctx, id)
	if err != nil {
		ls.RaiseError("entity not found: %s", id)
		return 0
	}

	// Clone for update
	updated := existing.Clone()

	// Merge properties if provided
	if ls.GetTop() >= 2 && ls.Get(2).Type() == lua.LTTable {
		propsTable := ls.CheckTable(2)
		newProps := luaTableToGoMap(propsTable)
		if updated.Properties == nil {
			updated.Properties = make(map[string]interface{})
		}
		for k, v := range newProps {
			updated.Properties[k] = v
		}
	}

	// Update content if provided (nil means not provided, empty string clears content)
	if ls.GetTop() >= 3 && ls.Get(3).Type() != lua.LTNil {
		updated.Content = ls.CheckString(3)
	}

	result, err := r.deps.EntityManager.UpdateEntity(ctx, updated)
	if err != nil {
		ls.RaiseError("update entity error: %s", err.Error())
		return 0
	}

	ls.Push(EntityToTable(ls, result.Entity))
	return 1
}

// luaDeleteEntity implements rela.delete_entity(id, cascade?) -> boolean
func (r *Runtime) luaDeleteEntity(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}

	cascade := ls.OptBool(2, false)

	if _, err := r.deps.EntityManager.DeleteEntity(context.Background(), id, cascade); err != nil {
		ls.RaiseError("delete entity error: %s", err.Error())
		return 0
	}

	ls.Push(lua.LTrue)
	return 1
}

// luaCreateRelation implements rela.create_relation(from, type, to, content?) -> table
func (r *Runtime) luaCreateRelation(ls *lua.LState) int {
	from := ls.CheckString(1)
	relType := ls.CheckString(2)
	to := ls.CheckString(3)

	if from == "" || relType == "" || to == "" {
		ls.RaiseError("from, type, and to are required")
		return 0
	}

	rel, err := r.deps.EntityManager.CreateRelation(
		context.Background(), from, relType, to, entitymanager.RelationOptions{})
	if err != nil {
		ls.RaiseError("create relation error: %s", err.Error())
		return 0
	}

	ls.Push(relationToTable(ls, rel))
	return 1
}

// luaDeleteRelation implements rela.delete_relation(from, type, to) -> boolean
func (r *Runtime) luaDeleteRelation(ls *lua.LState) int {
	from := ls.CheckString(1)
	relType := ls.CheckString(2)
	to := ls.CheckString(3)

	if from == "" || relType == "" || to == "" {
		ls.RaiseError("from, type, and to are required")
		return 0
	}

	if err := r.deps.EntityManager.DeleteRelation(context.Background(), from, relType, to); err != nil {
		ls.RaiseError("delete relation error: %s", err.Error())
		return 0
	}

	ls.Push(lua.LTrue)
	return 1
}

// luaFindPath implements rela.find_path(from, to) -> table
func (r *Runtime) luaFindPath(ls *lua.LState) int {
	from := ls.CheckString(1)
	to := ls.CheckString(2)

	if from == "" || to == "" {
		ls.RaiseError("from and to are required")
		return 0
	}

	path := r.deps.Tracer.FindPath(context.Background(), from, to)
	if path == nil {
		ls.Push(lua.LNil)
		return 1
	}

	result := ls.NewTable()
	for i, step := range path {
		stepTable := ls.NewTable()
		stepTable.RawSetString("id", lua.LString(step.ID))
		stepTable.RawSetString("type", lua.LString(step.Type))
		stepTable.RawSetString("title", lua.LString(step.Title))
		stepTable.RawSetString("relation", lua.LString(step.Relation))
		result.RawSetInt(i+1, stepTable)
	}
	ls.Push(result)
	return 1
}

// luaTableToGoMap converts a Lua table to a Go map[string]interface{}.
func luaTableToGoMap(t *lua.LTable) map[string]interface{} {
	m := make(map[string]interface{})
	t.ForEach(func(k, v lua.LValue) {
		var key string
		switch kv := k.(type) {
		case lua.LString:
			key = string(kv)
		case lua.LNumber:
			key = fmt.Sprintf("%v", float64(kv))
		default:
			key = k.String()
		}
		m[key] = luaValueToGo(v)
	})
	return m
}

// luaGetEntityTypes implements rela.get_entity_types() -> table
// Returns a table of entity type definitions with their properties.
func (r *Runtime) luaGetEntityTypes(ls *lua.LState) int {
	result := ls.NewTable()

	for name, et := range r.deps.Meta.Entities {
		typeTable := ls.NewTable()
		typeTable.RawSetString("name", lua.LString(name))
		typeTable.RawSetString("label", lua.LString(et.Label))
		typeTable.RawSetString("plural", lua.LString(et.Plural))

		// Properties
		propsTable := ls.NewTable()
		for propName, prop := range et.Properties {
			propTable := ls.NewTable()
			propTable.RawSetString("name", lua.LString(propName))
			propTable.RawSetString("type", lua.LString(prop.Type))
			propTable.RawSetString("required", lua.LBool(prop.Required))
			if prop.Default != "" {
				propTable.RawSetString("default", lua.LString(prop.Default))
			}
			if len(prop.Values) > 0 {
				valuesTable := ls.NewTable()
				for i, val := range prop.Values {
					valuesTable.RawSetInt(i+1, lua.LString(val))
				}
				propTable.RawSetString("values", valuesTable)
			}
			propsTable.RawSetString(propName, propTable)
		}
		typeTable.RawSetString("properties", propsTable)

		result.RawSetString(name, typeTable)
	}

	ls.Push(result)
	return 1
}

// luaGetRelationTypes implements rela.get_relation_types() -> table
// Returns a table of relation type definitions with their constraints.
func (r *Runtime) luaGetRelationTypes(ls *lua.LState) int {
	result := ls.NewTable()

	for name, rt := range r.deps.Meta.Relations {
		typeTable := ls.NewTable()
		typeTable.RawSetString("name", lua.LString(name))
		typeTable.RawSetString("label", lua.LString(rt.Label))

		// From constraints
		fromTable := ls.NewTable()
		for i, f := range rt.From {
			fromTable.RawSetInt(i+1, lua.LString(f))
		}
		typeTable.RawSetString("from", fromTable)

		// To constraints
		toTable := ls.NewTable()
		for i, t := range rt.To {
			toTable.RawSetInt(i+1, lua.LString(t))
		}
		typeTable.RawSetString("to", toTable)

		result.RawSetString(name, typeTable)
	}

	ls.Push(result)
	return 1
}

// sortableEntry holds an entity table and its sort key for sorting.
type sortableEntry struct {
	value lua.LValue
	prop  lua.LValue
}

// luaSortEntities implements rela.sort_entities(entities, property, direction?) -> table
// Sorts a list of entity tables by a property value.
// Direction is optional: "asc" (default) or "desc".
// Handles numeric comparison for property values that look like numbers.
func (r *Runtime) luaSortEntities(ls *lua.LState) int {
	entitiesTable := ls.CheckTable(1)
	property := ls.CheckString(2)
	direction := ls.OptString(3, "asc")

	if property == "" {
		ls.RaiseError("sort_entities: property cannot be empty")
		return 0
	}

	descending := direction == "desc"

	// Collect entities into a slice for sorting
	entries := make([]sortableEntry, 0, entitiesTable.Len())

	for i := 1; i <= entitiesTable.Len(); i++ {
		v := entitiesTable.RawGetInt(i)
		tbl, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		props := tbl.RawGetString("properties")
		propVal := lua.LNil
		if propsTbl, ok := props.(*lua.LTable); ok {
			propVal = propsTbl.RawGetString(property)
		}
		entries = append(entries, sortableEntry{value: v, prop: propVal})
	}

	// Sort entries using bubble sort (sufficient for typical entity counts)
	sortEntries(entries, descending)

	// Build result table
	result := ls.NewTable()
	for i, entry := range entries {
		result.RawSetInt(i+1, entry.value)
	}

	ls.Push(result)
	return 1
}

// sortEntries sorts entity entries by their property value using bubble sort.
func sortEntries(entries []sortableEntry, descending bool) {
	for i := range len(entries) - 1 {
		for j := range len(entries) - i - 1 {
			if shouldSwapEntries(entries[j].prop, entries[j+1].prop, descending) {
				entries[j], entries[j+1] = entries[j+1], entries[j]
			}
		}
	}
}

// shouldSwapEntries returns true if entries should be swapped for the desired order.
func shouldSwapEntries(a, b lua.LValue, descending bool) bool {
	aStr, aNum, aIsNum := luaValueToSortable(a)
	bStr, bNum, bIsNum := luaValueToSortable(b)

	var aLess bool
	if aIsNum && bIsNum {
		aLess = aNum < bNum
	} else {
		aLess = aStr < bStr
	}

	if descending {
		return aLess // swap if a < b (we want larger first)
	}
	return !aLess && (aStr != bStr || aNum != bNum) // swap if a > b
}

// luaValueToSortable converts a Lua value to sortable string and number representations.
func luaValueToSortable(v lua.LValue) (str string, num float64, isNum bool) {
	switch val := v.(type) {
	case lua.LNumber:
		return "", float64(val), true
	case lua.LString:
		s := string(val)
		// Try to parse as number for numeric sorting
		var n float64
		if _, err := fmt.Sscanf(s, "%f", &n); err == nil {
			return s, n, true
		}
		return s, 0, false
	case *lua.LNilType:
		return "", math.MaxFloat64, false // nil sorts last
	default:
		return v.String(), 0, false
	}
}

// hoursPerDay is the number of hours in a day.
const hoursPerDay = 24

// luaDaysSince implements rela.days_since(date_string) -> number
// Calculates the number of days between the given date and today.
// Accepts RFC3339 (2006-01-02T15:04:05Z07:00) or date-only (2006-01-02) formats.
// Returns -1 if the date cannot be parsed.
func luaDaysSince(ls *lua.LState) int {
	dateStr := ls.CheckString(1)
	if dateStr == "" {
		ls.Push(lua.LNumber(-1))
		return 1
	}

	t, err := parseDate(dateStr)
	if err != nil {
		ls.Push(lua.LNumber(-1))
		return 1
	}

	now := time.Now()
	days := int(now.Sub(t).Hours() / hoursPerDay)
	ls.Push(lua.LNumber(days))
	return 1
}
