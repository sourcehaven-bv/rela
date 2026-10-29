// The `$user` form default (TKT-YJSRBZ): a create form fills the field with
// the signed-in user's id from `/_me`, including when `/_me` answers after
// the form initialised, without overwriting input or a `prop.*` pre-fill.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { MeResponse } from '@/api/me'
import { dryRunCreateEntity } from '@/api'

let routeQuery: Record<string, string> = {}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  useRoute: () => ({ query: routeQuery, params: {}, path: '/form/task-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

let answerMe: (me: MeResponse) => void = () => {}
const getMe = vi.fn()

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getMe: () => getMe(),
    getTemplates: vi.fn().mockResolvedValue([]),
    dryRunCreateEntity: vi.fn().mockImplementation(async (_t: string, body: unknown) => ({
      properties: {
        owner: '',
        title: '',
        ...((body as { properties?: Record<string, unknown> })?.properties ?? {}),
      },
      _fields: {},
      _relations: {},
      warnings: [],
    })),
  }
})

const FORM = {
  id: 'task-form',
  entity: 'task',
  fields: [
    { property: 'title', label: 'Title' },
    { property: 'owner', label: 'Owner', default: '$user' },
  ],
}

type Form = VueWrapper & { vm: { isDirty: () => boolean; _originalData: () => string } }

function owner(wrapper: VueWrapper): string {
  return (wrapper.find('#field-owner').element as HTMLInputElement).value
}

const mounted: VueWrapper[] = []
afterEach(() => mounted.splice(0).forEach((w) => w.unmount()))

beforeEach(() => {
  routeQuery = {}
  getMe.mockReset()
})

async function mountForm() {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('task', {
    name: 'task',
    label: 'Task',
    id_type: 'sequential',
    properties: { title: { type: 'string' }, owner: { type: 'string' } },
  } as never)
  schema.loaded = true
  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id },
    global: { stubs: { MarkdownEditor: true, AutoSaveIndicator: true, HelpModal: true } },
  })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper as Form
}

function meLater() {
  getMe.mockImplementation(() => new Promise<MeResponse>((resolve) => (answerMe = resolve)))
}

describe('DynamicForm — $user default', () => {
  it('fills the field when /_me answers after the form initialised', async () => {
    meLater()
    const form = await mountForm()
    expect(owner(form)).toBe('')

    answerMe({ user: 'alice' })
    await flushPromises()
    expect(owner(form)).toBe('alice')
    expect(form.vm.isDirty(), 'a late default is not an unsaved change').toBe(false)
  })

  it('fills the field at once when /_me already answered', async () => {
    getMe.mockResolvedValue({ user: 'alice' })
    await mountForm()
    // A second form reads the cached answer during initialisation.
    expect(owner(await mountForm())).toBe('alice')
  })

  it('keeps what the user typed before /_me answered', async () => {
    meLater()
    const form = await mountForm()
    await form.find('#field-owner').setValue('bob')

    answerMe({ user: 'alice' })
    await flushPromises()
    expect(owner(form)).toBe('bob')
  })

  it('lets a prop.* pre-fill win', async () => {
    routeQuery = { 'prop.owner': 'carol' }
    meLater()
    const form = await mountForm()
    answerMe({ user: 'alice' })
    await flushPromises()
    expect(owner(form)).toBe('carol')
  })

  it.each([
    ['answers unknown', () => getMe.mockResolvedValue({ user: 'unknown' })],
    ['fails', () => getMe.mockRejectedValue(new Error('offline'))],
  ])('leaves the field empty when /_me %s', async (_name, arrange) => {
    arrange()
    expect(owner(await mountForm())).toBe('')
  })

  it('does not refill a field the user cleared', async () => {
    meLater()
    const form = await mountForm()
    await form.find('#field-owner').setValue('bob')
    await form.find('#field-owner').setValue('')

    answerMe({ user: 'alice' })
    await flushPromises()
    expect(owner(form)).toBe('')
  })

  it('adds only the late value to the baseline of an edited form', async () => {
    meLater()
    const form = await mountForm()
    const title = form.find('#field-title')
    await title.setValue('draft')

    answerMe({ user: 'alice' })
    await flushPromises()
    expect(owner(form)).toBe('alice')
    expect(form.vm.isDirty()).toBe(true)

    const baseline = JSON.parse(form.vm._originalData()).formData
    expect(baseline.owner, 'the late default alone is not an unsaved change').toBe('alice')
    expect(baseline.title, 'the edit stays unsaved').toBeUndefined()
  })

  it('runs the dry-run again with the late value', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      meLater()
      await mountForm()
      vi.mocked(dryRunCreateEntity).mockClear()

      answerMe({ user: 'alice' })
      await flushPromises()
      await vi.runAllTimersAsync()
      const calls = vi.mocked(dryRunCreateEntity).mock.calls
      const body = calls[calls.length - 1]?.[1] as {
        properties?: Record<string, unknown>
      }
      expect(body?.properties?.owner).toBe('alice')
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not ask /_me for a form without a $user default', async () => {
    const field = FORM.fields[1] as { default?: string }
    const saved = field.default
    field.default = 'literal'
    try {
      await mountForm()
      expect(getMe).not.toHaveBeenCalled()
    } finally {
      field.default = saved
    }
  })
})
