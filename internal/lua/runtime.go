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
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/principal"
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
//
// TODO(TKT-N0IKN9): Runtime is a god-object (120 methods). Decompose toward the
// 40-method load line; ratchet this number down as bindings move out.
//
// 119 → 120 (TKT-ZF2DTV): reader(), the single choke point every read binding
// resolves its ACL-bound handle through. One method that makes six call sites
// unable to skip the gate is a good trade against this line.
//
//plimsoll:max-methods=120
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

	// listRender is set (non-nil Rows) only for a LIST document render — the
	// lists: export_render override. It rides on document mode rather than
	// introducing a third mode: a list render wants document mode's stdout
	// capture and its rela.output warning verbatim, and a parallel mode
	// would have to be OR-ed into every one of those guards forever.
	// Entity renders leave this zero, so rela.document carries no row
	// bindings there.
	listRender ListRenderContext
	aiProvider ai.Provider // nil means AI is not configured
	cache      cacheStore  // nil means rela.cache.* is not registered
	scriptPath string      // set by RunFile; empty for RunString/inline

	// principal is the identity this runtime runs as, exposed read-only as
	// rela.principal (TKT-5U6NRR). Set by [WithPrincipal] from the caller's
	// resolved principal; the zero value renders as {user:"", tool:""} which
	// the binding maps to the documented unknown fallback. The runtime does
	// NOT read it from the context itself — keeping principal resolution at
	// the caller avoids a context-flow the linter (contextcheck) would flag
	// and keeps the lua package free of ctx-identity plumbing.
	principal principal.Principal

	// errorFrames holds typed stack frames captured by the PCall message
	// handler on the most recent failed run. Read via ErrorFrames() after
	// PCall returns an error. Reset before every PCall.
	errorFrames []StackFrame
}

