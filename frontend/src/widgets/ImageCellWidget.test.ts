import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ImageCellWidget from './ImageCellWidget.vue'
import { _setEntityPluralForTest, _resetEntityPluralsForTest } from '@/api/entities'

const path = (name: string) => `attachments/C-1/logo/c54fb91819f593e9/${name}`

function render(modelValue: unknown, context: { entityType?: string; entityId?: string } = {}) {
  return mount(ImageCellWidget, {
    props: { modelValue, mode: 'display', propertyName: 'logo', ...context },
  })
}

describe('ImageCellWidget', () => {
  beforeEach(() => {
    _resetEntityPluralsForTest()
    _setEntityPluralForTest('company', 'companies')
  })

  it('renders a lazy thumbnail from the row value and address', () => {
    const wrapper = render([path('logo.png')], { entityType: 'company', entityId: 'C-1' })
    const img = wrapper.find('img.image-cell')
    expect(img.attributes('src')).toBe('/api/v1/companies/C-1/_attachments/logo/logo.png')
    expect(img.attributes('loading')).toBe('lazy')
    expect(img.attributes('alt')).toBe('logo.png')
  })

  it('escapes the file name in the URL', () => {
    const wrapper = render([path('my logo #1.png')], { entityType: 'company', entityId: 'C-1' })
    expect(wrapper.find('img').attributes('src')).toBe(
      `/api/v1/companies/C-1/_attachments/logo/${encodeURIComponent('my logo #1.png')}`
    )
  })

  it('is not a link, so a clickable row keeps working', () => {
    const wrapper = render([path('logo.png')], { entityType: 'company', entityId: 'C-1' })
    expect(wrapper.find('a').exists()).toBe(false)
  })

  it('shows the name as text without the owning entity', () => {
    const wrapper = render([path('logo.png')])
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toBe('logo.png')
  })

  it('renders nothing for an empty value', () => {
    const wrapper = render([], { entityType: 'company', entityId: 'C-1' })
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toBe('')
  })
})
