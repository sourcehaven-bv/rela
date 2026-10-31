import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { defineComponent, h, onMounted } from 'vue'
import type * as fc from 'fast-check'

import { api } from '@/api/client'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { FakeServer, fakeAdapter, fakeEventSource, fixtureSnapshot, Gate } from '@/test/fakeApi'
import { fieldToken } from '@/test/fakeApi/tokens'
import EntityView from '@/views/EntityView.vue'
import { useEvents } from '@/composables/useEvents'
import { resetLiveSubscriptions } from '@/composables/useLiveSubscription'

// One user edits an entity's body in place and nobody else writes. Every save
// echoes back as entity:changed, and EntityDetail reloads the view for it
// (onEntityChanged → loadView). That reload is not held back while a save is
// in flight, and landView records the view as the autosave baseline whatever
// happened since the view was read. When the reload reads the entity before
// the next save lands and answers after it, the body's merge base and token go
// back to the older body. The user's next save then carries a stale token, the
// server answers 412, and the merge of (base: old body, ours: the newer typing,
// theirs: the user's own saved body) conflicts: the user is told someone else
// edited the body.

// The Milkdown editor does not run in jsdom. This stand-in reports each change
// as the real one does (update:model-value) and has the flush EntityBody calls.
const EditorStub = defineComponent({
  name: 'MilkdownEditor',
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  setup(props, { emit, expose }) {
    expose({ flush: () => {} })
    return () =>
      h('textarea', {
        'data-testid': 'body-editor',
        value: props.modelValue,
        onInput: (e: Event) => emit('update:modelValue', (e.target as HTMLTextAreaElement).value),
      })
  },
})

const CloseEvents = defineComponent({
  setup() {
    const events = useEvents()
    onMounted(() => events.disconnect())
    return () => null
  },
})

const Blank = defineComponent({ render: () => h('div') })

let wrapper: VueWrapper | null = null
const realAdapter = api.getAxios().defaults.adapter

function cleanup() {
  wrapper?.unmount()
  wrapper = null
  mount(CloseEvents).unmount()
  resetLiveSubscriptions()
  api.getAxios().defaults.adapter = realAdapter
  vi.unstubAllGlobals()
  vi.useRealTimers()
  document.body.innerHTML = ''
}

afterEach(cleanup)

const ID = 'NOTE-001'
const VIEW = `GET /_views/note/${ID}`
const PATCH = `PATCH /notes/${ID}`
const bodyToken = (body: string) => fieldToken(ID, 'note', '\x00content', body, true)

async function mountBody() {
  const server = new FakeServer(fixtureSnapshot)
  const created = server.handle({
    method: 'POST',
    path: '/notes',
    query: {},
    body: { properties: { title: 'Alpha' }, content: 'v0' },
  })
  expect((created.data as { id: string }).id).toBe(ID)

  vi.useFakeTimers({
    toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'Date'],
  })
  const pinia = createPinia()
  setActivePinia(pinia)
  api.getAxios().defaults.adapter = fakeAdapter(server).adapter
  await useSchemaStore().load()

  const gate = new Gate()
  const scheduler = gate as unknown as fc.Scheduler
  const fake = fakeAdapter(server, scheduler)
  api.getAxios().defaults.adapter = fake.adapter
  vi.stubGlobal('EventSource', fakeEventSource(server, scheduler))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/entity/:type/:id', name: 'entity', component: EntityView, props: true },
      { path: '/form/:id/:entityId', name: 'form-edit', component: Blank, props: true },
      { path: '/:rest(.*)*', component: Blank },
    ],
  })
  await router.push(`/entity/note/${ID}`)
  wrapper = mount(defineComponent({ render: () => h(RouterView) }), {
    attachTo: document.body,
    global: {
      plugins: [pinia, PiniaColada, router],
      stubs: { RouterView: false, RouterLink: false, MarkdownEditor: EditorStub },
    },
  })
  await flushPromises()
  await gate.drain()

  // Open the body's inline editor: its explicit edit button.
  const trigger = document.querySelector<HTMLElement>('.rl-inline-edit__edit-button')
  expect(trigger, 'the body has an edit button').not.toBeNull()
  trigger!.click()
  await flushPromises()
  const editor = document.querySelector<HTMLTextAreaElement>('[data-testid="body-editor"]')
  expect(editor, 'the body editor opened').not.toBeNull()

  const type = async (text: string) => {
    editor!.value = text
    editor!.dispatchEvent(new Event('input'))
    await flushPromises()
  }
  return { server, gate, fake, type }
}

describe('EntityDetail body inline edit, one user', () => {
  it('never reports a body conflict when the echo of a save is read before the next save', async () => {
    const { server, gate, fake, type } = await mountBody()
    const ui = useUIStore()
    const errorSpy = vi.spyOn(ui, 'error')

    // 1. Type, pause: the first save goes out and lands.
    await type('v1')
    vi.advanceTimersByTime(800)
    await flushPromises()
    await gate.release(`run ${PATCH}`)
    await gate.release(`deliver ${PATCH}`)
    expect(server.stored('note', ID)?.content).toBe('v1')

    // 2. Its echo arrives (entity:changed and the live view's change), and
    //    the page reloads the view. The user has nothing pending at that
    //    moment, so the reload is sent.
    const beforeReload = fake.requests.length
    await gate.release('sse entity:changed')
    vi.advanceTimersByTime(250)
    await flushPromises()
    const reloads = fake.requests.slice(beforeReload).map((r) => `${r.method} ${r.path}`)
    expect(reloads, 'the echo sends a reload of the view').toContain(VIEW)
    expect(
      reloads.filter((r) => r !== VIEW),
      'the echo sends nothing but reloads of the view'
    ).toEqual([])

    // 3. The user types again and the second save goes out while the reload
    //    is still on its way.
    await type('v2')
    vi.advanceTimersByTime(800)
    await flushPromises()

    // 4. The server reads the view (body v1), then applies the save (v2).
    //    The save's answer reaches the client first; the view, which is the
    //    slower request, after it.
    await gate.release(`run ${VIEW}`, 'last')
    await gate.release(`run ${PATCH}`)
    await gate.release(`deliver ${PATCH}`)
    await gate.release(`deliver ${VIEW}`, 'last')
    expect(server.stored('note', ID)?.content).toBe('v2')

    // 5. The user keeps typing in the open editor, from v2.
    await type('v3')
    vi.advanceTimersByTime(800)
    await flushPromises()
    await gate.drain()

    // The user-facing symptom first: the conflict warning.
    expect(
      errorSpy.mock.calls.map((c) => c[0]),
      'no conflict is reported to the only user'
    ).toEqual([])
    // The cause: the third save is guarded by the token of v1, which the late
    // view put back, instead of the token of v2, which the second save got.
    const refused = fake.requests.filter((r) => r.method === 'PATCH' && r.response?.status === 412)
    expect(
      refused.map((r) => (r.body as { preconditions?: unknown }).preconditions),
      `no save carries a stale body token (v1 = ${bodyToken('v1')}, v2 = ${bodyToken('v2')})`
    ).toEqual([])
    expect(server.stored('note', ID)?.content, 'the last typing is saved').toBe('v3')
  })
})
