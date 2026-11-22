// Create form on a read-only workspace (BUG-RO4WSP1).
//
// A read-only tenant refuses every non-read request, including the create
// form's dry-run POST. That failure used to mark the staged field set as
// "ready" while it was still empty, which hid every field. The form must keep
// its fields, say the workspace is read-only, and offer no Create button.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { api } from '@/api/client'
import { _setEntityPluralForTest } from '@/api/entities'
import { workspaceReadOnly } from '@/api/workspaceReadOnly'
import DynamicForm from './DynamicForm.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn(),
    back: vi.fn(),
    replace: vi.fn(),
    options: { history: { state: {} } },
  }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/note-form' }),
  onBeforeRouteLeave: vi.fn(),
  onBeforeRouteUpdate: vi.fn(),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

// The dry-run goes through the real API client, so its response interceptor
// sees the server's refusal exactly as in production.
const realAdapter = api.getAxios().defaults.adapter
const posts: string[] = []

function serve(readOnly: boolean): AxiosAdapter {
  return (config: InternalAxiosRequestConfig) => {
    if (config.method === 'post') posts.push(config.url ?? '')
    if (config.method === 'post' && readOnly) {
      const response = {
        data: {
          type: 'https://rela.dev/errors/tenant_read_only',
          title: 'This workspace is read-only',
        },
        status: 403,
        statusText: 'Forbidden',
        headers: {},
        config,
      }
      return Promise.reject(new AxiosError('forbidden', 'ERR_BAD_REQUEST', config, null, response))
    }
    // The dry-run lists the visible properties as keys of `properties`.
    const body = JSON.parse((config.data as string) || '{}') as { properties?: object }
    const properties = { title: '', ...body.properties }
    const data = { properties, _fields: {}, _relations: {}, warnings: [] }
    return Promise.resolve({ data, status: 200, statusText: 'OK', headers: {}, config })
  }
}

const ENTITY_TYPE = {
  name: 'note',
  label: 'Note',
  id_type: 'short',
  properties: { title: { type: 'string' } },
}

const FORM = { id: 'note-form', entity: 'note', fields: [{ property: 'title' }] }

const mounted: VueWrapper[] = []

afterEach(() => {
  const wrappers = mounted.splice(0)
  wrappers.forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
  workspaceReadOnly.value = false
  api.getAxios().defaults.adapter = realAdapter
})

async function mountForm() {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('note', ENTITY_TYPE as never)
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
  return wrapper
}

function createButton(wrapper: VueWrapper) {
  return wrapper.findAll('button').find((b) => b.text().trim().startsWith('Create'))
}

describe('DynamicForm on a read-only workspace', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    _setEntityPluralForTest('note', 'notes')
    posts.length = 0
    workspaceReadOnly.value = false
  })

  it('keeps the fields, says so and hides Create when the dry-run is refused', async () => {
    api.getAxios().defaults.adapter = serve(true)
    const wrapper = await mountForm()
    expect(workspaceReadOnly.value).toBe(true)
    expect(wrapper.find('#field-title').exists()).toBe(true)
    expect(wrapper.find('.workspace-read-only').text()).toContain('This workspace is read-only')
    expect(createButton(wrapper)).toBeUndefined()
  })

  it('sends no create from the keyboard shortcut and no dry-run per keystroke', async () => {
    api.getAxios().defaults.adapter = serve(true)
    const wrapper = await mountForm()
    const before = posts.length
    await wrapper.find('#field-title').setValue('Note')
    await wrapper.find('form').trigger('keydown', { key: 'Enter', metaKey: true })
    await wrapper.find('form').trigger('submit')
    await new Promise((r) => setTimeout(r, 600))
    await flushPromises()
    // Neither a create nor another dry-run reached the server.
    expect(posts.length).toBe(before)
  })

  it('shows Create on a writable workspace', async () => {
    api.getAxios().defaults.adapter = serve(false)
    const wrapper = await mountForm()
    expect(wrapper.find('#field-title').exists()).toBe(true)
    expect(wrapper.find('.workspace-read-only').exists()).toBe(false)
    expect(createButton(wrapper)).toBeTruthy()
  })
})
