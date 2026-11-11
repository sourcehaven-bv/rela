import { ref, type Ref } from 'vue'
import { relaBase } from '@/api/base'
import type { Seen } from '@/utils/inboxNotify'

/**
 * Browser-notification preferences for the inbox (TKT-48S88RB), kept per
 * browser in localStorage.
 *
 * Everything is scoped by the project base and the user. The desktop shell
 * mounts several projects on one origin (`/p/<id>/`), and a browser may be
 * shared by several people. Scoping means another user of the same browser
 * starts with notifications off and never inherits a seen set. Sign-out is a
 * full page load to the login proxy, so the SPA cannot clear anything then;
 * the scope makes clearing unnecessary.
 */

/** What the user chose. Off by default. */
export interface NotificationPrefs {
  enabled: boolean
  /** Mentions of the user and replies to their messages. */
  mentions: boolean
  /** Sessions waiting on the user. */
  waiting: boolean
}

export const DEFAULT_PREFS: NotificationPrefs = { enabled: false, mentions: true, waiting: true }

/** Whether this page can show notifications, and the browser's answer. */
export type NotificationSupport = 'unsupported' | NotificationPermission

/**
 * The scope of one project and user. JSON, so no base and user pair can
 * spell another's key whatever characters they hold.
 */
export function notificationScope(user: string): string {
  return JSON.stringify([relaBase(), user])
}

const prefsKey = (scope: string) => `rela.notify:${scope}`
const seenKey = (scope: string) => `rela.notify.seen:${scope}`

/** The Web Lock one tab per project and user holds to notify. */
export const notifierLockName = (scope: string) => `rela-inbox-notify:${scope}`

/** The BroadcastChannel tabs of one project announce their focus on. */
export const focusChannelName = () => `rela-focus:${relaBase()}`

/**
 * Whether the page can notify. The desktop shell's webview gets native
 * notifications later (follow-up of TKT-48S88RB), so it reports unsupported
 * rather than risk two notifications for one entry.
 */
let broken = false

export function notificationSupport(): NotificationSupport {
  if (broken) return 'unsupported'
  if (typeof window === 'undefined' || !('Notification' in window)) return 'unsupported'
  if ('_wails' in window) return 'unsupported'
  return Notification.permission
}

/**
 * The support state as a ref, shared by the Settings card and the notifier.
 * Call refreshSupport after asking for permission; a change made in the
 * browser's site settings arrives through the Permissions API where the
 * browser offers it, and otherwise when the tab is shown again.
 */
export const support = ref<NotificationSupport>('unsupported')
let watchingSupport = false

export function refreshSupport() {
  support.value = notificationSupport()
}

/** Starts following permission changes. Safe to call more than once. */
export function watchSupport() {
  refreshSupport()
  if (watchingSupport || typeof document === 'undefined') return
  watchingSupport = true
  document.addEventListener('visibilitychange', refreshSupport)
  navigator.permissions
    ?.query({ name: 'notifications' })
    .then((status) => {
      status.onchange = refreshSupport
    })
    .catch(() => {})
}

/** Asks the browser for permission. Call it from a click handler. */
export async function requestPermission(): Promise<NotificationSupport> {
  if (notificationSupport() === 'unsupported') return 'unsupported'
  try {
    await Notification.requestPermission()
  } catch {
    // Older Safari takes a callback and rejects the promise form.
  }
  refreshSupport()
  return support.value
}

function readJSON(key: string): unknown {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? null : JSON.parse(raw)
  } catch {
    return null
  }
}

/** Writes a value; false when storage is full or blocked. */
function writeJSON(key: string, value: unknown): boolean {
  try {
    localStorage.setItem(key, JSON.stringify(value))
    return true
  } catch {
    return false
  }
}

/** Reads the prefs; anything malformed reads as the defaults (off). */
export function loadPrefs(scope: string): NotificationPrefs {
  const raw = readJSON(prefsKey(scope))
  if (typeof raw !== 'object' || raw === null) return { ...DEFAULT_PREFS }
  const r = raw as Record<string, unknown>
  const bool = (v: unknown, d: boolean) => (typeof v === 'boolean' ? v : d)
  return {
    enabled: bool(r.enabled, DEFAULT_PREFS.enabled),
    mentions: bool(r.mentions, DEFAULT_PREFS.mentions),
    waiting: bool(r.waiting, DEFAULT_PREFS.waiting),
  }
}

/**
 * Stores the prefs. Turning notifications on drops the seen set, so the next
 * read is a silent baseline: what is already in the inbox does not notify.
 */
export function savePrefs(scope: string, prefs: NotificationPrefs): boolean {
  if (prefs.enabled && !loadPrefs(scope).enabled) clearSeen(scope)
  return writeJSON(prefsKey(scope), prefs)
}

/**
 * Whether this browser lets the page keep the seen set. Without storage
 * every read would be a silent baseline and nothing would ever notify.
 */
export function storageWorks(): boolean {
  const probe = 'rela.notify.probe'
  if (!writeJSON(probe, 1)) return false
  try {
    localStorage.removeItem(probe)
  } catch {
    // Written but not removable: still usable.
  }
  return true
}

/** What was seen, or null when nothing is stored (a silent baseline). */
export function loadSeen(scope: string): Seen | null {
  const raw = readJSON(seenKey(scope))
  if (typeof raw !== 'object' || raw === null) return null
  const r = raw as Record<string, unknown>
  if (!Array.isArray(r.ids)) return null
  return {
    ids: new Set(r.ids.filter((v): v is string => typeof v === 'string')),
    newest: typeof r.newest === 'string' ? r.newest : null,
  }
}

/** Replaces what was seen. */
export function saveSeen(scope: string, seen: Seen) {
  writeJSON(seenKey(scope), { ids: [...seen.ids], newest: seen.newest })
}

export function clearSeen(scope: string) {
  try {
    localStorage.removeItem(seenKey(scope))
  } catch {
    // Nothing stored that we can reach.
  }
}

// One ref per scope, shared by the Settings card and the notifier in the
// same tab, and refreshed when another tab writes the prefs.
const refs = new Map<string, Ref<NotificationPrefs>>()
let listening = false

function onStorage(e: StorageEvent) {
  for (const [scope, r] of refs) {
    if (e.key === null || e.key === prefsKey(scope)) r.value = loadPrefs(scope)
  }
}

/**
 * The prefs of a scope as a ref. Assign a whole new object to change them;
 * the assignment is stored.
 */
export function usePrefsRef(scope: string): Ref<NotificationPrefs> {
  let r = refs.get(scope)
  if (!r) {
    r = ref(loadPrefs(scope))
    refs.set(scope, r)
  }
  if (!listening && typeof window !== 'undefined') {
    window.addEventListener('storage', onStorage)
    listening = true
  }
  return r
}

/** Stores prefs for a scope and updates its shared ref. */
export function setPrefs(scope: string, prefs: NotificationPrefs) {
  savePrefs(scope, prefs)
  usePrefsRef(scope).value = { ...prefs }
}

/**
 * Marks the page as unable to notify, after the browser refused to build a
 * Notification although permission is granted (Chrome on Android).
 */
export function markUnsupported() {
  broken = true
  support.value = 'unsupported'
}

/** Resets module state between tests. */
export function resetNotificationPrefs() {
  refs.clear()
  if (listening) window.removeEventListener('storage', onStorage)
  listening = false
  if (watchingSupport) document.removeEventListener('visibilitychange', refreshSupport)
  watchingSupport = false
  broken = false
  support.value = 'unsupported'
}
