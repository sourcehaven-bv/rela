import { AsyncLocalStorage } from 'node:async_hooks'

import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref, type Ref } from 'vue'
import * as fc from 'fast-check'
import { diff3Merge } from 'node-diff3'

import { api } from '@/api/client'
import { _setEntityPluralForTest, getEntity } from '@/api/entities'
import type { Entity } from '@/types'
import {
  FakeServer,
  fakeAdapter,
  fixtureSnapshot,
  type FakeRequest,
  type FakeResponse,
} from '@/test/fakeApi'
import * as merge from './autoSaveMerge'
import {
  MAX_CONFLICT_ATTEMPTS,
  conflictMessage,
  contentConflictMessage,
  useAutoSave,
} from './useAutoSave'

// Two tabs autosaving one note through the verified fake (src/test/fakeApi),
// under fc.scheduler(). The scheduler decides when each request reaches the
// server and when its answer reaches its tab, so the two tabs' saves, 412s and
// conflict refetches interleave in every order. The tabs do not refresh each
// other: a tab learns of the other's writes only from its own answers.
//
// Fields are the properties `title` and `summary` and three slots of the body,
// which are separated by a fixed line so that edits to different slots merge
// and edits to one slot conflict. Every value a tab sets is new (`A3`, `B7`),
// so the tab that owns a value is known. Each request is tagged with the tab
// that sent it.
//
// Properties, checked across every run:
//
// - No invented values: each field ends at its initial value or at a value a
//   tab set, and the body keeps its three slots.
// - No blind overwrite: a write the server accepts changes a field only to
//   its sender's own value, and only from a value the sender was shown (its
//   seed, a value applied to its form, its own value, or the value its
//   conflict refetch returned before the conflict was reported).
// - No silent loss: a tab's last value for a field ends on the server, or
//   the server accepted it at some point, or the tab still shows it with an
//   error on that field.
// - No false conflict: a reported conflict has the other tab's value (or a
//   write whose answer was lost) on the server side.

vi.mock('./autoSaveMerge', async (importOriginal) => {
  const m = await importOriginal<typeof import('./autoSaveMerge')>()
  return { ...m, mergeProperty: vi.fn(m.mergeProperty), mergeText: vi.fn(m.mergeText) }
})

// Every save refreshes the git status; that request is noise here.
vi.mock('@/stores/git', () => ({ useGitStore: () => ({ fetchStatus: () => Promise.resolve() }) }))

type Tab = 'A' | 'B'
const TABS: Tab[] = ['A', 'B']
const PROPS = ['title', 'summary'] as const
const SLOTS = 3
const SEP = '\n--\n'
type Field = `p:${(typeof PROPS)[number]}` | `c:${number}`
const FIELDS: Field[] = [
  ...PROPS.map((p) => `p:${p}` as Field),
  ...Array.from({ length: SLOTS }, (_, i) => `c:${i}` as Field),
]
const NOTE = '/notes/NOTE-001'
const PRECONDITION_FAILED = 'https://rela.dev/errors/precondition_failed'

const initial: Record<Field, string> = Object.fromEntries(
  FIELDS.map((f) => [f, `${f.slice(2)}0`])
) as Record<Field, string>
const body = (slots: string[]) => slots.join(SEP)

/** The slots of a body, or null when it no longer has the shape the test writes. */
function slotsOf(text: string): string[] | null {
  const slots = text.split(SEP)
  return slots.length === SLOTS && slots.every((s) => !s.includes('\n')) ? slots : null
}

/** The fields an entity holds, by field name. */
function fieldsOf(e: { properties?: Record<string, unknown>; content?: string }) {
  const out: Partial<Record<Field, string>> = {}
  for (const p of PROPS) {
    const v = e.properties?.[p]
    if (typeof v === 'string') out[`p:${p}`] = v
  }
  const slots = slotsOf(e.content ?? '')
  slots?.forEach((s, i) => (out[`c:${i}`] = s))
  return out
}

const owner = (value: string): Tab | undefined =>
  value[0] === 'A' || value[0] === 'B' ? (value[0] as Tab) : undefined

// `exhaust` answers the next MAX_CONFLICT_ATTEMPTS saves with an empty 412,
// which is every attempt one save gets.
type Fault = 'fail' | 'empty412' | 'exhaust' | 'lost'
type Cmd =
  | { k: 'set'; tab: Tab; prop: (typeof PROPS)[number] }
  | { k: 'type'; tab: Tab; slot: number }
  | { k: 'deliver' }
  | { k: 'advance'; ms: number }
  | { k: 'flush'; tab: Tab }
  | { k: 'fault'; fault: Fault }

