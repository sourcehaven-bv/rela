package script

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// actionsDir is the directory where action scripts must be located.
const actionsDir = "actions"

// ActionResponse is the response returned from an action script.
// All fields are optional. Empty response means "200 OK with no body".
type ActionResponse struct {
	Redirect    string `json:"redirect,omitempty"`
	Message     string `json:"message,omitempty"`
	MessageType string `json:"message_type,omitempty"`
}

// validMessageTypes is the allowed enum for ActionResponse.MessageType.
var validMessageTypes = map[string]bool{
	"":        true,
	"success": true,
	"info":    true,
	"warning": true,
	"error":   true,
}

// ExecuteAction loads and runs a Lua action script. The script's Lua return
// value is interpreted as an ActionResponse. Path is loaded from the
// project's actions/ directory using os.OpenRoot for traversal-resistant
// access (rejects symlinks, ".." paths, absolute paths).
//
// The timeout applies to script execution. The caller is responsible for
// holding any necessary workspace lock — actions may mutate the graph.
//
// triggerEntity is optional — nil when the action is invoked without entity
// context. When non-nil it is exposed to the Lua script as the `entity` global.
//
// correlationID is stamped onto any *lua.ScriptError this returns so the
// HTTP response and the slog log line stay matched up. Callers without
// a correlation context (CLI, scheduler) may pass "".
func (e *Engine) ExecuteAction(
	scriptPath string,
	deps lua.WriteDeps,
	triggerEntity *entity.Entity,
	params map[string]string,
	timeout time.Duration,
	correlationID string,
) (*ActionResponse, error) {
	scriptCode, err := loadActionScript(deps.ProjectRoot, scriptPath)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	runtime, err := NewWriterRuntime(deps, scriptPath, &output,
		lua.WithParams(params),
		lua.WithActionMode(),
		lua.WithTimeout(timeout),
		lua.WithCache(e.cache),
	)
	if err != nil {
		return nil, err
	}
	defer runtime.Close()

	// RunActionString takes a chunk name but doesn't touch scriptPath,
	// so wire the namespace explicitly for rela.cache.* — otherwise
	// action scripts would always hit the inline/eval guard.
	runtime.SetScriptPath(scriptPath)

	if triggerEntity != nil {
		ls := runtime.LState()
		ls.SetGlobal("entity", lua.EntityToTable(ls, triggerEntity))
	}

	ret, err := runtime.RunActionString(scriptCode, scriptPath)
	if errors.Is(err, lua.ErrNoReturnValue) {
		return &ActionResponse{}, nil
	}
	if err != nil {
		// Path in the envelope is project-relative (e.g.,
		// "actions/foo.lua") for display; SourceFS is rooted at the
		// project so readSourceSlice can resolve that same path.
		// Lua's chunkname (used as scriptPath here) is the bare filename;
		// gopher-lua reports it back via the message handler as the
		// frame's Source. Re-prefix with actionsDir so frame paths line
		// up with the SourceFS root and the displayed Path.
		envelopePath := filepath.ToSlash(filepath.Join(actionsDir, scriptPath))
		frames := runtime.ErrorFrames()
		for i := range frames {
			if frames[i].Path == scriptPath {
				frames[i].Path = envelopePath
			}
		}
		return nil, lua.BuildScriptError(lua.BuildInput{
			Surface:        lua.SurfaceAction,
			Path:           envelopePath,
			EntityID:       triggerEntityID(triggerEntity),
			Args:           stringMapToAny(params),
			Frames:         frames,
			CapturedOutput: output.Bytes(),
			Err:            err,
			CorrelationID:  correlationID,
			SourceFS:       os.DirFS(deps.ProjectRoot),
		})
	}

	return parseActionResponse(ret)
}

func triggerEntityID(e *entity.Entity) string {
	if e == nil {
		return ""
	}
	return e.ID
}

