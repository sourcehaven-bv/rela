import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'
import PageTitle from '@/components/common/PageTitle.vue'
import {
  badgedTitle,
  composeTitle,
  setTitleBadgeCount,
  TITLE_HOLD_MS,
  usePageTitle,
  useDocumentTitle,
} from './useDocumentTitle'
import { usePageFrame, usePageHeader } from './usePageHeader'

const mounted: VueWrapper[] = []

function host(setup: () => void) {
  const w = mount(
    defineComponent({
      setup() {
        setup()
        return () => null
      },
    })
  )
  mounted.push(w)
  return w
}

afterEach(() => {
  vi.useRealTimers()
  while (mounted.length) mounted.pop()!.unmount()
  setTitleBadgeCount(0)
  document.title = ''
})

describe('composeTitle', () => {
  it('puts the page before the app name', () => {
    expect(composeTitle('Orders', 'Harbourmaster')).toBe('Orders · Harbourmaster')
  })

  it('shows one part when the other is empty or the same', () => {
    expect(composeTitle('', 'Harbourmaster')).toBe('Harbourmaster')
    expect(composeTitle('Orders', '')).toBe('Orders')
    expect(composeTitle('Harbourmaster', 'Harbourmaster')).toBe('Harbourmaster')
    expect(composeTitle(' ', ' ')).toBe('rela')
  })

  it('keeps the badge in front', () => {
    expect(badgedTitle(composeTitle('Orders', 'Harbourmaster'), 2)).toBe(
      '(2) Orders · Harbourmaster'
    )
  })
})

describe('useDocumentTitle', () => {
  it('names the tab after the app when no page names it in time', async () => {
    vi.useFakeTimers()
    document.title = 'rela'
    host(() => useDocumentTitle('Harbourmaster'))
    await nextTick()
    expect(document.title).toBe('rela')
    vi.advanceTimersByTime(TITLE_HOLD_MS)
    await nextTick()
    expect(document.title).toBe('Harbourmaster')
  })

  it('goes straight to the page title on first load, without the bare app name', async () => {
    vi.useFakeTimers()
    document.title = 'rela'
    host(() => useDocumentTitle('Harbourmaster'))
    const page = ref<string | undefined>(undefined)
    host(() => usePageTitle(page))
    await nextTick()
    expect(document.title).toBe('rela')
    page.value = 'Invoices'
    await nextTick()
    expect(document.title).toBe('Invoices · Harbourmaster')
  })

  it('keeps the previous page title while the next page loads', async () => {
    vi.useFakeTimers()
    host(() => useDocumentTitle('Harbourmaster'))
    const first = host(() => usePageTitle('Invoices'))
    await nextTick()
    expect(document.title).toBe('Invoices · Harbourmaster')

    first.unmount()
    const next = ref<string | undefined>(undefined)
    host(() => usePageTitle(next))
    await nextTick()
    vi.advanceTimersByTime(TITLE_HOLD_MS - 1)
    await nextTick()
    expect(document.title).toBe('Invoices · Harbourmaster')

    next.value = 'INV-001'
    await nextTick()
    expect(document.title).toBe('INV-001 · Harbourmaster')
    vi.advanceTimersByTime(TITLE_HOLD_MS)
    await nextTick()
    expect(document.title).toBe('INV-001 · Harbourmaster')
  })

  it('follows the app name as it loads', async () => {
    vi.useFakeTimers()
    const name = ref('rela')
    host(() => useDocumentTitle(name))
    name.value = 'Harbourmaster'
    vi.advanceTimersByTime(TITLE_HOLD_MS)
    await nextTick()
    expect(document.title).toBe('Harbourmaster')
  })

  it('takes the newest page title and falls back when it goes', async () => {
    host(() => useDocumentTitle('Harbourmaster'))
    const page = ref('Orders')
    const first = host(() => usePageTitle(page))
    await nextTick()
    expect(document.title).toBe('Orders · Harbourmaster')

    page.value = 'Quotes'
    await nextTick()
    expect(document.title).toBe('Quotes · Harbourmaster')

    const second = host(() => usePageTitle('Search'))
    await nextTick()
    expect(document.title).toBe('Search · Harbourmaster')

    second.unmount()
    await nextTick()
    expect(document.title).toBe('Quotes · Harbourmaster')

    vi.useFakeTimers()
    first.unmount()
    await nextTick()
    expect(document.title).toBe('Quotes · Harbourmaster')
    vi.advanceTimersByTime(TITLE_HOLD_MS)
    await nextTick()
    expect(document.title).toBe('Harbourmaster')
  })

  it('skips a page title that is still empty', async () => {
    host(() => useDocumentTitle('Harbourmaster'))
    host(() => usePageTitle('Orders'))
    host(() => usePageTitle(() => undefined))
    await nextTick()
    expect(document.title).toBe('Orders · Harbourmaster')
  })

  it('prefers the page header the shell paints', async () => {
    host(() => useDocumentTitle('Harbourmaster'))
    host(() => usePageTitle('Search'))
    host(() => usePageHeader().show({ title: 'Ships' }))
    await nextTick()
    expect(document.title).toBe('Ships · Harbourmaster')
  })

  it('prefers a tabbed page title over the header of the view in its tab', async () => {
    host(() => useDocumentTitle('Harbourmaster'))
    host(() =>
      usePageFrame().show({ title: 'Ship', tabs: [], active: 'a', select: () => undefined })
    )
    host(() => usePageHeader().show({ title: 'Crew' }))
    await nextTick()
    expect(document.title).toBe('Ship · Harbourmaster')
  })

  it('refuses a call outside a scope, which could never remove its title', () => {
    expect(() => usePageTitle('Orders')).toThrow(/scope/)
  })

  it('takes the title PageTitle shows', async () => {
    host(() => useDocumentTitle('Harbourmaster'))
    const w = mount(defineComponent({ render: () => h(PageTitle, { title: 'Inbox' }) }))
    mounted.push(w)
    await nextTick()
    expect(document.title).toBe('Inbox · Harbourmaster')
  })
})
