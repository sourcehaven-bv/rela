/**
 * Parsing the `@` mention query out of the text before the cursor.
 *
 * The completion menu is triggered by `@` and stays open while the user types
 * a query. `SlashProvider.getContent` hands back the paragraph text up to the
 * cursor; this module turns that into either an active query or nothing.
 *
 * The old backtick trigger inspected CodeMirror token types to decide whether
 * the cursor sat somewhere a reference made sense. ProseMirror has no token
 * stream, and it does not need one: `SlashProvider.getContent` already returns
 * undefined outside a paragraph, on a non-empty selection, on a non-text
 * selection, when the view is read-only, and when the editor lacks focus. All
 * that is left is deciding what counts as a query.
 */

/** The longest query accepted before the menu gives up and closes. */
const MAX_QUERY_LENGTH = 64

/**
 * Characters that end a mention query.
 *
 * A space ends it because entity IDs and the titles searched against them are
 * single-token, and because leaving the menu open across a space would make it
 * fire on ordinary prose containing an `@`. Backticks end it because they open
 * a code span, where a mention is not what the user means.
 */
const TERMINATORS = /[\s`]/

export interface MentionQuery {
  /** The text typed after the `@`, possibly empty right after the trigger. */
  query: string
  /**
   * How many characters back from the cursor the trigger sits, counting the
   * `@` itself. Insertion uses this to replace the trigger and query together.
   */
  matchLength: number
}

/**
 * Extracts the active mention query from the text before the cursor.
 *
 * Returns null when there is no active query, which is the signal to close the
 * menu.
 *
 * Nil: accepts undefined for `textBefore`, since `getContent` returns undefined
 * when the cursor is not somewhere a mention makes sense.
 */
export function parseMentionQuery(textBefore: string | undefined): MentionQuery | null {
  if (!textBefore) return null

  const at = textBefore.lastIndexOf('@')
  if (at === -1) return null

  const query = textBefore.slice(at + 1)
  if (query.length > MAX_QUERY_LENGTH) return null
  if (TERMINATORS.test(query)) return null

  // An `@` immediately after a word character is an email address or a handle,
  // not a mention trigger. Requiring a boundary before it keeps the menu out
  // of the way when someone types an address into prose.
  if (at > 0) {
    const before = textBefore[at - 1] ?? ''
    if (!/[\s([\]{}>,;:"']/.test(before)) return null
  }

  return { query, matchLength: query.length + 1 }
}
