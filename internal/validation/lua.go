package validation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	golua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// validationsDir is the directory where validation script files must be located.
const validationsDir = "validations"

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
	svc lua.Services
}

// newLuaExecutor creates a new Lua executor for validation.
// The services are wrapped to be read-only — Manager is nil so writes fail.
func newLuaExecutor(svc lua.Services, meta *metamodel.Metamodel, projectRoot string) *luaExecutor {
	svc.Manager = nil // read-only: disable writes
	if svc.Meta == nil {
		svc.Meta = meta
	}
	if svc.ProjectRoot == "" {
		svc.ProjectRoot = projectRoot
	}
	return &luaExecutor{svc: svc}
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
			slog.Warn("validation rule failed to load script", "rule", rule.Name, "error", err)
			return nil // fail open - skip rule on load error
		}
	}

	if code == "" {
		return nil // no Lua validation
	}

	// Create runtime with read-only workspace and discarded stdout.
	//
	// AI is intentionally NOT wired into validation rules: an AI-powered
	// rule would call out to a provider on every entity on every analyze
	// run, with no quota or kill switch. The 5s validation timeout would
	// also silently clip slow calls. AI-in-validations is tracked as a
	// follow-up that needs its own design (cost guardrails, opt-in
	// per rule, longer per-rule budget).
	runtime := lua.New(e.svc, io.Discard)
	defer runtime.Close()

	// Set arguments if provided
	if len(rule.LuaArgs) > 0 {
		runtime.SetArgs(rule.LuaArgs)
	}

	// Inject entity as global
	ls := runtime.LState()
	ls.SetGlobal("entity", lua.EntityToTable(ls, model.EntityToDomain(entity)))

	// Compile the code into a function
	fn, err := ls.LoadString(code)
	if err != nil {
		slog.Warn("validation rule Lua compile error", "rule", rule.Name, "error", err)
		return nil // fail open - skip rule on compile error
	}

	// Set execution timeout to prevent infinite loops
	ctx, cancel := context.WithTimeout(context.Background(), validationTimeout)
	defer cancel()
	ls.SetContext(ctx)

	// Push the function and call it with 0 args, expecting 1 return value
	ls.Push(fn)
	if err := ls.PCall(0, 1, nil); err != nil {
		slog.Warn("validation rule Lua runtime error", "rule", rule.Name, "error", err)
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
		slog.Warn("validation rule must return nil or table",
			"rule", rule.Name, "got", ret.Type().String())
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
		slog.Warn("validation rule violation table missing 'message' field", "rule", rule.Name)
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

// loadScript loads a Lua script from the validations/ directory.
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
	root, err := os.OpenRoot(e.svc.ProjectRoot)
	if err != nil {
		return "", errors.New("cannot access project directory")
	}
	defer root.Close()

	validationsRoot, err := root.OpenRoot(validationsDir)
	if err != nil {
		return "", errors.New("cannot access validations directory")
	}
	defer validationsRoot.Close()

	scriptFile, err := validationsRoot.Open(scriptPath)
	if err != nil {
		return "", fmt.Errorf("script not found: %s (must be in validations/ directory)", scriptPath)
	}
	defer scriptFile.Close()

	content, err := io.ReadAll(scriptFile)
	if err != nil {
		return "", fmt.Errorf("cannot read script: %s", scriptPath)
	}

	return string(content), nil
}
