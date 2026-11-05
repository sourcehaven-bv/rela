/**
 * Runs one entity body's co-editing session for EntityDetail (TKT-CJL604).
 *
 * The page edits the body alone ('single') or in a room ('collab'). This
 * composable owns the room and decides who saves what; the editor and the
 * autosave stay unaware of each other.
 *
 * Saving:
 * - Every client sees every change, because the editor emits remote edits
 *   too. Only the room's saver saves, after the usual typing pause.
 * - Any other client saves the latest document itself once the saver had
 *   `FALLBACK_MS` beyond its own pause and the document is still not the
 *   room's latest save (RR-PB4EXEM). A crashed saver therefore loses
 *   nothing. A client saves only while online: offline it cannot see its
 *   peers, and its copy may be behind theirs.
 * - A client that becomes the saver saves the document if no client did.
 * - A save that went through is recorded in the room, which moves the room's
 *   base; every client then moves its autosave base and its editor baseline,
 *   and a document that differs from the new base is saved again.
 * - "Saved" always means "the room's latest save". A document that returns
 *   to an earlier saved body, by undo for example, is saved again.
 *
 * The session ends in single mode when the room cannot be joined within
 * `SYNC_TIMEOUT_MS`, when the relay refuses the build twice, or when the room
 * expires. Each ending saves what the client holds first, whether or not it
 * is the saver.
 */
import { effectScope, ref, shallowRef, watch, type EffectScope } from 'vue'
import {
  openCollabRoom,
  userColor,
  type CollabRoom,
  type CollabRoomOptions,
  type EditorCollab,
  type RoomBase,
} from './collabRoom'

/** The saver's typing pause, the same as single-writer editing. */
export const SAVE_DEBOUNCE_MS = 800
/** How long a non-saver waits beyond the saver's pause before it saves. */
export const FALLBACK_MS = SAVE_DEBOUNCE_MS + 5_000
/** How long the editor may stay read-only waiting for the room. */
export const SYNC_TIMEOUT_MS = 5_000
/** sessionStorage key that limits the stale-build reload to one per build. */
export const STALE_RELOAD_KEY = 'rela.collab.staleReload'

/** The autosave calls this session makes. */
export interface BodyCollabSave {
  scheduleContentSave(content: string, debounceMs?: number): void
  setContentBase(content: string, token: string | undefined, resend?: boolean): void
  commitImmediately(): Promise<unknown>
}

/** The editor calls this session makes. */
export interface BodyCollabEditor {
  /** Emits a pending change now; the emit arrives through `input`. */
  flush(): void
  setBaseline(content: string): void
  /** Applies the change from `from` to `to` to the document as it is now. */
  applyChange(from: string, to: string): void
}

export type BodyCollabNotice = 'expired' | 'unavailable' | 'stale-build'

export interface BodyCollabDeps {
  /** Whether this principal may co-edit this body now: the server offers co-editing and the principal may update. */
  enabled: () => boolean
  /**
   * The principal's user id, the name peers see. Never the person title: a
   * title is read through the ACL, and peers may not be allowed to read it.
   */
  user: () => string | null | undefined
  /** Closes the editor after the room expired; the page shows the body read-only. */
  closeEditor: () => void
  notify: (notice: BodyCollabNotice) => void
  build?: string
  /** The relay process the page was served by (see collabRoom.ts). */
  epoch?: () => string
  reload?: () => void
  storage?: Pick<Storage, 'getItem' | 'setItem'>
  openRoom?: (opts: CollabRoomOptions) => CollabRoom
}

export interface BodyCollabTarget {
  type: string
  /** The served address, face included. */
  id: string
}

interface Session {
  target: BodyCollabTarget
  room: CollabRoom
  base: RoomBase
  save: BodyCollabSave
  scope: EffectScope
  synced: boolean
  latest: string | null
  fallbackTimer: ReturnType<typeof setTimeout> | null
  syncTimer: ReturnType<typeof setTimeout> | null
}