// stringMapToAny adapts the action's static params (always string→string)
// to the redactor's input type (map[string]any). Returns nil for empty
// maps so the envelope omits the Args field rather than emitting "{}".
func stringMapToAny(m map[string]string) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// loadActionScript loads a script from the project's actions/ directory using
// os.OpenRoot for traversal-resistant access.
func loadActionScript(projectRoot, scriptPath string) (string, error) {
	root, scriptCode, err := openLocalScript(projectRoot, actionsDir, scriptPath)
	if err != nil {
		return "", err
	}
	root.Close()
	return scriptCode, nil
}

// CheckActionScriptExists verifies that an action script can be loaded.
// Used at config-load time to fail fast on missing or invalid script paths.
func CheckActionScriptExists(projectRoot, scriptPath string) error {
	_, _, err := openLocalScript(projectRoot, actionsDir, scriptPath)
	return err
}

// openLocalScript loads a script file from {projectRoot}/{subdir}/{scriptPath}
// using os.OpenRoot for traversal-resistant access. Returns the opened root
// (which the caller must Close), the script content, and any error.
func openLocalScript(projectRoot, subdir, scriptPath string) (io.Closer, string, error) {
	if scriptPath == "" {
		return nil, "", errors.New("script path is empty")
	}
	if !strings.HasSuffix(scriptPath, ".lua") {
		return nil, "", fmt.Errorf("script must have .lua extension: %s", scriptPath)
	}
	// Reject absolute paths and ".." segments via filepath.IsLocal.
	// (os.OpenRoot would also catch these, but earlier rejection gives better errors.)
	if !isLocalPath(scriptPath) {
		return nil, "", fmt.Errorf(
			"script path must be a local path (no '..' or absolute paths): %s", scriptPath)
	}

	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, "", errors.New("cannot access project directory")
	}

	scriptsRoot, err := root.OpenRoot(subdir)
	if err != nil {
		root.Close()
		return nil, "", fmt.Errorf("cannot access %s directory", subdir)
	}
	defer scriptsRoot.Close()

	scriptFile, err := scriptsRoot.Open(scriptPath)
	if err != nil {
		root.Close()
		return nil, "", fmt.Errorf("script not found: %s (must be in %s/ directory)", scriptPath, subdir)
	}
	defer scriptFile.Close()

	content, err := io.ReadAll(scriptFile)
	if err != nil {
		root.Close()
		return nil, "", fmt.Errorf("cannot read script: %s", scriptPath)
	}

	return root, string(content), nil
}

// isLocalPath returns true if the path is local (no ".." segments, not absolute).
func isLocalPath(p string) bool {
	return filepath.IsLocal(p)
}

// parseActionResponse converts a Lua return value (already converted to Go
// via luaValueToGo) into an ActionResponse. Validates redirect format and
// message_type enum.
func parseActionResponse(ret interface{}) (*ActionResponse, error) {
	if ret == nil {
		return &ActionResponse{}, nil
	}

	m, ok := ret.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("action script must return a table, got %T", ret)
	}

	resp := &ActionResponse{}

	if v, ok := m["redirect"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("redirect must be a string, got %T", v)
		}
		if err := validateRedirect(s); err != nil {
			return nil, err
		}
		resp.Redirect = s
	}

	if v, ok := m["message"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("message must be a string, got %T", v)
		}
		resp.Message = s
	}

	if v, ok := m["message_type"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("message_type must be a string, got %T", v)
		}
		if !validMessageTypes[s] {
			return nil, fmt.Errorf(
				"invalid message_type %q (must be one of: success, info, warning, error)", s)
		}
		resp.MessageType = s
	}

	return resp, nil
}

// validateRedirect ensures a redirect URL is a relative path starting with "/"
// but not "//" (which would be a protocol-relative URL — open redirect risk).
func validateRedirect(s string) error {
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "/") {
		return fmt.Errorf("redirect must start with '/': %q", s)
	}
	if strings.HasPrefix(s, "//") {
		return fmt.Errorf("redirect must not start with '//' (open redirect): %q", s)
	}
	return nil
}