// cacheStore is the minimal contract the Lua cache bindings need from
// their backend. It's defined at the consumer (Runtime) so alternative
// implementations (a future disk-backed cache, a test fake) can drop in
// without touching wiring code. Kept unexported: callers still pass
// *Cache via WithCache and we keep the flexibility internally.
type cacheStore interface {
	get(namespacedKey string) ([]any, bool)
	set(namespacedKey string, values []any, ttl time.Duration)
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
// The parent ctx also carries values (Principal, triggered_by) into
// downstream Mutator calls — Lua write bindings derive their ctx from
// the parent so audit attribution flows naturally from caller →
// engine → Lua → Manager.
//
// Typical usage: pass cmd.Context() from a cobra RunE so that Ctrl+C
// interrupts script execution.
func WithContext(ctx context.Context) Option {
	return func(r *Runtime) {
		r.parentCtx = ctx
	}
}

// WithPrincipal sets the identity exposed read-only to scripts as
// rela.principal (TKT-5U6NRR). The caller resolves the principal (e.g.
// principal.From(ctx)) ONCE and passes the value here, so the lua package
// never reaches into the context for identity. Omitting this option leaves
// rela.principal as the unknown fallback.
func WithPrincipal(p principal.Principal) Option {
	return func(r *Runtime) {
		r.principal = p
	}
}

// callerCtx returns the parent context (or context.Background() if
// none was set). Used by write bindings to derive the ctx passed to
// the Mutator — so the caller's Principal and triggered_by flow
// through Lua into Manager.
func (r *Runtime) callerCtx() context.Context {
	if r.parentCtx != nil {
		return r.parentCtx
	}
	return context.Background()
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

// WithStandaloneDocumentMode marks the runtime as rendering a STANDALONE
// document — one declared without an `entity_type:` in data-entry.yaml, whose
// content is company-wide rather than about a single entity (TKT-M1AX6P). It
// sets document mode, so stdout capture and the rela.output warning behave
// exactly as for an entity render.
//
// Like [WithListDocumentMode] this is a SEPARATE constructor from
// [WithDocumentMode] rather than the latter called with an empty entryID:
// rela.document.entry_id must be Lua nil, and passing "" to express that is
// exactly the footgun registerContextBindings documents (empty string is
// truthy in Lua). Making the absence structural means no caller can get it
// wrong.
//
// Unlike [WithListDocumentMode] there are no row/query bindings — a standalone
// document has no driving list either. rela.document carries `id` alone.
func WithStandaloneDocumentMode(documentID string) Option {
	return func(r *Runtime) {
		r.isDocument = true
		r.documentID = documentID
	}
}

// WithListDocumentMode marks the runtime as rendering a LIST export (the
// `lists.<id>.export_render` override). It sets document mode, so stdout
// capture and the rela.output warning behave exactly as for an entity
// render, and additionally populates the row/query bindings on
// rela.document.
//
// It is a SEPARATE constructor from [WithDocumentMode] rather than extra
// optional arguments on it: a caller holding only WithDocumentMode cannot
// conjure a row provider, which keeps the typed-seam property that the
// script.Engine methods are the only way to enter either mode.
//
// entryID is deliberately not a parameter — a list render has no entry
// entity, and rela.document.entry_id stays absent (Lua nil) rather than
// empty-string. See registerContextBindings for why.
func WithListDocumentMode(documentID string, lrc ListRenderContext) Option {
	return func(r *Runtime) {
		r.isDocument = true
		r.documentID = documentID
		r.listRender = lrc
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
// ErrorFrames returns the typed Lua stack frames captured by the message
// handler on the most recent failed Run call. Empty for a successful run,
// or for failures (compile errors, Go-side errors) where no PCall ran.
//
// Frames are ordered innermost-first: frame[0] is where the error was
// raised; frame[len-1] is the main chunk.
func (r *Runtime) ErrorFrames() []StackFrame {
	return r.errorFrames
}

// pcallWithCapture runs r.L.PCall(0, MultRet, ...) with a message handler
// that walks the live Lua stack via GetStack/GetInfo and stores typed
// frames on the runtime. This sidesteps regex-parsing the Object/StackTrace
// strings: we get real line numbers, function names, and source paths
// straight from gopher-lua's debug API.
//
// See https://github.com/yuin/gopher-lua/issues/46 for the rationale.
func (r *Runtime) pcallWithCapture() error {
	r.errorFrames = nil
	handler := r.L.NewFunction(func(L *lua.LState) int {
		r.errorFrames = collectStackFrames(L)
		// Pass the original error object (arg #1) through unchanged so
		// gopher-lua wraps it into ApiError as usual.
		L.Push(L.Get(1))
		return 1
	})
	return r.L.PCall(0, lua.MultRet, handler)
}

// collectStackFrames walks the live Lua stack and returns user-visible
// frames (skipping built-in [G] frames with no source). Capped at
// maxStackFrames; safe to call from a message handler.
func collectStackFrames(ls *lua.LState) []StackFrame {
	var frames []StackFrame
	for level := range maxStackFrames {
		dbg, ok := ls.GetStack(level)
		if !ok {
			break
		}
		if _, err := ls.GetInfo("Slunf", dbg, lua.LNil); err != nil {
			break
		}
		// Skip built-in/C frames — they have no source and a CurrentLine
		// of -1. The user only cares about their own Lua code.
		if dbg.Source == "" || dbg.CurrentLine <= 0 {
			continue
		}
		frames = append(frames, StackFrame{
			Path: dbg.Source,
			Line: dbg.CurrentLine,
			Func: dbg.Name,
		})
	}
	return frames
}

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
	return r.pcallWithCapture()
}

// RunString executes Lua code from a string.
// Shebang lines (starting with #!) are automatically stripped.
func (r *Runtime) RunString(code string) error {
	r.applyTimeout()
	cleaned := stripShebang(code)
	fn, err := r.L.LoadString(cleaned)
	if err != nil {
		return err
	}
	r.L.Push(fn)
	return r.pcallWithCapture()
}

// ErrNoReturnValue is returned by RunActionString when the script did not
// return a value. Action handlers can use errors.Is to check for this.
var ErrNoReturnValue = errors.New("script did not return a value")

// RunActionString executes Lua code as an action, returning the script's
// top-of-stack return value as a Go interface{}. Returns ErrNoReturnValue
// if the script did not return any values.
func (r *Runtime) RunActionString(code, name string) (any, error) {
	r.applyTimeout()

	cleaned := stripShebang(code)
	fn, err := r.L.Load(strings.NewReader(cleaned), name)
	if err != nil {
		return nil, fmt.Errorf("cannot compile script: %w", err)
	}

	// Record stack depth before so we can detect if the script returned anything
	topBefore := r.L.GetTop()
	r.L.Push(fn)
	if pcallErr := r.pcallWithCapture(); pcallErr != nil {
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

// RunValidationString compiles `code` with `chunkname` as the chunk
// identity, runs it via the message-handler PCall, and returns the
// script's first return value as a raw gopher-lua LValue (LNil when
// the script returned nothing).
//
// Differs from RunActionString in that the return value is NOT
// converted via luaValueToGo — callers (validation rules) need direct
// access to *lua.LTable to walk return shapes that mix arrays and
// keyed entries (e.g. {message = ..., severity = ...}).
//
// The chunkname becomes the Path on each captured StackFrame, so a
// caller that uses chunkname == its envelope path gets matching frames
// without rewriting them. applyTimeout is invoked first so any
// WithTimeout / WithContext options on the runtime are honored.
func (r *Runtime) RunValidationString(code, chunkname string) (lua.LValue, error) {
	r.applyTimeout()

	cleaned := stripShebang(code)
	topBefore := r.L.GetTop()
	fn, err := r.L.Load(strings.NewReader(cleaned), chunkname)
	if err != nil {
		// Compile failures don't push anything, but be defensive so a
		// long-lived per-rule runtime can't accumulate orphaned LValues
		// across N entities (gopher-lua's stack ceiling is ~256).
		r.L.SetTop(topBefore)
		return nil, err
	}

	r.L.Push(fn)
	if pcallErr := r.pcallWithCapture(); pcallErr != nil {
		// PCall leaves the error message on the stack; reset to topBefore
		// so callers can invoke RunValidationString in a tight loop without
		// leaking stack slots per failed entity.
		r.L.SetTop(topBefore)
		return nil, pcallErr
	}
	topAfter := r.L.GetTop()

	if topAfter <= topBefore {
		return lua.LNil, nil
	}

	ret := r.L.Get(topBefore + 1)
	r.L.SetTop(topBefore)
	return ret, nil
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
		// rela.bypass_acl is registered ONLY when an elevated handle was wired
		// (an allow_acl_bypass automation action — TKT-D8T148). Absent BOTH, the
		// binding does not exist and a script cannot elevate.
		//
		// EITHER handle suffices (TKT-Y3JVFK). A document render is granted an
		// elevated READER and no elevated Mutator, so it can aggregate over rows
		// its caller cannot see while remaining unable to mutate: newElevatedHandle
		// omits the write methods when em is nil, so `admin.delete_entity` is
		// "attempt to call a nil value" rather than a guarded call. Reads-only is
		// structural, not a promise.
		if r.deps.ElevatedManager != nil || r.deps.ElevatedReader != nil {
			r.L.SetField(rela, "bypass_acl", r.L.NewFunction(r.luaBypassACL))
		}
	}
	r.registerContextBindings(rela)

	r.L.SetGlobal("rela", rela)

	// Top-level ai.* module (always registered; functions return a
	// typed not_configured error when no provider is wired).
	r.registerAIModule()

	// Top-level http.* module (always registered; no configuration needed).
	r.registerHTTPModule()

	// Top-level crypto.* module — generic hashing primitives so a Lua action
	// can sign an outbound request to an HMAC-authenticated upstream. Always
	// registered; no configuration needed. Free function (see crypto.go) to keep
	// the Runtime method count flat.
	registerCryptoModule(r)
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
	// rela.now_unix: the current time as unix seconds, stamped once at runtime
	// construction (like rela.today above). The Lua sandbox excludes the `os`
	// library, so a script has no os.time(); this is the sanctioned way to read
	// the current time — needed, e.g., to date an HMAC-signed outbound request
	// (see examples/idp-sync.lua). An action runtime is built per invocation, so
	// the value is fresh each run.
	r.L.SetField(rela, "now_unix", lua.LNumber(time.Now().Unix()))

	// Date and RRULE utility functions
	registerDateHelpers(r.L, rela)

	// Markdown AST and generation helpers module (rela.md.*)
	r.registerMarkdownModule(rela)

	// JSON encode/decode helpers module (rela.json.*)
	r.registerJSONModule(rela)
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

// freezeTable returns a read-only proxy over data: reads fall through to the
// backing table via __index, while any assignment raises a Lua error via
// __newindex. The backing table holds the actual key/values and is not exposed
// directly, so even existing keys cannot be reassigned. Used for rela.principal
// (TKT-5U6NRR) to make its read-only contract enforced rather than conventional.
func freezeTable(ls *lua.LState, data *lua.LTable) *lua.LTable {
	proxy := ls.NewTable()
	mt := ls.NewTable()
	ls.SetField(mt, "__index", data)
	ls.SetField(mt, "__newindex", ls.NewFunction(func(s *lua.LState) int {
		s.RaiseError("attempt to modify a read-only table")
		return 0
	}))
	// __metatable hides the metatable from getmetatable() and blocks
	// setmetatable() from swapping it out to bypass the guard.
	ls.SetField(mt, "__metatable", lua.LString("read-only"))
	ls.SetMetatable(proxy, mt)
	return proxy
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

	// Principal table: the identity on whose behalf this runtime executes,
	// supplied by the caller via [WithPrincipal] (TKT-5U6NRR). Write-path
	// automations use this to attribute relations to the acting user — e.g.
	// stamping a `created-by` edge from the request principal (X-Rela-User) at
	// submit time, which the client cannot forge. When WithPrincipal was not
	// passed (CLI, unstamped contexts), the zero Principal renders as the
	// documented {user="unknown", tool="unknown"} fallback so scripts can
	// always read the field safely.
	//
	// READ-ONLY by design (PLAN-XKMJ AC13 spoofing defense). This field only
	// *reads* the identity; it is NOT a write/attribution hook. Audit
	// attribution always derives from callerCtx() inside the write bindings,
	// never from this table — so even a script that mutates a local copy
	// cannot forge who a write is recorded as. To make the read-only contract
	// enforced rather than conventional, the table is frozen: assigning to it
	// raises a Lua error. (`rela.audit*` / `with_principal` / `with_triggered_by`
	// remain absent — those WOULD be rewrite vectors.)
	// "unknown" mirrors the fallback principal.From returns for an unstamped
	// context, so a script reads the same value whether identity was absent or
	// simply not passed via WithPrincipal.
	pUser, pTool := r.principal.User, r.principal.Tool
	if pUser == "" {
		pUser = "unknown"
	}
	if pTool == "" {
		pTool = "unknown"
	}
	principalTable := r.L.NewTable()
	r.L.SetField(principalTable, "user", lua.LString(pUser))
	r.L.SetField(principalTable, "tool", lua.LString(pTool))
	r.L.SetField(rela, "principal", freezeTable(r.L, principalTable))

	// rela.cache.{get,set,memoize} when a cache is wired. The binding
	// itself guards against inline/eval contexts (empty scriptPath) by
	// raising a Lua error on any call, so a runtime with a cache but
	// no script path still behaves safely.
	r.registerCacheBindings(rela)

	// rela.url submodule — typed URL builders for the SPA's routes.
	r.registerURLModule(rela)

	// Document-mode context: rela.mode + rela.document.*.
	// Only populated when WithDocumentMode, WithStandaloneDocumentMode or
	// WithListDocumentMode was applied. In every other context rela.mode and
	// rela.document are absent (Lua nil), so a script that branches on them
	// sees nil outside document renders.
	if r.isDocument {
		r.L.SetField(rela, "mode", lua.LString("document"))
		docTable := r.L.NewTable()
		r.L.SetField(docTable, "id", lua.LString(r.documentID))

		// entry_id is set ONLY when there is a real entry entity. An entity
		// render always has one (the caller path-validates it before we get
		// here), so this is byte-identical to the previous unconditional
		// set; a LIST render and a STANDALONE document render have none and
		// must see Lua nil, never "".
		// Empty-string would be the worse lie: it is truthy in Lua, so the
		// idiomatic `if rela.document.entry_id then rela.get_entity(...)`
		// guard would pass and then raise ("entity ID cannot be empty"),
		// and `entry_id or default` would yield "" instead of the default.
		if r.documentEntry != "" {
			r.L.SetField(docTable, "entry_id", lua.LString(r.documentEntry))
		}

		// List-render bindings. Kept as closures over the provider rather
		// than *Runtime methods on purpose: Runtime is pinned at the
		// plimsoll load line (max-methods=120), and these would push it
		// over. They are also genuinely per-render state, not runtime
		// behavior.
		if r.listRender.Rows != nil {
			registerListDocumentFields(r.L, docTable, r.listRender)
		}

		r.L.SetField(rela, "document", docTable)
	}
}

// registerListDocumentFields populates the LIST-render half of
// rela.document: the query context plus the lazy row accessors.
//
// A plain function rather than a *Runtime method because it needs nothing
// from the Runtime — the LState and the render context are the whole input.
// (It also keeps Runtime off its plimsoll load line, which sits at exactly
// max-methods=120. That is a consequence, not the reason: the fix for a full
// Runtime is to take fields OFF it — see TKT-N0IKN9 — not to route new
// methods around the counter.)
//
// Rows are materialized ONE AT A TIME. A 5000-row export therefore holds a
// single Lua entity table at a time rather than 5000 (each of which is a
// table plus a properties sub-table plus two closures). That is what makes
// the caller's existing row cap the only bound needed here — no second,
// separately-tuned cap for the script path.
func registerListDocumentFields(ls *lua.LState, docTable *lua.LTable, lrc ListRenderContext) {
	rows := lrc.Rows
	q := lrc.Query

	ls.SetField(docTable, "list_id", lua.LString(lrc.ListID))
	ls.SetField(docTable, "entity_type", lua.LString(q.EntityType))
	// count is ground truth (the rows the script can actually reach); total is
	// the caller's pre-cap count, and truncated is derived from the two rather
	// than stored, so those three can't drift apart.
	//
	// A caller that under-reports Total would still be incoherent, so clamp:
	// total can never be less than the rows we are about to hand over. That
	// makes "total >= count" an invariant of what the script sees rather than
	// a promise about what every caller passes.
	count := rows.Len()
	total := max(q.Total, count)
	ls.SetField(docTable, "count", lua.LNumber(count))
	ls.SetField(docTable, "total", lua.LNumber(total))
	ls.SetField(docTable, "truncated", lua.LBool(total > count))

	// The resolved request context, frozen so "read-only" is enforced
	// rather than conventional (same treatment rela.principal gets).
	queryTable := ls.NewTable()
	// Always set, even when empty. Unlike entry_id — where absence is
	// semantically right because a list HAS no entry entity — "no search
	// term" is a representable value, so the empty string is the honest
	// answer. Omitting it made `"Search: " .. rela.document.query.q` work on
	// every filtered export and hard-error on every unfiltered one.
	ls.SetField(queryTable, "q", lua.LString(q.Q))
	filters := ls.NewTable()
	for k, v := range q.Filters {
		ls.SetField(filters, k, lua.LString(v))
	}
	ls.SetField(queryTable, "filters", freezeTable(ls, filters))
	sort := ls.NewTable()
	for i, s := range q.Sort {
		spec := ls.NewTable()
		ls.SetField(spec, "property", lua.LString(s.Property))
		ls.SetField(spec, "direction", lua.LString(s.Direction))
		sort.RawSetInt(i+1, freezeTable(ls, spec))
	}
	ls.SetField(queryTable, "sort", freezeTable(ls, sort))
	ls.SetField(docTable, "query", freezeTable(ls, queryTable))

	// row(i) -> entity table | nil. 1-based, per Lua convention.
	ls.SetField(docTable, "row", ls.NewFunction(func(s *lua.LState) int {
		i := s.CheckInt(1)
		e := rows.At(i - 1)
		if e == nil {
			s.Push(lua.LNil)
			return 1
		}
		s.Push(EntityToTable(s, e))
		return 1
	}))

	// rows() -> stateful iterator, for `for _, row in rela.document.rows() do`.
	// Each CALL to rows() mints a fresh cursor, so a script may walk the set
	// more than once; each walk re-materializes its tables. That is the
	// deliberate trade for flat memory — CPU, not RAM. Do not memoize the
	// materialized tables here; that would reintroduce exactly the O(n)
	// retention the laziness exists to avoid.
	ls.SetField(docTable, "rows", ls.NewFunction(func(s *lua.LState) int {
		i := 0
		s.Push(s.NewFunction(func(inner *lua.LState) int {
			if i >= rows.Len() {
				inner.Push(lua.LNil)
				return 1
			}
			e := rows.At(i)
			i++
			inner.Push(lua.LNumber(i))
			inner.Push(EntityToTable(inner, e))
			return 2
		}))
		return 1
	}))
}

// reader returns the read-out handle, raising when it is absent.
//
// Absent means DENY, never "fall back to the raw store" (RR-X9NVHI): a
// wiring site that forgets to supply a reader must fail loudly rather than
// silently hand a script ungated access to the graph. Every read binding
// funnels through here so the check cannot be forgotten at one of them.
func (r *Runtime) reader(ls *lua.LState, binding string) (EntityReader, bool) {
	if r.deps.VisibleReader == nil {
		ls.RaiseError("%s: no reader is configured for this runtime", binding)
		return nil, false
	}
	return r.deps.VisibleReader, true
}

// luaGetEntity implements rela.get_entity(id) -> table|nil
func (r *Runtime) luaGetEntity(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}
	rd, ok := r.reader(ls, "rela.get_entity")
	if !ok {
		return 0
	}

	e, err := rd.GetEntity(r.callerCtx(), id)
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
	filterExpr, opts, err := listEntitiesArgs(ls)
	if err != nil {
		ls.RaiseError("rela.list_entities: %s", err.Error())
		return 0
	}
	rd, ok := r.reader(ls, "rela.list_entities")
	if !ok {
		return 0
	}

	// The bound is applied by stopping the iterator, not by slicing a
	// materialized set: rows past the limit are never loaded, redacted, or
	// turned into Lua tables. On fsstore that is a saved file read and parse
	// per skipped row.
	//
	// It bounds rows EXAMINED. With a property filter (which the store cannot
	// express, so it runs here) a full page in can be fewer rows out, and the
	// caller cannot tell "that is everything" from "the rest was filtered".
	// Resolving that needs the cursor -- DEC-IYHLNF stage 2.
	entities := make([]*entity.Entity, 0, min(opts.limit, initialRowCapacity))
	for e, err := range rd.ListEntities(r.callerCtx(), store.EntityQuery{Type: entityType}) {
		if err != nil {
			// RAISE, never break-and-return-what-we-have (TKT-FVQ4). A short
			// list is indistinguishable from a genuinely short result, so
			// swallowing this hands the script a wrong answer that looks right.
			ls.RaiseError("rela.list_entities: %s", err.Error())
			return 0
		}
		entities = append(entities, e)
		if len(entities) >= opts.limit {
			break
		}
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
// opts can have: from, type, to — each an optional string.
func (r *Runtime) luaGetRelations(ls *lua.LState) int {
	// Shares relationQuery with the elevated admin.get_relations so the two
	// surfaces cannot drift on what a filter means (RR-D7KXKV). A non-string
	// option raises here as it does there: silently dropping it turns a scoped
	// question into a whole-graph one that the script reads as scoped.
	q, err := relationQuery(ls)
	if err != nil {
		ls.RaiseError("rela.get_relations: %s", err.Error())
		return 0
	}

	rd, ok := r.reader(ls, "rela.get_relations")
	if !ok {
		return 0
	}

	// Peer-gated (RR-7GDT1Y): the reader drops relations whose FROM or TO
	// the caller cannot see, so an explicit opts.from can legitimately
	// return fewer rows than the graph holds. An empty result means "no
	// edges you may see", not "no edges".
	result := ls.NewTable()
	idx := 1
	for rel, err := range rd.ListRelations(r.callerCtx(), q) {
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

	trace := r.deps.Tracer.TraceFrom(r.callerCtx(), id, maxDepth)
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

	trace := r.deps.Tracer.TraceTo(r.callerCtx(), id, maxDepth)
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

	// Names withheld by field-level ACL, as a set so a script can both
	// iterate it and test membership. Always present (empty when nothing
	// was hidden) — on ungated runtimes (CLI, MCP, docs) it is always
	// empty because no policy was evaluated. Values are NOT here; they
	// were stripped from properties above.
	redacted := ls.NewTable()
	for _, name := range e.Redacted {
		redacted.RawSetString(name, lua.LTrue)
	}
	t.RawSetString("redacted", redacted)

	// Add prop(name, default) method via a function field
	t.RawSetString("prop", ls.NewFunction(luaEntityProp))

	// Add is_redacted(name) method to distinguish withheld from unset
	t.RawSetString("is_redacted", ls.NewFunction(luaEntityIsRedacted))

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

// luaEntityIsRedacted implements entity:is_redacted(name) -> bool
// Reports whether field-level ACL withheld the named property from the
// reading principal, letting a script render "[redacted]" rather than a
// blank that reads as "never set".
//
// False means "not withheld", which on an ungated runtime (CLI, MCP,
// docs) is every property — those paths evaluate no policy.
func luaEntityIsRedacted(ls *lua.LState) int {
	self := ls.CheckTable(1)
	name := ls.CheckString(2)

	redactedVal := self.RawGetString("redacted")
	redacted, ok := redactedVal.(*lua.LTable)
	if !ok {
		ls.Push(lua.LFalse)
		return 1
	}

	ls.Push(lua.LBool(lua.LVAsBool(redacted.RawGetString(name))))
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
func GoToLuaValue(ls *lua.LState, v any) lua.LValue {
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
	case []any:
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
	case map[string]any:
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
func luaValueToGo(lv lua.LValue) any {
	return luaValueToGoSeen(lv, make(map[*lua.LTable]bool))
}

// luaNumberToGo converts a Lua number to the most faithful Go type.
// gopher-lua has a single float64-backed number type, but a value that
// is integral and fits in int64 round-trips better as an int64 — an
// entity ID, a ticket number, or epoch-nanos would otherwise lose its
// integer type (and re-serialize in exponential / trailing-.0 form). The
// reverse direction (GoToLuaValue) already preserves int/int64, so this
// closes the lossy leg. Non-integral or out-of-int64-range values stay
// float64. (Integers beyond 2^53 can't be held by the float64 LNumber in
// the first place, so this is type-faithful up to that ceiling.)
func luaNumberToGo(n lua.LNumber) any {
	f := float64(n)
	if f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64 {
		return int64(f)
	}
	return f
}

func luaValueToGoSeen(lv lua.LValue, seen map[*lua.LTable]bool) any {
	switch v := lv.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return luaNumberToGo(v)
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
func luaTableToGoSeen(t *lua.LTable, seen map[*lua.LTable]bool) any {
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
		arr := make([]any, maxN)
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
	m := make(map[string]any)
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
	rd, ok := r.reader(ls, "rela.search")
	if !ok {
		return 0
	}
	result := ls.NewTable()
	i := 1
	ctx := r.callerCtx()
	for hit, err := range r.deps.Searcher.Search(ctx, search.Query{Text: query, Limit: limit}) {
		if err != nil {
			ls.RaiseError("search error: %s", err.Error())
			return 0
		}
		// Fetch the full entity for the lua table (search hits are minimal).
		// This hydration is ALSO the gate: hits the caller may not read fail
		// here and are skipped, so no hidden entity or property reaches the
		// script. The hit LIST itself is un-gated — see ReadDeps.Searcher for
		// the residual (TKT-GGQ0JT class).
		e, err := rd.GetEntity(ctx, hit.ID)
		if err != nil {
			// A denied hit arrives as ErrNotFound and is skipped silently —
			// that is the gate working. Anything else is a real fault, and
			// since redaction makes short results EXPECTED it would
			// otherwise be indistinguishable from normal policy behavior
			// (RR-QSP6X2).
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("lua: rela.search: hit hydration failed",
					"id", hit.ID, "err", err)
			}
			continue
		}
		result.RawSetInt(i, EntityToTable(ls, e))
		i++
	}
	ls.Push(result)
	return 1
}

// luaCreateEntity implements rela.create_entity(type, properties, content?, id?) -> (entity, warnings).
//
// Multi-return contract follows string.gsub semantics (NOT io.open):
// both return values can be non-nil simultaneously. The first value
// is the created entity; the second is a Lua array of DEC-HWZHA soft
// validation warnings, or nil when there are none. Hard errors
// (unknown entity type, bad ID prefix) still raise via RaiseError.
//
// Existing scripts that do `local e = rela.create_entity(...)`
// continue to work unchanged — the second return is silently dropped.
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
		r.callerCtx(), newE, entity.CreateOptions{ID: customID})
	if err != nil {
		ls.RaiseError("create entity error: %s", err.Error())
		return 0
	}

	ls.Push(EntityToTable(ls, result.Entity))
	ls.Push(WarningsToTable(ls, result.Warnings))
	return 2
}

// luaUpdateEntity implements rela.update_entity(id, properties, content?) -> (entity, warnings).
//
// Multi-return contract follows string.gsub semantics (NOT io.open):
// both return values can be non-nil simultaneously. The first value
// is the updated entity; the second is a Lua array of DEC-HWZHA soft
// validation warnings, or nil when there are none. Hard errors
// (entity not found, unknown type, bad ID prefix) still raise via
// RaiseError.
//
// Existing scripts that do `local e = rela.update_entity(...)`
// continue to work unchanged — the second return is silently dropped.
func (r *Runtime) luaUpdateEntity(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}

	ctx := r.callerCtx()

	// A TARGETED write: name only what the script actually supplied and let
	// the manager merge it against the raw stored entity. The binding holds
	// no store handle at all, so the read-before-write that used to erase a
	// caller's hidden properties is now unreachable from here (TKT-80EWGM).
	var patch entity.Patch

	// Merge properties if provided
	if ls.GetTop() >= 2 && ls.Get(2).Type() == lua.LTTable {
		patch.Properties = luaTableToGoMap(ls.CheckTable(2))
	}

	// Update content if provided (nil means not provided, empty string clears content)
	if ls.GetTop() >= 3 && ls.Get(3).Type() != lua.LTNil {
		content := ls.CheckString(3)
		patch.Content = &content
	}

	result, err := r.deps.EntityManager.PatchEntity(ctx, id, patch)
	if err != nil {
		// Preserve the pre-TKT-80EWGM message for a missing entity: scripts
		// match on it. The check is STRUCTURAL (see [NotFoundError]) — a
		// text match would misreport an illegal-transition rejection, whose
		// message embeds the caller's own property value, as a 404.
		if isEntityNotFound(err) {
			ls.RaiseError("entity not found: %s", id)
			return 0
		}
		ls.RaiseError("update entity error: %s", err.Error())
		return 0
	}

	ls.Push(EntityToTable(ls, result.Entity))
	ls.Push(WarningsToTable(ls, result.Warnings))
	return 2
}

// WarningsToTable converts a slice of entity.Warning to a Lua
// table of {code, path, detail} sub-tables. Returns lua.LNil (NOT
// an empty table) when the slice is empty so scripts can use the
// `for _, w in ipairs(warnings or {})` pattern idiomatically and
// simple `if warnings then` truthiness checks work.
//
// This is the second return value of rela.update_entity and
// rela.create_entity, following string.gsub's "(value, count)"
// pattern — both returns can be non-nil simultaneously, and the
// second is additional success information, NOT an error indicator.
func WarningsToTable(ls *lua.LState, warnings []entity.Warning) lua.LValue {
	if len(warnings) == 0 {
		return lua.LNil
	}
	tbl := ls.NewTable()
	for _, w := range warnings {
		wt := ls.NewTable()
		ls.SetField(wt, "code", lua.LString(w.Code))
		ls.SetField(wt, "path", lua.LString(w.Path))
		ls.SetField(wt, "detail", lua.LString(w.Detail))
		tbl.Append(wt)
	}
	return tbl
}

// luaBypassACL implements rela.bypass_acl(fn) (TKT-D8T148). It invokes fn with
// a single argument `admin`: a handle whose reads and writes skip the ACL,
// backed by the elevated Mutator and/or EntityReader wired into WriteDeps.
// Elevation is therefore an OBJECT CAPABILITY scoped to the closure — the gated
// rela.* bindings are never elevated; only access through `admin` bypasses ACL.
//
// The handle carries only the capabilities that were wired (TKT-Y3JVFK): a
// document render gets an elevated reader and no Mutator, so `admin` has the
// three read methods and no write methods at all. See newElevatedHandle.
//
// After fn returns (or raises), `admin` is INVALIDATED: its methods raise. A
// script that squirrels `admin` into a global and calls it later gets a dead
// handle, so the lexical scope is enforced, not merely conventional (mirrors
// the frozen rela.principal). fn's return value(s) propagate to the caller; a
// raise inside fn propagates too (a failed elevated write must surface).
func (r *Runtime) luaBypassACL(ls *lua.LState) int {
	fn := ls.CheckFunction(1)
	if r.deps.ElevatedManager == nil && r.deps.ElevatedReader == nil {
		// Defensive: the binding is only registered when at least one elevated
		// handle is set, but fail loud rather than silently no-op if that ever
		// drifts. Note this raises only when BOTH are absent — a reader-only
		// elevation is legitimate (TKT-Y3JVFK), and a manager-only one keeps
		// its existing behavior of raising per-method inside readGuard.
		ls.RaiseError("rela.bypass_acl: no elevated handle is available")
		return 0
	}

	// live gates every admin.* call; set false after fn returns so a captured
	// handle is dead outside the closure's dynamic extent.
	live := true
	// reads accumulates the distinct elevated read bindings this closure
	// used, for the single post-closure audit record (TKT-ACSBSA).
	reads := &readUsage{}
	admin := r.newElevatedHandle(ls, r.deps.ElevatedManager, r.deps.ElevatedReader, &live, reads)

	// Invalidate on every exit path (normal return or Lua error). pcall keeps
	// the runtime alive so we can flip `live` before re-raising.
	//
	// The audit record rides the SAME defer, so a closure that reads raw data
	// and then raises still leaves a trace. Recording only on the success
	// path would let a script read everything and erase the evidence by
	// failing — the exact shape an attacker would choose.
	defer func() {
		live = false
		recordElevatedReads(r.callerCtx(), r.deps.ElevationRecorder, reads)
	}()

	ls.Push(fn)
	ls.Push(admin)
	// Protected call so we can guarantee invalidation even when fn raises,
	// then re-surface the error to the caller.
	if err := ls.PCall(1, lua.MultRet, nil); err != nil {
		live = false
		ls.RaiseError("rela.bypass_acl: %s", err.Error())
		return 0
	}
	// Return whatever fn returned (already on the stack after PCall).
	return ls.GetTop() - 1
}

// newElevatedHandle builds the `admin` table passed to a rela.bypass_acl
// closure. Its methods route to the elevated Mutator `em` (writes) and the
// elevated EntityReader `er` (reads), and check `*live` first, so they raise
// once the closure has returned. No principal, no nested bypass.
//
// Write surface: create_relation, delete_relation, delete_entity — the
// link/unlink + remove operations the system-invariant use cases (e.g.
// authorship stamping via created-by) need. create_entity / update_entity are a
// deliberate follow-up: they marshal a full entity table and aren't required by
// the motivating case; gating elevated *entity* creation is a larger surface
// best added with its own tests.
//
// Read surface (TKT-ACSBSA): get_entity, list_entities, get_relations —
// mirroring the gated rela.* bindings one-for-one so a script can lift a read
// into the closure without rewriting it. Reads are RAW: full properties, no
// row gate, no redaction. A half-elevated read is a confusing contract and the
// closure is already the boundary.
//
// A nil `er` leaves the three read methods present but RAISING, not absent.
// Absence would make `if admin.get_entity then` silently take the
// no-elevation branch on a misconfigured deployment; raising names the
// missing capability. Writes behave the same way via the outer nil check on
// ElevatedManager.
// readUsage accumulates which elevated read bindings a bypass_acl closure
// actually used, so the post-closure audit record can name them. Order is
// first-use, and each binding appears once — the record answers "what kind
// of raw access happened", not "how many times".
//
// Not safe for concurrent use, and does not need to be: a Lua state is
// single-goroutine, and one readUsage is scoped to one closure.
type readUsage struct{ names []string }

// mark records a use of binding `name`, ignoring repeats.
func (u *readUsage) mark(name string) {
	if slices.Contains(u.names, name) {
		return
	}
	u.names = append(u.names, name)
}

// recordElevatedReads emits the single post-closure audit notification when
// the closure used its read elevation. Silent when no recorder is wired or
// when the closure performed no elevated reads — a bypass_acl block that
// only writes is already covered by entitymanager's OpACLBypass rows, and
// an empty record would just add noise to the log.
func recordElevatedReads(ctx context.Context, rec ElevationRecorder, u *readUsage) {
	if rec == nil || len(u.names) == 0 {
		return
	}
	rec.RecordElevatedRead(ctx, u.names)
}

func (r *Runtime) newElevatedHandle(
	ls *lua.LState, em Mutator, er EntityReader, live *bool, reads *readUsage,
) *lua.LTable {
	t := ls.NewTable()
	guard := func(name string) bool {
		if !*live {
			ls.RaiseError("rela.bypass_acl: handle %q used outside its closure (invalidated)", name)
			return false
		}
		return true
	}
	// readGuard adds the wired-reader check to the liveness check. Both are
	// required before any elevated read touches the store.
	//
	// It deliberately does NOT mark the binding as used. Marking here would
	// audit a read that never reached the store — the argument-validation
	// raises (empty id, empty type) fire AFTER this guard, so a closure doing
	// only `pcall(admin.get_entity, "")` would produce an `acl-bypass-read`
	// row claiming a disclosure that never happened. Each method calls
	// reads.mark immediately before its er.* call instead, so the audit row
	// means what it says.
	readGuard := func(name string) bool {
		if !guard(name) {
			return false
		}
		if er == nil {
			ls.RaiseError("rela.bypass_acl: %s: no elevated reader is configured for this runtime", name)
			return false
		}
		return true
	}
	// Write methods are ABSENT (not present-and-raising) when no elevated
	// Mutator was wired — the asymmetry with `er` above is deliberate
	// (TKT-Y3JVFK). A nil `er` means "elevation was intended but the reader is
	// missing", i.e. a misconfiguration, so raising names the missing
	// capability. A nil `em` on a document render means the opposite: the
	// runtime is READ-ONLY BY CONSTRUCTION and never had writes to lose, so
	// `admin.delete_entity == nil` is the honest contract, and a script probing
	// `if admin.delete_entity then` correctly learns it cannot mutate. It also
	// makes "a render cannot write" structural rather than a guarded promise.
	if em != nil {
		registerElevatedWrites(ls, t, em, guard, r.callerCtx)
	}
	registerElevatedReads(ls, t, er, readGuard, r.callerCtx, reads)
	return t
}

// registerElevatedWrites adds the raw write methods to the `admin` table
// (TKT-D8T148). Split out for the same reason as registerElevatedReads — the
// function-length limit — and called only when an elevated Mutator was wired,
// so a read-only elevation has no write methods at all (TKT-Y3JVFK).
//
// create_entity / update_entity remain absent by design: they marshal a full
// entity table and were deferred with their own tests as the follow-up noted
// in newElevatedHandle's doc.
func registerElevatedWrites(
	ls *lua.LState, t *lua.LTable, em Mutator, guard func(string) bool,
	ctxFn func() context.Context,
) {
	ls.SetField(t, "create_relation", ls.NewFunction(func(s *lua.LState) int {
		if !guard("create_relation") {
			return 0
		}
		from, relType, to := s.CheckString(1), s.CheckString(2), s.CheckString(3)
		if _, err := em.CreateRelation(ctxFn(), from, relType, to, entity.RelationOptions{}); err != nil {
			s.RaiseError("bypass_acl create_relation error: %s", err.Error())
			return 0
		}
		s.Push(lua.LTrue)
		return 1
	}))
	ls.SetField(t, "delete_relation", ls.NewFunction(func(s *lua.LState) int {
		if !guard("delete_relation") {
			return 0
		}
		from, relType, to := s.CheckString(1), s.CheckString(2), s.CheckString(3)
		if err := em.DeleteRelation(ctxFn(), from, relType, to); err != nil {
			s.RaiseError("bypass_acl delete_relation error: %s", err.Error())
			return 0
		}
		s.Push(lua.LTrue)
		return 1
	}))
	ls.SetField(t, "delete_entity", ls.NewFunction(func(s *lua.LState) int {
		if !guard("delete_entity") {
			return 0
		}
		id := s.CheckString(1)
		cascade := s.OptBool(2, false)
		if _, err := em.DeleteEntity(ctxFn(), id, cascade); err != nil {
			s.RaiseError("bypass_acl delete_entity error: %s", err.Error())
			return 0
		}
		s.Push(lua.LTrue)
		return 1
	}))
}

// registerElevatedReads adds the three raw read methods to the `admin` table
// (TKT-ACSBSA). Split out of newElevatedHandle to keep each function within
// the length limit; `readGuard` carries both the liveness and wired-reader
// checks so neither can be forgotten at an individual method.
//
// Deliberately NOT sharing code with the gated luaGetEntity / luaListEntities
// / luaGetRelations bindings: those funnel through r.reader() (which resolves
// VisibleReader), and a shared helper parameterized by reader would be one
// edit away from letting a gated binding read raw. The duplication here is
// small and it keeps the two read paths physically separate.
func registerElevatedReads(
	ls *lua.LState, t *lua.LTable, er EntityReader, readGuard func(string) bool,
	ctxFn func() context.Context, reads *readUsage,
) {
	ls.SetField(t, "get_entity", ls.NewFunction(elevatedGetEntity(er, readGuard, ctxFn, reads)))
	ls.SetField(t, "list_entities", ls.NewFunction(elevatedListEntities(er, readGuard, ctxFn, reads)))
	ls.SetField(t, "get_relations", ls.NewFunction(elevatedGetRelations(er, readGuard, ctxFn, reads)))
}

// elevatedGetEntity builds admin.get_entity(id) -> table|nil.
//
// Returns nil on a miss, matching rela.get_entity. The two nils mean
// different things, though: under elevation a nil means the entity
// genuinely does not exist, where the gated binding's nil is the
// deliberately ambiguous "missing or hidden" that keeps it oracle-free.
func elevatedGetEntity(
	er EntityReader, readGuard func(string) bool, ctxFn func() context.Context,
	reads *readUsage,
) func(*lua.LState) int {
	return func(s *lua.LState) int {
		if !readGuard("get_entity") {
			return 0
		}
		id := s.CheckString(1)
		if id == "" {
			s.RaiseError("bypass_acl get_entity: entity ID cannot be empty")
			return 0
		}
		reads.mark("get_entity")
		e, err := er.GetEntity(ctxFn(), id)
		if err != nil {
			// Only a genuine MISS is nil. Any other error (store down, driver
			// failure) RAISES — masking it as nil would make the documented
			// contract ("nil means it does not exist") false, and would break
			// the motivating use case: a uniqueness check that reads nil on a
			// transient outage concludes "no duplicate" and lets the invariant
			// the elevated read exists to enforce be violated. The two list
			// bindings already raise on iteration errors; this keeps the three
			// consistent.
			if errors.Is(err, store.ErrNotFound) {
				s.Push(lua.LNil)
				return 1
			}
			s.RaiseError("bypass_acl get_entity error: %s", err.Error())
			return 0
		}
		s.Push(EntityToTable(s, e))
		return 1
	}
}

// elevatedListEntities builds admin.list_entities(type) -> table.
//
// No filter-expression argument: rela.list_entities' filter is a
// convenience over an already-gated set, and adding an expression parser to
// the elevated path widens it for no gain — a script can filter the
// returned table in Lua. Unbounded, like its gated counterpart
// (TKT-YWDGZD tracks paging for both).
func elevatedListEntities(
	er EntityReader, readGuard func(string) bool, ctxFn func() context.Context,
	reads *readUsage,
) func(*lua.LState) int {
	return func(s *lua.LState) int {
		if !readGuard("list_entities") {
			return 0
		}
		entityType := s.CheckString(1)
		if entityType == "" {
			s.RaiseError("bypass_acl list_entities: entity type cannot be empty")
			return 0
		}
		reads.mark("list_entities")
		result := s.NewTable()
		idx := 1
		for e, err := range er.ListEntities(ctxFn(), store.EntityQuery{Type: entityType}) {
			if err != nil {
				s.RaiseError("bypass_acl list_entities error: %s", err.Error())
				return 0
			}
			result.RawSetInt(idx, EntityToTable(s, e))
			idx++
		}
		s.Push(result)
		return 1
	}
}

// elevatedGetRelations builds admin.get_relations(opts?) -> table, with
// opts.{from,type,to}.
//
// NOT peer-gated (unlike rela.get_relations): an edge is returned even when
// neither endpoint would be visible to the caller. Re-adding the peer drop
// here would look like a safety improvement and would silently make the
// elevated view incomplete.
func elevatedGetRelations(
	er EntityReader, readGuard func(string) bool, ctxFn func() context.Context,
	reads *readUsage,
) func(*lua.LState) int {
	return func(s *lua.LState) int {
		if !readGuard("get_relations") {
			return 0
		}
		q, err := relationQuery(s)
		if err != nil {
			s.RaiseError("bypass_acl get_relations: %s", err.Error())
			return 0
		}
		reads.mark("get_relations")
		result := s.NewTable()
		idx := 1
		for rel, err := range er.ListRelations(ctxFn(), q) {
			if err != nil {
				s.RaiseError("bypass_acl get_relations error: %s", err.Error())
				return 0
			}
			result.RawSetInt(idx, relationToTable(s, rel))
			idx++
		}
		s.Push(result)
		return 1
	}
}

// relationQuery reads the optional {from,type,to} options table off the Lua
// stack. An absent or non-table argument yields the zero query, which matches
// every relation.
//
// Shared by the gated rela.get_relations and the elevated admin.get_relations
// so the two cannot disagree about what a filter means. Callers differ only in
// what they do with the result: the gated reader additionally peer-gates the
// rows, the elevated one does not.
func relationQuery(s *lua.LState) (store.RelationQuery, error) {
	var q store.RelationQuery
	if s.GetTop() < 1 || s.Get(1).Type() != lua.LTTable {
		return q, nil
	}
	opts := s.CheckTable(1)
	fields := []struct {
		name string
		dst  *string
	}{
		{"from", &q.From}, {"type", &q.Type}, {"to", &q.To},
	}
	for _, f := range fields {
		switch v := opts.RawGetString(f.name).(type) {
		case *lua.LNilType:
			// Absent: no constraint on this field.
		case lua.LString:
			*f.dst = string(v)
		default:
			// A non-string is REJECTED rather than skipped. Silently dropping
			// it turns a mistyped filter (`{from = 12345}` when an id came
			// back as a number) into an unfiltered whole-graph edge dump that
			// the script reads as a filtered result. On the elevated path
			// nothing gates that at all; on the gated path peer-gating bounds
			// the rows to the caller's own view, so the disclosure is bounded
			// but the answer is still silently the wrong question.
			return q, fmt.Errorf(
				"option %q must be a string, got %s", f.name, v.Type())
		}
	}
	return q, nil
}

// luaDeleteEntity implements rela.delete_entity(id, cascade?) -> boolean
func (r *Runtime) luaDeleteEntity(ls *lua.LState) int {
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("entity ID cannot be empty")
		return 0
	}

	cascade := ls.OptBool(2, false)

	if _, err := r.deps.EntityManager.DeleteEntity(r.callerCtx(), id, cascade); err != nil {
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
		r.callerCtx(), from, relType, to, entity.RelationOptions{})
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

	if err := r.deps.EntityManager.DeleteRelation(r.callerCtx(), from, relType, to); err != nil {
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

	path := r.deps.Tracer.FindPath(r.callerCtx(), from, to)
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
func luaTableToGoMap(t *lua.LTable) map[string]any {
	m := make(map[string]any)
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

// sortEntries sorts entity entries by their property value. Stable so
// entries with equal sort keys keep their input order.
func sortEntries(entries []sortableEntry, descending bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		if descending {
			return entryLess(entries[j].prop, entries[i].prop)
		}
		return entryLess(entries[i].prop, entries[j].prop)
	})
}

// entryLess reports whether a sorts before b. Two numeric values compare
// numerically; otherwise both compare as strings.
func entryLess(a, b lua.LValue) bool {
	aStr, aNum, aIsNum := luaValueToSortable(a)
	bStr, bNum, bIsNum := luaValueToSortable(b)
	if aIsNum && bIsNum {
		return aNum < bNum
	}
	return aStr < bStr
}

// luaValueToSortable converts a Lua value to sortable string and number
// representations. A string is treated as numeric only when it parses
// *entirely* as a number — strconv.ParseFloat over the trimmed value —
// so "1.2.0" or "3 blind mice" sort lexicographically rather than being
// silently reduced to their numeric prefix (1 and 3), which the old
// fmt.Sscanf("%f") accepted.
func luaValueToSortable(v lua.LValue) (str string, num float64, isNum bool) {
	switch val := v.(type) {
	case lua.LNumber:
		return "", float64(val), true
	case lua.LString:
		s := string(val)
		if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
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
