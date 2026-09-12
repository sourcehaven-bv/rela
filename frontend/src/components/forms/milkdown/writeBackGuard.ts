/**
 * The write-back guard.
 *
 * A WYSIWYG editor round-trips every body it opens: markdown in, document
 * tree, markdown out. Two things can go wrong, and they need different
 * answers.
 *
 * *Byte churn* is a reformat that says the same thing — a table re-padded to
 * its column widths, a heading rewritten from setext to ATX. Writing it back
 * would put a diff in git history for an entity the user only looked at. The
 * guard suppresses it by returning the original bytes.
 *
 * *Semantic drift* is a round-trip that changes meaning. That is a bug in the
 * parse/serialize pair, and the guard cannot repair it. It also returns the
 * original bytes, so the bug costs the user a lost edit rather than a silently
 * corrupted body, and reports it so the failure is visible rather than
 * absorbed.
 *
 * The corpus test measures both across every entity body in the repository:
 * zero semantic drift, and byte churn on roughly 44% of files. The guard is
 * what makes that 44% cost nothing.
 */
import { unified } from 'unified'
import remarkParse from 'remark-parse'
import remarkGfm from 'remark-gfm'
import { isSemanticallyEqual } from './serializerContract'

const parser = unified().use(remarkParse).use(remarkGfm)

function parse(markdown: string): { type: string } {
  return parser.runSync(parser.parse(markdown)) as { type: string }
}

/** What `guardWriteBack` decided, so a caller can report or count it. */
export type WriteBackVerdict =
  /** The editor's output is byte-identical to what it was given. */
  | 'unchanged'
  /** The user edited the body; the editor's output is what to save. */
  | 'edited'
  /** Same meaning, different bytes. The original is kept. */
  | 'churn-suppressed'
  /** Meaning changed without an edit. A bug; the original is kept. */
  | 'drift-blocked'

export interface WriteBackResult {
  /** The markdown to save. */
  value: string
  verdict: WriteBackVerdict
}

/**
 * Decides what to save, given what the editor was loaded with and what it now
 * produces.
 *
 * `dirty` is the editor's own record of whether the user typed anything. It is
 * the only thing that distinguishes a real edit from a round-trip artifact:
 * without it, an edit that happens to reformat a table is indistinguishable
 * from opening the entity and touching nothing.
 *
 * Nil: rejected — both arguments are required strings.
 */
export function guardWriteBack(
  original: string,
  current: string,
  dirty: boolean
): WriteBackResult {
  if (current === original) return { value: original, verdict: 'unchanged' }
  if (dirty) return { value: current, verdict: 'edited' }

  // Not dirty but the bytes moved: the round-trip reformatted something.
  // Suppress it if it means the same thing, and refuse it if it does not.
  if (isSemanticallyEqual(parse(original), parse(current))) {
    return { value: original, verdict: 'churn-suppressed' }
  }
  return { value: original, verdict: 'drift-blocked' }
}
