import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent } from 'vue'
import RelationDefaultPicker from './RelationDefaultPicker.vue'
import { getEntity } from '@/api'
import { ApiError } from '@/api/errors'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'

vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return { ...actual, getEntity: vi.fn() }
})

const getEntitySpy = getEntity as unknown as ReturnType<typeof vi.fn>

// Stands in for the search modal, so a test can see its props and make it
// emit a choice.
const PickerStub = defineComponent({
  name: 'EntityPickerModal',
  props: ['open', 'types', 'maxResults', 'label'],
  emits: ['close', 'select'],
  template: '<div class="picker-stub" />',
})

function entity(id: string, type: string, title: string): Entity {
  return { id, type, _title: title, properties: {} }
}

function notFound(): ApiError {
  return new ApiError('not found', { kind: 'http', status: 404, original: null })
}

function render(props: { modelValue: string; types: string[] }) {
  return mount(RelationDefaultPicker, {
    props: { label: 'implements', ...props },
    global: { stubs: { EntityPickerModal: PickerStub } },
  })
}

const titleText = (w: ReturnType<typeof render>) => w.find('.relation-default-title').text()
const mark = (w: ReturnType<typeof render>) => w.find('.relation-default-mark')

beforeEach(() => {
  setActivePinia(createPinia())
  getEntitySpy.mockReset()
})

