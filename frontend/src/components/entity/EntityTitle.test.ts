// EntityTitle edits the heading in place and emits only a real change.

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import EntityTitle from './EntityTitle.vue'

function mountTitle(props: { editable: boolean; value?: string; required?: boolean }) {
  return mount(EntityTitle, {
    props: { title: 'Shown title', value: props.value ?? 'Raw title', ...props },
    attachTo: document.body,
  })
}

async function editTo(wrapper: ReturnType<typeof mountTitle>, next: string) {
  await wrapper.find('button').trigger('click')
  const input = wrapper.find('input')
  await input.setValue(next)
  await input.trigger('keydown', { key: 'Enter' })
}

describe('EntityTitle', () => {
  it('is a plain heading when not editable', () => {
    const wrapper = mountTitle({ editable: false })
    expect(wrapper.find('h1').text()).toBe('Shown title')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('edits the raw value, not the shown title', async () => {
    const wrapper = mountTitle({ editable: true })
    await wrapper.find('button').trigger('click')
    expect((wrapper.find('input').element as HTMLInputElement).value).toBe('Raw title')
  })

  it('emits the trimmed new value', async () => {
    const wrapper = mountTitle({ editable: true })
    await editTo(wrapper, '  New title ')
    expect(wrapper.emitted('commit')).toEqual([['New title']])
  })

  it('does not emit an unchanged value', async () => {
    const wrapper = mountTitle({ editable: true })
    await editTo(wrapper, 'Raw title ')
    expect(wrapper.emitted('commit')).toBeUndefined()
  })

  it('drops an empty value when the title is required', async () => {
    const wrapper = mountTitle({ editable: true, required: true })
    await editTo(wrapper, '   ')
    expect(wrapper.emitted('commit')).toBeUndefined()
  })

  it('allows clearing an optional title', async () => {
    const wrapper = mountTitle({ editable: true })
    await editTo(wrapper, '')
    expect(wrapper.emitted('commit')).toEqual([['']])
  })
})
