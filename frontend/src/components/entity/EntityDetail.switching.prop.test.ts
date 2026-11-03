import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { createMemoryHistory, createRouter, RouterView, type Router } from 'vue-router'
import { defineComponent, h, onMounted } from 'vue'
import * as fc from 'fast-check'

import { api } from '@/api/client'
import { useSchemaStore } from '@/stores/schema'
import {
  FakeServer,
  fakeAdapter,
  fakeEventSource,
  fixtureSnapshot,
  Gate,
  type FakeRequest,
  type FakeResponse,
} from '@/test/fakeApi'
import EntityView from '@/views/EntityView.vue'
import { useEvents } from '@/composables/useEvents'
import { resetLiveSubscriptions } from '@/composables/useLiveSubscription'

// Switching between entities on one detail page, against the verified fake
// (src/test/fakeApi). The page keeps one EntityDetail across route changes,
// as the list's side panel and the full page both do, so the answer for the
// entity the user left can still arrive after the one they moved to.
//
// The scheduler decides when each request runs and when its answer arrives.
// Two properties:
//
// - Once every answer has arrived, the page shows the entity the route names,
//   and Edit opens that entity. While an answer is still on its way, Edit
//   opens the entity on screen, which is the one the user can see.
// - A title typed while one entity is on screen is saved to that entity or
//   not at all, never to another.
//
// The page is identified by its summary, which every entity has its own of and
// no command changes, so that the title is free to edit.

const TITLES = ['Alpha', 'Bravo', 'Charlie']
const ids = TITLES.map((_, i) => `NOTE-00${i + 1}`)
const summaryOf = (id: string) => `about ${id}`

// Commands both properties use. `deliver` lets the scheduler run one step,
// `advance` moves the clock (debounces, the live subscription's PUT), and
// `fail` makes the next view fetch to reach the server answer 503, as during
// a deploy. Only view fetches fail: a failed save would make the title
// property's expectations depend on autosave's retries.
type Common =
  { k: 'select'; i: number } | { k: 'deliver' } | { k: 'advance'; ms: number } | { k: 'fail' }

const common: fc.WeightedArbitrary<Common>[] = [
  {
    weight: 1,
    arbitrary: fc.record({
      k: fc.constant('select' as const),
      i: fc.nat({ max: TITLES.length - 1 }),
    }),
  },
  { weight: 3, arbitrary: fc.constant({ k: 'deliver' } as const) },
  {
    weight: 1,
    arbitrary: fc.record({ k: fc.constant('advance' as const), ms: fc.constantFrom(100, 900) }),
  },
  { weight: 1, arbitrary: fc.constant({ k: 'fail' } as const) },
]

type Cmd = Common | { k: 'edit' }
const cmd: fc.Arbitrary<Cmd> = fc.oneof(...common, {
  weight: 1,
  arbitrary: fc.constant({ k: 'edit' } as const),
})

type TitleCmd = Common | { k: 'type' } | { k: 'commit' }
const titleCmd: fc.Arbitrary<TitleCmd> = fc.oneof(
  ...common,
  { weight: 2, arbitrary: fc.constant({ k: 'type' } as const) },
  { weight: 2, arbitrary: fc.constant({ k: 'commit' } as const) }
)

let failNextView = false
// A refusal for one request, as when the entity was deleted or access was
// revoked: the status is what a gated read answers.
let loadFault: { path: string; status: number } | null = null

function fault(req: FakeRequest): FakeResponse | undefined {
  if (loadFault && req.method === 'GET' && req.path === loadFault.path) {
    const { status } = loadFault
    loadFault = null
    return { status, data: { title: 'Not Found', status } }
  }
  if (!failNextView || req.method !== 'GET' || !req.path.startsWith('/_views/')) return undefined
  failNextView = false
  return { status: 503, data: { title: 'Service Unavailable', status: 503 } }
}

/** Runs a command both properties share; returns false for any other. */
async function runCommon(c: { k: string }, s: fc.Scheduler, router: Router): Promise<boolean> {
  const sc = c as Common
  if (sc.k === 'select') {
    await router.push(`/entity/note/${ids[sc.i]}`)
  } else if (sc.k === 'deliver') {
    if (s.count() > 0) await s.waitNext(1)
  } else if (sc.k === 'advance') {
    vi.advanceTimersByTime(sc.ms)
  } else if (sc.k === 'fail') {
    failNextView = true
  } else {
    return false
  }
  await flushPromises()
  return true
}

const FormStub = defineComponent({
  props: { id: String, entityId: String },
  render() {
    return h('div', { 'data-testid': 'form-edit' }, this.entityId)
  },
})
const Blank = defineComponent({ render: () => h('div') })

let wrapper: VueWrapper | null = null
const realAdapter = api.getAxios().defaults.adapter

// Runs after every property run, failed ones included: a run that throws
// must not leave its page mounted for the next run, or for the shrinker.
// The event stream and the live subscriptions are module state, shared by
// every page, and outlive an unmount. Each run closes them, so the next run
// opens its own stream on its own fake server.
const CloseEvents = defineComponent({
  setup() {
    const events = useEvents()
    onMounted(() => events.disconnect())
    return () => null
  },
})

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