export function useBodyCollab(deps: BodyCollabDeps) {
  const mode = ref<'single' | 'collab'>('single')
  const editorCollab = shallowRef<EditorCollab | null>(null)
  let session: Session | null = null
  let editor: BodyCollabEditor | null = null

  /**
   * Hooks for the autosave instance of `target`. Bound to the target, so the
   * instance of a row the page has left never consults another row's room.
   */
  function autoSaveHooks(target: BodyCollabTarget) {
    const mine = () =>
      session && session.target.type === target.type && session.target.id === target.id
        ? session
        : null
    return {
      /**
       * The document to send over a 412 whose body is the room's latest
       * save. The room's document already holds that body, so the save
       * sends the document as it is now. Null for any other body: a body
       * from outside the room, or an older room body restored from history,
       * is merged.
       */
      resendContent: (theirs: string): string | null => {
        const s = mine()
        if (!s?.synced || !s.room.isCurrentBase(theirs)) return null
        return s.latest
      },
      onContentSaved: (stored: string, token: string | undefined, sent: string) => {
        mine()?.room.recordSave(stored, token, sent)
      },
      /** Whether edits to this row's body go through a room. */
      inRoom: () => mine() !== null,
      /**
       * Puts a merged body into the room as the change from `from`, the text
       * the save was made from, so every peer gets it and what peers typed
       * since stays. Only a merge: a stored body or a revert is older than
       * what peers have typed since. Not while offline, where the copy may
       * be behind the peers'.
       */
      mergedContent: (from: string, to: string) => {
        const s = mine()
        if (s?.synced && s.room.online.value) editor?.applyChange(from, to)
      },
    }
  }

  /**
   * Joins the room for `target` when co-editing applies. Returns whether it
   * did; on false the page edits alone.
   */
  function start(target: BodyCollabTarget, base: RoomBase, save: BodyCollabSave): boolean {
    end()
    const user = deps.user()
    if (!deps.enabled() || !user) return false
    const room = (deps.openRoom ?? openCollabRoom)({
      type: target.type,
      id: target.id,
      user: { name: user, color: userColor(user) },
      build: deps.build,
      epoch: deps.epoch?.() || undefined,
    })
    const s: Session = {
      target,
      room,
      base,
      save,
      scope: effectScope(true),
      synced: false,
      latest: null,
      fallbackTimer: null,
      syncTimer: null,
    }
    session = s
    editorCollab.value = { room, base }
    mode.value = 'collab'

    s.syncTimer = setTimeout(() => {
      s.syncTimer = null
      if (session === s && !s.synced) void fallBack(s, 'unavailable')
    }, SYNC_TIMEOUT_MS)

    s.scope.run(() => {
      watch(room.status, (status) => {
        if (session !== s) return
        if (status === 'expired') void expire(s)
        else if (status === 'stale-build') void staleBuild(s)
        else if (status === 'failed') {
          if (s.synced) void expire(s)
          else void fallBack(s, 'unavailable')
        }
      })
      watch(room.base, (next) => {
        if (session !== s || !next) return
        s.save.setContentBase(next.content, next.token)
        editor?.setBaseline(next.content)
        // A save that answered after newer text was typed, or one that
        // raced another peer's, leaves the document ahead of the base.
        if (s.synced && s.latest !== null) input(s.latest)
      })
      watch(room.isSaver, (isSaver) => {
        if (session === s && s.synced && isSaver) saveIfUnsaved(s)
      })
      watch(room.online, (online) => {
        if (session === s && s.synced && online && s.latest !== null) input(s.latest)
      })
    })

    void room.whenSynced().then((ok) => {
      if (session !== s || !ok) return
      onSynced(s)
    })
    return true
  }

  function onSynced(s: Session): void {
    s.synced = true
    if (s.syncTimer) clearTimeout(s.syncTimer)
    s.syncTimer = null
    const roomBase = s.room.base.value
    // The room's document derives from the room's base, not from this
    // client's GET. When the two differ, the store may hold an edit the room
    // has not seen; saving on the room's token turns that into a 412 and a
    // merge rather than an overwrite (RR-QEQ6711).
    if (roomBase && roomBase.token !== s.base.token) {
      s.save.setContentBase(roomBase.content, roomBase.token, true)
      editor?.flush()
      s.save.scheduleContentSave(s.latest ?? roomBase.content, 0)
      return
    }
    if (s.room.isSaver.value) saveIfUnsaved(s)
  }

  /** Saves the document now unless it is the room's latest save. */
  function saveIfUnsaved(s: Session): void {
    editor?.flush()
    if (s.latest !== null && !s.room.isCurrentBase(s.latest)) {
      s.save.scheduleContentSave(s.latest, 0)
    }
  }

  /**
   * Before leaving: flushes the editor and saves what this client holds,
   * saver or not. Without it a non-saver's last edits are lost when the
   * saver is gone.
   */
  function saveHeld(s: Session): void {
    if (!s.synced) return
    editor?.flush()
    if (s.latest !== null && !s.room.isCurrentBase(s.latest)) {
      s.save.scheduleContentSave(s.latest, 0)
    }
  }

  /**
   * Takes a document the editor emitted. Returns false when no room is
   * active, and the page saves the edit itself.
   */
  function input(next: string): boolean {
    const s = session
    if (!s) return false
    s.latest = next
    if (s.fallbackTimer) clearTimeout(s.fallbackTimer)
    s.fallbackTimer = null
    if (s.room.isSaver.value) {
      // Also for the room's latest save: it replaces a pending save of a
      // body typed since, and the autosave sends nothing for its own base.
      s.save.scheduleContentSave(next, SAVE_DEBOUNCE_MS)
      return true
    }
    if (s.room.isCurrentBase(next)) return true
    s.fallbackTimer = setTimeout(() => {
      s.fallbackTimer = null
      if (
        session === s &&
        s.room.online.value &&
        s.latest !== null &&
        !s.room.isCurrentBase(s.latest)
      ) {
        s.save.scheduleContentSave(s.latest, 0)
      }
    }, FALLBACK_MS)
    return true
  }

  function bindEditor(next: BodyCollabEditor | null): void {
    editor = next
  }

  /** Saves what the editor holds, then leaves the room. */
  async function stop(): Promise<void> {
    const s = session
    if (!s) return
    saveHeld(s)
    end()
    await s.save.commitImmediately()
  }

  async function expire(s: Session): Promise<void> {
    saveHeld(s)
    end()
    deps.closeEditor()
    deps.notify('expired')
    await s.save.commitImmediately()
  }

  async function fallBack(s: Session, notice: BodyCollabNotice): Promise<void> {
    saveHeld(s)
    end()
    deps.notify(notice)
    await s.save.commitImmediately()
  }

  /**
   * The relay serves another build. One reload fetches it; a second refusal
   * for the same build means reloading does not help, so edit alone.
   */
  async function staleBuild(s: Session): Promise<void> {
    const storage = deps.storage ?? sessionStorage
    const build = deps.build ?? ''
    if (storage.getItem(STALE_RELOAD_KEY) === build) {
      await fallBack(s, 'unavailable')
      return
    }
    storage.setItem(STALE_RELOAD_KEY, build)
    saveHeld(s)
    end()
    deps.notify('stale-build')
    await s.save.commitImmediately()
    ;(deps.reload ?? (() => window.location.reload()))()
  }

  function end(): void {
    const s = session
    if (!s) return
    session = null
    if (s.fallbackTimer) clearTimeout(s.fallbackTimer)
    if (s.syncTimer) clearTimeout(s.syncTimer)
    s.scope.stop()
    s.room.destroy()
    editorCollab.value = null
    mode.value = 'single'
  }

  return { mode, editorCollab, autoSaveHooks, start, input, bindEditor, stop }
}
