import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import DateWidget from './DateWidget.vue'

function mountEdit() {
  return mount(DateWidget, {
    props: { mode: 'edit', modelValue: '2026-11-24', label: 'Due', propertyName: 'due' },
  })
}

describe('DateWidget phrase box (TKT-ICADEGW)', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('keeps the native picker as the first input', () => {
    const inputs = mountEdit().findAll('input')
    expect(inputs.map((i) => i.attributes('type'))).toEqual(['date', 'text'])
  })

  it('sets the date as a phrase is typed and clears the box on Enter', async () => {
    vi.useFakeTimers({ now: new Date(2026, 10, 24, 12) })
    const w = mountEdit()
    const box = w.find('input[type="text"]')
    await box.setValue('morgen')
    expect(w.emitted('update:modelValue')?.slice(-1)[0]).toEqual(['2026-11-25'])
    const enter = new KeyboardEvent('keydown', { key: 'Enter', cancelable: true })
    box.element.dispatchEvent(enter)
    expect(enter.defaultPrevented).toBe(false)
    await w.vm.$nextTick()
    expect((box.element as HTMLInputElement).value).toBe('')
  })

  it('flags a phrase it does not understand and keeps Enter', async () => {
    const w = mountEdit()
    const box = w.find('input[type="text"]')
    await box.setValue('next banana')
    expect(w.emitted('update:modelValue')).toBeUndefined()
    const enter = new KeyboardEvent('keydown', { key: 'Enter', cancelable: true })
    box.element.dispatchEvent(enter)
    await w.vm.$nextTick()
    expect(enter.defaultPrevented).toBe(true)
    expect(box.attributes('aria-invalid')).toBe('true')
    expect(w.find('[role="status"]').text()).not.toBe('')
  })

  it('puts the earlier value back when a phrase stops parsing', async () => {
    vi.useFakeTimers({ now: new Date(2026, 10, 24, 12) })
    const w = mountEdit()
    const box = w.find('input[type="text"]')
    // Typing fires input events only; setValue would also fire change.
    const type = async (text: string) => {
      ;(box.element as HTMLInputElement).value = text
      await box.trigger('input')
    }
    await type('ma') // Monday, on the way to "maart"
    await type('maa')
    expect(w.emitted('update:modelValue')).toEqual([['2026-11-30'], ['2026-11-24']])
  })

  it('clears the phrase when a date is picked by hand', async () => {
    const w = mountEdit()
    const box = w.find('input[type="text"]')
    await box.setValue('next banana')
    await box.trigger('change')
    await w.find('input[type="date"]').setValue('2026-12-01')
    expect((box.element as HTMLInputElement).value).toBe('')
    expect(box.attributes('aria-invalid')).toBeUndefined()
  })

  it('does nothing for an empty box', async () => {
    const w = mountEdit()
    const box = w.find('input[type="text"]')
    await box.setValue('  ')
    await box.trigger('change')
    expect(box.attributes('aria-invalid')).toBeUndefined()
    expect(w.emitted('update:modelValue')).toBeUndefined()
  })
})
