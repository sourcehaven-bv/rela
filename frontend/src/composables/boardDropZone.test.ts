import { describe, it, expect, afterEach } from 'vitest'
import { zoneAt } from 'rela-components/composables/useBoardDnd'

// Which part of a card a held card is over (TKT-0VBQW0H). A list reorders by
// top and bottom, a wrapping grid by left and right; with `group` the middle
// half of a card is a zone of its own.

function cardIn(display: 'grid' | 'flex'): HTMLElement {
  const parent = document.createElement('div')
  parent.style.display = display
  const card = document.createElement('div')
  parent.appendChild(card)
  document.body.appendChild(parent)
  card.getBoundingClientRect = () =>
    ({ left: 0, top: 0, width: 200, height: 100, right: 200, bottom: 100, x: 0, y: 0 }) as DOMRect
  return card
}

afterEach(() => {
  document.body.replaceChildren()
})

describe('zoneAt', () => {
  it.each([
    ['grid, left quarter', 'grid', 20, 50, true, 'left'],
    ['grid, middle, grouping', 'grid', 100, 50, true, 'center'],
    ['grid, right quarter', 'grid', 180, 50, true, 'right'],
    ['grid, middle-left, no grouping', 'grid', 90, 50, false, 'left'],
    ['grid, middle-right, no grouping', 'grid', 110, 50, false, 'right'],
    ['list, top quarter', 'flex', 100, 10, true, 'top'],
    ['list, middle, grouping', 'flex', 100, 50, true, 'center'],
    ['list, bottom quarter', 'flex', 100, 90, true, 'bottom'],
    ['list, middle-top, no grouping', 'flex', 100, 40, false, 'top'],
  ] as const)('%s', (_name, display, clientX, clientY, group, want) => {
    expect(zoneAt({ clientX, clientY }, cardIn(display), group)).toBe(want)
  })
})
