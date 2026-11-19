import { describe, expect, it, vi } from 'vitest'
import { focusRunCard, requestedRun } from './focusRun'

function panelWith(...ids: string[]): HTMLElement {
  const root = document.createElement('section')
  for (const id of ids) {
    const card = document.createElement('article')
    card.dataset.runId = id
    root.appendChild(card)
  }
  document.body.appendChild(root)
  return root
}

describe('focusRunCard', () => {
  it('scrolls to and focuses the named run', () => {
    const root = panelWith('RUN-1', 'RUN-2')
    const card = root.querySelector<HTMLElement>('[data-run-id="RUN-2"]')!
    card.scrollIntoView = vi.fn()

    expect(focusRunCard(root, 'RUN-2')).toBe(true)
    expect(card.scrollIntoView).toHaveBeenCalled()
    expect(card.getAttribute('tabindex')).toBe('-1')
    expect(document.activeElement).toBe(card)
  })

  it('reports a run that is not on the panel', () => {
    expect(focusRunCard(panelWith('RUN-1'), 'RUN-9')).toBe(false)
    expect(focusRunCard(null, 'RUN-1')).toBe(false)
    expect(focusRunCard(panelWith('RUN-1'), '')).toBe(false)
  })
})

describe('requestedRun', () => {
  it('reads ?run= as a string only', () => {
    expect(requestedRun({ run: 'RUN-1' })).toBe('RUN-1')
    expect(requestedRun({ run: ['RUN-1'] })).toBe('')
    expect(requestedRun({})).toBe('')
  })
})
