package lua

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	glua "github.com/yuin/gopher-lua"
)

// ScriptError carries structured context about a failed Lua script run so
// callers can render rich error UIs (source slice, captured print() output,
// stack frames) instead of opaque "action failed" messages.
//
// Built by BuildScriptError from the underlying gopher-lua error and the
// caller's surface-specific context. Every Lua execution surface (data-entry
// action, document render, automation, MCP lua_run/lua_eval) wraps its
// failures in *ScriptError so HTTP and MCP handlers can branch on the type.
type ScriptError struct {
	Surface        Surface        `json:"surface"`
	Path           string         `json:"path"`
	EntityID       string         `json:"entity_id,omitempty"`
	Args           map[string]any `json:"args,omitempty"`
	LuaMessage     string         `json:"lua_message"`
	LuaLine        int            `json:"lua_line,omitempty"`
	Stack          []StackFrame   `json:"stack,omitempty"`
	Source         []SourceLine   `json:"source,omitempty"`
	CapturedOutput string         `json:"captured_output,omitempty"`
	CorrelationID  string         `json:"correlation_id,omitempty"`
}

// Surface identifies which Lua execution path produced an error.
// Closed enum: every legitimate value is one of the Surface* constants.
// Defined as a typed string so callers writing `switch se.Surface { ... }`
// get exhaustiveness analysis from go/analysis-driven linters and so the
// JSON wire shape stays a plain string.
type Surface string

// StackFrame is one entry in a Lua stack trace.
type StackFrame struct {
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
	Func string `json:"func,omitempty"`
}

// SourceLine is one line of script source returned alongside the failing
// line so the UI can show context. Highlight is true for the failing line.
type SourceLine struct {
	N         int    `json:"n"`
	Text      string `json:"text"`
	Highlight bool   `json:"highlight,omitempty"`
}

// Surface constants used in ScriptError.Surface.
const (
	SurfaceAction     Surface = "action"
	SurfaceDocument   Surface = "document"
	SurfaceAutomation Surface = "automation"
	SurfaceLuaRun     Surface = "lua_run"
	SurfaceLuaEval    Surface = "lua_eval"
)

// BuildInput aggregates the inputs to BuildScriptError. Callers fill in only
// the fields they have; unset fields are omitted from the resulting envelope.
type BuildInput struct {
	Surface        Surface
	Path           string
	EntityID       string
	Args           map[string]any
	CapturedOutput []byte
	Err            error
	CorrelationID  string

	// Frames is the typed stack capture from Runtime.ErrorFrames(). When
	// non-empty, it's the source of truth for LuaLine and Stack; otherwise
	// we fall back to extracting what we can from Err's string form.
	Frames []StackFrame

	// SourceFS is rooted such that Path can be opened directly.
	// When nil, no source slice is populated.
	SourceFS fs.FS
	// SourceContext is the number of lines of context to include on each
	// side of the failing line. Defaults to defaultSourceContext.
	SourceContext int
}

// Limits applied during envelope construction. Picked to be generous for
// real scripts while preventing degenerate inputs (huge files, infinite
// print loops) from blowing up response size.
const (
	defaultSourceContext   = 3
	maxSourceFileSize      = 200 * 1024
	maxStackFrames         = 32
	maxFrameStringLen      = 1024
	maxStringValueLen      = 4 * 1024
	maxCapturedOutputBytes = 16 * 1024
	redactedPlaceholder    = "<redacted>"
	truncatedPlaceholder   = "...[truncated]"
)

// BuildScriptError constructs a *ScriptError from a Lua failure.
//
// Preferred path: Frames from Runtime.ErrorFrames() carry typed line numbers
// and source paths captured by the PCall message handler. When Frames is
// empty (e.g., compile errors that fail before PCall, or non-Lua errors)
// we fall back to typed extraction via *glua.ApiError, then to the bare
// error message.
//
// Args and CapturedOutput are redacted before storage; SourceFS is read
// only for paths that resolve cleanly inside the FS root.
func BuildScriptError(in BuildInput) *ScriptError {
	se := &ScriptError{
		Surface:       in.Surface,
		Path:          in.Path,
		EntityID:      in.EntityID,
		CorrelationID: in.CorrelationID,
		Args:          redactArgs(in.Args),
	}

	if len(in.CapturedOutput) > 0 {
		se.CapturedOutput = redactCapturedOutput(in.CapturedOutput)
	}

	if len(in.Frames) > 0 {
		se.Stack = trimFrames(in.Frames)
	}
	se.fillFromError(in.Err)

	if se.LuaLine > 0 && in.SourceFS != nil {
		// When `require`d helpers are involved, the innermost frame's
		// path is what the user actually wants to see — fall back to the
		// surface-supplied Path otherwise.
		sourcePath := se.Path
		if frame := se.deepestUserFrame(); frame != nil && frame.Path != "" {
			sourcePath = frame.Path
		}
		ctx := in.SourceContext
		if ctx <= 0 {
			ctx = defaultSourceContext
		}
		se.Source = readSourceSlice(in.SourceFS, sourcePath, se.LuaLine, ctx)
	}

	return se
}

