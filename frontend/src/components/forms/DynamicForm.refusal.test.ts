// A validation refusal of a create is an answer, not a fault (BUG-UZGMHIK).
//
// The form shows the server's 422 at its fields and in a toast. It used to
// log the refusal with console.error as well, so an expected answer read as
// an application error in the console.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { api } from '@/api/client'
import { _setEntityPluralForTest } from '@/api/entities'
import DynamicForm from './DynamicForm.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn(),
    back: vi.fn(),
    replace: vi.fn(),
    options: { history: { state: {} } },
  }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/task-form' }),
  onBeforeRouteLeave: vi.fn(),
  onBeforeRouteUpdate: vi.fn(),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

const realAdapter = api.getAxios().defaults.adapter

/** Answers the dry-run, and the create with `status` and a problem body. */
function serve(status: number, fieldErrors = true): AxiosAdapter {
  return (config: InternalAxiosRequestConfig) => {
    const body = JSON.parse((config.data as string) || '{}') as { properties?: object }
    if (config.method === 'post' && !(config.url ?? '').includes('dry_run')) {
      const data =
        status === 422
          ? {
              type: 'https://rela.dev/errors/validation',
              title: 'Validation failed',
              status,
              errors: fieldErrors ? [{ field: 'title', message: 'Title is too short' }] : [],
            }
          : { title: 'Internal error', status }
      const response = { data, status, statusText: '', headers: {}, config }
      return Promise.reject(new AxiosError('refused', 'ERR_BAD_REQUEST', config, null, response))
    }
    const properties = { title: '', ...body.properties }
    const data = { properties, _fields: {}, _relations: {}, warnings: [] }
    return Promise.resolve({ data, status: 200, statusText: 'OK', headers: {}, config })
  }
}

const FORM = { id: 'task-form', entity: 'task', fields: [{ property: 'title' }] }

const mounted: VueWrapper[] = []

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  api.getAxios().defaults.adapter = realAdapter
  vi.restoreAllMocks()
})

async function submitWith(status: number, fieldErrors = true) {
  api.getAxios().defaults.adapter = serve(status, fieldErrors)
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('task', {
    name: 'task',
    label: 'Task',
    id_type: 'short',
    properties: { title: { type: 'string' } },
  } as never)
  schema.loaded = true
  useEntitiesStore()
  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
        HelpModal: true,
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  await wrapper.find('#field-title').setValue('x')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  return wrapper
}

describe('DynamicForm refused create', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    _setEntityPluralForTest('task', 'tasks')
  })

  it('shows a 422 at its field and logs nothing', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    const toast = vi.spyOn(useUIStore(), 'error')
    const wrapper = await submitWith(422)
    expect(wrapper.text()).toContain('Title is too short')
    expect(toast).toHaveBeenCalled()
    expect(logged).not.toHaveBeenCalled()
  })

  it('logs nothing for a 422 without field errors', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    const toast = vi.spyOn(useUIStore(), 'error')
    await submitWith(422, false)
    expect(toast).toHaveBeenCalled()
    expect(logged).not.toHaveBeenCalled()
  })

  it('still logs an unexpected failure', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    await submitWith(500)
    expect(logged).toHaveBeenCalled()
  })
})
