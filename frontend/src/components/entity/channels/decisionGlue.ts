import { DECISIONS_BLOCKED_KEY } from 'rela-components/components/channel/channelFields'
import { classifyChannelError } from './channelErrors'

/**
 * The glue between useSessionDecisions and a surface that shows its
 * buttons: the open channel and the inbox row (TKT-V05706X). One copy, so
 * both block and answer a refusal the same way.
 */

/**
 * The pending keys the library sees while a decision or a send runs: every
 * decision is blocked; only the posting decision's own key shows it as
 * sending.
 */
export function blockedPending(pending: ReadonlySet<string>): ReadonlySet<string> {
  return new Set([...pending, DECISIONS_BLOCKED_KEY])
}

/**
 * Where a refusal's message goes when channelErrors places it in a form:
 * `form` when the request came from a form that shows it, `toast` when
 * there is none.
 */
export type RefusalTarget = 'form' | 'toast'

/** What a surface does after a refusal, per channelErrors' `then`. */
export interface RefusalSteps {
  /** Read the channel (or the inbox) again: affordances or state moved. */
  refetch(): void
  /** The channel is gone for this reader. */
  gone(): void
  /** The configuration moved: reload the descriptor. */
  reloadSchema(): void
  /** Shows a message that does not belong in a form. */
  toast(message: string): void
}

/**
 * Answers a refused channel operation by the channelErrors rules. Returns
 * the message when it belongs in a form and `target` is `form`, else shows
 * it as a toast and returns undefined.
 */
export function handleChannelRefusal(
  err: unknown,
  target: RefusalTarget,
  steps: RefusalSteps
): string | undefined {
  const out = classifyChannelError(err)
  if (out.then === 'refetch') steps.refetch()
  else if (out.then === 'list') steps.gone()
  else if (out.then === 'schema') steps.reloadSchema()
  if (out.place === 'form' && target === 'form') return out.message
  steps.toast(out.message)
  return undefined
}
