/**
 * The editor's formatting commands, named once and shared.
 *
 * Declaring them in one place keeps a command's label, icon and effect
 * identical wherever it is reached from, and means adding a block type is one
 * edit rather than several.
 *
 * Commands are named by their registered SLICE NAME (a string), not by the
 * `.key` of the imported `$Command` object. That is not a style choice.
 * `$command(key, cmd)` assigns `plugin.key` inside the function it returns, so
 * `.key` is undefined until the plugin has run — reading it at module load
 * yields undefined, and `callCommand(undefined)` throws inside the ctx
 * container rather than failing visibly at the call site. `callCommand` accepts
 * the slice name directly, which is resolved at call time and cannot go stale.
 *
 * The names must match what the presets register. `commandNamesExistInEditor`
 * in the editor test asserts exactly that against a live instance, so a rename
 * upstream fails a test instead of silently deadening a button.
 */

/**
 * How to tell whether a command's formatting is already active at the cursor.
 *
 * A mark command asks whether the mark is in the stored marks or spans the
 * selection; a node command asks what the enclosing block is. The two need
 * different lookups, so the descriptor names which one applies rather than
 * making the toolbar guess from the command.
 */
export type ActiveProbe =
  | { kind: 'mark'; mark: string }
  | { kind: 'node'; node: string; attrs?: Record<string, unknown> }
  | { kind: 'none' }

export interface EditorCommand {
  /** Stable identifier, used as a key and in tests. */
  id: string
  /** Human label, shown in the `/` menu and as the toolbar button tooltip. */
  label: string
  /**
   * The registered slice name of the Milkdown command to run.
   *
   * A string rather than a `CmdKey`; see the module comment for why.
   */
  command: string
  /** Payload passed to the command, when it takes one. */
  payload?: unknown
  /** How to decide whether this formatting is already on. */
  probe: ActiveProbe
  /**
   * How to undo this formatting when it is already active.
   *
   * Milkdown's `WrapInHeading` and friends are one-way: applied to a block
   * that is already that type they no-op, so a second press appears to do
   * nothing. Naming the inverse here makes those buttons genuine toggles,
   * which is what a pressed-looking button leads a user to expect.
   *
   * A slice name covers most cases. `'lift'` is the exception: unwrapping a
   * blockquote has no command in the preset (`TurnIntoText` would convert the
   * paragraph INSIDE the quote, which is already a paragraph, so the quote
   * survives). It maps to ProseMirror's own `lift`, applied by the editor.
   *
   * Marks need no entry; their `toggle*` commands already invert themselves.
   */
  toggleTo?: string | 'lift'
  /** Words that should match this command in the `/` menu. */
  keywords: string[]
}

/** Inline formatting, offered in the toolbar only. */
export const INLINE_COMMANDS: EditorCommand[] = [
  {
    id: 'strong',
    label: 'Bold',
    command: 'ToggleStrong',
    probe: { kind: 'mark', mark: 'strong' },
    keywords: ['bold', 'strong'],
  },
  {
    id: 'emphasis',
    label: 'Italic',
    command: 'ToggleEmphasis',
    probe: { kind: 'mark', mark: 'emphasis' },
    keywords: ['italic', 'emphasis'],
  },
  {
    id: 'strikethrough',
    label: 'Strikethrough',
    command: 'ToggleStrikeThrough',
    probe: { kind: 'mark', mark: 'strike_through' },
    keywords: ['strike', 'strikethrough'],
  },
  {
    id: 'inlineCode',
    label: 'Inline code',
    command: 'ToggleInlineCode',
    probe: { kind: 'mark', mark: 'inlineCode' },
    keywords: ['code', 'inline'],
  },
]

/** Block-level structure, offered in both the toolbar and the `/` menu. */
export const BLOCK_COMMANDS: EditorCommand[] = [
  {
    id: 'h1',
    label: 'Heading 1',
    command: 'WrapInHeading',
    payload: 1,
    probe: { kind: 'node', node: 'heading', attrs: { level: 1 } },
    toggleTo: 'TurnIntoText',
    keywords: ['h1', 'heading', 'title'],
  },
  {
    id: 'h2',
    label: 'Heading 2',
    command: 'WrapInHeading',
    payload: 2,
    probe: { kind: 'node', node: 'heading', attrs: { level: 2 } },
    toggleTo: 'TurnIntoText',
    keywords: ['h2', 'heading', 'subtitle'],
  },
  {
    id: 'h3',
    label: 'Heading 3',
    command: 'WrapInHeading',
    payload: 3,
    probe: { kind: 'node', node: 'heading', attrs: { level: 3 } },
    toggleTo: 'TurnIntoText',
    keywords: ['h3', 'heading'],
  },
  {
    id: 'bulletList',
    label: 'Bullet list',
    command: 'WrapInBulletList',
    probe: { kind: 'node', node: 'bullet_list' },
    toggleTo: 'LiftListItem',
    keywords: ['bullet', 'list', 'unordered', 'ul'],
  },
  {
    id: 'orderedList',
    label: 'Numbered list',
    command: 'WrapInOrderedList',
    probe: { kind: 'node', node: 'ordered_list' },
    toggleTo: 'LiftListItem',
    keywords: ['number', 'ordered', 'list', 'ol'],
  },
  {
    id: 'blockquote',
    label: 'Quote',
    command: 'WrapInBlockquote',
    probe: { kind: 'node', node: 'blockquote' },
    toggleTo: 'lift',
    keywords: ['quote', 'blockquote', 'citation'],
  },
  {
    id: 'codeBlock',
    label: 'Code block',
    command: 'CreateCodeBlock',
    probe: { kind: 'node', node: 'code_block' },
    toggleTo: 'TurnIntoText',
    keywords: ['code', 'block', 'fence', 'pre'],
  },
  {
    id: 'table',
    label: 'Table',
    command: 'InsertTable',
    probe: { kind: 'none' },
    keywords: ['table', 'grid'],
  },
]

/**
 * Ranks block commands against a `/` query.
 *
 * A prefix match on the label or any keyword outranks a substring match, so
 * typing `co` offers "Code block" before "Quote" (which matches only via
 * "citation"). An empty query lists everything in declaration order, which is
 * the order a user scanning the menu expects.
 */
export function filterBlockCommands(query: string): EditorCommand[] {
  const q = query.trim().toLowerCase()
  if (!q) return BLOCK_COMMANDS

  const scored: Array<{ cmd: EditorCommand; score: number }> = []
  for (const cmd of BLOCK_COMMANDS) {
    const terms = [cmd.label.toLowerCase(), ...cmd.keywords]
    let score = -1
    for (const term of terms) {
      if (term.startsWith(q)) {
        score = 2
        break
      }
      if (term.includes(q)) score = Math.max(score, 1)
    }
    if (score >= 0) scored.push({ cmd, score })
  }
  // Stable within a score band: `sort` is stable in every engine this targets,
  // so equal scores keep declaration order.
  scored.sort((a, b) => b.score - a.score)
  return scored.map((s) => s.cmd)
}
