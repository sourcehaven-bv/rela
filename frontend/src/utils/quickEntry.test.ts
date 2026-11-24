import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import type { LocationQuery } from 'vue-router'
import { isQuickEntryPopup } from './quickEntry'

describe('isQuickEntryPopup', () => {
  const cases: { name: string; query: LocationQuery; want: boolean }[] = [
    { name: 'quick', query: { popup: '1' }, want: true },
    { name: 'quick', query: {}, want: false },
    { name: 'quick', query: { popup: '0' }, want: false },
    { name: 'list', query: { popup: '1' }, want: false },
  ]
  it.each(cases)('$name $query → $want', ({ name, query, want }) => {
    expect(isQuickEntryPopup({ name, query })).toBe(want)
  })
})

// jsdom cannot mount the whole shell cheaply, so pin the template order: the
// popup branch must come before the app frame that holds RlAppShell (sidebar,
// hamburger), so a popup never renders either.
describe('App.vue popup branch', () => {
  it('renders the popup without RlAppShell', () => {
    const src = readFileSync(resolve(__dirname, '../App.vue'), 'utf8')
    const popup = src.indexOf('v-else-if="quickPopup"')
    const shell = src.indexOf('<RlAppShell')
    expect(popup).toBeGreaterThan(-1)
    expect(popup).toBeLessThan(shell)
    const branch = src.slice(popup, src.indexOf('</div>', popup))
    expect(branch).not.toMatch(/RlAppShell|Sidebar|mobile-menu/)
  })
})
