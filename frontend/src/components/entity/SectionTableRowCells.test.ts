// The read-only cells of a table section row carry what the stacked phone
// layout needs (BUG-6WQ1U8S): a label per cell, the heading class on the
// first, and the classes that leave out empty and `primary: false` cells.
// The layout itself is CSS in EntityDetail, which Vitest does not apply.

import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import SectionTableRowCells from './SectionTableRowCells.vue'
import type { ViewColumn, ViewRow } from '@/api/views'

const COLUMNS: ViewColumn[] = [
  { property: 'invoice_number', label: 'Number', link: 'detail' },
  { property: 'issued', label: 'Issued' },
  { property: 'due', label: 'Due', primary: false },
  { property: 'pdf_url', label: 'PDF' },
]

const ROW: ViewRow = {
  entityId: 'INV-1',
  entityType: 'invoice',
  editFormId: 'invoice',
  cells: [
    { values: ['HM021-0001'], link: '/entity/invoice/INV-1' },
    { values: ['21 Nov 2026'] },
    { values: ['5 Dec 2026'] },
    { values: [] },
  ],
}

function mountRow() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  return mount(
    {
      components: { SectionTableRowCells },
      template: '<table><tbody><tr><SectionTableRowCells v-bind="$attrs" /></tr></tbody></table>',
    },
    {
      attrs: {
        row: ROW,
        columns: COLUMNS,
        editable: false,
        onPropertyApplied: () => {},
        onError: () => {},
      },
      global: { plugins: [router] },
    }
  )
}

describe('SectionTableRowCells stacked layout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('labels each cell with its column name', () => {
    const cells = mountRow().findAll('td:not(.actions-cell)')
    expect(cells.map((c) => c.find('.section-cell__label').text())).toEqual([
      'Number',
      'Issued',
      'Due',
      'PDF',
    ])
    for (const cell of cells) {
      expect(cell.find('.section-cell__label').attributes('aria-hidden')).toBe('true')
      expect(cell.attributes('role')).toBe('cell')
    }
  })

  it('marks the heading, the dropped column and the empty cell', () => {
    const cells = mountRow().findAll('td:not(.actions-cell)')
    expect(cells[0]!.classes()).toContain('section-cell--title')
    for (const name of ['section-cell--title', 'section-cell--secondary', 'section-cell--empty']) {
      expect(cells[1]!.classes()).not.toContain(name)
    }
    expect(cells[2]!.classes()).toContain('section-cell--secondary')
    expect(cells[3]!.classes()).toContain('section-cell--empty')
  })
})
