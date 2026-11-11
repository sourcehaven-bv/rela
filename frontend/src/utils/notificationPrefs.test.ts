import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  DEFAULT_PREFS,
  clearSeen,
  loadPrefs,
  loadSeen,
  markUnsupported,
  notificationScope,
  notificationSupport,
  refreshSupport,
  requestPermission,
  resetNotificationPrefs,
  saveSeen,
  setPrefs,
  storageWorks,
  support,
  usePrefsRef,
} from './notificationPrefs'

function stubNotification(permission: NotificationPermission, answer = permission) {
  const N = {
    permission,
    requestPermission: vi.fn(async () => {
      N.permission = answer
      return answer
    }),
  }
  vi.stubGlobal('Notification', N)
  return N
}

beforeEach(() => {
  localStorage.clear()
  resetNotificationPrefs()
})

afterEach(() => {
  vi.unstubAllGlobals()
  delete (window as unknown as Record<string, unknown>)._wails
})

describe('notificationPrefs', () => {
  it('scopes by base and user', () => {
    expect(notificationScope('alice')).toBe('["/","alice"]')
  })

  it('reads the defaults (off) when nothing or garbage is stored', () => {
    const s = notificationScope('alice')
    expect(loadPrefs(s)).toEqual(DEFAULT_PREFS)
    localStorage.setItem(`rela.notify:${s}`, '{not json')
    expect(loadPrefs(s)).toEqual(DEFAULT_PREFS)
    localStorage.setItem(`rela.notify:${s}`, JSON.stringify({ enabled: 'yes', mentions: false }))
    expect(loadPrefs(s)).toEqual({ ...DEFAULT_PREFS, mentions: false })
  })

  it('keeps users apart', () => {
    setPrefs(notificationScope('alice'), { ...DEFAULT_PREFS, enabled: true })
    expect(loadPrefs(notificationScope('alice')).enabled).toBe(true)
    expect(loadPrefs(notificationScope('bob')).enabled).toBe(false)
  })

  it('drops the seen set when notifications are turned on, not otherwise', () => {
    const s = notificationScope('alice')
    saveSeen(s, { ids: new Set(['a']), newest: null })
    setPrefs(s, { ...DEFAULT_PREFS, enabled: false, waiting: false })
    expect(loadSeen(s)?.ids).toEqual(new Set(['a']))
    setPrefs(s, { ...DEFAULT_PREFS, enabled: true })
    expect(loadSeen(s)).toBeNull()
    saveSeen(s, { ids: new Set(['b']), newest: '2026-11-10T10:00:00Z' })
    setPrefs(s, { ...DEFAULT_PREFS, enabled: true, mentions: false })
    expect(loadSeen(s)).toEqual({ ids: new Set(['b']), newest: '2026-11-10T10:00:00Z' })
  })

  it('reads a malformed seen set as none, and drops non-strings', () => {
    const s = notificationScope('alice')
    localStorage.setItem(`rela.notify.seen:${s}`, '"x"')
    expect(loadSeen(s)).toBeNull()
    localStorage.setItem(`rela.notify.seen:${s}`, JSON.stringify(['a']))
    expect(loadSeen(s)).toBeNull()
    localStorage.setItem(
      `rela.notify.seen:${s}`,
      JSON.stringify({ ids: ['a', 1, null], newest: 5 })
    )
    expect(loadSeen(s)).toEqual({ ids: new Set(['a']), newest: null })
    clearSeen(s)
    expect(loadSeen(s)).toBeNull()
  })

  it('tells whether storage works', () => {
    expect(storageWorks()).toBe(true)
    const spy = vi.spyOn(localStorage, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    expect(storageWorks()).toBe(false)
    spy.mockRestore()
  })

  it('keeps a page marked unsupported, until reset', () => {
    stubNotification('granted')
    markUnsupported()
    refreshSupport()
    expect(support.value).toBe('unsupported')
    resetNotificationPrefs()
    refreshSupport()
    expect(support.value).toBe('granted')
  })

  it('shares one ref per scope and follows writes from other tabs', () => {
    const s = notificationScope('alice')
    const a = usePrefsRef(s)
    expect(usePrefsRef(s)).toBe(a)
    localStorage.setItem(`rela.notify:${s}`, JSON.stringify({ ...DEFAULT_PREFS, enabled: true }))
    window.dispatchEvent(new StorageEvent('storage', { key: `rela.notify:${s}` }))
    expect(a.value.enabled).toBe(true)
  })

  it('reports unsupported without the API and in the desktop shell', () => {
    vi.stubGlobal('Notification', undefined)
    delete (window as unknown as Record<string, unknown>).Notification
    expect(notificationSupport()).toBe('unsupported')
    stubNotification('granted')
    expect(notificationSupport()).toBe('granted')
    ;(window as unknown as Record<string, unknown>)._wails = {}
    expect(notificationSupport()).toBe('unsupported')
  })

  it('asks for permission and records the answer', async () => {
    const N = stubNotification('default', 'denied')
    expect(await requestPermission()).toBe('denied')
    expect(N.requestPermission).toHaveBeenCalledTimes(1)
    expect(support.value).toBe('denied')
  })
})
