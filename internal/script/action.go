package script

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// actionsDir is the directory where action scripts must be located.
const actionsDir = "actions"

// ActionResponse is the response returned from an action script.
// All fields are optional. Empty response means "200 OK with no body".
//
// It carries TWO vocabularies, and a script picks exactly one:
//
//   - {redirect, message, message_type} — the SPA-toast shape, unchanged since
//     before TKT-EFMRQM and still the default. Serialized as a rela JSON
//     envelope by the handler.
//   - {status, body, content_type} — the rich shape (TKT-EFMRQM), for a
//     third-party consumer that needs a specific status and payload.
//
// Mixing them is a contract error rather than a precedence rule: the two are
// answered to different consumers, so honoring one and dropping the other is a
// silent half-delivery. [ActionResponse.IsRich] is the discriminator.
type ActionResponse struct {
	Redirect    string `json:"redirect,omitempty"`
	Message     string `json:"message,omitempty"`
	MessageType string `json:"message_type,omitempty"`

	// Status is the HTTP status for the rich shape. Constrained to 2xx/4xx/5xx
	// at parse time; zero when the script returned the SPA shape.
	Status int `json:"-"`

	// Body is the response payload verbatim. A string, never a table: encoding
	// a table here would silently choose a serialization the script did not
	// ask for, and its content_type would then be a guess.
	Body string `json:"-"`

	// ContentType is validated against a small allowlist at parse time and is
	// never sniffed — see validActionContentTypes.
	ContentType string `json:"-"`
}