const tab = fc.constantFrom<Tab>(...TABS)
const cmd: fc.Arbitrary<Cmd> = fc.oneof(
  {
    weight: 3,
    arbitrary: fc.record({ k: fc.constant('set' as const), tab, prop: fc.constantFrom(...PROPS) }),
  },
  {
    weight: 3,
    arbitrary: fc.record({
      k: fc.constant('type' as const),
      tab,
      slot: fc.nat({ max: SLOTS - 1 }),
    }),
  },
  { weight: 6, arbitrary: fc.constant({ k: 'deliver' } as const) },
  {
    weight: 2,
    arbitrary: fc.record({
      k: fc.constant('advance' as const),
      ms: fc.constantFrom(100, 900, 2000),
    }),
  },
  { weight: 1, arbitrary: fc.record({ k: fc.constant('flush' as const), tab }) },
  {
    weight: 2,
    arbitrary: fc.record({
      k: fc.constant('fault' as const),
      fault: fc.constantFrom<Fault>('fail', 'empty412', 'exhaust', 'lost'),
    }),
  }
)

// Totals across every run of one property, for the floors.
interface Effect {
  accepted: number
  conflicts412: number
  merged: number
  reported: number
  faults: number
  exhausted: number
}

const realAdapter = api.getAxios().defaults.adapter
const actual = await vi.importActual<typeof import('./autoSaveMerge')>('./autoSaveMerge')

afterEach(() => {
  api.getAxios().defaults.adapter = realAdapter
  vi.useRealTimers()
  vi.mocked(merge.mergeProperty).mockImplementation(actual.mergeProperty)
  vi.mocked(merge.mergeText).mockImplementation(actual.mergeText)
})

