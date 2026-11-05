import { afterEach, describe, expect, it, vi } from 'vitest'
import * as Y from 'yjs'
import * as encoding from 'lib0/encoding'
import * as syncProtocol from 'y-protocols/sync'
import { FRAGMENT, openCollabRoom, seedClientId, userColor, type CollabRoom } from './collabRoom'

/** A socket that never opens, so a room runs without a relay. */
class SilentSocket {
  static readonly OPEN = 1
  readyState = 0
  binaryType = 'arraybuffer'
  onopen: (() => void) | null = null
  onclose: ((event: { code: number }) => void) | null = null
  onmessage: ((event: { data: ArrayBuffer }) => void) | null = null
  onerror: (() => void) | null = null
  static urls: string[] = []
  static last: SilentSocket | null = null
  constructor(public url: string) {
    SilentSocket.urls.push(url)
    SilentSocket.last = this
  }
  send() {}
  close() {}
}

/**
 * Opens the room's socket and answers with an empty sync step 2, as the
 * relay's first answer does, so the room counts as online.
 */
function goOnline(socket: SilentSocket) {
  socket.readyState = 1
  socket.onopen?.()
  const enc = encoding.createEncoder()
  encoding.writeVarUint(enc, 0)
  syncProtocol.writeSyncStep2(enc, new Y.Doc())
  socket.onmessage?.({ data: encoding.toUint8Array(enc).buffer as ArrayBuffer })
}

const rooms: CollabRoom[] = []

function room(name = 'alice'): CollabRoom {
  const r = openCollabRoom({
    type: 'ticket',
    id: 'TKT-1',
    user: { name, color: 'red' },
    serverUrl: 'ws://relay',
    WebSocketImpl: SilentSocket as unknown as typeof WebSocket,
  })
  rooms.push(r)
  return r
}

/** Delivers every update of each room's doc to the others. */
function exchange(...rs: CollabRoom[]) {
  for (const a of rs) {
    for (const b of rs) {
      if (a !== b) Y.applyUpdate(b.doc, Y.encodeStateAsUpdate(a.doc))
    }
  }
}

function paragraphs(...texts: string[]) {
  return (fragment: Y.XmlFragment) => {
    fragment.insert(
      0,
      texts.map((t) => {
        const p = new Y.XmlElement('paragraph')
        p.insert(0, [new Y.XmlText(t)])
        return p
      })
    )
  }
}

function text(r: CollabRoom): string {
  return r.doc
    .getXmlFragment(FRAGMENT)
    .toArray()
    .map((c) => (c as Y.XmlElement).toArray().join(''))
    .join('|')
}

afterEach(() => {
  for (const r of rooms.splice(0)) r.destroy()
  vi.useRealTimers()
})

describe('collabRoom', () => {
  it('builds the relay URL from the address and build', () => {
    const r = openCollabRoom({
      type: 'ticket',
      id: 'TKT-1@draft',
      user: { name: 'a', color: 'red' },
      build: 'b1',
      epoch: 'e1',
      serverUrl: 'ws://relay',
      WebSocketImpl: SilentSocket as unknown as typeof WebSocket,
    })
    rooms.push(r)
    expect(SilentSocket.urls[SilentSocket.urls.length - 1]).toBe(
      'ws://relay/api/v1/_collab/ticket/TKT-1%40draft?build=b1&epoch=e1'
    )
  })

  it('reads a 4410 as expiry once synced and as a failure before', () => {
    const synced = room()
    const s1 = SilentSocket.last!
    goOnline(s1)
    s1.onclose?.({ code: 4410 })
    expect(synced.status.value).toBe('expired')
    expect(synced.online.value).toBe(false)

    const fresh = room()
    SilentSocket.last!.onclose?.({ code: 4410 })
    expect(fresh.status.value).toBe('failed')

    const stale = room()
    SilentSocket.last!.onclose?.({ code: 4409 })
    expect(stale.status.value).toBe('stale-build')
  })

  it('merges two seeds of the same body into one copy', () => {
    const a = room('a')
    const b = room('b')
    a.seed({ content: 'one\n\ntwo', token: 't1' }, paragraphs('one', 'two'))
    b.seed({ content: 'one\n\ntwo', token: 't1' }, paragraphs('one', 'two'))
    exchange(a, b)
    expect(text(a)).toBe('one|two')
    expect(text(b)).toBe('one|two')
  })

  it('keeps one seed when two clients seed different bodies', () => {
    const a = room('a')
    const b = room('b')
    a.seed({ content: 'old', token: 't0' }, paragraphs('old'))
    b.seed({ content: 'new', token: 't1' }, paragraphs('new'))
    exchange(a, b)
    exchange(a, b)
    expect(text(a)).toBe(text(b))
    expect(['old', 'new']).toContain(text(a))
    expect(a.base.value).toEqual(b.base.value)
    expect(a.base.value?.content).toBe(text(a))
  })

  it('does not seed a room that has content', () => {
    const a = room()
    a.seed({ content: 'x' }, paragraphs('x'))
    a.seed({ content: 'y' }, paragraphs('y'))
    expect(text(a)).toBe('x')
  })

  it("recognizes the room's latest save by its stored and its sent body", () => {
    const a = room('a')
    const b = room('b')
    a.recordSave('old body\n', 't0')
    a.recordSave('stored body\n', 'tok', 'stored  body')
    exchange(a, b)
    expect(b.isCurrentBase('stored body\n')).toBe(true)
    expect(b.isCurrentBase('stored  body')).toBe(true)
    expect(b.isCurrentBase('old body\n')).toBe(false)
    expect(b.isCurrentBase('someone else')).toBe(false)
    expect(b.base.value).toEqual({ content: 'stored body\n', token: 'tok', sent: 'stored  body' })
  })

  it('elects the lowest client id as saver, and only while online', () => {
    const a = room('a')
    const socket = SilentSocket.last!
    const b = room('b')
    expect(a.isSaver.value).toBe(false)
    goOnline(socket)
    expect(a.online.value).toBe(true)
    expect(a.isSaver.value).toBe(true)
    // Tell a about b's awareness state.
    a.awareness.states.set(b.doc.clientID, { user: { name: 'b', color: 'red' } })
    a.awareness.meta.set(b.doc.clientID, { clock: 0, lastUpdated: Date.now() })
    a.awareness.emit('change', [{ added: [b.doc.clientID], updated: [], removed: [] }, 'test'])
    expect(a.isSaver.value).toBe(a.doc.clientID < b.doc.clientID)
  })

  it('derives stable hashes', () => {
    expect(seedClientId('abc')).toBe(seedClientId('abc'))
    expect(seedClientId('abc')).not.toBe(seedClientId('abd'))
    expect(seedClientId('')).not.toBe(0)
  })

  it('reports false from whenSynced once destroyed', async () => {
    const a = room()
    const p = a.whenSynced()
    a.destroy()
    expect(await p).toBe(false)
  })
})

describe('userColor', () => {
  it('is a stable #rrggbb colour, the only format y-prosemirror accepts', () => {
    for (const name of ['alice@example.com', 'bob@example.com', '']) {
      expect(userColor(name)).toMatch(/^#[0-9a-f]{6}$/)
      expect(userColor(name)).toBe(userColor(name))
    }
    expect(userColor('alice@example.com')).not.toBe(userColor('bob@example.com'))
  })
})
