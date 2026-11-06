/**
 * One co-editing room: the shared document of one entity body (TKT-CJL604).
 *
 * The relay (`/api/v1/_collab/{type}/{id}`) only forwards document and
 * awareness updates. It never saves. Saving stays on the ordinary PATCH path,
 * under each client's own ACL and audit, and the room coordinates it:
 *
 * - **One saver.** The connected peer with the lowest awareness client id
 *   saves. The others save only as a fallback, when the document has gone
 *   unsaved for a while (see `useBodyCollab`). The election decides liveness
 *   only: a peer that claims the role and never saves is covered by the
 *   fallback. A disconnected client is never the saver: it cannot see its
 *   peers, and a merge into its copy would duplicate text on reconnect.
 * - **A shared base.** The `rela` map holds the room's latest save: the body
 *   stored, its version token, and the body sent (`base`). A save is checked
 *   against it, so a write from outside the room fails the precondition and
 *   takes the merge path. A 412 whose current body is this latest save is
 *   the room's own write, which the document already holds, so the save
 *   resends the document instead of merging. This deviates from BUG-04V034W
 *   ("a 412 is never read as the client's own write") for the room's latest
 *   save only; see `isCurrentBase`. An older body, such as a version restored
 *   from history, is never taken for the room's own.
 *
 * A room the relay dropped must not be refilled from a client's old
 * document, because a peer may have seeded a new one from the store and the
 * two would merge into duplicated text. A room is dropped when it stays empty
 * for `collabRoomIdle`, so a client that has not heard from the relay for
 * `EXPIRE_MS` stops reconnecting and reports `expired`. A room is also gone
 * after a server restart: the client joins with the server's epoch, and the
 * relay of a new process closes it with `CLOSE_GONE`. In both cases the host
 * saves what it holds and closes the editor.
 */
import { ref, shallowRef, type Ref, type ShallowRef } from 'vue'
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'
import type { Awareness } from 'y-protocols/awareness'
import * as syncProtocol from 'y-protocols/sync'
import * as encoding from 'lib0/encoding'

/** The fragment Milkdown's collab plugin binds (`CollabService.bindDoc`). */
export const FRAGMENT = 'prosemirror'

/** Close code: the client's SPA build differs from the server's. */
export const CLOSE_STALE_BUILD = 4409
/** Close code: the room belongs to an earlier server process. */
export const CLOSE_GONE = 4410

/**
 * How long a client may go without hearing from the relay and still rejoin.
 * The relay keeps an empty room for five minutes (`collabRoomIdle`); the
 * margin covers a peer the relay noticed leaving later than it left.
 */
export const EXPIRE_MS = 3 * 60_000

/**
 * How often a connected client asks the relay for its state. The relay sends
 * nothing to a peer alone in a room, and y-websocket drops a connection that
 * hears nothing for 30 seconds; the answer keeps it open and keeps
 * `lastHeard` current.
 */
export const KEEPALIVE_MS = 10_000

export type CollabStatus = 'connecting' | 'synced' | 'expired' | 'stale-build' | 'failed'

/** The stored body the room's document derives from, with its version token. */
export interface RoomBase {
  content: string
  token?: string
  /** The body the save sent, when the server stored another form of it. */
  sent?: string
}

export interface CollabUser {
  name: string
  color: string
}

export interface CollabRoomOptions {
  type: string
  /** The entity address the page shows, face included when it shows one. */
  id: string
  user: CollabUser
  /** The SPA build id; the relay closes a mismatch with 4409. */
  build?: string
  /** The relay process the page was served by; another closes with 4410. */
  epoch?: string
  /** Defaults to the page's own origin. */
  serverUrl?: string
  /** For tests. */
  WebSocketImpl?: typeof WebSocket
  now?: () => number
}

export interface CollabRoom {
  readonly doc: Y.Doc
  readonly awareness: Awareness
  readonly status: Ref<CollabStatus>
  /** Whether this client is the room's saver. Never while offline. */
  readonly isSaver: Ref<boolean>
  /** Whether this client is connected and synced with the room. */
  readonly online: Ref<boolean>
  /** The room's base, or null before the room has one. */
  readonly base: ShallowRef<RoomBase | null>
  /** Resolves true once synced, false if the room ended first. */
  whenSynced(): Promise<boolean>
  /**
   * Fills an empty room from `base`. `build` writes the document into the
   * fragment it is given. Deterministic: two clients seeding the same body
   * produce the same items, so they merge into one copy.
   */
  seed(base: RoomBase, build: (fragment: Y.XmlFragment) => void): void
  /** Moves the room's base to a body the room wrote. */
  recordSave(stored: string, token: string | undefined, sent?: string): void
  /** Whether `content` is the room's latest save, as stored or as sent. */
  isCurrentBase(content: string): boolean
  destroy(): void
}

