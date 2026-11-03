// The fake data-entry API must answer every request in its scope exactly as
// rela-server does. This test runs random request sequences through the
// SPA's own api functions against both and compares the answers and the
// events. Ids and version tokens are deterministic on both sides, so the
// comparison is exact, not by pattern.
//
// Run with `npm run contract`. CONTRACT_UPDATE=1 rewrites the committed
// schema and config snapshot from the live server.

import * as fs from 'node:fs'
import * as http from 'node:http'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

import axios, { type AxiosAdapter } from 'axios'
import * as fc from 'fast-check'
import { beforeAll, describe, expect, inject, it } from 'vitest'

import { api } from '@/api/client'
import { ApiError } from '@/api/errors'
import {
  createEntity,
  deleteEntity,
  getEntity,
  listEntities,
  registerEntityPlurals,
  updateEntity,
  type EntityPatch,
} from '@/api/entities'
import { getMentionedBy } from '@/api/mentions'
import { liveEntityPath, livePresencePath, liveViewPath, putLiveSubscriptions } from '@/api/live'
import { getPresence } from '@/api/presence'
import { fetchView } from '@/api/views'
import {
  FakeServer,
  INERT_ROUTES,
  fakeAdapter,
  fixtureSnapshot,
  type FakeEvent,
} from '@/test/fakeApi'

const here = path.dirname(fileURLToPath(import.meta.url))
const snapshotPath = path.join(here, 'fixture/snapshot.json')
const serverUrl = inject('contractServerUrl')
const TYPE = 'note'
const NUM_RUNS = Number(process.env.CONTRACT_RUNS ?? 200)
// sseFlushInterval in internal/dataentry/watcher.go.
const BATCH_WINDOW_MS = 200

// Talks to the server directly, outside the code under comparison: for the
// snapshot check, the per-run reset and the event stream.
const direct = axios.create({
  baseURL: `${serverUrl}/api/v1`,
  headers: { Origin: serverUrl },
  adapter: 'http',
})

beforeAll(() => {
  registerEntityPlurals(new Map([[TYPE, 'notes']]))
})

describe('fake API snapshot', () => {
  it('serves the schema and config the server serves', async () => {
    const live = {
      schema: (await direct.get('/_schema')).data,
      config: (await direct.get('/_config')).data,
    }
    // The fake imports the snapshot, so this run still compares against the
    // old one; the next run uses what was written.
    if (process.env.CONTRACT_UPDATE === '1') {
      fs.writeFileSync(snapshotPath, JSON.stringify(live, null, 2) + '\n')
      return
    }
    expect(fixtureSnapshot).toEqual(live)
  })

  it.each(INERT_ROUTES.map((r) => [r.method, r.example, r]))(
    'inert %s %s matches the server',
    async (_m, _e, r) => {
      const res = await direct.request({ method: r.method, url: r.example })
      const [path, search] = r.example.split('?')
      const got = new FakeServer(fixtureSnapshot).handle({
        method: r.method,
        path: path!,
        query: Object.fromEntries(new URLSearchParams(search)),
        headers: {},
      })
      expect({ status: got.status, data: got.data }).toEqual({ status: res.status, data: res.data })
    }
  )
})

// ---------------------------------------------------------------------------
// Request sequences

// stale: the oldest token seen, which conflicts only if the field changed
// since. bogus: a token no value hashes to, so the request always conflicts.
type Pre = 'none' | 'fresh' | 'stale' | 'bogus' | 'unwritten'
type LiveKind = 'view' | 'entity' | 'presence'

type Op =
  | { k: 'create'; props: Record<string, unknown>; content?: string }
  | { k: 'get' | 'view' | 'presence' | 'mentions' | 'delete'; ref: number }
  | { k: 'list'; page: number; perPage: number }
  | { k: 'subscribe'; subs: { kind: LiveKind; ref: number }[] }
  // Waits out the server's change batch; see the event check in the run.
  | { k: 'quiet' }
  | {
      k: 'patch'
      ref: number
      props: Record<string, unknown>
      unset: string[]
      content?: string
      pre: Pre
    }

