package docs

import (
	"context"

	lua "github.com/yuin/gopher-lua"
)

// tierBBindings holds the injected, may-be-nil, fail-loud Tier-B capabilities:
// the browser Capturer behind screenshot{} and the APIClient behind api{}.
//
// They are grouped because they share a property nothing else in the doc
// runtime has: each is supplied by the CLI and is legitimately nil, which is
// why each carries its own *Err string explaining why it could not be built.
// Keeping them off docRuntime keeps the core docs package browser-free and
// server-free by construction — screenshot{} and api{} can only reach what is
// on this struct, not the metamodel, policy, store or tracer they have no
// business touching.
//
// Nil: a nil capturer/apiClient is accepted and expected — it is the signal
// that the capability was not injected, and the binding fails loud rather than
// skipping, because a skipped assertion looks exactly like a passing one.
type tierBBindings struct {
	// capturer renders screenshot{} islands. nil ⇒ screenshot{} fails loud (the
	// Tier-B browser dependency is injected only by the CLI, keeping core docs
	// browser-free).
	capturer Capturer
	// capturerErr is the reason the capturer could not be constructed (e.g. no
	// Chrome), surfaced in the fail-loud message when a screenshot{} needs it.
	capturerErr string

	// apiClient serves api{} islands. nil ⇒ api{} fails loud, for the same
	// reason a nil capturer does. Unlike the capturer it needs no browser or
	// built frontend.
	apiClient APIClient
	// apiClientErr is why the client could not be constructed, surfaced in the
	// fail-loud message.
	apiClientErr string

	// projectDir is the documented project's root (schema/config copied into the
	// screenshot temp project). Empty in a schema-only build.
	projectDir string

	// outDir is where screenshot{} PNGs are written (derived from the build's
	// --out); empty ⇒ PNGs written next to the cwd and referenced by basename.
	outDir string

	// seed returns the seed ops recorded SO FAR by create()/link(). It is a
	// function, not a captured slice: registration happens once before any
	// island runs, and the ops accumulate as the manual executes, so a value
	// snapshotted at construction would always be empty. This is the whole seam
	// onto the seed recorder — the Tier-B bindings read the ops and never touch
	// the recorder itself.
	seed func() []SeedOp

	// ctx bounds a capture / API call. Stored because the doc.* bindings are
	// gopher-lua callbacks (func(*lua.LState) int) that cannot take a context
	// parameter; the runtime is short-lived and request-scoped.
	ctx context.Context //nolint:containedctx // request-scoped Lua-binding callbacks

	// emit appends rendered markdown to the current statement island's buffer.
	emit func(string)
	// fail records a resolve BuildError and aborts the current island. It is the
	// runtime's luaFail, which owns the typed-error stashing wrapLuaErr depends
	// on; the Tier-B bindings only need to call it.
	fail func(ls *lua.LState, format string, args ...any) int
}

// luaFailer is the fail-loud seam shared by the doc.* bindings: record a typed
// resolve error and abort the current island. Declared at the call site so a
// helper like rejectUnknownKeys does not need the whole docRuntime.
type luaFailer interface {
	luaFail(ls *lua.LState, format string, args ...any) int
}

// luaFail satisfies [luaFailer] by delegating to the runtime's fail hook, so
// the Tier-B bindings share rejectUnknownKeys with the rest of the verbs.
func (b *tierBBindings) luaFail(ls *lua.LState, format string, args ...any) int {
	return b.fail(ls, format, args...)
}
