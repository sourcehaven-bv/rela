package validation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	golua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// scriptsDir is the directory where script files must be located.
const scriptsDir = "scripts"

// validationTimeout is the maximum execution time for a single validation rule.
// This prevents infinite loops in malicious or buggy Lua code.
const validationTimeout = 5 * time.Second

// luaExecutor handles Lua validation execution.
type luaExecutor struct {
	ws          lua.WorkspaceInterface
	meta        *metamodel.Metamodel
	projectRoot string
}

// newLuaExecutor creates a new Lua executor for validation.
func newLuaExecutor(ws lua.WorkspaceInterface, meta *metamodel.Metamodel, projectRoot string) *luaExecutor {
	return &luaExecutor{
		ws:          newReadOnlyWorkspace(ws),
		meta:        meta,
		projectRoot: projectRoot,
	}
}

// validate runs Lua validation for an entity and returns true if valid.
// Returns true (pass) if:
//   - No Lua code specified (rule.Lua and rule.LuaFile both empty)
//   - Lua returns true or a truthy non-nil value
//
// Returns false (violation) if:
//   - Lua returns false or nil (including no return statement)
//
// Errors are logged but do not propagate - validation fails open to avoid
// blocking the entire validation run due to a single broken rule.
func (e *luaExecutor) validate(entity *model.Entity, rule metamodel.ValidationRule) bool {
	code := rule.Lua
	if code == "" && rule.LuaFile != "" {
		var err error
		code, err = e.loadScript(rule.LuaFile)
		if err != nil {
			log.Printf("validation rule %q: %v", rule.Name, err)
			return true // fail open - skip rule on load error
		}
	}

	if code == "" {
		return true // no Lua validation
	}

	// Create runtime with read-only workspace and discarded stdout
	runtime := lua.New(e.ws, e.meta, e.projectRoot, io.Discard)
	defer runtime.Close()

	// Inject entity as global
	ls := runtime.LState()
	ls.SetGlobal("entity", lua.EntityToTable(ls, entity))

	// Compile the code into a function
	fn, err := ls.LoadString(code)
	if err != nil {
		log.Printf("validation rule %q: Lua compile error: %v", rule.Name, err)
		return true // fail open - skip rule on compile error
	}

	// Set execution timeout to prevent infinite loops
	ctx, cancel := context.WithTimeout(context.Background(), validationTimeout)
	defer cancel()
	ls.SetContext(ctx)

	// Push the function and call it with 0 args, expecting 1 return value
	ls.Push(fn)
	if err := ls.PCall(0, 1, nil); err != nil {
		log.Printf("validation rule %q: Lua runtime error: %v", rule.Name, err)
		return true // fail open - skip rule on runtime error
	}

	// Get return value from stack (PCall with NRet=1 leaves one value on stack)
	ret := ls.Get(-1)
	ls.Pop(1) // clean up stack

	// Handle different return types:
	// - LTrue: pass
	// - LFalse, LNil: violation
	// - Other truthy values (string, number, table): pass
	switch ret {
	case golua.LNil, golua.LFalse:
		return false // violation
	case golua.LTrue:
		return true // pass
	default:
		// Any other truthy value (string, number, table) = pass
		return true
	}
}

// loadScript loads a Lua script from the scripts/ directory.
// Uses os.OpenRoot for traversal-resistant file access.
func (e *luaExecutor) loadScript(scriptPath string) (string, error) {
	// Security: Validate path is local (no "..", no absolute paths)
	if !filepath.IsLocal(scriptPath) {
		return "", fmt.Errorf(
			"script path must be a local path (no '..' or absolute paths): %s", scriptPath)
	}

	// Security: Must have .lua extension
	if !strings.HasSuffix(scriptPath, ".lua") {
		return "", fmt.Errorf("script must have .lua extension: %s", scriptPath)
	}

	// Use os.OpenRoot for traversal-resistant access.
	// Error messages intentionally omit system paths to prevent information leakage.
	root, err := os.OpenRoot(e.projectRoot)
	if err != nil {
		return "", errors.New("cannot access project directory")
	}
	defer root.Close()

	scriptsRoot, err := root.OpenRoot(scriptsDir)
	if err != nil {
		return "", errors.New("cannot access scripts directory")
	}
	defer scriptsRoot.Close()

	scriptFile, err := scriptsRoot.Open(scriptPath)
	if err != nil {
		return "", fmt.Errorf("script not found: %s (must be in scripts/ directory)", scriptPath)
	}
	defer scriptFile.Close()

	content, err := io.ReadAll(scriptFile)
	if err != nil {
		return "", fmt.Errorf("cannot read script: %s", scriptPath)
	}

	return string(content), nil
}
