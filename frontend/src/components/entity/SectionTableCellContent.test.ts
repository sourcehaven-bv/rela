import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { prefixResolvedLinks } from '@/router/spaceLinks'
import SectionTableCellContent from './SectionTableCellContent.vue'

function mountIn(space: string | undefined, link: string, values: string[] | null = ['2026-001']) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:rest(.*)*', component: { template: '<div />' } }],
  })
  prefixResolvedLinks(router, () => space)
  return mount(SectionTableCellContent, {
    props: {
      cell: { values, link },
      row: { entityId: 'INV-001', entityType: 'invoice', cells: {} },
    } as never,
    global: { plugins: [router], stubs: { ViewTableCell: true } },
  })
}

describe('SectionTableCellContent', () => {
  it('links into the current space', () => {
    const a = mountIn('portal', '/entity/invoice/INV-001').find('a')
    expect(a.attributes('href')).toBe('/s/portal/entity/invoice/INV-001')
  })

  it('keeps the bare path without spaces', () => {
    const a = mountIn(undefined, '/entity/invoice/INV-001').find('a')
    expect(a.attributes('href')).toBe('/entity/invoice/INV-001')
  })

  it('renders no link for a path that is not same-origin', () => {
    expect(mountIn('portal', '//evil.example/x').find('a').exists()).toBe(false)
  })

  // BUG-6WQ1U8S: an empty anchor is an invisible tab stop, and the stacked
  // phone layout hides the empty cell it sits in.
  it('renders no link for an empty cell', () => {
    expect(mountIn('portal', '/entity/invoice/INV-001', null).find('a').exists()).toBe(false)
    expect(mountIn('portal', '/entity/invoice/INV-001', []).find('a').exists()).toBe(false)
  })
})
