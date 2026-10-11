import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import IconPicker from '../IconPicker.vue'
import { iconNames } from '@/utils/icons'

const optionValues = (wrapper: ReturnType<typeof mount>) =>
  wrapper.findAll('option').map((o) => o.attributes('value'))

describe('IconPicker', () => {
  it('offers only the icons the server accepts, plus default and none', () => {
    const values = optionValues(mount(IconPicker, { props: { modelValue: '' } }))
    expect(values.slice(0, 2)).toEqual(['', 'none'])
    expect(values.slice(2)).toEqual(iconNames())
  })

  it('keeps a stored unknown name selectable, so opening the screen changes nothing', () => {
    const values = optionValues(mount(IconPicker, { props: { modelValue: 'rocketship' } }))
    expect(values).toContain('rocketship')
  })

  it('reports the default as no value', async () => {
    const wrapper = mount(IconPicker, { props: { modelValue: 'list' } })
    await wrapper.find('select').setValue('')
    const events = wrapper.emitted('update:modelValue') ?? []
    expect(events[events.length - 1]).toEqual([undefined])
  })
})