describe('RelationDefaultPicker', () => {
  it('shows None and reads nothing without a saved target', async () => {
    const w = render({ modelValue: '', types: ['feature'] })
    await flushPromises()
    expect(w.text()).toContain('None')
    expect(getEntitySpy).not.toHaveBeenCalled()
  })

  it('shows the saved target title from one entity read', async () => {
    getEntitySpy.mockResolvedValue(entity('F-1', 'feature', 'Login'))
    const w = render({ modelValue: 'F-1', types: ['feature'] })
    await flushPromises()
    expect(getEntitySpy).toHaveBeenCalledTimes(1)
    expect(getEntitySpy).toHaveBeenCalledWith(
      'feature',
      'F-1',
      { world: undefined },
      expect.any(AbortSignal)
    )
    expect(titleText(w)).toBe('Login')
    expect(mark(w).exists()).toBe(false)
  })

  it('does not show the id while the read runs', async () => {
    getEntitySpy.mockReturnValue(new Promise(() => {}))
    const w = render({ modelValue: 'F-1', types: ['feature'] })
    await flushPromises()
    expect(titleText(w)).toBe('')
  })

  it('tries the next target type only after a 404', async () => {
    getEntitySpy
      .mockRejectedValueOnce(notFound())
      .mockResolvedValueOnce(entity('T-1', 'ticket', 'A ticket'))
    const w = render({ modelValue: 'T-1', types: ['epic', 'ticket'] })
    await flushPromises()
    expect(getEntitySpy.mock.calls.map((c) => c[0])).toEqual(['epic', 'ticket'])
    expect(titleText(w)).toBe('A ticket')
  })

  it('tries the type whose id prefix the id carries first', async () => {
    const schema = useSchemaStore()
    schema.entityTypes.set('epic', { name: 'epic', id_prefix: 'EPIC', properties: {} } as never)
    schema.entityTypes.set('ticket', { name: 'ticket', id_prefix: 'TKT', properties: {} } as never)
    getEntitySpy.mockResolvedValue(entity('TKT-1', 'ticket', 'A ticket'))
    const w = render({ modelValue: 'TKT-1', types: ['epic', 'ticket'] })
    await flushPromises()
    expect(getEntitySpy.mock.calls.map((c) => c[0])).toEqual(['ticket'])
    expect(titleText(w)).toBe('A ticket')
  })

  it('marks a target no type answers for, showing its id', async () => {
    getEntitySpy.mockRejectedValue(notFound())
    const w = render({ modelValue: 'F-9', types: ['feature', 'epic'] })
    await flushPromises()
    expect(getEntitySpy).toHaveBeenCalledTimes(2)
    expect(titleText(w)).toBe('F-9')
    expect(mark(w).text()).toBe('not found')
  })

  it('reports a failed read as a failure, not as not found', async () => {
    getEntitySpy.mockRejectedValue(
      new ApiError('boom', { kind: 'http', status: 500, original: null })
    )
    const w = render({ modelValue: 'F-1', types: ['feature', 'epic'] })
    await flushPromises()
    expect(getEntitySpy).toHaveBeenCalledTimes(1)
    expect(titleText(w)).toBe('F-1')
    expect(mark(w).text()).toBe('could not load')
  })

  it('drops a superseded read', async () => {
    let resolveFirst: (e: Entity) => void = () => {}
    getEntitySpy
      .mockReturnValueOnce(new Promise<Entity>((r) => (resolveFirst = r)))
      .mockResolvedValueOnce(entity('F-2', 'feature', 'Second'))
    const w = render({ modelValue: 'F-1', types: ['feature'] })
    await w.setProps({ modelValue: 'F-2' })
    await flushPromises()
    expect(titleText(w)).toBe('Second')
    const firstSignal = getEntitySpy.mock.calls[0][3] as AbortSignal
    expect(firstSignal.aborted).toBe(true)
    resolveFirst(entity('F-1', 'feature', 'First'))
    await flushPromises()
    expect(titleText(w)).toBe('Second')
  })

  it('aborts a read in flight on unmount', async () => {
    getEntitySpy.mockReturnValue(new Promise(() => {}))
    const w = render({ modelValue: 'F-1', types: ['feature'] })
    await flushPromises()
    w.unmount()
    expect((getEntitySpy.mock.calls[0][3] as AbortSignal).aborted).toBe(true)
  })

  it('searches the target types with a limit and emits the chosen id without a read', async () => {
    const w = render({ modelValue: '', types: ['epic', 'ticket'] })
    const picker = w.findComponent(PickerStub)
    expect(picker.props('open')).toBe(false)
    await w.find('button').trigger('click')
    expect(picker.props('open')).toBe(true)
    expect(picker.props('types')).toEqual(['epic', 'ticket'])
    expect(picker.props('maxResults')).toBe(20)

    picker.vm.$emit('select', 'E-1', entity('E-1', 'epic', 'Big epic'))
    expect(w.emitted('update:modelValue')).toEqual([['E-1']])
    await w.setProps({ modelValue: 'E-1' })
    await flushPromises()
    expect(titleText(w)).toBe('Big epic')
    expect(getEntitySpy).not.toHaveBeenCalled()
  })

  it('names the buttons after their visible verb', async () => {
    getEntitySpy.mockResolvedValue(entity('F-1', 'feature', 'Login'))
    const empty = render({ modelValue: '', types: ['feature'] })
    expect(empty.find('button').attributes('aria-label')).toBe(
      'Choose the default target of implements'
    )
    const set = render({ modelValue: 'F-1', types: ['feature'] })
    await flushPromises()
    expect(set.find('button').text()).toBe('Change…')
    expect(set.find('button').attributes('aria-label')).toBe(
      'Change the default target of implements'
    )
    const described = set.find('button').attributes('aria-describedby')
    expect(described).toBeTruthy()
    expect(set.find(`#${described}`).text()).toContain('Login')
  })

  it('offers no search for a relation without target types', () => {
    const w = render({ modelValue: '', types: [] })
    expect(w.find('button').attributes('disabled')).toBeDefined()
    expect(w.findComponent(PickerStub).exists()).toBe(false)
  })

  it('clears the saved target', async () => {
    getEntitySpy.mockResolvedValue(entity('F-1', 'feature', 'Login'))
    const w = render({ modelValue: 'F-1', types: ['feature'] })
    await flushPromises()
    const clear = w.findAll('button').find((b) => b.text() === 'Clear')
    expect(clear).toBeDefined()
    await clear!.trigger('click')
    expect(w.emitted('update:modelValue')).toEqual([['']])
  })
})