function seed(): FakeServer {
  const server = new FakeServer(fixtureSnapshot)
  for (const title of TITLES) {
    const res = server.handle({
      method: 'POST',
      path: '/notes',
      query: {},
      body: { properties: { title } },
    })
    const id = (res.data as { id: string }).id
    server.handle({
      method: 'PATCH',
      path: `/notes/${id}`,
      query: {},
      body: { properties: { summary: summaryOf(id) } },
    })
  }
  return server
}

async function mountPage(server: FakeServer, s: fc.Scheduler): Promise<Router> {
  const pinia = createPinia()
  setActivePinia(pinia)
  // The schema loads before the page mounts, as App.vue does, and outside
  // the scheduler: its order is not what this test is about.
  api.getAxios().defaults.adapter = fakeAdapter(server).adapter
  await useSchemaStore().load()

  failNextView = false
  loadFault = null
  api.getAxios().defaults.adapter = fakeAdapter(server, s, { fault }).adapter
  vi.stubGlobal('EventSource', fakeEventSource(server, s))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/entity/:type/:id', name: 'entity', component: EntityView, props: true },
      { path: '/form/:id/:entityId', name: 'form-edit', component: FormStub, props: true },
      { path: '/:rest(.*)*', component: Blank },
    ],
  })
  await router.push(`/entity/note/${ids[0]}`)
  wrapper = mount(defineComponent({ render: () => h(RouterView) }), {
    attachTo: document.body,
    global: {
      plugins: [pinia, PiniaColada, router],
      stubs: { RouterView: false, RouterLink: false },
    },
  })
  await flushPromises()
  return router
}

/** The id of the entity the page shows, read off its summary, or null. */
function onScreen(): string | null {
  const shown = document
    .querySelector('[data-property="summary"] .display-value')
    ?.textContent?.trim()
  return ids.find((id) => summaryOf(id) === shown) ?? null
}

// Edit is a router link styled as a button.
function editButton(): HTMLElement | undefined {
  return [
    ...document.querySelectorAll<HTMLElement>('.entity-detail a, .entity-detail button'),
  ].find((b) => b.querySelector('.rl-button__label')?.textContent?.trim() === 'Edit')
}

// Lets every request and every timer run out. A delivered answer can start a
// debounce, and a timer can send a request, so it takes a few rounds.
async function settle(s: fc.Scheduler) {
  for (let i = 0; i < 5; i++) {
    vi.advanceTimersByTime(2000)
    await s.waitIdle()
    await flushPromises()
  }
  expect(s.count(), 'every request and event has run').toBe(0)
}

// Fakes the clock for autosave's debounce. The scheduler and flushPromises
// run on promises and setImmediate, which stay real.
function fakeClock() {
  vi.useFakeTimers({
    toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'Date'],
  })
}

/** Presses Edit; returns the entity the form opened, or undefined without a button. */
async function pressEdit(router: Router): Promise<string | undefined> {
  const button = editButton()
  if (!button) return undefined
  button.click()
  await flushPromises()
  await router.isReady()
  const route = router.currentRoute.value
  expect(route.name, 'Edit opens the edit form').toBe('form-edit')
  return String(route.params.entityId)
}

