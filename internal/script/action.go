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

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
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
func (e *Engine) ExecuteAction(
	scriptPath string,
	ctx metamodel.ScriptContext,
	params map[string]string,
	timeout time.Duration,
) (*ActionResponse, error) {
	scriptCode, err := loadActionScript(ctx.GetProjectRoot(), scriptPath)
	if err != nil {
		return nil, err
	}

	svc, ok := ctx.GetWorkspace().(lua.Services)
	if !ok {
		return nil, fmt.Errorf("workspace does not provide lua.Services")
	}
	if svc.Meta == nil {
		svc.Meta = ctx.GetMeta()
	}
	if svc.ProjectRoot == "" {
		svc.ProjectRoot = ctx.GetProjectRoot()
	}

	var output bytes.Buffer
	relaDir := filepath.Join(ctx.GetProjectRoot(), project.CacheDir)
	ctxOpts, ctxErr := lua.LoadContextOptions(relaDir, scriptPath)
	if ctxErr != nil {
		return nil, ctxErr
	}
	luaOpts := append([]lua.Option{
		lua.WithParams(params),
		lua.WithActionMode(),
		lua.WithTimeout(timeout),
	}, ctxOpts...)

	runtime := lua.New(svc, &output, luaOpts...)
	defer runtime.Close()

	ret, err := runtime.RunActionString(scriptCode, scriptPath)
	if errors.Is(err, lua.ErrNoReturnValue) {
		return &ActionResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return parseActionResponse(ret)
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
