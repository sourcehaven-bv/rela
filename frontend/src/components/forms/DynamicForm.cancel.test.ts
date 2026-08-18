// Cancel/Back navigation on a form (BUG-ZE4354).
//
// `router.back()` is a browser-history operation with no route target. On a
// form opened cold — pasted URL, bookmark, new tab — there is no in-app entry
// behind it, so back walked out of the SPA entirely. To the user that reads as
// "the Cancel button does nothing".
//
// The signal that distinguishes the two cases is `history.state.back`, which
// vue-router writes on a push and which a full page load leaves null no matter
// how deep the browser's own history is. `window.history.length` cannot be
// used: it counts pre-SPA entries and reads >= 2 even on a fresh open.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'

const push = vi.fn()
const back = vi.fn()

// `historyState` stands in for `router.options.history.state`: null-ish `back`
// means the app was loaded cold at this URL.
let historyState: { back?: string | null } = {}
let routeQuery: Record<string, string> = {}

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push,
    back,
    replace: vi.fn(),
    options: { history: { get state() { return historyState } } },
  }),
  useRoute: () => ({ query: routeQuery, params: {}, path: '/form/ticket-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getTemplates: vi.fn().mockResolvedValue([]),
    dryRunCreateEntity: vi.fn().mockImplementation(async (_t: string, body: unknown) => ({
      properties: (body as { properties?: Record<string, unknown> })?.properties ?? {},
      _fields: {},
      _relations: {},
      warnings: [],
    })),
  }
})

const ENTITY_TYPE = {
  name: 'ticket',
  label: 'Ticket',
  id_type: 'short',
  properties: { title: { type: 'string' } },
}

const FORM = { id: 'ticket-form', entity: 'ticket', fields: [{ property: 'title' }] }

// BUG-2OXEW0: unmount every component, or its in-flight async work logs after
// the file finishes and races vitest's worker teardown.
const mounted: VueWrapper[] = []

afterEach(() => {
  // splice first: if one unmount throws, the rest still tear down and the
  // array cannot leak into the next test.
  const wrappers = mounted.splice(0)
  wrappers.forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
})

async function mountForm(opts: { list?: boolean } = {}) {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('ticket', ENTITY_TYPE as never)
  if (opts.list !== false) {
    schema.lists.set('all_tickets', { entity: 'ticket', title: 'All' } as never)
  }
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

async function clickCancel(wrapper: Awaited<ReturnType<typeof mountForm>>) {
  const btn = wrapper.findAll('button').find((b) => b.text().includes('Cancel'))
  expect(btn, 'Cancel button should render').toBeTruthy()
  await btn!.trigger('click')
  await flushPromises()
}

describe('DynamicForm — Cancel navigation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    historyState = {}
    routeQuery = {}
  })

  it('goes back when the SPA has an in-app entry behind it', async () => {
    // The common path: reached the form from a list or detail page.
    historyState = { back: '/list/all_tickets' }
    await clickCancel(await mountForm())

    expect(back).toHaveBeenCalledTimes(1)
    expect(push).not.toHaveBeenCalled()
  })

  it('pushes the type list instead of leaving the SPA when opened cold', async () => {
    // The bug: no in-app entry, so back() would walk out of the app.
    await clickCancel(await mountForm())

    expect(back).not.toHaveBeenCalled()
    expect(push).toHaveBeenCalledWith('/list/all_tickets')
  })

  it('falls back to the dashboard when the type has no list', async () => {
    await clickCancel(await mountForm({ list: false }))

    expect(back).not.toHaveBeenCalled()
    expect(push).toHaveBeenCalledWith('/')
  })

  it('prefers return_to over history', async () => {
    // The submit path already honours return_to; Cancel not consulting it was
    // the asymmetry at the heart of the bug. An explicit target beats history
    // even when going back would have worked.
    routeQuery = { return_to: '/list/open_tickets' }
    historyState = { back: '/somewhere/else' }
    await clickCancel(await mountForm())

    expect(push).toHaveBeenCalledWith('/list/open_tickets')
    expect(back).not.toHaveBeenCalled()
  })

  it('ignores an unsafe return_to and still stays in the app', async () => {
    // readReturnTo rejects off-origin values; the fallback must then apply
    // rather than the form navigating to an attacker-supplied URL.
    routeQuery = { return_to: 'https://evil.example.com/phish' }
    await clickCancel(await mountForm())

    expect(push).toHaveBeenCalledWith('/list/all_tickets')
    expect(push).not.toHaveBeenCalledWith('https://evil.example.com/phish')
  })
})
