import { describe, expect, it } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import * as fc from 'fast-check'

import { ApiError } from '@/api/errors'
import { useEntitySession, type EntitySession, type ReadOutcome } from './useEntitySession'

// useEntitySession against a tiny in-memory server (BUG-04V034W). Each entity
// is one counter; a write increments it and answers with the new value, a read
// answers with the value it saw. The scheduler decides when each request runs
// on the server and when its answer arrives, so a read can see the server
// before a write and answer after it. Writes can fail before or after they
// reach the server, a read can wait in `prepare` while writes complete, and
// a held edit can turn into a write.
//
// Properties, checked every time a value lands:
// - it is for the address on screen;
// - it is never older than the last write to that address whose answer has
//   arrived, failed ones included when they reached the server;
// - no read sent after it has landed already.
// And once everything has run, one more invalidation brings the screen up to
// the server.

const ADDRESSES = ['A', 'B']

interface Shown {
  address: string
  version: number
}

// A read's answer, with the order its request was sent in.
interface Answer extends Shown {
  sent: number
}

type WriteFault = 'none' | 'before' | 'after'

type Cmd =
  | { k: 'read'; prepare: boolean }
  | { k: 'refresh' }
  | { k: 'write'; fault: WriteFault }
  | { k: 'invalidate'; full: boolean }
  | { k: 'move'; i: number }
  | { k: 'hold' }
  | { k: 'release' }
  | { k: 'fail'; refuse: boolean }
  | { k: 'step' }

const cmd: fc.Arbitrary<Cmd> = fc.oneof(
  {
    weight: 2,
    arbitrary: fc.record({ k: fc.constant('read' as const), prepare: fc.boolean() }),
  },
  { weight: 1, arbitrary: fc.constant({ k: 'refresh' } as const) },
  {
    weight: 3,
    arbitrary: fc.record({
      k: fc.constant('write' as const),
      fault: fc.constantFrom<WriteFault>('none', 'none', 'none', 'before', 'after'),
    }),
  },
  {
    weight: 2,
    arbitrary: fc.record({ k: fc.constant('invalidate' as const), full: fc.boolean() }),
  },
  {
    weight: 1,
    arbitrary: fc.record({
      k: fc.constant('move' as const),
      i: fc.nat({ max: ADDRESSES.length - 1 }),
    }),
  },
  { weight: 1, arbitrary: fc.constant({ k: 'hold' } as const) },
  { weight: 1, arbitrary: fc.constant({ k: 'release' } as const) },
  { weight: 1, arbitrary: fc.record({ k: fc.constant('fail' as const), refuse: fc.boolean() }) },
  { weight: 6, arbitrary: fc.constant({ k: 'step' } as const) }
)

const failure = (status: number) =>
  new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })

// Steps that took effect, over all runs. A run whose reads never land or
// whose writes never complete passes trivially.
const effect = { landed: 0, written: 0, retried: 0, reloads: 0, prepared: 0 }