// Strings the server and the fake must store and hash identically. Go's JSON
// encoder escapes <, > and & and the two line separators; JSON.stringify
// does not, so those get generated on purpose. The store refuses NUL in
// property text. Only `summary` and the body get one, because with NUL in two
// properties the server names whichever its map iteration meets first.
const noNUL = (s: string) => !s.includes('\u0000')
const text = fc.oneof(
  fc.string({ maxLength: 12 }).filter(noNUL),
  fc.string({ unit: 'binary', maxLength: 6 }).filter(noNUL),
  fc.constantFrom('', ' ', '<a & b>', 'line\u2028sep\u2029', 'é日本', 'a\r\nb', '"\\')
)
const textOrNUL = fc.oneof(text, fc.constant('a\u0000b'))
const status = fc.constantFrom('open', 'done')
const props = fc.record(
  {
    title: fc.oneof(text, fc.constant(null)),
    status,
    summary: fc.oneof(textOrNUL, fc.constant(null)),
  },
  { requiredKeys: [] }
)
// Most requests address the first two entities, and patches are the most
// common request. A stale precondition needs a read, a write and a second
// write to the same field of the same entity; spread evenly over eight
// entities and six kinds of request, that sequence is too rare to catch a
// fake that never answers 412.
const ref = fc.oneof(
  { weight: 4, arbitrary: fc.nat({ max: 1 }) },
  { weight: 1, arbitrary: fc.nat({ max: 7 }) }
)
const create = fc.record({
  k: fc.constant('create' as const),
  props,
  content: fc.option(textOrNUL, { nil: undefined }),
})
const op: fc.Arbitrary<Op> = fc.oneof(
  create,
  fc.constant({ k: 'quiet' } as const),
  {
    weight: 2,
    arbitrary: fc.record({
      k: fc.constantFrom('get' as const, 'view' as const, 'presence' as const, 'mentions' as const),
      ref,
    }),
  },
  fc.record({ k: fc.constant('delete' as const), ref }),
  fc.record({
    k: fc.constant('subscribe' as const),
    subs: fc.array(
      fc.record({ kind: fc.constantFrom<LiveKind>('view', 'entity', 'presence'), ref }),
      {
        maxLength: 4,
      }
    ),
  }),
  fc.record({
    k: fc.constant('list' as const),
    page: fc.integer({ min: 1, max: 3 }),
    perPage: fc.integer({ min: 1, max: 4 }),
  }),
  {
    weight: 4,
    arbitrary: fc.record({
      k: fc.constant('patch' as const),
      ref,
      props,
      unset: fc.subarray(['title', 'status', 'summary']),
      content: fc.option(textOrNUL, { nil: undefined }),
      pre: fc.constantFrom<Pre>('none', 'fresh', 'stale', 'bogus', 'unwritten'),
    }),
  }
)

type Outcome = { ok: true; data: unknown } | { ok: false; status?: number; data: unknown }

interface Versions {
  properties: Record<string, string>
  content: string
}

// One side of the comparison: where the api client points, the tokens that
// side has seen per entity (oldest first), its event stream, and the live
// subscriptions it holds (key to path).
interface Side {
  name: string
  adapter: AxiosAdapter | 'http'
  baseURL: string
  seen: Map<string, Versions[]>
  events: FakeEvent[]
  conn: string
  subs: Map<string, string>
}

function livePathFor(kind: LiveKind, id: string): string {
  if (kind === 'view') return liveViewPath(TYPE, id)
  if (kind === 'entity') return liveEntityPath(TYPE, id)
  return livePresencePath(TYPE, id)
}

async function run(side: Side, fn: () => Promise<unknown>): Promise<Outcome> {
  const client = api.getAxios()
  client.defaults.adapter = side.adapter
  client.defaults.baseURL = side.baseURL
  try {
    const data = await fn()
    remember(side, data)
    return { ok: true, data }
  } catch (err) {
    if (!(err instanceof ApiError)) throw err
    const res = (err.original as { response?: { data?: unknown } }).response
    remember(side, { versions: (res?.data as { versions?: unknown })?.versions, failed: true })
    return { ok: false, status: err.status, data: res?.data }
  }
}

function remember(side: Side, data: unknown) {
  const d = data as {
    id?: string
    _versions?: Versions
    entry?: { id: string; _versions?: Versions }
  }
  const e = d?.entry ?? d
  if (e?.id && e._versions) {
    const list = side.seen.get(e.id) ?? []
    list.push(e._versions)
    side.seen.set(e.id, list)
  }
}

function buildPatch(side: Side, id: string, o: Extract<Op, { k: 'patch' }>): EntityPatch {
  const patch: EntityPatch = { properties: o.props }
  if (o.unset.length) patch.properties_unset = o.unset.filter((k) => !(k in o.props))
  if (o.content !== undefined) patch.content = o.content
  const seen = side.seen.get(id) ?? []
  const base = o.pre === 'stale' ? seen[0] : seen[seen.length - 1]
  if (o.pre === 'none' || (!base && o.pre !== 'bogus')) return patch
  if (o.pre === 'unwritten') {
    const notWritten = ['title', 'status', 'summary'].find(
      (k) => !(k in o.props) && !patch.properties_unset?.includes(k)
    )
    if (notWritten)
      patch.preconditions = { properties: { [notWritten]: base!.properties[notWritten]! } }
    return patch
  }
  const written = [...Object.keys(o.props), ...(patch.properties_unset ?? [])]
  const pre: { properties?: Record<string, string>; content?: string } = {}
  const token = (t: () => string) => (o.pre === 'bogus' ? '00000000' : t())
  for (const k of written) (pre.properties ??= {})[k] = token(() => base!.properties[k]!)
  if (o.content !== undefined) pre.content = token(() => base!.content)
  if (pre.properties || pre.content !== undefined) patch.preconditions = pre
  return patch
}