async function run(s: fc.Scheduler, cmds: Cmd[], effect: Effect): Promise<string[]> {
  const violations: string[] = []
  const server = new FakeServer(fixtureSnapshot)
  server.handle({
    method: 'POST',
    path: '/notes',
    query: {},
    body: {
      properties: { title: initial['p:title'], summary: initial['p:summary'] },
      content: body(FIELDS.filter((f) => f.startsWith('c:')).map((f) => initial[f])),
    },
  })

  // Which tab sent a request: set by each tab's runWrite, carried by every
  // await into the adapter. A request outside a save (the seed) has none.
  const sender = new AsyncLocalStorage<Tab>()
  // Every value each field has held on the server, and the values a tab
  // wrote whose answer it never got.
  const everWritten = new Set<string>()
  const lostValues = new Set<string>()

  interface TabState {
    name: Tab
    shown: Record<Field, Set<string>>
    own: Set<string>
    last: Partial<Record<Field, string>>
    // The fields of the last conflict refetch this tab's save received.
    refetched: Partial<Record<Field, string>> | null
    // The requests of this tab's current save refused with a 412: what they
    // wrote, and the values their preconditions were taken from.
    refused: { ours: Partial<Record<Field, string>>; base: Partial<Record<Field, string>> } | null
    formData: Ref<Record<string, unknown>>
    content: Ref<string>
    autosave: ReturnType<typeof useAutoSave>
    counter: number
  }
  const tabs = {} as Record<Tab, TabState>

  // A write the server accepts must change each field it changes from a
  // value the sender was shown to one the sender owns.
  function checkWrite(by: Tab, req: FakeRequest, before: Partial<Record<Field, string>>) {
    const b = req.body as { properties?: Record<string, unknown>; content?: string }
    const written: Partial<Record<Field, string>> = {}
    for (const p of PROPS)
      if (typeof b.properties?.[p] === 'string') written[`p:${p}`] = b.properties[p] as string
    if (b.content !== undefined) {
      const slots = slotsOf(b.content)
      if (!slots)
        violations.push(`${by} sent a body without its three slots: ${JSON.stringify(b.content)}`)
      slots?.forEach((v, i) => (written[`c:${i}`] = v))
    }
    for (const [f, v] of Object.entries(written) as [Field, string][]) {
      const from = before[f]!
      if (from === v) continue
      if (!tabs[by].own.has(v)) violations.push(`${by} wrote ${f}=${v}, which it does not own`)
      if (!tabs[by].shown[f].has(from))
        violations.push(`${by} overwrote ${f}=${from}, which it was never shown, with ${v}`)
    }
    return written
  }

  const { adapter } = fakeAdapter(server, s, {
    fault(req) {
      const by = sender.getStore()
      if (!by || req.path !== NOTE) return undefined
      const f = pendingFault
      if (req.method === 'PATCH') {
        if (f === 'exhaust') {
          pendingFault = null
          empty412Left = MAX_CONFLICT_ATTEMPTS
        }
        if (f === 'fail' || f === 'empty412' || empty412Left > 0) {
          if (empty412Left > 0) empty412Left--
          else pendingFault = null
          effect.faults++
          if (f === 'fail') return unavailable()
          // A write landed between the server's check and its write: a 412
          // that names no field.
          const versions = current()._versions
          return {
            status: 412,
            data: {
              type: PRECONDITION_FAILED,
              title: 'Entity has been modified',
              status: 412,
              conflicts: {},
              versions,
            },
          }
        }
        const before = fieldsOf(current())
        const res = server.handle(req)
        if (res.status === 412) {
          effect.conflicts412++
          // One save can be refused more than once; the first request to
          // carry a field is the one its base was taken from.
          const r = refusal(req)
          const prev = tabs[by].refused
          tabs[by].refused = prev
            ? { ours: { ...r.ours, ...prev.ours }, base: { ...r.base, ...prev.base } }
            : r
        }
        if (res.status !== 200) return res
        const written = checkWrite(by, req, before)
        remember()
        effect.accepted++
        const values = Object.values(written) as string[]
        if (values.some((v) => owner(v) !== by && owner(v) !== undefined)) effect.merged++
        for (const v of values) everWritten.add(v)
        if (f === 'lost') {
          // Applied, but the answer never arrives.
          pendingFault = null
          effect.faults++
          for (const v of values) lostValues.add(v)
          return unavailable()
        }
        return res
      }
      if (req.method === 'GET') {
        if (f === 'fail') {
          pendingFault = null
          effect.faults++
          return unavailable()
        }
        const res = server.handle(req)
        if (res.status === 200) tabs[by].refetched = fieldsOf(res.data as Entity)
        return res
      }
      return undefined
    },
  })
  let pendingFault: Fault | null = null
  let empty412Left = 0
  // Every value a field has held, by its version token, so the base a
  // precondition names can be read back.
  const byToken = new Map<string, string>()
  function remember() {
    const e = current()
    const fields = fieldsOf(e)
    for (const p of PROPS) {
      const token = e._versions?.properties[p]
      if (token) byToken.set(`p:${p}:${token}`, fields[`p:${p}`]!)
    }
    if (e._versions?.content) byToken.set(`body:${e._versions.content}`, e.content ?? '')
  }
  function refusal(req: FakeRequest) {
    const b = req.body as {
      properties?: Record<string, unknown>
      content?: string
      preconditions?: { properties?: Record<string, string>; content?: string }
    }
    const ours: Partial<Record<Field, string>> = {}
    const base: Partial<Record<Field, string>> = {}
    for (const p of PROPS) {
      if (typeof b.properties?.[p] === 'string') ours[`p:${p}`] = b.properties[p] as string
      const token = b.preconditions?.properties?.[p]
      if (token) base[`p:${p}`] = byToken.get(`p:${p}:${token}`)
    }
    slotsOf(b.content ?? '')?.forEach((v, i) => (ours[`c:${i}`] = v))
    const token = b.preconditions?.content
    slotsOf((token && byToken.get(`body:${token}`)) || '')?.forEach((v, i) => (base[`c:${i}`] = v))
    return { ours, base }
  }
  function current(): Entity {
    return server.handle({ method: 'GET', path: NOTE, query: {} }).data as Entity
  }

  remember()
  // The seed reads run outside the scheduler, as the page load is not what
  // this test is about.
  api.getAxios().defaults.adapter = fakeAdapter(server).adapter
  for (const name of TABS) {
    const pinia = createPinia()
    setActivePinia(pinia)
    const seed = await getEntity('note', 'NOTE-001')
    const formData = ref<Record<string, unknown>>({ ...seed.properties })
    const content = ref(seed.content ?? '')
    const shown = Object.fromEntries(FIELDS.map((f) => [f, new Set<string>()])) as TabState['shown']
    for (const [f, v] of Object.entries(fieldsOf(seed)) as [Field, string][]) shown[f].add(v)
    const state: TabState = {
      name,
      shown,
      own: new Set(),
      last: {},
      refetched: null,
      refused: null,
      formData,
      content,
      counter: 0,
      autosave: null as unknown as ReturnType<typeof useAutoSave>,
    }
    state.autosave = useAutoSave({
      target: { type: 'note', id: 'NOTE-001' },
      initialServerSnapshot: seed,
      formData,
      contentRef: content,
      inverseToCanonical: new Map(),
      buildRelationsBody: () => null,
      disableRelationsChannel: true,
      applyServerProperty: (k, v) => {
        if (v === undefined) delete formData.value[k]
        else formData.value[k] = v
        if (typeof v === 'string' && FIELDS.includes(`p:${k}` as Field))
          shown[`p:${k}` as Field].add(v)
      },
      applyServerContent: (text) => {
        content.value = text
        const slots = slotsOf(text)
        if (!slots)
          violations.push(
            `${name}'s editor was given a body without its three slots: ${JSON.stringify(text)}`
          )
        slots?.forEach((v, i) => shown[`c:${i}`].add(v))
      },
      onError: (msg, info) => {
        if (msg !== conflictMessage() && msg !== contentConflictMessage()) {
          if (info?.status === 412) effect.exhausted++
          return
        }
        effect.reported++
        const theirs = state.refetched
        if (!theirs) {
          violations.push(`${name} reported a conflict without a refetch`)
          return
        }
        // Which fields conflicted: changed by this tab's refused request and
        // by the server, each from the base the request was checked against,
        // to different values.
        const refused = state.refused
        const conflicted: Field[] = []
        if (refused) {
          const fields =
            info?.channel === 'content'
              ? FIELDS.filter((f) => f.startsWith('c:'))
              : info?.property
                ? [`p:${info.property}` as Field]
                : []
          for (const f of fields) {
            const base = refused.base[f]
            const ours = refused.ours[f]
            const v = theirs[f]
            if (base !== undefined && ours !== base && v !== base && ours !== v) conflicted.push(f)
          }
        }
        // The other side of a conflict is the other tab's value, or a write
        // whose answer was lost, never only this tab's own values.
        const other = conflicted.some((f) => {
          const v = theirs[f]!
          return owner(v) !== name || lostValues.has(v)
        })
        if (!other)
          violations.push(
            `${name} reported a conflict against its own values: sent ${JSON.stringify(refused?.ours)} on ${JSON.stringify(refused?.base)}, server ${JSON.stringify(theirs)}`
          )
        for (const [f, v] of Object.entries(theirs) as [Field, string][]) shown[f].add(v)
      },
      runWrite: (send) => {
        state.refused = null
        state.refetched = null
        return sender.run(name, send)
      },
    })
    tabs[name] = state
  }
  api.getAxios().defaults.adapter = adapter

  function newValue(t: TabState, f: Field) {
    const v = `${t.name}${++t.counter}`
    t.own.add(v)
    t.shown[f].add(v)
    t.last[f] = v
    return v
  }

  for (const c of cmds) {
    if (c.k === 'set') {
      const t = tabs[c.tab]
      const v = newValue(t, `p:${c.prop}`)
      t.formData.value[c.prop] = v
      t.autosave.scheduleFieldSave(c.prop, v)
    } else if (c.k === 'type') {
      const t = tabs[c.tab]
      const slots = slotsOf(t.content.value)
      if (!slots) continue
      slots[c.slot] = newValue(t, `c:${c.slot}`)
      t.content.value = body(slots)
      t.autosave.scheduleContentSave(t.content.value)
    } else if (c.k === 'deliver') {
      if (s.count() > 0) await s.waitNext(1)
    } else if (c.k === 'advance') {
      vi.advanceTimersByTime(c.ms)
    } else if (c.k === 'flush') {
      void tabs[c.tab].autosave.commitImmediately(1e9)
    } else {
      pendingFault = c.fault
    }
    await flushPromises()
  }

  // Let both tabs send everything and every answer arrive.
  // A fault still waiting hits one of these final saves.
  for (const t of TABS) void tabs[t].autosave.commitImmediately(1e9)
  for (let round = 0; round < 40; round++) {
    vi.advanceTimersByTime(2000)
    await flushPromises()
    while (s.count() > 0) {
      await s.waitNext(1)
      await flushPromises()
    }
    const busy = TABS.some(
      (t) => tabs[t].autosave.inFlightCount.value + tabs[t].autosave.pendingCount.value > 0
    )
    if (!busy) break
    if (round === 39) violations.push('a tab was still saving after the final drain')
  }

  const final = current()
  const end = fieldsOf(final)
  if (!slotsOf(final.content ?? ''))
    violations.push(`the body lost its slots: ${JSON.stringify(final.content)}`)
  for (const f of FIELDS) {
    const v = end[f]
    if (v !== initial[f] && !(v && owner(v) && everWritten.has(v)))
      violations.push(`${f} ended at ${v}, which no tab wrote`)
  }
  for (const name of TABS) {
    const t = tabs[name]
    for (const [f, v] of Object.entries(t.last) as [Field, string][]) {
      if (end[f] === v || everWritten.has(v)) continue
      const isBody = f.startsWith('c:')
      const showing = isBody
        ? slotsOf(t.content.value)?.[Number(f.slice(2))]
        : t.formData.value[f.slice(2)]
      const error = isBody
        ? t.autosave.contentError.value
        : t.autosave.fieldErrors.value[f.slice(2)]
      if (showing !== v || !error)
        violations.push(
          `${name}'s ${f}=${v} was lost: the server has ${end[f]}, the tab shows ${String(showing)}, error ${JSON.stringify(error ?? null)}`
        )
    }
  }
  // A held field keeps its value only while its error shows; a hold that
  // outlives its error would hide every later server value without a word.
  for (const name of TABS) {
    const t = tabs[name]
    const held = t.autosave.unsavedFields()
    for (const prop of held.props)
      if (!t.autosave.fieldErrors.value[prop])
        violations.push(`${name} holds p:${prop} without an error`)
    if (held.content && !t.autosave.contentError.value)
      violations.push(`${name} holds the body without an error`)
  }
  return violations
}

