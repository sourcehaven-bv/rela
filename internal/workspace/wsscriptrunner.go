package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// wsScriptRunner is the [autocascade.ScriptRunner] adapter the
// workspace constructs once at New() time and hands to Manager. It
// resolves [lua.WriteDeps] per dispatch.
//
// **Why per-call resolution.** [lua.WriteDeps] needs an EntityManager
// reference so Lua scripts can call create/update/delete from inside a
// cascade. That EntityManager has to be the same one the trigger
// originated from (otherwise nested writes bypass policy). Binding it
// at Manager construction is impossible — Manager is what we're
// building, and `entitymanager` cannot import `lua` without creating
// a cycle (`lua` already depends on `entitymanager`). The per-call
// `w.LuaWriteDeps()` materializes the bundle at workspace scope —
// where both halves of the cycle are visible — and forwards each
// dispatch through `newLuaScriptRunner`.
//
// **Termination.** Lua scripts inside a cascade can call back into
// `lua.WriteDeps.EntityManager.CreateEntity`, which routes through
// the workspace's wsEntityManager → entitymanager.Manager → another
// cascade. The recursion is bounded by [autocascade.MaxDepth] (50).
type wsScriptRunner struct{ w *Workspace }

// Compile-time assertion that *wsScriptRunner satisfies the consumer-
// side interface — surfaces any future contract change at the type
// itself rather than at the wiring site.
var _ autocascade.ScriptRunner = (*wsScriptRunner)(nil)

// Run satisfies [autocascade.ScriptRunner.Run].
func (r *wsScriptRunner) Run(ctx context.Context, a autocascade.ScriptAction) error {
	return script.NewLuaScriptRunner(r.w.scriptExec, r.w.LuaWriteDeps()).Run(ctx, a)
}
