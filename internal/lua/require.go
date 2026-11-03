package lua

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// ModuleDir is the project directory that holds shared Lua modules. A script
// loads lib/fmt/date.lua with require("fmt.date").
const ModuleDir = "lib"

// maxModuleNameLen bounds a module name. Nesting deeper than this is not a
// real layout, and the bound keeps a computed name from building a huge path.
const maxModuleNameLen = 128

// moduleNamePattern is the allowlist for module names: dot-separated
// segments of lower-case ASCII letters, digits and underscores. It rules out
// separators, "..", absolute paths, volumes and NUL bytes before a path is
// built from the name.
var moduleNamePattern = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)*$`)

// maxModuleSize bounds a module's source. It equals the largest file the
// error envelope excerpts, so every module that loads can be shown when it
// fails. The file is read whole before the check; lib/ is operator-authored,
// so the bound catches a mistake, not an attacker.
const maxModuleSize = maxSourceFileSize

// maxModuleDepth bounds how deeply modules may require each other while
// loading. Each level runs on a thread of its own (see moduleLoader.run),
// which the caller's call-stack limit does not count.
const maxModuleDepth = 32

// moduleRegistrySize is the initial register count of a module's thread. It
// grows on demand up to the parent's size, so a module costs kilobytes, not
// the megabyte a full-size registry would.
const moduleRegistrySize = 1024

// standardModules are the libraries the sandbox opens. require returns them
// by name, as standard Lua does.
var standardModules = map[string]bool{
	lua.TabLibName: true, lua.StringLibName: true, lua.MathLibName: true, lua.CoroutineLibName: true,
}

// reservedModules are names a lib/ module may not take as its first segment:
// rela's own globals and the standard libraries, opened or not. A module
// named after one would be shadowed, or would shadow it, depending on which
// loads first.
var reservedModules = map[string]bool{
	"rela": true, "http": true, "ai": true, "crypto": true, "mail": true,
	"string": true, "table": true, "math": true, "coroutine": true,
	"os": true, "io": true, "debug": true, "package": true, "channel": true,
	"utf8": true, "bit32": true,
}

// moduleLoader is the rela-owned require. gopher-lua's own require needs the
// package library, which reads through the os package and so would bypass
// the project's file containment; rela never opens it.
//
// Loaded modules are cached per runtime, so a module required twice in one
// run executes once. Each run builds a fresh runtime, so an edited module
// takes effect on the next run, as an edited script does.
type moduleLoader struct {
	// files reads lib/. Nil: accepted — require then raises for every name
	// that is not a standard library, because the runtime has no project to
	// read modules from.
	files  ProjectFiles
	loaded map[string]lua.LValue
	// loading is the chain of modules being loaded, outermost first. A name
	// already on it is a cycle.
	loading []string
	// pendingFrame is the module line an error raised by require points at:
	// a compile error, or an error in the module's top level, which runs on
	// a thread of its own. No frame on the caller's stack carries that line;
	// the runtime's message handler adds it (takePendingFrame).
	pendingFrame   *StackFrame
	pendingMessage string
}

// installRequire replaces the base library's require with a module loader.
func installRequire(ls *lua.LState, files ProjectFiles) *moduleLoader {
	m := &moduleLoader{files: files, loaded: map[string]lua.LValue{}}
	ls.SetGlobal("require", ls.NewFunction(m.require))
	return m
}

func (m *moduleLoader) require(ls *lua.LState) int {
	m.pendingFrame = nil
	nameValue, ok := ls.Get(1).(lua.LString)
	if !ok {
		ls.ArgError(1, "module name must be a string, got "+ls.Get(1).Type().String())
	}
	name := string(nameValue)
	if standardModules[name] {
		// From the registry, not the global: a script may have reassigned
		// the global.
		loaded, ok := ls.GetField(ls.Get(lua.RegistryIndex), "_LOADED").(*lua.LTable)
		if !ok {
			ls.RaiseError("require: standard library table missing")
		}
		ls.Push(loaded.RawGetString(name))
		return 1
	}
	if len(name) > maxModuleNameLen || !moduleNamePattern.MatchString(name) {
		ls.ArgError(1, "invalid module name "+quoteName(name)+
			": use dot-separated lower-case letters, digits and underscores, e.g. \"fmt.date\"")
	}
	if first, _, _ := strings.Cut(name, "."); reservedModules[first] {
		ls.ArgError(1, "module name "+quoteName(name)+" is reserved: "+first+" is a built-in name")
	}
	if v, ok := m.loaded[name]; ok {
		ls.Push(v)
		return 1
	}

	for i, n := range m.loading {
		if n == name {
			chain := append(append([]string{}, m.loading[i:]...), name)
			ls.RaiseError("require cycle: %s", strings.Join(chain, " -> "))
		}
	}

	if m.files == nil {
		ls.RaiseError("require %s: project modules are not available in this context", name)
	}
	if len(m.loading) >= maxModuleDepth {
		ls.RaiseError("require %s: modules nested more than %d deep while loading (%s -> ...)",
			name, maxModuleDepth, m.loading[0])
	}

	path := ModuleDir + "/" + strings.ReplaceAll(name, ".", "/") + ".lua"
	fn := m.compile(ls, name, path)

	m.loading = append(m.loading, name)
	defer func() { m.loading = m.loading[:len(m.loading)-1] }()

	ret := m.run(ls, name, path, fn)
	if ret == lua.LNil {
		ret = lua.LTrue
	}
	m.loaded[name] = ret
	ls.Push(ret)
	return 1
}

// run executes a module's top-level code in its own coroutine and returns its
// first result. A yield from the top level (coroutine.yield, rela.flow.emit)
// would otherwise suspend the caller in the middle of this Go function, which
// gopher-lua cannot resume; in a thread of its own the yield is caught and
// reported instead. The thread inherits the caller's context, so the run's
// timeout still applies.
func (m *moduleLoader) run(ls *lua.LState, name, path string, fn *lua.LFunction) lua.LValue {
	co, cancel := newModuleThread(ls)
	if cancel != nil {
		defer cancel()
	}
	defer co.Close()
	status, err, values := ls.Resume(co, fn, lua.LString(name))
	switch status {
	case lua.ResumeOK:
	case lua.ResumeYield:
		ls.RaiseError("module %s (%s) yielded while loading; "+
			"yield (coroutine.yield, rela.flow.emit) from a function the module returns instead", name, path)
	case lua.ResumeError:
		var errValue lua.LValue = lua.LString(err.Error())
		var ae *lua.ApiError
		if errors.As(err, &ae) {
			errValue = ae.Object
		}
		msg, isString := errValue.(lua.LString)
		if !isString {
			// A table or other value is passed through unchanged, as Lua's
			// own require does, so pcall(require, ...) sees what error() got.
			m.pendingFrame = nil
			ls.Error(errValue, 0)
		}
		// The module's frames are on the abandoned thread, so the caller's
		// message handler cannot see them; the line the error names is kept.
		// A nested require may already have recorded a deeper line (a
		// compile error in the inner module); that one wins.
		if m.pendingFrame == nil || !strings.HasSuffix(string(msg), m.pendingMessage) {
			m.pendingFrame = nil
			if mm := moduleErrorPosition.FindStringSubmatch(string(msg)); mm != nil {
				if line, convErr := strconv.Atoi(mm[2]); convErr == nil {
					m.pendingFrame = &StackFrame{Path: mm[1], Line: line}
				}
			}
			m.pendingMessage = string(msg)
		}
		ls.Error(msg, 0)
	}
	if len(values) == 0 {
		return lua.LNil
	}
	return values[0]
}

// newModuleThread is ls.NewThread with a small, growable registry and a
// call stack that grows on demand, both bounded by ls's own limits.
// NewThread copies ls.Options; they are swapped for the call only.
func newModuleThread(ls *lua.LState) (*lua.LState, context.CancelFunc) {
	opts := ls.Options
	defer func() { ls.Options = opts }()
	if opts.RegistrySize > moduleRegistrySize {
		ls.Options.RegistrySize = moduleRegistrySize
		ls.Options.RegistryMaxSize = max(opts.RegistryMaxSize, opts.RegistrySize)
		if ls.Options.RegistryGrowStep < 1 {
			ls.Options.RegistryGrowStep = lua.RegistryGrowStep
		}
	}
	ls.Options.MinimizeStackMemory = true
	return ls.NewThread()
}

// moduleErrorPosition matches the "lib/x.lua:12:" prefix gopher-lua puts on an
// error raised in a module.
var moduleErrorPosition = regexp.MustCompile(`^(` + ModuleDir + `/[a-z0-9_/]+\.lua):(\d+):`)

// compile reads and compiles the module at path, raising a Lua error that
// names the module and its project-relative path, never a filesystem path.
func (m *moduleLoader) compile(ls *lua.LState, name, path string) *lua.LFunction {
	ctx := ls.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	src, err := m.files.Load(ctx, path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		ls.RaiseError("module not found: %s (looked in %s)", name, path)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		ls.RaiseError("cannot read module %s (%s): the run was cancelled or timed out", name, path)
	case errors.Is(err, fs.ErrPermission):
		ls.RaiseError("cannot read module %s (%s): permission denied", name, path)
	case err != nil:
		// The underlying error may carry an absolute path, so it goes to
		// the server log and the script sees the likely causes.
		slog.Warn("lua: cannot read module", "module", name, "path", path, "error", err)
		ls.RaiseError("cannot read module %s (%s): not a regular file, a link that leaves %s/, "+
			"or a read error (see the server log)", name, path, ModuleDir)
	case len(src) > maxModuleSize:
		ls.RaiseError("module %s (%s) is larger than %d bytes", name, path, maxModuleSize)
	}

	fn, err := ls.Load(strings.NewReader(stripShebang(string(src))), path)
	if err != nil {
		msg := err.Error()
		var ae *lua.ApiError
		if errors.As(err, &ae) {
			msg = strings.TrimSpace(ae.Object.String())
		}
		if line := parseCompileErrorLine(msg); line > 0 {
			m.pendingFrame = &StackFrame{Path: path, Line: line}
		}
		m.pendingMessage = "cannot compile module " + name + ": " + msg
		ls.RaiseError("%s", m.pendingMessage)
	}
	return fn
}

// takePendingFrame returns the module line of the error errValue reports, and
// clears it. It returns nil when errValue is some other error, such as a later
// one raised after the script caught the module's error.
func (m *moduleLoader) takePendingFrame(errValue lua.LValue) *StackFrame {
	frame := m.pendingFrame
	m.pendingFrame = nil
	if frame == nil || !strings.HasSuffix(errValue.String(), m.pendingMessage) {
		return nil
	}
	return frame
}

// quoteName renders a rejected module name for an error message, bounded so
// a huge computed name does not flood the error.
func quoteName(name string) string {
	const maxShown = 40
	if len(name) > maxShown {
		name = name[:maxShown] + "..."
	}
	return `"` + strings.ToValidUTF8(strings.ReplaceAll(name, "\x00", `\0`), "?") + `"`
}

// FailedInModule reports whether the innermost frame of a failed run is in a
// lib/ module. An envelope builder for inline code uses it to read the source
// excerpt from the project files, where the module lives.
func FailedInModule(frames []StackFrame) bool {
	return len(frames) > 0 && strings.HasPrefix(frames[0].Path, ModuleDir+"/")
}