function unavailable(): FakeResponse {
  return { status: 503, data: { title: 'Service Unavailable', status: 503 } }
}

// Runs the property; `found` collects the violations of the failing runs.
async function property(numRuns: number, found: string[] = []) {
  const effect: Effect = {
    accepted: 0,
    conflicts412: 0,
    merged: 0,
    reported: 0,
    faults: 0,
    exhausted: 0,
  }
  await fc.assert(
    fc.asyncProperty(
      fc.scheduler(),
      fc.array(cmd, { minLength: 10, maxLength: 40 }),
      async (s, cmds) => {
        vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
        vi.spyOn(Math, 'random').mockReturnValue(0.5)
        try {
          const violations = await run(s, cmds, effect)
          found.push(...violations)
          expect(violations).toEqual([])
        } finally {
          vi.useRealTimers()
          vi.mocked(Math.random).mockRestore()
        }
      }
    ),
    { numRuns }
  )
  return effect
}

describe('two tabs autosaving one note', () => {
  _setEntityPluralForTest('note', 'notes')

  it('merges what can be merged and reports the rest', async () => {
    const numRuns = Number(process.env.PROP_RUNS ?? 300)
    const effect = await property(numRuns)
    expect(effect.accepted, 'writes accepted').toBeGreaterThanOrEqual(numRuns * 2)
    expect(effect.conflicts412, '412s answered').toBeGreaterThanOrEqual(numRuns)
    expect(effect.reported, 'conflicts reported').toBeGreaterThanOrEqual(numRuns / 2)
    expect(effect.merged, "writes that carried the other tab's value").toBeGreaterThanOrEqual(
      numRuns / 8
    )
    expect(effect.faults, 'faults that hit a save').toBeGreaterThanOrEqual(numRuns)
    expect(effect.exhausted, 'saves out of attempts').toBeGreaterThanOrEqual(numRuns / 10)
  }, 120_000)

  // The faults the property is for: a merge that writes ours whatever the
  // other tab did.
  it('catches a property merge that always writes ours', async () => {
    vi.mocked(merge.mergeProperty).mockImplementation(() => ({ kind: 'write' }))
    const found: string[] = []
    await expect(property(200, found)).rejects.toThrow()
    expect(found.join('\n')).toMatch(/overwrote p:/)
  }, 120_000)

  it('catches a body merge that always writes ours', async () => {
    vi.mocked(merge.mergeText).mockImplementation((_b, ours) => ({ ok: true, merged: ours }))
    const found: string[] = []
    await expect(property(200, found)).rejects.toThrow()
    expect(found.join('\n')).toMatch(/overwrote c:/)
  }, 120_000)

  it('edits to neighbouring slots merge cleanly', () => {
    const merged = diff3Merge(['A1', '--', 's1'], ['s0', '--', 's1'], ['s0', '--', 'B1'])
    expect(merged).toEqual([{ ok: ['A1', '--', 'B1'] }])
  })
})
