/**
 * The <rela-editor> formatting toolbar, in plain DOM.
 *
 * The SPA renders the same buttons from a Vue component. This bundle has no
 * framework, so the toolbar is built through the DOM API instead. What it is
 * NOT is a second catalogue of commands: the buttons come from the shared
 * `EditorCommand` lists and the glyphs from the shared icon table, so a command
 * added in one editor appears in both.
 *
 * Presentational, like the Vue one: it renders buttons and their state and
 * calls back. It holds no editor reference, so which command a button fires and
 * whether it is active are both decided by the caller from ProseMirror state.
 */
import {
  INLINE_COMMANDS,
  BLOCK_COMMANDS,
  type EditorCommand,
} from '@/components/forms/milkdown/editorCommands'
import { TABLE_COMMANDS } from '@/components/forms/milkdown/tableCommands'
import { createIconSvg } from '@/components/forms/milkdown/editorIcons'

export interface ToolbarHandle {
  /** The toolbar's root element, for the caller to place. */
  readonly root: HTMLElement
  /**
   * Repaints active/unavailable state and shows or hides the table group.
   */
  update(active: ReadonlySet<string>, unavailable: ReadonlySet<string>, inTable: boolean): void
  destroy(): void
}

export interface ToolbarCallbacks {
  /** Run a formatting command. Never called for an unavailable one. */
  run(command: EditorCommand): void
  /** Open the entity-reference picker. */
  insertRef(): void
}

/** The reference button, which is not a formatting command. */
const REF_BUTTON_ID = 'entityRef'

function button(
  id: string,
  label: string,
  onActivate: () => void,
  unavailable: () => boolean
): HTMLButtonElement {
  const el = document.createElement('button')
  el.type = 'button'
  el.className = 'rela-toolbar-button'
  el.dataset.command = id
  el.title = label
  el.setAttribute('aria-label', label)
  el.appendChild(createIconSvg(id))
  // Prevent the mousedown default so pressing a button does not pull focus out
  // of the document: without it the selection collapses before the command
  // runs, and formatting a selection from the toolbar becomes impossible.
  el.addEventListener('mousedown', (e) => e.preventDefault())
  el.addEventListener('click', () => {
    // `aria-disabled` does not prevent activation, so the handler refuses.
    if (unavailable()) return
    onActivate()
  })
  return el
}

function divider(): HTMLElement {
  const el = document.createElement('span')
  el.className = 'rela-toolbar-divider'
  el.setAttribute('role', 'separator')
  return el
}

function group(): HTMLElement {
  const el = document.createElement('div')
  el.className = 'rela-toolbar-group'
  return el
}

/**
 * Builds the toolbar.
 *
 * `hasBridge` decides whether the entity-reference button appears at all: the
 * picker it opens reads the graph through the app bridge, so without one the
 * button could only open an empty dialog.
 */
export function createToolbar(cb: ToolbarCallbacks, hasBridge: boolean): ToolbarHandle {
  const root = document.createElement('div')
  root.className = 'rela-toolbar'
  root.setAttribute('role', 'toolbar')
  root.setAttribute('aria-label', 'Formatting')

  const buttons = new Map<string, HTMLButtonElement>()
  let unavailableIds: ReadonlySet<string> = new Set()

  const add = (parent: HTMLElement, cmd: EditorCommand): void => {
    const el = button(
      cmd.id,
      cmd.label,
      () => cb.run(cmd),
      () => unavailableIds.has(cmd.id)
    )
    buttons.set(cmd.id, el)
    parent.appendChild(el)
  }

  const inline = group()
  for (const cmd of INLINE_COMMANDS) add(inline, cmd)
  root.appendChild(inline)

  root.appendChild(divider())

  const block = group()
  for (const cmd of BLOCK_COMMANDS) add(block, cmd)
  root.appendChild(block)

  if (hasBridge) {
    root.appendChild(divider())
    const refGroup = group()
    const refButton = button(
      REF_BUTTON_ID,
      'Insert entity reference',
      () => cb.insertRef(),
      () => false
    )
    refGroup.appendChild(refButton)
    root.appendChild(refGroup)
  }

  // The table group is hidden rather than disabled outside a table: seven
  // permanently-greyed buttons is a lot of dead chrome to carry on every
  // paragraph, and unlike the block commands these have no meaning outside a
  // table to hint at.
  const tableDivider = divider()
  const tableGroup = group()
  for (const cmd of TABLE_COMMANDS) add(tableGroup, cmd)
  root.appendChild(tableDivider)
  root.appendChild(tableGroup)

  const setTableVisible = (visible: boolean): void => {
    tableDivider.hidden = !visible
    tableGroup.hidden = !visible
  }
  setTableVisible(false)

  return {
    root,
    update(active, unavailable, inTable) {
      unavailableIds = unavailable
      for (const [id, el] of buttons) {
        const isActive = active.has(id)
        const isUnavailable = unavailable.has(id)
        el.classList.toggle('is-active', isActive)
        el.classList.toggle('is-unavailable', isUnavailable)
        el.setAttribute('aria-pressed', String(isActive))
        // `aria-disabled`, not the native `disabled`: a native disabled control
        // cannot be focused, so if the cursor moves into a context that
        // disables the button the user is tabbed to, focus drops to <body>
        // mid-interaction.
        el.setAttribute('aria-disabled', String(isUnavailable))
      }
      setTableVisible(inTable)
    },
    destroy() {
      root.remove()
      buttons.clear()
    },
  }
}
