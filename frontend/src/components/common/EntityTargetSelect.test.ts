import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import EntityTargetSelect from './EntityTargetSelect.vue'
import type { TargetCandidate } from '@/types'

// A candidate whose _title differs from id (the interesting case: the committed
// value must be the bare title, NOT "Title (ID)").
const jeroen: TargetCandidate = { id: 'PERS-JV', _title: 'Jeroen Vloothuis' }
const marc: TargetCandidate = { id: 'PERS-MC', _title: 'Marc' }
const alice: TargetCandidate = { id: 'PERS-AL', _title: 'Alice' }

function manyCandidates(n: number): TargetCandidate[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `PERS-${i}`,
    _title: `Person ${String(i).padStart(3, '0')}`,
  }))
}

describe('EntityTargetSelect — select mode', () => {
  it('renders a <select> with an All option plus one option per candidate', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen, marc], modelValue: '', mode: 'select' },
    })
    const options = wrapper.findAll('option')
    // All + 2 candidates
    expect(options).toHaveLength(3)
    expect(options[0].text()).toBe('All')
    expect(options[0].attributes('value')).toBe('')
  })

  it('option VALUE is the bare title, not "Title (ID)"', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen], modelValue: '', mode: 'select' },
    })
    const opt = wrapper.findAll('option')[1]
    // Value committed to the filter must be the bare title (backend matches it).
    expect(opt.attributes('value')).toBe('Jeroen Vloothuis')
    // Label MAY show the id for disambiguation.
    expect(opt.text()).toBe('Jeroen Vloothuis (PERS-JV)')
  })

  it('emits the bare title when an option is selected', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen, marc], modelValue: '', mode: 'select' },
    })
    const select = wrapper.find('select')
    await select.setValue('Jeroen Vloothuis')
    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual(['Jeroen Vloothuis'])
  })

  it('selecting All emits empty (clears the filter)', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen], modelValue: 'Jeroen Vloothuis', mode: 'select' },
    })
    await wrapper.find('select').setValue('')
    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual([''])
  })

  it('options are sorted by title case-insensitively', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [marc, alice, jeroen], modelValue: '', mode: 'select' },
    })
    const labels = wrapper
      .findAll('option')
      .slice(1)
      .map((o) => o.attributes('value'))
    expect(labels).toEqual(['Alice', 'Jeroen Vloothuis', 'Marc'])
  })

  it('deduplicates candidates that share a display title and drops the ambiguous id', () => {
    const dupe: TargetCandidate = { id: 'PERS-DUP', _title: 'Marc' }
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [marc, dupe], modelValue: '', mode: 'select' },
    })
    // All + one "Marc" (collapsed).
    const options = wrapper.findAll('option')
    expect(options).toHaveLength(2)
    // A collided title matches BOTH entities, so the label must NOT pin one id.
    expect(options[1].text()).toBe('Marc')
    expect(options[1].attributes('value')).toBe('Marc')
  })

  it('renders a placeholder option for a committed value absent from candidates', () => {
    // Deep-linked / externally-set title that is not among the loaded options
    // (beyond the fetch cap, or pre-load). The control must still reflect it as
    // selected rather than silently showing "All" while the filter is applied.
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [marc], modelValue: 'Jeroen Vloothuis', mode: 'select' },
    })
    const select = wrapper.find<HTMLSelectElement>('select')
    // The bound value resolves to a real option (not blank).
    expect(select.element.value).toBe('Jeroen Vloothuis')
    const values = wrapper.findAll('option').map((o) => o.attributes('value'))
    expect(values).toContain('Jeroen Vloothuis')
  })

  it('does not add a placeholder option when the committed value is a known candidate', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [marc, jeroen], modelValue: 'Marc', mode: 'select' },
    })
    // All + Jeroen + Marc = 3; no synthetic duplicate for 'Marc'.
    const marcOptions = wrapper.findAll('option').filter((o) => o.attributes('value') === 'Marc')
    expect(marcOptions).toHaveLength(1)
  })

  it('falls back to id when a candidate has no _title', () => {
    const titleless: TargetCandidate = { id: 'PERS-XX' }
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [titleless], modelValue: '', mode: 'select' },
    })
    const opt = wrapper.findAll('option')[1]
    expect(opt.attributes('value')).toBe('PERS-XX')
    expect(opt.text()).toBe('PERS-XX')
  })

  it('deep-link value binds directly as the selected option', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen, marc], modelValue: 'Jeroen Vloothuis', mode: 'select' },
    })
    const select = wrapper.find<HTMLSelectElement>('select')
    expect(select.element.value).toBe('Jeroen Vloothuis')
  })
})

describe('EntityTargetSelect — typeahead mode', () => {
  it('renders a combobox input (not a native select)', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: manyCandidates(11), modelValue: '', mode: 'typeahead' },
    })
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('input[role="combobox"]').exists()).toBe(true)
  })

  it('typing narrows the visible options', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen, marc, alice], modelValue: '', mode: 'typeahead' },
    })
    const input = wrapper.find('input[role="combobox"]')
    await input.trigger('focus')
    // All + 3 options initially
    expect(wrapper.findAll('.dropdown-item').length).toBe(4)

    await input.setValue('mar')
    // All + just Marc
    const items = wrapper.findAll('.dropdown-item').map((d) => d.text())
    expect(items).toContain('All')
    expect(items).toContain('Marc (PERS-MC)')
    expect(items).not.toContain('Jeroen Vloothuis (PERS-JV)')
  })

  it('clicking an option commits the bare title and closes the dropdown', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen, marc], modelValue: '', mode: 'typeahead' },
    })
    await wrapper.find('input[role="combobox"]').trigger('focus')
    const jeroenItem = wrapper.findAll('.dropdown-item').find((d) => d.text().startsWith('Jeroen'))
    await jeroenItem!.trigger('click')

    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual(['Jeroen Vloothuis'])
    expect(wrapper.find('.dropdown').exists()).toBe(false)
  })

  it('search query is component-local: an external modelValue change does not clobber typing', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen, marc], modelValue: '', mode: 'typeahead' },
    })
    const input = wrapper.find<HTMLInputElement>('input[role="combobox"]')
    await input.trigger('focus')
    await input.setValue('jer')
    // External committed-value update arrives (back/forward nav, SSE).
    await wrapper.setProps({ modelValue: 'Marc' } as Record<string, unknown>)
    // The user's in-progress search must survive.
    expect(input.element.value).toBe('jer')
  })

  it('a partial search string is never committed on click-away', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen], modelValue: '', mode: 'typeahead' },
      attachTo: document.body,
    })
    const input = wrapper.find('input[role="combobox"]')
    await input.trigger('focus')
    await input.setValue('jer')
    // Click outside the widget.
    document.body.click()
    await wrapper.vm.$nextTick()
    // No value was ever emitted from a partial search.
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })

  it('clear button resets the committed value to empty', async () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen], modelValue: 'Jeroen Vloothuis', mode: 'typeahead' },
    })
    const clear = wrapper.find('.clear-selection')
    expect(clear.exists()).toBe(true)
    await clear.trigger('click')
    const emits = wrapper.emitted('update:modelValue')
    expect(emits?.[emits.length - 1]).toEqual([''])
  })

  it('shows the committed title as the input placeholder when a value is set', () => {
    const wrapper = mount(EntityTargetSelect, {
      props: { candidates: [jeroen], modelValue: 'Jeroen Vloothuis', mode: 'typeahead' },
    })
    const input = wrapper.find<HTMLInputElement>('input[role="combobox"]')
    expect(input.attributes('placeholder')).toBe('Jeroen Vloothuis')
  })
})