// AttachCapturedOutput attaches captured stdout to an existing ScriptError
// and returns the receiver. Used by callers that own the print() buffer
// outside the engine that produced the error (notably the data-entry
// document renderer, which extracts the *ScriptError via errors.As and
// then attaches the bytes it captured before the script failed).
func (e *ScriptError) AttachCapturedOutput(b []byte) *ScriptError {
	if e == nil || len(b) == 0 {
		return e
	}
	e.CapturedOutput = redactCapturedOutput(b)
	return e
}

// trimFrames clamps a frame slice at maxStackFrames and trims overly long
// names so a runaway script can't blow up response size.
func trimFrames(in []StackFrame) []StackFrame {
	out := in
	if len(out) > maxStackFrames {
		out = out[:maxStackFrames]
	}
	for i := range out {
		if len(out[i].Func) > maxFrameStringLen {
			out[i].Func = out[i].Func[:maxFrameStringLen]
		}
	}
	return out
}

// Error renders the failure as "<path>:<line>: <message>", deliberately
// matching gopher-lua's stock format so log lines are pasteable into
// editor "go to file:line" affordances. This format is also what the
// workspace automation layer flattens into its []string error slice
// (see internal/workspace.formatAutomationError) — changing the layout
// here changes what those log lines look like.
func (e *ScriptError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path != "" && e.LuaLine > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.LuaLine, e.LuaMessage)
	}
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.LuaMessage)
	}
	return e.LuaMessage
}

func (e *ScriptError) fillFromError(err error) {
	if err == nil {
		return
	}

	var ae *glua.ApiError
	if !errors.As(err, &ae) {
		// Not a typed Lua error — preserve the message, populate line
		// from any captured frames, and bail.
		e.LuaMessage = err.Error()
		if frame := e.deepestUserFrame(); frame != nil {
			e.LuaLine = frame.Line
		}
		return
	}

	switch ae.Type {
	case glua.ApiErrorSyntax, glua.ApiErrorFile:
		// Compile-time failure: PCall never ran, so no captured frames.
		// The Object string carries the canonical
		// "<chunkname> line:N(column:N) near 'tok': parse error" form.
		msg := strings.TrimSpace(ae.Object.String())
		e.LuaMessage = msg
		e.LuaLine = parseCompileErrorLine(msg)
	default:
		// Runtime failure. The message handler captured typed frames;
		// the Object string is the raw error message (may be prefixed
		// with "<chunk>:<line>: " for string-typed errors, or just
		// "table: 0x..." for non-string raises).
		e.LuaMessage = stripChunkLinePrefix(ae.Object.String())
		if frame := e.deepestUserFrame(); frame != nil {
			e.LuaLine = frame.Line
		}
	}
}

// deepestUserFrame returns the first frame in the trace that has a real
// path and line — the place the user actually wants to look. nil if none.
func (e *ScriptError) deepestUserFrame() *StackFrame {
	for i := range e.Stack {
		f := &e.Stack[i]
		if f.Path != "" && f.Line > 0 {
			return f
		}
	}
	return nil
}

// chunkLinePrefixRe matches the leading "<chunkname>:<line>: " that
// gopher-lua prepends to runtime error messages where the raised value
// is a string. We strip it because the structured ScriptError already
// carries Path and LuaLine separately — leaving the prefix in
// LuaMessage just clutters the UI.
var chunkLinePrefixRe = regexp.MustCompile(`^.+?:\d+:\s*`)

func stripChunkLinePrefix(s string) string {
	return chunkLinePrefixRe.ReplaceAllString(s, "")
}

// compileErrorLineRe matches gopher-lua's compile-error format:
// "<chunkname> line:N(column:N) near 'tok': parse error".
var compileErrorLineRe = regexp.MustCompile(`line:(\d+)\(column:\d+\)`)

