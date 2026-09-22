/**
 * Deciding whether the `@` completion menu should be showing.
 *
 * `SlashProvider`'s own `shouldShow` only fires while the trigger character is
 * the last thing typed, so it cannot follow a query as the user extends it.
 * This replaces it: the text before the cursor is re-read on every update and
 * the answer is derived from that.
 *
 * Extracted from the editor component because it carries the one piece of
 * state that is easy to get wrong — `activeMatchLength`, the width of the span
 * an insertion will DELETE. Leave a stale value behind and the next insertion
 * eats characters that are no longer part of a query.
 */
import { parseMentionQuery } from './mentionQuery'

export interface MentionTriggerHost {
  /** True while the menu is showing. */
  isOpen: () => boolean
  /** Hides the menu. */
  close: () => void
  /** Reports the query the menu should be searching for. */
  setQuery: (query: string) => void
}

export interface MentionTrigger {
  /** Answers `SlashProvider.shouldShow` for the text before the cursor. */
  shouldShow: (content: string | undefined) => boolean
  /**
   * Width of the `@query` span before the cursor, or 0 when there is none.
   *
   * An insertion replaces exactly this many characters, so a non-positive
   * value must mean "insert nothing" rather than "insert at the cursor".
   */
  matchLength: () => number
  /** Suppresses the menu for the query currently typed, until it changes. */
  dismiss: (query: string) => void
}

export function createMentionTrigger(host: MentionTriggerHost): MentionTrigger {
  /**
   * The query Escape dismissed.
   *
   * Without this the menu reopens on the next update, because the text before
   * the cursor still parses as a query. Editing it — typing or deleting —
   * produces a different query and the menu is welcome back.
   */
  let dismissedQuery: string | null = null
  let activeMatchLength = 0

  return {
    shouldShow(content) {
      const match = parseMentionQuery(content)
      if (!match) {
        if (host.isOpen()) host.close()
        dismissedQuery = null
        // Cleared along with the menu: this is the span an insertion deletes.
        activeMatchLength = 0
        return false
      }
      if (dismissedQuery !== null && match.query === dismissedQuery) return false
      dismissedQuery = null
      activeMatchLength = match.matchLength
      host.setQuery(match.query)
      return true
    },
    matchLength: () => activeMatchLength,
    dismiss(query) {
      dismissedQuery = query
      activeMatchLength = 0
    },
  }
}
