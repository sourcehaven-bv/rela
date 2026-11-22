import { keepInView } from '@/utils/keepInView'

/**
 * Brings one run card of the Flows panel into view: the card whose
 * `data-run-id` is `runId`. A next-action hint for a flow task links to the
 * subject page with `?run=<id>`, so the step form is in view on arrival.
 *
 * The step form is what the reader came for, so it is the target when the
 * card shows one (`data-step-anchor`), else the card. The page renders the
 * view and the panels above the Flows panel after it, so the target is kept
 * at the top until the page settles or the user scrolls (BUG-UZGMHIK); a
 * single scroll left the form at the bottom edge of the pane.
 *
 * The card takes focus too (it is made focusable for that), so a keyboard
 * or screen-reader user lands on the step rather than at the top of the
 * page. Returns a function that stops keeping it in view, or null when the
 * card is not on the panel.
 */
export function focusRunCard(
  root: HTMLElement | null | undefined,
  runId: string
): (() => void) | null {
  if (!root || !runId) return null
  const find = () =>
    Array.from(root.querySelectorAll<HTMLElement>('[data-run-id]')).find(
      (el) => el.dataset.runId === runId
    )
  const card = find()
  if (!card) return null
  if (!card.hasAttribute('tabindex')) card.setAttribute('tabindex', '-1')
  card.focus({ preventScroll: true })
  return keepInView(() => {
    const now = find()
    return now?.querySelector<HTMLElement>('[data-step-anchor]') ?? now
  })
}

/** The run a route asks the Flows panel to bring into view, or ''. */
export function requestedRun(query: Record<string, unknown>): string {
  const v = query.run
  return typeof v === 'string' ? v : ''
}
