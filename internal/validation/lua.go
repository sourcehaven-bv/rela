package validation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	golua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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

// luaRuleContext carries the per-rule Lua state built once in
// CheckRule and reused across every entity that hits the rule's Lua
// branch. The runtime is owned by CheckRule and Closed via defer
// there; this struct just holds the references the entity loop needs.
type luaRuleContext struct {
	runtime      *lua.Runtime
	code         string // already-loaded script source
	envelopePath string // "validations/<rule-name>" or "validations/<file>"
	sourceFS     fs.FS  // os.DirFS(projectRoot) for lua_file rules; nil for inline
}

// buildLuaRuleContext loads the rule's Lua source and constructs the
// per-rule runtime. Returns:
//
//   - (ctx, nil): runtime is ready; caller must Close it.
//   - (nil, *LoadError): script-load failure (lua_file: missing,
//     traversal-rejected, etc.); rule is skipped, surfaces as
//     LoadError in the Result.
//   - (nil, nil): rule has no Lua at all (caller shouldn't have asked).
func (s *Service) buildLuaRuleContext(
	ctx context.Context,
	rule metamodel.ValidationRule,
) (*luaRuleContext, *LoadError) {
	code := rule.Lua
	envelopePath := "validations/" + rule.Name
	var sourceFS fs.FS
	if code == "" && rule.LuaFile != "" {
		loaded, err := s.loadLuaScript(rule.LuaFile)
		if err != nil {
			return nil, &LoadError{RuleName: rule.Name, Message: err.Error()}
		}
		code = loaded
		envelopePath = "validations/" + filepath.ToSlash(rule.LuaFile)
		// Source slice context is read from project root; readSourceSlice
		// then opens "validations/<file>" relative to that root.
		if s.deps.ProjectRoot != "" {
			sourceFS = os.DirFS(s.deps.ProjectRoot)
		}
	}
	if code == "" {
		return nil, nil
	}
	// Reader runtime: read-only bindings; no mutation, no AI.
	// AI is intentionally absent — an AI-powered rule would call out on
	// every entity in every analyze run with no quota or kill switch
	// (see PLAN-KAK2R Scope-out / TKT-LR5YC).
	//
	// Timeout and parent ctx are passed via runtime options so
	// applyTimeout() (called inside RunValidationString before each
	// Load) derives a 5s budget rooted at ctx; canceling ctx
	// interrupts an in-flight rule and the timeout fires within ~5s
	// for runaway scripts.
	opts := []lua.Option{
		lua.WithTimeout(validationTimeout),
		lua.WithContext(ctx),
	}
	if s.cache != nil {
		opts = append(opts, lua.WithCache(s.cache))
	}
	runtime := lua.NewReader(s.deps, io.Discard, opts...)

	if len(rule.LuaArgs) > 0 {
		runtime.SetArgs(rule.LuaArgs)
	}
	// The script-path doubles as the rela.cache.* namespace; using the
	// envelope path keeps the namespace stable per rule (validations/
	// prefixed) without colliding with real script files.
	runtime.SetScriptPath(envelopePath)

	return &luaRuleContext{
		runtime:      runtime,
		code:         code,
		envelopePath: envelopePath,
		sourceFS:     sourceFS,
	}, nil
}

