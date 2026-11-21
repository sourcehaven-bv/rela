import { afterEach, describe, expect, it, vi } from 'vitest'
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
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('scrolls to and focuses the named run', () => {
    const root = panelWith('RUN-1', 'RUN-2')
    const card = root.querySelector<HTMLElement>('[data-run-id="RUN-2"]')!
    card.scrollIntoView = vi.fn()

    const stop = focusRunCard(root, 'RUN-2')
    expect(stop).toBeTypeOf('function')
    stop!()
    expect(card.scrollIntoView).toHaveBeenCalledWith({ block: 'start' })
    expect(card.getAttribute('tabindex')).toBe('-1')
    expect(document.activeElement).toBe(card)
  })

  // BUG-UZGMHIK: the step form is the target, not the card around it.
  it('scrolls the step form of the card when it shows one', () => {
    const root = panelWith('RUN-1')
    const card = root.querySelector<HTMLElement>('[data-run-id="RUN-1"]')!
    const step = document.createElement('section')
    step.setAttribute('data-step-anchor', '')
    card.appendChild(step)
    card.scrollIntoView = vi.fn()
    step.scrollIntoView = vi.fn()

    focusRunCard(root, 'RUN-1')!()
    expect(step.scrollIntoView).toHaveBeenCalled()
    expect(card.scrollIntoView).not.toHaveBeenCalled()
  })

  it('reports a run that is not on the panel', () => {
    expect(focusRunCard(panelWith('RUN-1'), 'RUN-9')).toBeNull()
    expect(focusRunCard(null, 'RUN-1')).toBeNull()
    expect(focusRunCard(panelWith('RUN-1'), '')).toBeNull()
  })
})

describe('requestedRun', () => {
  it('reads ?run= as a string only', () => {
    expect(requestedRun({ run: 'RUN-1' })).toBe('RUN-1')
    expect(requestedRun({ run: ['RUN-1'] })).toBe('')
    expect(requestedRun({})).toBe('')
  })
})