async function apply(
  side: Side,
  created: string[],
  o: Exclude<Op, { k: 'quiet' }>,
  at: number
): Promise<Outcome> {
  const idOf = (r: number) => created[r] ?? 'NOTE-999'
  switch (o.k) {
    case 'subscribe': {
      // Keys are new on every PUT, so each subscription's baseline is the
      // answer at its own PUT.
      const subs = o.subs.map((x, i) => ({
        key: `op${at}.${i}`,
        path: livePathFor(x.kind, idOf(x.ref)),
      }))
      const res = await run(side, () => putLiveSubscriptions(side.conn, subs))
      if (res.ok) side.subs = new Map(subs.map((x) => [x.key, x.path]))
      return res
    }
    case 'create':
      return run(side, async () => {
        const e = await createEntity(TYPE, {
          properties: o.props,
          ...(o.content !== undefined ? { content: o.content } : {}),
        })
        if (side.name === 'server') created.push(e.id)
        return e
      })
    case 'get':
      return run(side, () => getEntity(TYPE, idOf(o.ref)))
    case 'view':
      return run(side, () => fetchView(TYPE, idOf(o.ref)))
    case 'presence':
      return run(side, () => getPresence(TYPE, idOf(o.ref)))
    case 'mentions':
      return run(side, () => getMentionedBy(TYPE, idOf(o.ref)))
    case 'delete':
      return run(side, () => deleteEntity(TYPE, idOf(o.ref)))
    case 'list':
      return run(side, () => listEntities(TYPE, { page: o.page, per_page: o.perPage }))
    case 'patch':
      return run(side, () => updateEntity(TYPE, idOf(o.ref), buildPatch(side, idOf(o.ref), o)))
  }
}

// Reads the server's event stream into `into` until `stop` aborts. Uses
// node:http because happy-dom's fetch applies browser CORS rules.
// Resolves with the connection id once the server has sent `ready`.
function streamEvents(into: FakeEvent[], stop: AbortSignal): Promise<string> {
  return new Promise((resolve, reject) => {
    const req = http.get(
      `${serverUrl}/api/v1/_events`,
      { headers: { Origin: serverUrl }, signal: stop },
      (res) => {
        let buf = ''
        res.setEncoding('utf8')
        res.on('data', (chunk: string) => {
          buf += chunk
          let end: number
          while ((end = buf.indexOf('\n\n')) >= 0) {
            const frame = buf.slice(0, end)
            buf = buf.slice(end + 2)
            const event = /^event: (.*)$/m.exec(frame)?.[1]
            const data = /^data: (.*)$/m.exec(frame)?.[1]
            if (event === 'ready') resolve((JSON.parse(data!) as { conn: string }).conn)
            else if (event && event !== 'ping') into.push({ event, data: data && JSON.parse(data) })
          }
        })
      }
    )
    req.on('error', (err) => (stop.aborted ? resolve('') : reject(err)))
  })
}

async function waitFor(cond: () => boolean, ms: number) {
  const deadline = Date.now() + ms
  while (!cond() && Date.now() < deadline) await new Promise((r) => setTimeout(r, 20))
}

// Empties the server of notes and reports the last sequence number it used,
// so the fake can continue from it.
async function resetServer(): Promise<number> {
  const all = (await direct.get('/notes', { params: { per_page: 1000 } })).data as {
    data: { id: string }[]
  }
  for (const e of all.data) await direct.delete(`/notes/${e.id}`)
  const probe = (await direct.post('/notes', { properties: { title: 'probe' } })).data as {
    id: string
  }
  await direct.delete(`/notes/${probe.id}`)
  return Number(probe.id.split('-')[1])
}

