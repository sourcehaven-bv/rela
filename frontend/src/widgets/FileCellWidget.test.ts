import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import FileCellWidget from './FileCellWidget.vue'
import { _setEntityPluralForTest, _resetEntityPluralsForTest } from '@/api/entities'

const path = (name: string) => `attachments/T-1/bijlagen/c54fb91819f593e9/${name}`

function render(modelValue: unknown, context: { entityType?: string; entityId?: string } = {}) {
  return mount(FileCellWidget, {
    props: { modelValue, mode: 'display', propertyName: 'bijlagen', ...context },
  })
}

describe('FileCellWidget', () => {
  beforeEach(() => {
    _resetEntityPluralsForTest()
    _setEntityPluralForTest('taak', 'taken')
  })

  it('renders a paperclip and the file name, with the full name as title', () => {
    const name = 'a-very-long-file-name-that-the-cell-truncates-with-an-ellipsis.pdf'
    const wrapper = render([path(name)], { entityType: 'taak', entityId: 'T-1' })
    const link = wrapper.find('a.file-cell-file')
    expect(wrapper.find('svg.file-cell-icon').exists()).toBe(true)
    expect(link.find('.file-cell-name').text()).toBe(name)
    expect(link.attributes('title')).toBe(name)
    expect(link.attributes('download')).toBe(name)
  })

  it('accepts a single string value', () => {
    const wrapper = render(path('a.pdf'), { entityType: 'taak', entityId: 'T-1' })
    expect(wrapper.find('a').attributes('href')).toBe(
      '/api/v1/taken/T-1/_attachments/bijlagen/a.pdf'
    )
  })

  it('escapes the file name in the download URL', () => {
    const name = 'q2 rapport #1.pdf'
    const wrapper = render([path(name)], { entityType: 'taak', entityId: 'T-1' })
    expect(wrapper.find('a').attributes('href')).toBe(
      `/api/v1/taken/T-1/_attachments/bijlagen/${encodeURIComponent(name)}`
    )
  })

  it('renders names without links when the owning entity is unknown', () => {
    const wrapper = render([path('a.pdf')])
    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.text()).toBe('a.pdf')
  })

  it('renders names without links when the entity type has no plural', () => {
    const wrapper = render([path('a.pdf')], { entityType: 'onbekend', entityId: 'X-1' })
    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.text()).toBe('a.pdf')
  })

  it('renders several files as plain text without the owning entity', () => {
    const names = ['b.pdf', 'a.pdf']
    const wrapper = render(names.map(path))
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.text()).toBe(`${names.length} files`)
    expect(wrapper.find('.file-cell').attributes('title')).toBe([...names].sort().join('\n'))
  })

  it('lets a click through when it renders plain text', async () => {
    const outer = vi.fn()
    const wrapper = mount({
      components: { FileCellWidget },
      template: `<div @click="outer"><FileCellWidget :model-value="value" mode="display" property-name="bijlagen" /></div>`,
      setup: () => ({ outer, value: [path('a.pdf')] }),
    })
    await wrapper.find('.file-cell').trigger('click')
    expect(outer).toHaveBeenCalledOnce()
  })

  it('summarizes several files in a collapsed list', () => {
    const names = ['b.pdf', 'a.pdf', 'c.pdf']
    const wrapper = render(names.map(path), { entityType: 'taak', entityId: 'T-1' })
    const details = wrapper.find('details')
    expect(details.attributes('open')).toBeUndefined()
    expect(details.find('summary').text()).toBe('3 files')
    expect(details.findAll('li a').map((a) => a.attributes('title'))).toEqual([...names].sort())
  })

  it('renders nothing for an empty value', () => {
    const wrapper = render([])
    expect(wrapper.text()).toBe('')
    expect(wrapper.find('.file-cell').exists()).toBe(false)
  })

  it('does not let a click reach an enclosing row link', async () => {
    const outer = vi.fn()
    const wrapper = mount(
      {
        components: { FileCellWidget },
        template: `<div @click="outer"><FileCellWidget :model-value="value" mode="display" property-name="bijlagen" entity-type="taak" entity-id="T-1" /></div>`,
        setup: () => ({ outer, value: [path('a.pdf')] }),
      },
      { attachTo: document.body }
    )
    await wrapper.find('.file-cell').trigger('click')
    expect(outer).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