describe('EntityDetail switching between entities', () => {
  it('shows and edits the entity the route names once its answer is in', async () => {
    await fc.assert(
      fc.asyncProperty(fc.scheduler(), fc.array(cmd, { maxLength: 16 }), async (s, cmds) => {
        const server = seed()
        fakeClock()
        try {
          const router = await mountPage(server, s)
          let routed = ids[0]!

          for (const c of cmds) {
            if (c.k === 'select') routed = ids[c.i]!
            if (await runCommon(c, s, router)) continue
            const shown = onScreen()
            const opened = await pressEdit(router)
            if (opened === undefined) continue
            expect(opened, 'Edit opens the entity on screen').toBe(shown)
            await router.push(`/entity/note/${routed}`)
            await flushPromises()
          }

          await settle(s)
          // A failed load says so and shows no entity; a failed refresh keeps
          // what is on screen, which must then be the routed entity.
          if (document.querySelector('.rl-status-region--error')) {
            expect(onScreen(), 'a failed load shows no entity').toBeNull()
            expect(editButton(), 'a failed load offers no Edit').toBeUndefined()
            return
          }
          expect(onScreen(), 'the page shows the routed entity').toBe(routed)
          expect(await pressEdit(router), 'Edit opens the routed entity').toBe(routed)
        } finally {
          cleanup()
        }
      }),
      { numRuns: Number(process.env.PROP_RUNS ?? 50) }
    )
  }, 120_000)

  it('saves a title to the entity it was typed on, or not at all', async () => {
    const numRuns = Number(process.env.PROP_RUNS ?? 50)
    // Runs that committed a title while its entity was on screen. A run that
    // commits nothing passes trivially, so too few of them is a failure.
    let committing = 0
    await fc.assert(
      fc.asyncProperty(fc.scheduler(), fc.array(titleCmd, { maxLength: 16 }), async (s, cmds) => {
        const server = seed()
        fakeClock()
        try {
          const router = await mountPage(server, s)
          // The first entity is on screen before the user starts typing.
          await settle(s)
          // Each typed title is unique, so a stored title names the command
          // that typed it.
          const typedOn = new Map<string, string>()
          let open: { input: HTMLInputElement; startedOn: string } | null = null
          // The last title committed while the entity it was typed on was
          // still on screen, per entity: what that entity must end up with.
          const owed = new Map<string, string>()
          // An entity whose title was committed after the page moved on: it
          // may keep its earlier title or take this one.
          const loose = new Map<string, Set<string>>()

          for (const [n, c] of cmds.entries()) {
            if (await runCommon(c, s, router)) continue
            if (c.k === 'type') {
              const startedOn = onScreen()
              const trigger = document.querySelector<HTMLElement>(
                '.entity-title .rl-inline-edit__trigger'
              )
              if (open || !startedOn || !trigger) continue
              trigger.click()
              await flushPromises()
              const input = document.querySelector<HTMLInputElement>('.entity-title input')
              if (!input) continue
              const title = `title ${n}`
              input.value = title
              input.dispatchEvent(new Event('input'))
              typedOn.set(title, startedOn)
              open = { input, startedOn }
            } else if (open) {
              const { input, startedOn } = open
              open = null
              if (!input.isConnected) continue
              const at = onScreen()
              input.dispatchEvent(new FocusEvent('focusout', { bubbles: true }))
              await flushPromises()
              const title = input.value
              if (at === startedOn) {
                owed.set(startedOn, title)
                loose.delete(startedOn)
              } else {
                loose.set(startedOn, (loose.get(startedOn) ?? new Set()).add(title))
              }
            }
          }

          await settle(s)
          if (owed.size > 0) committing++
          for (const id of ids) {
            const stored = String(server.stored('note', id)?.properties.title)
            const typed = typedOn.get(stored)
            if (typed !== undefined) {
              expect(typed, `${id} holds "${stored}", which was typed on ${typed}`).toBe(id)
            }
            const want = owed.get(id)
            if (want !== undefined && !loose.get(id)?.has(stored)) {
              expect(stored, `${id} keeps the title committed on it`).toBe(want)
            }
          }
        } finally {
          cleanup()
        }
      }),
      { numRuns }
    )
    expect(committing, 'runs that committed a title').toBeGreaterThanOrEqual(numRuns / 10)
  }, 120_000)

  // The orders below take about nine steps in a fixed sequence, which random
  // runs rarely reach, so they are pinned here. Each is the order a reviewer
  // found by hand: the live subscription's PUT starts a refresh after the
  // load, and the refresh fails.
  describe('a load and the live refresh that follows it', () => {
    it('shows the entity the load fetched when the later refresh fails', async () => {
      const { gate } = await switchToBravoWithRefresh()
      failNextView = true
      await gate.release('run GET /_views/note/NOTE-002') // the refresh: 503
      await gate.release('deliver GET /_views/note/NOTE-002', 'last') // the refresh
      await gate.release('deliver GET /_views/note/NOTE-002') // the load
      await gate.drain()
      expect(document.querySelector('.rl-status-region--error')).toBeNull()
      expect(onScreen()).toBe(ids[1])
    })

    it('drops the held entity when the load is refused and the refresh fails', async () => {
      const { gate } = await switchToBravoWithRefresh({ loadStatus: 404 })
      failNextView = true
      await gate.release('run GET /_views/note/NOTE-002') // the refresh: 503
      await gate.release('deliver GET /_views/note/NOTE-002', 'last') // the refresh
      await gate.release('deliver GET /_views/note/NOTE-002') // the load: 404
      await gate.drain()
      expect(document.querySelector('.rl-status-region--error')).not.toBeNull()
      expect(onScreen()).toBeNull()
    })
  })
})

/**
 * Opens Alpha, switches to Bravo, and lets the load for Bravo reach the
 * server and the live subscription's PUT answer. The PUT's answer starts a
 * refresh of Bravo, held before it reaches the server, with the load's
 * answer held too.
 */
async function switchToBravoWithRefresh(opts: { loadStatus?: number } = {}) {
  const server = seed()
  fakeClock()
  const gate = new Gate()
  const router = await mountPage(server, gate as unknown as fc.Scheduler)
  await gate.drain()
  expect(onScreen()).toBe(ids[0])

  await router.push(`/entity/note/${ids[1]}`)
  await flushPromises()
  if (opts.loadStatus) {
    loadFault = { path: `/_views/note/${ids[1]}`, status: opts.loadStatus }
  }
  await gate.release('run GET /_views/note/NOTE-002') // the load
  vi.advanceTimersByTime(100) // the subscription PUT goes out
  await flushPromises()
  await gate.release('run PUT /_events/')
  await gate.release('deliver PUT /_events/')
  return { gate, router }
}
