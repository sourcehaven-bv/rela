/**
 * Working out which toolbar commands would actually do something.
 *
 * A button that looks clickable but silently no-ops is worse than one that is
 * plainly unavailable: the user presses it, nothing happens, and there is
 * nothing to explain why. Heading inside a list item is the case that prompted
 * this — `WrapInHeading` cannot apply there, so the button did nothing.
 *
 * The check is a DRY RUN, not a reimplementation of each command's rules.
 * A ProseMirror `Command` takes `(state, dispatch?)` and, when `dispatch` is
 * omitted, reports whether it WOULD apply while changing nothing. Asking the
 * command itself means the toolbar cannot drift from what the command does;
 * hand-written "can I?" predicates would be a second copy of every rule.
 *
 * Milkdown's own `commandsCtx.call` always passes `view.dispatch`, so it
 * cannot dry-run. Going through `commandsCtx.get(slice)` returns the raw
 * factory, and calling it with the payload yields the plain ProseMirror
 * command to probe.
 */
import type { Ctx } from '@milkdown/kit/ctx'
import { commandsCtx } from '@milkdown/kit/core'
import type { EditorState } from '@milkdown/kit/prose/state'
import type { EditorCommand } from './editorCommands'

/** A ProseMirror command, in the shape the dry run needs. */
type ProseCommand = (state: EditorState, dispatch?: unknown) => boolean

/**
 * Whether a named command would apply to the current state.
 *
 * Returns false when the slice is not registered, so an unknown name disables
 * the button rather than throwing when it is pressed.
 */
function wouldApply(ctx: Ctx, state: EditorState, slice: string, payload?: unknown): boolean {
  try {
    const factory = ctx.get(commandsCtx).get(slice) as unknown as
      ((payload?: unknown) => ProseCommand) | undefined
    if (typeof factory !== 'function') return false
    // No dispatch: ProseMirror's contract is that the command reports
    // applicability and performs no change.
    return factory(payload)(state) === true
  } catch {
    return false
  }
}

/**
 * The ids of the commands that would have no effect right now.
 *
 * An ACTIVE command is judged by its inverse, because that is what pressing it
 * will run. Reversing this would disable exactly the buttons a user needs to
 * turn formatting off: inside a bullet list `WrapInBulletList` is itself
 * inapplicable, so probing the forward command would grey out the button that
 * removes the list.
 *
 * `'lift'` is not a registered slice (see `EditorCommand.toggleTo`), so it is
 * handled by the caller, which owns the ProseMirror `lift` import. A command
 * whose inverse is `lift` is never reported unavailable while active.
 */
export function unavailableCommandIds(
  ctx: Ctx,
  state: EditorState,
  commands: readonly EditorCommand[],
  activeIds: ReadonlySet<string>,
  liftApplies: boolean
): Set<string> {
  const unavailable = new Set<string>()
  for (const cmd of commands) {
    const active = activeIds.has(cmd.id)
    let ok: boolean
    if (active && cmd.toggleTo === 'lift') {
      ok = liftApplies
    } else if (active && cmd.toggleTo) {
      ok = wouldApply(ctx, state, cmd.toggleTo)
    } else {
      ok = wouldApply(ctx, state, cmd.command, cmd.payload)
    }
    if (!ok) unavailable.add(cmd.id)
  }
  return unavailable
}
