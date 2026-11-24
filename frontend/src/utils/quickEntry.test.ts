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

describe('fitQuickEntryPopup', () => {
  it('sizes the window to the content, capped at the maximum', async () => {
    const { fitQuickEntryPopup, QUICK_ENTRY_POPUP_MAX_HEIGHT } = await import('./quickEntry')
    const calls: [number, number][] = []
    const w = window as unknown as { wails?: unknown }
    w.wails = {
      Window: { SetSize: (a: number, b: number) => (calls.push([a, b]), Promise.resolve()) },
    }
    const observers: (() => void)[] = []
    globalThis.ResizeObserver = class {
      constructor(cb: () => void) {
        observers.push(cb)
      }
      observe() {}
      disconnect() {}
    } as unknown as typeof ResizeObserver
    const el = document.createElement('div')
    let h = 300
    Object.defineProperty(el, 'scrollHeight', { get: () => h })
    const stop = fitQuickEntryPopup(el)
    h = 5000
    observers[0]()
    stop()
    delete w.wails
    expect(calls).toEqual([
      [480, 300],
      [480, QUICK_ENTRY_POPUP_MAX_HEIGHT],
    ])
  })
})