// IsRich reports whether the script chose the {status, body, content_type}
// vocabulary. Only a rich response bypasses the SPA envelope.
func (r *ActionResponse) IsRich() bool {
	return r != nil && (r.Status != 0 || r.Body != "")
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
// The caller is responsible for holding any necessary workspace lock — actions
// may mutate the graph. See [ActionInvocation] for what each argument means; it
// is the positional form of that struct, kept for the callers that predate it.
func (e *Engine) ExecuteAction(
	ctx context.Context,
	scriptPath string,
	deps lua.WriteDeps,
	triggerEntity *entity.Entity,
	params map[string]string,
	timeout time.Duration,
	correlationID string,
) (*ActionResponse, error) {
	return e.ExecuteActionRequest(ctx, scriptPath, deps, ActionInvocation{
		TriggerEntity: triggerEntity,
		Params:        params,
		Timeout:       timeout,
		CorrelationID: correlationID,
	})
}

// ActionInvocation carries the per-call inputs of one action execution.
//
// Grouped rather than added to [Engine.ExecuteAction]'s positional list, which
// was already at seven arguments: the request-scoped inputs (TKT-EFMRQM) would
// have made a call site an unreadable run of nils, and the compiler cannot tell
// two adjacent same-typed arguments apart when one is swapped for the other.
type ActionInvocation struct {
	// TriggerEntity is optional — nil when the action is invoked without entity
	// context. When non-nil it is exposed to the script as the `entity` global.
	// It MUST already have passed the caller's read gate: this package does no
	// ACL of its own.
	TriggerEntity *entity.Entity

	// Params are the action's static config params, as rela.params.
	Params map[string]string

	// Request is the inbound HTTP request, as rela.request (TKT-EFMRQM). Nil
	// for a script that did not opt in, which leaves rela.request absent.
	Request *lua.Request

	// Timeout bounds script execution.
	Timeout time.Duration

	// CorrelationID is stamped onto any *lua.ScriptError so the HTTP response
	// and the slog line stay matched up. Callers without a correlation context
	// (CLI, scheduler) may leave it empty.
	CorrelationID string
}

// ExecuteActionRequest is [Engine.ExecuteAction] taking its per-call inputs as
// a struct, which is what a request-scoped invocation needs.
func (e *Engine) ExecuteActionRequest(
	ctx context.Context,
	scriptPath string,
	deps lua.WriteDeps,
	inv ActionInvocation,
) (*ActionResponse, error) {
	triggerEntity, params, correlationID := inv.TriggerEntity, inv.Params, inv.CorrelationID

	scriptCode, err := loadActionScript(deps.ProjectRoot, scriptPath)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	runtime, err := NewWriterRuntime(deps, scriptPath, &output,
		lua.WithParams(params),
		lua.WithRequest(inv.Request),
		lua.WithActionMode(),
		lua.WithTimeout(inv.Timeout),
		lua.WithCache(e.cache),
		lua.WithContext(ctx),
		lua.WithPrincipal(principal.From(ctx)),
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

	// ctx is threaded into the runtime via lua.WithContext(ctx) above;
	// RunActionString applies it to the LState. contextcheck can't follow that
	// flow across the gopher-lua SetContext boundary.
	//nolint:contextcheck // ctx threaded via WithContext; see comment above
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

// ReadActionScript returns an action script's source, for callers that want to
// INSPECT it at config-load time rather than run it.
//
// Same load as [CheckActionScriptExists] — which already reads the whole body
// and throws it away — differing only in handing the bytes back. Two functions
// rather than one because the two callers want different things from a
// failure: existence-checking is fatal at boot, whereas a lint that cannot read
// a script should stay quiet rather than block startup over a file it was only
// going to look at.
//
// Nil: never returns a nil error with an empty body for a readable script; an
// empty script file yields ("", nil).
func ReadActionScript(projectRoot, scriptPath string) (string, error) {
	return loadActionScript(projectRoot, scriptPath)
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
func parseActionResponse(ret any) (*ActionResponse, error) {
	if ret == nil {
		return &ActionResponse{}, nil
	}

	m, ok := ret.(map[string]any)
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

	if err := parseRichActionResponse(m, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// validActionContentTypes is the allowlist a script may choose from for a rich
// response body (TKT-EFMRQM). Compared as the lowercased media type, with any
// parameters (`; charset=utf-8`) stripped first.
//
// An ALLOWLIST rather than an arbitrary string, for two reasons that survive
// the response hardening the handler also applies (nosniff + a
// `sandbox; default-src 'none'` CSP + no-store, the export/attachment recipe):
//
//   - This endpoint is same-origin with the SPA and reachable from a browser,
//     so an active-content type (text/html, image/svg+xml,
//     application/xhtml+xml) turns a script bug into a same-origin scripting
//     sink. The CSP and nosniff are what make that survivable; the allowlist is
//     what means a single header regression is not immediately exploitable.
//     Defence in depth is the whole point — one of the two mechanisms failing
//     should not be a vulnerability.
//   - The set an actual consumer needs is small and machine-readable. A hook
//     answering Icinga, a CI runner, or a payment provider needs JSON, plain
//     text, XML or CSV. Nothing in the motivating use cases wants HTML, and a
//     script that genuinely needs a novel type is an operator asking for one
//     more entry here, which is a reviewable change.
//
// Deliberately NOT including text/html: an operator who wants a rendered page
// has documents (`/_documents/...`), which already run Lua through the
// machinery built for it.
var validActionContentTypes = map[string]bool{
	"application/json": true,
	"text/plain":       true,
	"text/csv":         true,
	"application/xml":  true,
	"text/xml":         true,
}

// defaultActionContentType is what a rich response with a body but no declared
// content_type is served as. text/plain, never a sniffed type: an omitted
// declaration is the case most likely to be an oversight, and the inert choice
// is the right default for an oversight.
const defaultActionContentType = "text/plain; charset=utf-8"

// parseRichActionResponse reads the {status, body, content_type} vocabulary off
// a script's return table onto resp, refusing a return that also used the SPA
// vocabulary.
func parseRichActionResponse(m map[string]any, resp *ActionResponse) error {
	_, hasStatus := m["status"]
	_, hasBody := m["body"]
	_, hasContentType := m["content_type"]
	if !hasStatus && !hasBody && !hasContentType {
		return nil
	}

	if resp.Redirect != "" || resp.Message != "" || resp.MessageType != "" {
		return errors.New(
			"action response cannot combine {redirect, message, message_type} with {status, body, content_type}: " +
				"return one shape or the other")
	}

	if hasStatus {
		status, err := actionStatusFrom(m["status"])
		if err != nil {
			return err
		}
		resp.Status = status
	}

	if hasBody {
		s, ok := m["body"].(string)
		if !ok {
			return fmt.Errorf(
				"body must be a string, got %T (encode a table yourself, e.g. json.encode(t), "+
					"so the content_type matches what you produced)", m["body"])
		}
		resp.Body = s
	}

	if hasContentType {
		s, ok := m["content_type"].(string)
		if !ok {
			return fmt.Errorf("content_type must be a string, got %T", m["content_type"])
		}
		if err := validateActionContentType(s); err != nil {
			return err
		}
		resp.ContentType = s
	}

	// A content_type on its own would otherwise leave IsRich false and be
	// silently dropped by the SPA envelope.
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	if resp.ContentType == "" {
		resp.ContentType = defaultActionContentType
	}
	return nil
}

// actionStatusFrom converts a Lua number to a status code a handler may send.
//
// BOTH int64 and float64 must be handled. Lua has one float64-backed number
// type, but lua.luaValueToGo narrows an integral value to int64 on the way out
// (so an id or a count round-trips as an integer), while a fractional one stays
// float64 — so `status = 200` arrives as int64 and `status = 200.5` as float64.
// Accepting only one of the two is a bug that unit tests over hand-built maps
// cannot see, because the narrowing happens in the layer above them.
//
// A fractional value is REFUSED rather than truncated: truncating would let a
// typo become a quietly different status.
//
// The accepted range is 2xx, 4xx and 5xx. 1xx and 3xx are refused because they
// are protocol-level rather than application-level: a 1xx makes net/http's
// state machine expect a continuation that never comes, and a 3xx needs a
// Location header this shape has no field for — the SPA vocabulary's
// `redirect:` is the supported way to redirect, and it is separately validated
// against open-redirect.
func actionStatusFrom(v any) (int, error) {
	var status int
	switch n := v.(type) {
	case int64:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return 0, fmt.Errorf("status %d is out of range", n)
		}
		status = int(n)
	case float64:
		status = int(n)
		if float64(status) != n {
			return 0, fmt.Errorf("status must be a whole number, got %v", n)
		}
	default:
		return 0, fmt.Errorf("status must be a number, got %T", v)
	}
	switch {
	case status >= 200 && status <= 299,
		status >= 400 && status <= 599:
		return status, nil
	default:
		return 0, fmt.Errorf(
			"status %d is not allowed (use 2xx, 4xx or 5xx; a redirect uses the redirect field)", status)
	}
}

// validateActionContentType checks a script-chosen content type against the
// allowlist. Parameters are permitted (`; charset=utf-8`) but the media type
// itself must be listed.
func validateActionContentType(s string) error {
	// mime.ParseMediaType rejects CR/LF and other header-injection shapes
	// outright, so the split below never sees a smuggled header. Parsing rather
	// than string-cutting is what makes that true.
	mediaType, _, err := mime.ParseMediaType(s)
	if err != nil {
		return fmt.Errorf("content_type %q is not a valid media type: %w", s, err)
	}
	if !validActionContentTypes[strings.ToLower(mediaType)] {
		return fmt.Errorf(
			"content_type %q is not allowed (allowed: %s)", s, strings.Join(sortedContentTypes(), ", "))
	}
	return nil
}

// sortedContentTypes renders the allowlist for an error message, in a stable
// order so the message does not shuffle between runs.
func sortedContentTypes() []string {
	out := slices.Collect(maps.Keys(validActionContentTypes))
	slices.Sort(out)
	return out
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