describe('fake API against rela-server', () => {
  it('answers every request sequence as the server does', async () => {
    await fc.assert(
      // Each sequence starts with a few creates, so most requests address an
      // entity that exists.
      fc.asyncProperty(
        fc
          .tuple(fc.array(create, { minLength: 1, maxLength: 3 }), fc.array(op, { maxLength: 12 }))
          .map(([first, rest]): Op[] => [...first, ...rest]),
        async (ops) => {
          const last = await resetServer()
          const fake = new FakeServer(fixtureSnapshot, { lastSequence: { [TYPE]: last } })
          const server: Side = {
            name: 'server',
            adapter: 'http',
            baseURL: `${serverUrl}/api/v1`,
            seen: new Map(),
            events: [],
            conn: '',
            subs: new Map(),
          }
          const fakeSide: Side = {
            name: 'fake',
            adapter: fakeAdapter(fake).adapter,
            baseURL: '/api/v1',
            seen: new Map(),
            events: [],
            conn: '',
            subs: new Map(),
          }
          const stream = fake.connect((e) => fakeSide.events.push(e))
          fakeSide.conn = stream.conn
          api.getAxios().defaults.headers.common.Origin = serverUrl

          const stop = new AbortController()
          // Resolves once the server has sent `ready`, so no write below is
          // made before the stream is listening.
          server.conn = await streamEvents(server.events, stop.signal)
          try {
            const created: string[] = []
            // Each live subscription's answer when it was taken, read from the
            // server.
            const initial = new Map<string, string>()
            const read = async (path: string) => {
              const res = await direct.get(path.slice('/api/v1'.length), {
                validateStatus: () => true,
              })
              return JSON.stringify([res.status, res.data])
            }
            // The server batches changes over a 200ms window and sends one
            // entity:changed per type per batch (sseFlushInterval in
            // internal/dataentry/watcher.go), so the count depends on timing.
            // The fake sends one per write, which is what the server sends for
            // writes more than a window apart. What must agree is which events
            // occur between two quiet points: none for a stretch without a
            // successful write. Without the quiet points, the creates every
            // run starts with would hide a PATCH that sends nothing.
            const changed = (side: Side, from: number, to = side.events.length) =>
              [
                ...new Set(
                  side.events
                    .slice(from, to)
                    .filter((e) => e.event === 'entity:changed')
                    .map((e) => JSON.stringify(e))
                ),
              ].sort()
            const marks = [{ server: 0, fake: 0 }]
            const lastMark = () => marks[marks.length - 1]!
            const caughtUp = () =>
              changed(server, lastMark().server).length >= changed(fakeSide, lastMark().fake).length
            for (const [at, o] of ops.entries()) {
              if (o.k === 'quiet') {
                await new Promise((r) => setTimeout(r, BATCH_WINDOW_MS + 100))
                await waitFor(caughtUp, 2000)
                marks.push({ server: server.events.length, fake: fakeSide.events.length })
                continue
              }
              const want = await apply(server, created, o, at)
              const got = await apply(fakeSide, created, o, at)
              expect(got, `${o.k} ${JSON.stringify(o)}`).toEqual(want)
              // A round after every request: the narrowest window, so the fake
              // sends every frame any wider window could.
              fakeSide.events.push(...stream.liveRound())
              if (o.k === 'subscribe' && want.ok) {
                for (const [key, path] of server.subs) initial.set(key, await read(path))
              }
            }

            // Live rounds run per 500ms window on the server, so a value
            // changed and changed back inside one sends nothing there. Two
            // things hold for any window: the server sends no key the fake
            // does not, and a subscription whose answer ends up different from
            // its baseline is reported by both.
            const liveKeys = (side: Side) =>
              new Set(
                side.events
                  .filter((e) => e.event === 'live:changed')
                  .map((e) => (e.data as { key: string }).key)
              )
            const owed: string[] = []
            for (const [key, path] of server.subs) {
              if ((await read(path)) !== initial.get(key)) owed.push(key)
            }
            await waitFor(() => caughtUp() && owed.every((k) => liveKeys(server).has(k)), 2000)
            marks.push({ server: server.events.length, fake: fakeSide.events.length })
            for (let i = 1; i < marks.length; i++) {
              expect(
                changed(fakeSide, marks[i - 1]!.fake, marks[i]!.fake),
                `entity:changed between quiet points ${i - 1} and ${i}`
              ).toEqual(changed(server, marks[i - 1]!.server, marks[i]!.server))
            }
            const fakeKeys = liveKeys(fakeSide)
            expect(
              [...liveKeys(server)].filter((k) => !fakeKeys.has(k)),
              'live frames only the server sent'
            ).toEqual([])
            expect(
              owed.filter((k) => !fakeKeys.has(k)),
              'changed subscriptions the fake missed'
            ).toEqual([])
            expect(
              owed.filter((k) => !liveKeys(server).has(k)),
              'changed subscriptions the server missed'
            ).toEqual([])
          } finally {
            stop.abort()
            stream.close()
          }
        }
      ),
      {
        numRuns: NUM_RUNS,
        endOnFailure: false,
        // A field changed after it was read: the one 412 a random sequence
        // reaches only rarely.
        examples: [
          [
            [
              { k: 'create', props: { title: 't' } },
              { k: 'quiet' },
              { k: 'get', ref: 0 },
              { k: 'patch', ref: 0, props: { status: 'done' }, unset: [], pre: 'none' },
              { k: 'patch', ref: 0, props: { status: 'open' }, unset: [], pre: 'stale' },
            ],
          ],
        ],
      }
    )
  })
})
