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
export function guardWriteBack(original: string, current: string, dirty: boolean): WriteBackResult {
  if (current === original) return { value: original, verdict: 'unchanged' }
  if (dirty) return { value: current, verdict: 'edited' }

  // Not dirty but the bytes moved: the round-trip reformatted something.
  // Suppress it if it means the same thing, and refuse it if it does not.
  if (isSemanticallyEqual(parse(original), parse(current))) {
    return { value: original, verdict: 'churn-suppressed' }
  }
  return { value: original, verdict: 'drift-blocked' }
}

/** What the editor should do with a freshly serialized document. */
export type EmitDecision =
  { action: 'ignore' } | { action: 'emit'; value: string } | { action: 'report-drift' }

/**
 * Decides what a serialization change means for the parent.
 *
 * Pulled out of the editor's markdown listener so it can be tested without
 * mounting an editor. The listener is debounced and does not fire for a
 * programmatic dispatch, which is exactly why the original inline version was
 * never covered: the only assertions possible against it were negative ones.
 *
 * `settled` suppresses the editor echoing its own load back at the parent.
 * `original` is the pristine input the guard compares against, and must NOT
 * be the settled value — see the note on `originalValue` in MilkdownEditor.
 */
export function decideEmit(
  markdown: string,
  original: string,
  settled: string,
  dirty: boolean,
  lastEmitted: string | null
): EmitDecision {
  // The editor echoing back exactly what it loaded, with the parent not yet
  // told anything different. Once something HAS been emitted this is no longer
  // sufficient: reverting an edit lands back on `settled`, and the parent still
  // holds the intermediate value.
  if (markdown === settled && lastEmitted === null) return { action: 'ignore' }

  const guarded = guardWriteBack(original, markdown, dirty)
  if (guarded.verdict === 'drift-blocked') return { action: 'report-drift' }

  // What the parent should now hold. For churn this is the original bytes,
  // which is the whole point of the guard.
  const next = guarded.value

  // Nothing to say only if the parent ALREADY holds this. Comparing against
  // `original` alone was wrong: a user who types and then reverts ends up back
  // at the original text, and the parent would keep the intermediate value it
  // was told about and save that instead.
  if (lastEmitted === null ? next === original : next === lastEmitted) {
    return { action: 'ignore' }
  }
  return { action: 'emit', value: next }
}