// validateLuaWithRuntime executes the rule's Lua against ent using the
// runtime owned by luaCtx. Returns LuaViolations parsed from the rule's
// return value, or a *lua.ScriptError when Lua fails (compile, runtime,
// timeout, contract violation).
//
// Validation rules run against a reader runtime that cannot mutate the
// graph; mutation bindings are not registered, so rela.create_entity et
// al raise "attempt to call a nil value" from the VM.
//
// Expected return shapes from the script:
//
//   - nil (or no return): validation passes.
//   - table with "message" field: single violation.
//   - array of tables: multiple violations.
//
// Each violation table:
//   - message (string, required)
//   - severity (string, optional): "error"|"warning", defaults to rule
//
// Anything else surfaces as a synthesized *lua.ScriptError so the
// operator running rela analyze sees a structured envelope rather than
// a silent skip.
func (s *Service) validateLuaWithRuntime(
	ent *entity.Entity,
	rule metamodel.ValidationRule,
	luaCtx *luaRuleContext,
) ([]LuaViolation, *lua.ScriptError) {
	ls := luaCtx.runtime.LState()
	// Reset the entity global per entity so a long-lived runtime
	// doesn't expose the previous iteration's entity to this rule.
	ls.SetGlobal("entity", lua.EntityToTable(ls, ent))

	// Timeout + ctx are wired via WithTimeout/WithContext on the
	// runtime; applyTimeout (inside RunValidationString) derives a
	// fresh 5s budget per entity rooted at the parent ctx.
	ret, err := luaCtx.runtime.RunValidationString(luaCtx.code, luaCtx.envelopePath)
	if err != nil {
		return nil, lua.BuildScriptError(lua.BuildInput{
			Surface:  lua.SurfaceValidation,
			Path:     luaCtx.envelopePath,
			EntityID: ent.ID,
			Err:      err,
			Frames:   luaCtx.runtime.ErrorFrames(),
			SourceFS: luaCtx.sourceFS,
		})
	}

	violations, contractErr := parseLuaReturnValue(ret, rule, luaCtx.envelopePath, ent.ID)
	if contractErr != nil {
		return nil, contractErr
	}
	return violations, nil
}

// parseLuaReturnValue interprets the Lua return value as violations.
// On contract violations (non-table, missing message field) it
// synthesizes a *lua.ScriptError so the operator sees a structured
// envelope rather than a silent skip.
func parseLuaReturnValue(
	ret golua.LValue,
	rule metamodel.ValidationRule,
	envelopePath, entityID string,
) ([]LuaViolation, *lua.ScriptError) {
	// nil = pass
	if ret == golua.LNil {
		return nil, nil
	}

	// Must be a table
	tbl, ok := ret.(*golua.LTable)
	if !ok {
		return nil, contractError(envelopePath, entityID,
			"validation rule must return nil or table, got "+ret.Type().String())
	}

	// Check if it's a single violation (has "message" key) or array of violations
	if msg := tbl.RawGetString("message"); msg != golua.LNil {
		// Single violation
		v, err := luaTableToViolation(tbl, rule, envelopePath, entityID)
		if err != nil {
			return nil, err
		}
		return []LuaViolation{*v}, nil
	}

	// Array of violations - iterate numeric keys. We collect contract
	// errors separately so the first malformed item surfaces; valid
	// preceding items are discarded (the rule's return is malformed
	// regardless of which item tripped).
	var violations []LuaViolation
	var firstErr *lua.ScriptError
	tbl.ForEach(func(key, value golua.LValue) {
		if firstErr != nil {
			return
		}
		// Only process numeric keys (array elements)
		if _, ok := key.(golua.LNumber); !ok {
			return
		}
		itemTbl, ok := value.(*golua.LTable)
		if !ok {
			firstErr = contractError(envelopePath, entityID,
				"validation rule array element must be a table, got "+value.Type().String())
			return
		}
		v, err := luaTableToViolation(itemTbl, rule, envelopePath, entityID)
		if err != nil {
			firstErr = err
			return
		}
		violations = append(violations, *v)
	})
	if firstErr != nil {
		return nil, firstErr
	}

	return violations, nil
}

// luaTableToViolation converts a Lua table to a LuaViolation. A
// missing or empty `message` field is a contract violation rendered
// as *lua.ScriptError.
func luaTableToViolation(
	tbl *golua.LTable,
	rule metamodel.ValidationRule,
	envelopePath, entityID string,
) (*LuaViolation, *lua.ScriptError) {
	// Message is required
	msgVal := tbl.RawGetString("message")
	msg, ok := msgVal.(golua.LString)
	if !ok || msg == "" {
		return nil, contractError(envelopePath, entityID,
			"validation rule violation table missing 'message' field")
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
	}, nil
}

// contractError synthesizes a *lua.ScriptError for a return-shape
// contract violation. No frames or LuaLine — the rule ran fine, it
// just returned the wrong shape.
func contractError(envelopePath, entityID, message string) *lua.ScriptError {
	return &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       envelopePath,
		EntityID:   entityID,
		LuaMessage: message,
	}
}

// loadLuaScript loads a Lua script from the validations/ directory.
// Uses os.OpenRoot for traversal-resistant file access.
func (s *Service) loadLuaScript(scriptPath string) (string, error) {
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
	root, err := os.OpenRoot(s.deps.ProjectRoot)
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