async function run(s: fc.Scheduler, cmds: Cmd[]) {
  const server = new Map(ADDRESSES.map((a) => [a, 0]))
  const done = new Map(ADDRESSES.map((a) => [a, 0]))
  const on = ref(ADDRESSES[0]!)
  const holds = ref(false)
  let shown: Shown | null = null
  let failNext: 'refuse' | 'fail' | null = null
  let sends = 0
  let lastLandedSend = 0
  // Assertions inside a callback would surface as unhandled rejections,
  // which fast-check cannot shrink. They are collected and checked per run.
  const violations: string[] = []
  const started: Promise<unknown>[] = []

  async function serverRead(address: string): Promise<Answer> {
    const sent = ++sends
    await s.schedule(Promise.resolve(), `run read ${address}`)
    const fault = failNext
    failNext = null
    if (fault) throw failure(fault === 'refuse' ? 404 : 503)
    const seen = { address, version: server.get(address)!, sent }
    await s.schedule(Promise.resolve(), `deliver read ${address}`)
    return seen
  }

  async function serverWrite(address: string, fault: WriteFault): Promise<Shown> {
    await s.schedule(Promise.resolve(), `run write ${address}`)
    if (fault === 'before') throw failure(503)
    const version = server.get(address)! + 1
    server.set(address, version)
    await s.schedule(Promise.resolve(), `deliver write ${address}`)
    // The answer has arrived: from here on no older value may land. This is
    // before the session counts the write.
    done.set(address, Math.max(done.get(address)!, version))
    if (fault === 'after') throw failure(503)
    return { address, version }
  }

  function land(v: Answer) {
    if (v.address !== on.value)
      violations.push(`landed ${v.address} while ${on.value} is on screen`)
    if (v.version < done.get(v.address)!) {
      violations.push(`landed ${v.address}@${v.version} after write @${done.get(v.address)}`)
    }
    if (v.sent <= lastLandedSend) {
      violations.push(`landed read #${v.sent} after read #${lastLandedSend}`)
    }
    lastLandedSend = v.sent
    shown = { address: v.address, version: v.version }
    effect.landed++
  }

  const scope = effectScope()
  const session: EntitySession = scope.run(() =>
    useEntitySession(() => on.value, {
      holdsWrites: () => holds.value,
      reload: (full) => {
        effect.reloads++
        return read(!full, false)
      },
      onRefused: () => {
        shown = null
      },
    })
  )!

  function read(background: boolean, prepare: boolean): Promise<ReadOutcome<Answer>> {
    const address = on.value
    let attempts = 0
    return session.read(
      () => {
        if (++attempts > 1) effect.retried++
        return serverRead(address)
      },
      {
        land,
        background,
        quiet: background,
        // A step during which writes may complete.
        prepare: prepare
          ? async () => {
              effect.prepared++
              await s.schedule(Promise.resolve(), 'prepare')
              return true
            }
          : undefined,
      }
    )
  }

  function write(fault: WriteFault) {
    const address = on.value
    return session
      .write(() => serverWrite(address, fault))
      .then(
        (v) => {
          effect.written++
          // The page shows its own write's answer, as autosave does.
          if (on.value === address) shown = v
        },
        () => {}
      )
  }

  try {
    for (const c of cmds) {
      switch (c.k) {
        case 'read':
          started.push(read(false, c.prepare))
          break
        case 'refresh':
          started.push(read(true, false))
          break
        case 'write':
          started.push(write(c.fault))
          break
        case 'invalidate':
          session.invalidate(c.full)
          break
        case 'move':
          on.value = ADDRESSES[c.i]!
          break
        case 'hold':
          holds.value = true
          break
        case 'release':
          // A debounce fires: the held edit becomes a write.
          if (holds.value) {
            holds.value = false
            started.push(write('none'))
          }
          break
        case 'fail':
          failNext = c.refuse ? 'refuse' : 'fail'
          break
        case 'step':
          if (s.count() > 0) await s.waitOne()
          break
      }
      await nextTick()
    }

    // Let everything run out, then ask once more: the screen catches up.
    failNext = null
    holds.value = false
    await nextTick()
    await s.waitIdle()
    session.invalidate()
    for (let i = 0; i < 10 && (s.count() > 0 || i === 0); i++) {
      await s.waitIdle()
      await nextTick()
    }
    await Promise.all(started)
    expect(violations).toEqual([])
    expect(s.count(), 'every request has run').toBe(0)
    expect(shown, 'the screen shows the server once everything has run').toEqual({
      address: on.value,
      version: server.get(on.value),
    })
  } finally {
    scope.stop()
  }
}

describe('useEntitySession', () => {
  it('never lands a value older than a completed write, or for another address', async () => {
    const numRuns = Number(process.env.PROP_RUNS ?? 300)
    effect.landed = effect.written = effect.retried = effect.reloads = effect.prepared = 0
    await fc.assert(
      fc.asyncProperty(fc.scheduler(), fc.array(cmd, { maxLength: 40, size: 'medium' }), run),
      { numRuns }
    )
    expect(effect.landed, 'values landed').toBeGreaterThanOrEqual(numRuns)
    expect(effect.written, 'writes completed').toBeGreaterThanOrEqual(numRuns)
    expect(effect.retried, 'reads sent again after a write overtook them').toBeGreaterThan(
      numRuns / 20
    )
    expect(effect.reloads, 'reloads after an invalidation').toBeGreaterThanOrEqual(numRuns)
    expect(effect.prepared, 'reads that waited in prepare').toBeGreaterThanOrEqual(numRuns / 4)
  }, 120_000)
})
