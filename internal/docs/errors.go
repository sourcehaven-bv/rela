package docs

import "fmt"

// BuildError is a fail-loud error tied to a specific line of the manual source.
// The doc language treats any unresolved island — a parse error, an unknown
// type/field, a Lua error, or (under strict mode) an empty resolve — as a build
// failure rather than silently emitting nothing, and reports the offending
// MANUAL line so the author can find it.
type BuildError struct {
	Line int    // 1-based manual source line
	Kind string // short category: "parse", "lua", "resolve", "strict"
	Msg  string // human-readable detail
	Snip string // the offending island text (optional)
}

func (e *BuildError) Error() string {
	loc := fmt.Sprintf("manual:%d", e.Line)
	if e.Kind != "" {
		return fmt.Sprintf("%s: %s: %s", loc, e.Kind, e.Msg)
	}
	return fmt.Sprintf("%s: %s", loc, e.Msg)
}
