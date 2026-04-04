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

// LuaViolation represents a violation returned from a Lua validation script.
type LuaViolation struct {
	Message  string // Custom error message (required)
	Severity string // "error" or "warning" (optional, defaults to rule's severity)
}

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

// validate runs Lua validation for an entity and returns any violations.
//
// Lua scripts should return:
//   - nil (or no return): validation passes
//   - table with "message" field: single violation
//   - array of tables: multiple violations
//
// Each violation table can have:
//   - message (string, required): the error message
//   - severity (string, optional): "error" or "warning", defaults to rule's severity
//
// Example Lua returns:
//
//	return nil  -- pass
//	return { message = "Field is required" }  -- single violation
//	return { message = "Field is required", severity = "warning" }  -- with severity
//	return {
//	  { message = "Missing owner" },
//	  { message = "Invalid status", severity = "error" }
//	}  -- multiple violations
//
// Errors are logged but do not propagate - validation fails open to avoid
// blocking the entire validation run due to a single broken rule.
func (e *luaExecutor) validate(
	entity *model.Entity,
	rule metamodel.ValidationRule,
) []LuaViolation {
	code := rule.Lua
	if code == "" && rule.LuaFile != "" {
		var err error
		code, err = e.loadScript(rule.LuaFile)
		if err != nil {
			log.Printf("validation rule %q: %v", rule.Name, err)
			return nil // fail open - skip rule on load error
		}
	}

	if code == "" {
		return nil // no Lua validation
	}

	// Create runtime with read-only workspace and discarded stdout
	runtime := lua.New(e.ws, e.meta, e.projectRoot, io.Discard)
	defer runtime.Close()

	// Set arguments if provided
	if len(rule.LuaArgs) > 0 {
		runtime.SetArgs(rule.LuaArgs)
	}

	// Inject entity as global
	ls := runtime.LState()
	ls.SetGlobal("entity", lua.EntityToTable(ls, entity))

	// Compile the code into a function
	fn, err := ls.LoadString(code)
	if err != nil {
		log.Printf("validation rule %q: Lua compile error: %v", rule.Name, err)
		return nil // fail open - skip rule on compile error
	}

	// Set execution timeout to prevent infinite loops
	ctx, cancel := context.WithTimeout(context.Background(), validationTimeout)
	defer cancel()
	ls.SetContext(ctx)

	// Push the function and call it with 0 args, expecting 1 return value
	ls.Push(fn)
	if err := ls.PCall(0, 1, nil); err != nil {
		log.Printf("validation rule %q: Lua runtime error: %v", rule.Name, err)
		return nil // fail open - skip rule on runtime error
	}

	// Get return value from stack
	ret := ls.Get(-1)
	ls.Pop(1)

	return e.parseReturnValue(ret, rule)
}

// parseReturnValue interprets the Lua return value as violations.
func (e *luaExecutor) parseReturnValue(
	ret golua.LValue,
	rule metamodel.ValidationRule,
) []LuaViolation {
	// nil = pass
	if ret == golua.LNil {
		return nil
	}

	// Must be a table
	tbl, ok := ret.(*golua.LTable)
	if !ok {
		log.Printf("validation rule %q: Lua must return nil or table, got %s",
			rule.Name, ret.Type().String())
		return nil // fail open
	}

	// Check if it's a single violation (has "message" key) or array of violations
	if msg := tbl.RawGetString("message"); msg != golua.LNil {
		// Single violation
		v := e.tableToViolation(tbl, rule)
		if v == nil {
			return nil
		}
		return []LuaViolation{*v}
	}

	// Array of violations - iterate numeric keys
	var violations []LuaViolation
	tbl.ForEach(func(key, value golua.LValue) {
		// Only process numeric keys (array elements)
		if _, ok := key.(golua.LNumber); !ok {
			return
		}
		if itemTbl, ok := value.(*golua.LTable); ok {
			if v := e.tableToViolation(itemTbl, rule); v != nil {
				violations = append(violations, *v)
			}
		}
	})

	return violations
}

// tableToViolation converts a Lua table to a LuaViolation.
func (e *luaExecutor) tableToViolation(
	tbl *golua.LTable,
	rule metamodel.ValidationRule,
) *LuaViolation {
	// Message is required
	msgVal := tbl.RawGetString("message")
	msg, ok := msgVal.(golua.LString)
	if !ok || msg == "" {
		log.Printf("validation rule %q: violation table missing 'message' field", rule.Name)
		return nil
	}

	// Severity is optional, defaults to rule's severity
	severity := rule.GetSeverity()
	if sevVal := tbl.RawGetString("severity"); sevVal != golua.LNil {
		if sev, ok := sevVal.(golua.LString); ok {
			sevStr := string(sev)
			if sevStr == "error" || sevStr == "warning" {
				severity = sevStr
			}
		}
	}

	return &LuaViolation{
		Message:  string(msg),
		Severity: severity,
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