func parseCompileErrorLine(s string) int {
	m := compileErrorLineRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// readSourceSlice returns a ±context-line slice around the failing line
// from the script source. Returns nil on any access error (file too big,
// outside FS root, missing). The FS is responsible for traversal safety —
// callers should use a rooted filesystem (e.g., os.DirFS(scriptsRoot)).
func readSourceSlice(srcFS fs.FS, path string, failingLine, context int) []SourceLine {
	if path == "" || failingLine <= 0 || srcFS == nil {
		return nil
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	// Belt-and-braces over os.DirFS, which since Go 1.20 already rejects
	// reads escaping its root. The check stays so callers passing an FS
	// with weaker guarantees (e.g. a hand-rolled fs.FS, MapFS in tests)
	// can't be tricked into reading outside their intended scope.
	if clean == "" || strings.HasPrefix(clean, "../") || clean == ".." || filepath.IsAbs(clean) {
		return nil
	}

	info, err := fs.Stat(srcFS, clean)
	if err != nil || info.IsDir() || info.Size() > maxSourceFileSize {
		return nil
	}
	data, err := fs.ReadFile(srcFS, clean)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	// Trim a trailing empty line caused by a final newline so line numbers
	// align with editor expectations.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if failingLine > len(lines) {
		return nil
	}

	from := failingLine - context
	if from < 1 {
		from = 1
	}
	to := failingLine + context
	if to > len(lines) {
		to = len(lines)
	}

	out := make([]SourceLine, 0, to-from+1)
	for n := from; n <= to; n++ {
		out = append(out, SourceLine{
			N:         n,
			Text:      lines[n-1],
			Highlight: n == failingLine,
		})
	}
	return out
}

// Redaction.
//
// Args and captured output may contain user secrets (the script can stuff
// anything it likes into either). We apply two layers of defense before
// these fields cross any wire:
//
//  1. Key-name denylist on map entries (case-insensitive, full-key match).
//  2. Value-shape detection on string values (JWT-shaped, long hex, long
//     base64-ish) — catches secrets passed in under non-obvious keys.
//
// Plus a length cap on string values and the captured-output buffer.

var (
	redactedKeyRe = regexp.MustCompile(`(?i)^(password|token|secret|api[_-]?key|authorization|bearer|cookie|session|credential|pat|client[_-]?secret|private[_-]?key|webhook[_-]?url)$`)
	jwtRe         = regexp.MustCompile(`^eyJ[A-Za-z0-9_=-]+\.`)
	longHexRe     = regexp.MustCompile(`^[A-Fa-f0-9]{32,}$`)
	longBase64Re  = regexp.MustCompile(`^[A-Za-z0-9+/=_-]{32,}$`)
)

// redactArgs returns a deep-copied args map with sensitive values replaced
// by redactedPlaceholder. Returns nil when the input is empty.
func redactArgs(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if redactedKeyRe.MatchString(k) {
			out[k] = redactedPlaceholder
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

func redactValue(v any) any {
	switch x := v.(type) {
	case string:
		return redactString(x)
	case map[string]any:
		return redactArgs(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = redactValue(item)
		}
		return out
	default:
		return v
	}
}

func redactString(s string) string {
	if jwtRe.MatchString(s) || longHexRe.MatchString(s) || longBase64Re.MatchString(s) {
		return redactedPlaceholder
	}
	if len(s) > maxStringValueLen {
		return safeTruncate(s, maxStringValueLen) + truncatedPlaceholder
	}
	return s
}

// redactCapturedOutput applies redactString to each line and caps the
// total at maxCapturedOutputBytes, appending a truncation marker when
// data is dropped.
func redactCapturedOutput(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	var sb strings.Builder
	truncated := false
	for i, line := range lines {
		redacted := redactString(line)
		nextLen := sb.Len() + len(redacted)
		if i+1 < len(lines) {
			nextLen++
		}
		if nextLen > maxCapturedOutputBytes {
			room := maxCapturedOutputBytes - sb.Len()
			if room > 0 && room < len(redacted) {
				sb.WriteString(safeTruncate(redacted, room))
			} else if room >= len(redacted) {
				sb.WriteString(redacted)
			}
			truncated = true
			break
		}
		sb.WriteString(redacted)
		if i+1 < len(lines) {
			sb.WriteByte('\n')
		}
	}
	if truncated {
		dropped := len(b) - sb.Len()
		fmt.Fprintf(&sb, "\n...[truncated, %d more bytes]", dropped)
	}
	return sb.String()
}

// safeTruncate cuts s to at most n bytes, trimming any incomplete
// multi-byte UTF-8 sequence at the boundary so the returned string
// remains valid UTF-8.
func safeTruncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		if n <= 0 {
			return ""
		}
		return s
	}
	cut := s[:n]
	for !utf8.ValidString(cut) && cut != "" {
		cut = cut[:len(cut)-1]
	}
	return cut
}
