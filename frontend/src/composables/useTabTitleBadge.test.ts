import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import type { InboxSummary } from '@/types/channels'
import { badgedTitle, titleCount, useTabTitleBadge } from './useTabTitleBadge'

const summary = (mentions: number, waiting: number): InboxSummary => ({
  mentions,
  waiting,
  waiting_truncated: false,
  unread_channels: 9,
  degraded: false,
})

afterEach(() => {
  document.title = ''
})

describe('useTabTitleBadge', () => {
  it('counts mentions and waiting entries, not unread channels', () => {
    expect(titleCount(undefined)).toBe(0)
    expect(titleCount(summary(2, 1))).toBe(3)
    expect(badgedTitle('rela', 0)).toBe('rela')
    expect(badgedTitle('rela', 3)).toBe('(3) rela')
  })

  it('follows the summary and restores the title on unmount', async () => {
    document.title = 'rela'
    const s = ref<InboxSummary | undefined>(summary(1, 0))
    const w = mount(
      defineComponent({
        setup() {
          useTabTitleBadge(s)
          return () => null
        },
      })
    )
    expect(document.title).toBe('(1) rela')
    s.value = summary(0, 0)
    await nextTick()
    expect(document.title).toBe('rela')
    s.value = undefined
    await nextTick()
    expect(document.title).toBe('rela')
    s.value = summary(4, 2)
    await nextTick()
    expect(document.title).toBe('(6) rela')
    w.unmount()
    expect(document.title).toBe('rela')
  })
})
