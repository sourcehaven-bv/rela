// Package cli — script-error rendering helpers.
//
// formatScriptError translates a *lua.ScriptError into the multi-line
// human-readable form used by `rela analyze` and `rela validate`. Kept
// in the cli package (not lua) because the layout is shaped by the
// CLI's other output (indentation, severity prefixes) and the lua
// package has no opinion on terminal rendering.
package cli

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// scriptErrorMessageMaxLen caps the first-line message length so a
// runaway error from a misbehaving Lua script can't push the output
// to the right of the screen forever. Source-slice context still
// surfaces below the truncated message.
const scriptErrorMessageMaxLen = 240

// formatScriptError renders a *lua.ScriptError as a multi-line block:
//
//	<path>:<line>: <message>
//	     N | <source line>
//	     N | <highlighted line>     <- the failing line
//	     N | <source line>
//
// When LuaLine is zero (e.g. contract violations, unframed errors)
// the path-only form "<path>: <message>" is used. When Source is
// empty, only the headline is emitted. Lines are joined with "\n"
// so the caller can decide whether to print them as one
// WriteMessage or split.
func formatScriptError(se *lua.ScriptError) string {
	if se == nil {
		return ""
	}
	var b strings.Builder
	headline := se.Error()
	if len(headline) > scriptErrorMessageMaxLen {
		headline = headline[:scriptErrorMessageMaxLen] + "..."
	}
	b.WriteString(headline)
	for _, line := range se.Source {
		b.WriteByte('\n')
		marker := "  "
		if line.Highlight {
			marker = "> "
		}
		fmt.Fprintf(&b, "  %s%4d | %s", marker, line.N, line.Text)
	}
	return b.String()
}
