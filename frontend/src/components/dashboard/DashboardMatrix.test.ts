import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import DashboardMatrix from './DashboardMatrix.vue'
import type { DashboardMatrix as Matrix } from '@/types'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
}

const matrix: Matrix = {
  row_property: 'impact',
  column_property: 'likelihood',
  row_title: 'Impact',
  column_title: 'Likelihood',
  rows: [
    { value: 'low', label: 'Low' },
    { value: 'high', label: 'High' },
  ],
  columns: [
    { value: '1', label: '1' },
    { value: '2', label: '2' },
  ],
  cells: [
    { row: 'low', column: '1', count: 1 },
    { row: 'high', column: '2', count: 4, color: 'kit-red-700' },
  ],
  unplaced: 3,
}

function mountMatrix(toned: boolean) {
  return mount(DashboardMatrix, {
    props: { matrix, list: 'risks', title: 'Heat map', toned },
    global: { stubs: { RouterLink: RouterLinkStub, 'router-link': RouterLinkStub } },
  })
}

describe('DashboardMatrix', () => {
  it('links each non-empty cell to the list filtered on both values', () => {
    const cells = mountMatrix(true).findAll('[data-testid="matrix-cell"]')
    expect(cells).toHaveLength(2)
    expect(JSON.parse(cells[1].attributes('data-to') ?? '')).toEqual({
      path: '/list/risks',
      query: { 'filter[impact]': 'high', 'filter[likelihood]': '2' },
    })
    expect(cells[1].attributes('aria-label')).toBe('Impact High, Likelihood 2: 4')
  })

  it('titles the column axis above its values and the row axis beside its values', () => {
    const w = mountMatrix(true)
    const columnTitle = w.get('[data-testid="matrix-column-title"]')
    expect(columnTitle.text()).toBe('Likelihood')
    expect(columnTitle.attributes('colspan')).toBe('2')
    expect(columnTitle.attributes('scope')).toBe('colgroup')
    const rowTitles = w.findAll('[data-testid="matrix-row-title"]')
    expect(rowTitles).toHaveLength(1)
    expect(rowTitles[0].text()).toBe('Impact')
    expect(rowTitles[0].attributes('rowspan')).toBe('2')
    expect(rowTitles[0].attributes('scope')).toBe('rowgroup')
  })

  it('shows the rows in the order the server sent them', () => {
    const labels = mountMatrix(true)
      .findAll('th[scope="row"]')
      .map((th) => th.text())
    expect(labels).toEqual(['Low', 'High'])
  })

  it('paints a toned cell from its style class and leaves an uncoloured one neutral', () => {
    const cells = mountMatrix(true).findAll('[data-testid="matrix-cell"]')
    expect(cells[1].attributes('style')).toContain('var(--rl-kit-red-700)')
    expect(cells[0].attributes('style') ?? '').not.toContain('background')
  })

  it('shades by count without tone_by', () => {
    const cells = mountMatrix(false).findAll('[data-testid="matrix-cell"]')
    expect(cells[0].classes()).toContain('matrix-cell--shaded')
    expect(cells[0].attributes('style')).toContain('--cell-shade: 26%')
  })

  it('reports rows it could not place', () => {
    expect(mountMatrix(true).text()).toContain(
      '3 not on the grid: no Impact or Likelihood, or a value the grid has no place for'
    )
  })
})
