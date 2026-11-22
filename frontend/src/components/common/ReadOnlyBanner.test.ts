import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { i18n } from '@/i18n'
import { workspaceReadOnly } from '@/api/workspaceReadOnly'
import ReadOnlyBanner from './ReadOnlyBanner.vue'

describe('ReadOnlyBanner', () => {
  afterEach(() => {
    workspaceReadOnly.value = false
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
})
