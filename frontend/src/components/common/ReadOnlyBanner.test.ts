import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { i18n } from '@/i18n'
import { workspaceReadOnly } from '@/api/workspaceReadOnly'
import ReadOnlyBanner from './ReadOnlyBanner.vue'

describe('ReadOnlyBanner', () => {
  afterEach(() => {
    workspaceReadOnly.value = false
    document.head.querySelector('meta[name="rela-read-only-text"]')?.remove()
  })

  it('shows only while the workspace is read-only', async () => {
    const wrapper = mount(ReadOnlyBanner, { global: { plugins: [i18n] } })
    expect(wrapper.find('[data-testid="read-only-banner"]').exists()).toBe(false)
    workspaceReadOnly.value = true
    await nextTick()
    expect(wrapper.get('[data-testid="read-only-banner"]').text()).toBe(
      'This workspace is read-only. Creating and editing are turned off.'
    )
  })

  it('shows account.read_only_text from the shell instead of the default', async () => {
    const meta = document.createElement('meta')
    meta.name = 'rela-read-only-text'
    meta.content = 'Your trial has ended. You can still read everything.'
    document.head.appendChild(meta)
    workspaceReadOnly.value = true
    const wrapper = mount(ReadOnlyBanner, { global: { plugins: [i18n] } })
    await nextTick()
    expect(wrapper.get('[data-testid="read-only-banner"]').text()).toBe(
      'Your trial has ended. You can still read everything.'
    )
  })
})