/**
 * A co-editing room and the stored body this client loaded, as the editor
 * takes them.
 *
 * `base` seeds an empty room. A room that already has a document keeps it,
 * and the room's own base becomes the editor's baseline.
 */
export interface EditorCollab {
  room: CollabRoom
  base: RoomBase
}

export function openCollabRoom(opts: CollabRoomOptions): CollabRoom {
  const now = opts.now ?? Date.now
  const doc = new Y.Doc()
  // Top-level types only: a nested type two clients create at once keeps
  // one of them and drops the other's contents.
  const meta = doc.getMap<unknown>('rela')
  const seeds = doc.getMap<boolean>('relaSeeds')
  const params: Record<string, string> = {}
  if (opts.build) params.build = opts.build
  if (opts.epoch) params.epoch = opts.epoch
  const provider = new WebsocketProvider(
    `${opts.serverUrl ?? defaultServerUrl()}/api/v1/_collab/${encodeURIComponent(opts.type)}`,
    encodeURIComponent(opts.id),
    doc,
    {
      params,
      // Tabs of one browser go through the relay like everyone else: a
      // BroadcastChannel would carry updates the relay never authorized.
      disableBc: true,
      maxBackoffTime: 2500,
      ...(opts.WebSocketImpl ? { WebSocketPolyfill: opts.WebSocketImpl } : {}),
    }
  )
  const awareness = provider.awareness
  awareness.setLocalStateField('user', opts.user)

  const status = ref<CollabStatus>('connecting')
  const isSaver = ref(false)
  const online = ref(false)
  const base = shallowRef<RoomBase | null>(null)
  let everSynced = false
  let destroyed = false
  const syncWaiters: ((ok: boolean) => void)[] = []

  function settleWaiters(ok: boolean) {
    for (const w of syncWaiters.splice(0)) w(ok)
  }

  function end(next: CollabStatus) {
    if (status.value !== 'connecting' && status.value !== 'synced') return
    status.value = next
    provider.disconnect()
    setOnline()
    settleWaiters(false)
  }

  function setOnline() {
    online.value =
      !destroyed &&
      (status.value === 'connecting' || status.value === 'synced') &&
      provider.wsconnected &&
      provider.synced
    electSaver()
  }

  function lastHeard(): number {
    return provider.wsLastMessageReceived
  }

  function expiredNow(): boolean {
    return everSynced && now() - lastHeard() > EXPIRE_MS
  }

  provider.on('sync', (synced: boolean) => {
    if (destroyed) return
    if (!synced) {
      // Before the provider drops the peers' awareness, so the election
      // that follows already sees this client offline.
      setOnline()
      return
    }
    everSynced = true
    if (status.value === 'connecting') status.value = 'synced'
    pruneSeeds()
    readBase()
    setOnline()
    settleWaiters(true)
  })
  provider.on('status', ({ status: s }: { status: string }) => {
    setOnline()
    // A reconnect starts here, before the socket opens; ending now means the
    // old document never reaches a room that may have been rebuilt.
    if (s === 'connecting' && expiredNow()) end('expired')
  })
  provider.on('closed', (event: { code: number }) => {
    if (event.code === CLOSE_STALE_BUILD) end('stale-build')
    else if (event.code === CLOSE_GONE && everSynced) end('expired')
    else end('failed')
  })

  const keepalive = setInterval(() => {
    if (expiredNow() && !provider.wsconnected) {
      end('expired')
      return
    }
    const ws = provider.ws
    if (!provider.wsconnected || !ws || ws.readyState !== 1) return
    const enc = encoding.createEncoder()
    encoding.writeVarUint(enc, 0) // messageSync
    syncProtocol.writeSyncStep1(enc, doc)
    ws.send(encoding.toUint8Array(enc))
  }, KEEPALIVE_MS)

  function electSaver() {
    if (!online.value) {
      isSaver.value = false
      return
    }
    let lowest = doc.clientID
    for (const [client, state] of awareness.getStates()) {
      if (state && (state as { user?: unknown }).user && client < lowest) lowest = client
    }
    isSaver.value = lowest === doc.clientID
  }
  awareness.on('change', electSaver)
  electSaver()

  function readBase() {
    const b = meta.get('base') as RoomBase | undefined
    if (b && typeof b.content === 'string') {
      const cur = base.value
      if (!cur || cur.content !== b.content || cur.token !== b.token || cur.sent !== b.sent)
        base.value = { ...b }
    }
  }
  meta.observe(() => {
    readBase()
    pruneSeeds()
  })
  seeds.observe(() => pruneSeeds())

  // Two clients that seeded different bodies at the same moment both land
  // their seed. The map keeps one `seed`; every client deletes the top-level
  // items the other seeds inserted, so the document holds one copy.
  function pruneSeeds() {
    const winner = meta.get('seed')
    if (typeof winner !== 'number' || seeds.size < 2) return
    const fragment = doc.getXmlFragment(FRAGMENT)
    const losers: number[] = []
    fragment.toArray().forEach((child, i) => {
      const client = (child as unknown as { _item?: { id: { client: number } } })._item?.id.client
      if (client !== undefined && client !== winner && seeds.has(String(client))) losers.push(i)
    })
    if (!losers.length) return
    doc.transact(() => {
      for (const i of losers.reverse()) fragment.delete(i, 1)
    })
  }

  function seed(b: RoomBase, build: (fragment: Y.XmlFragment) => void) {
    if (doc.getXmlFragment(FRAGMENT).length > 0 || meta.has('seed')) return
    const client = seedClientId(b.content)
    const tmp = new Y.Doc()
    tmp.clientID = client
    tmp.transact(() => {
      build(tmp.getXmlFragment(FRAGMENT))
      const m = tmp.getMap<unknown>('rela')
      m.set('seed', client)
      m.set('base', { content: b.content, token: b.token })
      tmp.getMap<boolean>('relaSeeds').set(String(client), true)
    })
    Y.applyUpdate(doc, Y.encodeStateAsUpdate(tmp))
    tmp.destroy()
  }

  function recordSave(stored: string, token: string | undefined, sent?: string) {
    const next: RoomBase = { content: stored, token }
    if (sent !== undefined && sent !== stored) next.sent = sent
    meta.set('base', next)
  }

  function isCurrentBase(content: string): boolean {
    const b = base.value
    return b !== null && (content === b.content || content === b.sent)
  }

  function whenSynced(): Promise<boolean> {
    if (status.value === 'synced') return Promise.resolve(true)
    if (status.value !== 'connecting') return Promise.resolve(false)
    return new Promise((resolve) => syncWaiters.push(resolve))
  }

  function destroy() {
    if (destroyed) return
    destroyed = true
    online.value = false
    isSaver.value = false
    clearInterval(keepalive)
    settleWaiters(false)
    awareness.off('change', electSaver)
    provider.destroy()
    doc.destroy()
  }

  return {
    doc,
    awareness,
    status,
    isSaver,
    online,
    base,
    whenSynced,
    seed,
    recordSave,
    isCurrentBase,
    destroy,
  }
}

function defaultServerUrl(): string {
  // http: becomes ws: and https: becomes wss:.
  const { protocol, host } = window.location
  return `${protocol.replace(/^http/, 'ws')}//${host}`
}

/**
 * The client id a seed is written under: a 32-bit FNV-1a hash of the body,
 * never 0. Equal bodies give equal ids, so equal seeds are the same items.
 */
export function seedClientId(content: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < content.length; i++) {
    h ^= content.charCodeAt(i)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return h === 0 ? 1 : h
}

/** A stable color per user name, for cursors. */
export function userColor(name: string): string {
  return hslToHex(seedClientId(name) % 360, 0.7, 0.45)
}

// y-prosemirror accepts only #rrggbb and warns on every awareness update
// about any other format.
function hslToHex(hue: number, sat: number, light: number): string {
  const a = sat * Math.min(light, 1 - light)
  const channel = (n: number) => {
    const k = (n + hue / 30) % 12
    const v = light - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))
    return Math.round(v * 255)
      .toString(16)
      .padStart(2, '0')
  }
  return `#${channel(0)}${channel(8)}${channel(4)}`
}
