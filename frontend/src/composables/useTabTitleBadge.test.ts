import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import type { InboxSummary } from '@/types/channels'
import { titleCount, useTabTitleBadge } from './useTabTitleBadge'
import { TITLE_HOLD_MS, useDocumentTitle } from './useDocumentTitle'

const summary = (mentions: number, waiting: number): InboxSummary => ({
  mentions,
  waiting,
  waiting_truncated: false,
  unread_channels: 9,
  degraded: false,
})

const mounted: VueWrapper[] = []

afterEach(() => {
  vi.useRealTimers()
  mounted.splice(0).forEach((w) => w.unmount())
  document.title = ''
})

describe('useTabTitleBadge', () => {
  it('counts mentions and waiting entries, not unread channels', () => {
    expect(titleCount(undefined)).toBe(0)
    expect(titleCount(summary(2, 1))).toBe(3)
  })

  it('follows the summary and clears the count on unmount', async () => {
    vi.useFakeTimers()
    const s = ref<InboxSummary | undefined>(summary(1, 0))
    const shell = mount(
      defineComponent({
        setup() {
          useDocumentTitle('Portal')
          return () => null
        },
      })
    )
    mounted.push(shell)
    const w = mount(
      defineComponent({
        setup() {
          useTabTitleBadge(s)
          return () => null
        },
      })
    )
    // No page names the tab, so the app name shows after the hold.
    vi.advanceTimersByTime(TITLE_HOLD_MS)
    await nextTick()
    expect(document.title).toBe('(1) Portal')
    s.value = summary(0, 0)
    await nextTick()
    expect(document.title).toBe('Portal')
    s.value = undefined
    await nextTick()
    expect(document.title).toBe('Portal')
    s.value = summary(4, 2)
    await nextTick()
    expect(document.title).toBe('(6) Portal')
    w.unmount()
    await nextTick()
    expect(document.title).toBe('Portal')
  })
})
