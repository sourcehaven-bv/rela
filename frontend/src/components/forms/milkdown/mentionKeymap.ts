/**
 * Keyboard handling for the `@` completion menu.
 *
 * Extracted from `MilkdownEditor.vue` so the menu's key semantics are testable
 * without mounting an editor, and so the component does not keep growing (it
 * sits on the 500-line god-component limit). The editor still owns the document
 * and the slash provider; this only decides what a keystroke MEANS while the
 * menu is open, and calls back for the parts that touch ProseMirror.
 */
import { shouldClearScopeOnBackspace } from './mentionQuery'
import type { MentionChoice, MentionMenuController } from './useMentionMenu'

/**
 * What the keymap needs from the editor.
 *
 * Declared here, at the call site, rather than exposing the editor's internals:
 * each member is one thing the handler cannot do for itself.
 */
export interface MentionKeymapHost {
  /** The paragraph text before the cursor, or undefined outside a mention. */
  queryText: () => string | undefined
  /** Acts on the highlighted row: insert an entity, or scope to a type. */
  commit: (choice: MentionChoice) => void
  /** Closes the menu and remembers the dismissed query so it stays shut. */
  dismiss: (query: string) => void
}

/**
 * Builds the capture-phase keydown handler.
 *
 * Must be bound in the CAPTURE phase so it runs before ProseMirror's own
 * keymap: otherwise Enter inserts a paragraph break and the arrow keys move the
 * cursor instead of the highlight.
 *
 * The `@` and `/` menus cannot both be open — `@` needs a boundary character
 * before it and `/` only fires at the start of a block — so checking the
 * mention menu first resolves a stray overlap one way rather than acting on
 * both.
 */
export function createMentionKeydownHandler(
  menu: MentionMenuController,
  host: MentionKeymapHost
): (event: KeyboardEvent) => void {
  return function onKeydownCapture(event: KeyboardEvent): void {
    if (!menu.state.open) return
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        menu.moveHighlight(1)
        break
      case 'ArrowUp':
        event.preventDefault()
        menu.moveHighlight(-1)
        break
      case 'Enter':
      case 'Tab': {
        const choice = menu.current()
        if (!choice) return
        event.preventDefault()
        host.commit(choice as MentionChoice)
        break
      }
      case 'Backspace': {
        // The query is read from the LIVE document rather than from
        // `menu.state.query`, which lags one tick behind it. See
        // `shouldClearScopeOnBackspace`.
        if (!shouldClearScopeOnBackspace(host.queryText(), menu.state.selectedType !== null)) {
          return
        }
        event.preventDefault()
        menu.clearType()
        break
      }
      case 'Escape':
        event.preventDefault()
        host.dismiss(menu.state.query)
        break
      default:
        break
    }
  }
}
