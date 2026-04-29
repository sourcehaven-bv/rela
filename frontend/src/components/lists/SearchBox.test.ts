import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import SearchBox from './SearchBox.vue'

describe('SearchBox', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces input emits to a single update after the typing pause', async () => {
    const wrapper = mount(SearchBox, { props: { modelValue: '' } })
    const input = wrapper.find<HTMLInputElement>('input[type="search"]')

    input.element.value = 'a'
    await input.trigger('input')
    input.element.value = 'ab'
    await input.trigger('input')
    input.element.value = 'abc'
    await input.trigger('input')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    vi.advanceTimersByTime(250)
    await nextTick()

    const emits = wrapper.emitted('update:modelValue')
    expect(emits).toHaveLength(1)
    expect(emits?.[0]).toEqual(['abc'])
  })

  it('Enter flushes the debounce immediately', async () => {
    const wrapper = mount(SearchBox, { props: { modelValue: '' } })
    const input = wrapper.find<HTMLInputElement>('input[type="search"]')

    input.element.value = 'foo'
    await input.trigger('input')
    await input.trigger('keyup.enter')

    const emits = wrapper.emitted('update:modelValue')
    expect(emits).toHaveLength(1)
    expect(emits?.[0]).toEqual(['foo'])
  })

  it('clear button emits empty value and removes itself', async () => {
    const wrapper = mount(SearchBox, { props: { modelValue: 'seeded' } })
    const clear = wrapper.find('.clear-btn')
    expect(clear.exists()).toBe(true)

    await clear.trigger('click')
    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual([''])
  })

  it('Escape with content clears the box; Escape with empty box blurs', async () => {
    const wrapper = mount(SearchBox, {
      props: { modelValue: 'foo' },
      attachTo: document.body,
    })
    const input = wrapper.find<HTMLInputElement>('input[type="search"]')

    await input.trigger('keydown', { key: 'Escape' })
    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual([''])

    wrapper.unmount()
  })

  it('external value change replaces the buffer when no debounce is pending', async () => {
    const wrapper = mount(SearchBox, { props: { modelValue: 'a' } })
    expect(wrapper.find<HTMLInputElement>('input').element.value).toBe('a')

    await wrapper.setProps({ modelValue: 'b' })
    expect(wrapper.find<HTMLInputElement>('input').element.value).toBe('b')
  })

  it('external value change does NOT clobber in-progress typed input', async () => {
    const wrapper = mount(SearchBox, { props: { modelValue: '' } })
    const input = wrapper.find<HTMLInputElement>('input[type="search"]')

    input.element.value = 'typed'
    await input.trigger('input')
    // External update arrives while debounce is in flight (e.g. URL echo).
    await wrapper.setProps({ modelValue: 'external' })
    // Buffer must stay as the user's typed value.
    expect(input.element.value).toBe('typed')

    vi.advanceTimersByTime(250)
    await nextTick()

    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual(['typed'])
  })
})
