/**
 * Brings one run card of the Flows panel into view: the card whose
 * `data-run-id` is `runId`. A next-action hint for a flow task links to the
 * subject page with `?run=<id>`, so the step form is in view on arrival.
 *
 * The card takes focus too (it is made focusable for that), so a keyboard
 * or screen-reader user lands on the step rather than at the top of the
 * page. Returns whether the card was found.
 */
export function focusRunCard(root: HTMLElement | null | undefined, runId: string): boolean {
  if (!root || !runId) return false
  const card = Array.from(root.querySelectorAll<HTMLElement>('[data-run-id]')).find(
    (el) => el.dataset.runId === runId
  )
  if (!card) return false
  card.scrollIntoView?.({ block: 'start' })
  if (!card.hasAttribute('tabindex')) card.setAttribute('tabindex', '-1')
  card.focus({ preventScroll: true })
  return true
}

/** The run a route asks the Flows panel to bring into view, or ''. */
export function requestedRun(query: Record<string, unknown>): string {
  const v = query.run
  return typeof v === 'string' ? v : ''
}
