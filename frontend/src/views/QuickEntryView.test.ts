import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { i18n } from '@/i18n'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import QuickEntryView from './QuickEntryView.vue'

vi.mock('@/api/assign', () => ({
  getAssign: vi.fn(async () => ({
    properties: { status: 'inbox' },
    relations: { in: { data: [{ id: 'P-1', type: 'project' }] } },
  })),
}))

const { FormStub } = vi.hoisted(() => ({
  FormStub: {
    name: 'DynamicForm',
    props: ['formId', 'embedded', 'embeddedPrefill'],
    emits: ['inline-created', 'inline-cancelled'],
    template: '<div class="form-stub" />',
  },
}))
vi.mock('@/components/forms/DynamicForm.vue', () => ({ default: FormStub }))

async function mountAt(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div/>' } },
      { path: '/quick', name: 'quick', component: QuickEntryView },
    ],
  })
  await router.push(path)
  const w = mount(QuickEntryView, { global: { plugins: [router, i18n] } })
  await flushPromises()
  return w
}

describe('QuickEntryView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    delete (window as unknown as { wails?: unknown }).wails
  })

  it('passes the list assign defaults as the prefill', async () => {
    useSchemaStore().quickEntry = { form: 'task-form', list: 'inbox' }
    const w = await mountAt('/quick')
    const form = w.findComponent({ name: 'DynamicForm' })
    expect(form.props('formId')).toBe('task-form')
    expect(form.props('embeddedPrefill')).toEqual({
      properties: { status: 'inbox' },
      relations: { in: [{ id: 'P-1', type: 'project' }] },
    })
  })

  it('shows a toast and a fresh form on the web', async () => {
    useSchemaStore().quickEntry = { form: 'task-form' }
    const ui = useUIStore()
    const spy = vi.spyOn(ui, 'success')
    const w = await mountAt('/quick')
    w.findComponent({ name: 'DynamicForm' }).vm.$emit('inline-created', { id: 'T-1' })
    await flushPromises()
    expect(spy).toHaveBeenCalled()
    expect(w.findComponent({ name: 'DynamicForm' }).exists()).toBe(true)
  })

  it('closes the desktop popup on save', async () => {
    useSchemaStore().quickEntry = { form: 'task-form' }
    const close = vi.fn()
    ;(window as unknown as { wails: unknown }).wails = { Window: { Close: close } }
    const w = await mountAt('/quick?popup=1')
    w.findComponent({ name: 'DynamicForm' }).vm.$emit('inline-created', { id: 'T-1' })
    expect(close).toHaveBeenCalled()
  })

  it('warns when the list defaults fail to load', async () => {
    const { getAssign } = await import('@/api/assign')
    vi.mocked(getAssign).mockRejectedValueOnce(new Error('boom'))
    useSchemaStore().quickEntry = { form: 'task-form', list: 'inbox' }
    const spy = vi.spyOn(useUIStore(), 'warning')
    const w = await mountAt('/quick')
    expect(spy).toHaveBeenCalled()
    expect(w.findComponent({ name: 'DynamicForm' }).props('embeddedPrefill')).toBeUndefined()
  })

  it('says so when quick entry is not configured', async () => {
    const w = await mountAt('/quick')
    expect(w.findComponent({ name: 'DynamicForm' }).exists()).toBe(false)
  })
})
