import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import NotificationSetting from './NotificationSetting.vue'
import {
  DEFAULT_PREFS,
  loadPrefs,
  notificationScope,
  resetNotificationPrefs,
  setPrefs,
} from '@/utils/notificationPrefs'

const me = vi.hoisted(() => ({ user: 'alice' as string | undefined }))
vi.mock('@/queries/me', async () => {
  const { ref } = await import('vue')
  return { useMe: () => ({ data: ref(me.user ? { user: me.user } : undefined) }) }
})

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

const scope = () => notificationScope('alice')

function render() {
  return mount(NotificationSetting)
}

beforeEach(() => {
  localStorage.clear()
  resetNotificationPrefs()
  me.user = 'alice'
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('NotificationSetting', () => {
  it('is off by default', () => {
    stubNotification('default')
    const w = render()
    expect(w.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(w.find('[data-testid="notification-notice"]').exists()).toBe(false)
  })

  it('asks for permission on the click and turns on when granted', async () => {
    const N = stubNotification('default', 'granted')
    const w = render()
    await w.get('[role="switch"]').trigger('click')
    await flushPromises()
    expect(N.requestPermission).toHaveBeenCalledTimes(1)
    expect(loadPrefs(scope()).enabled).toBe(true)
    expect(w.get('[role="switch"]').attributes('aria-checked')).toBe('true')
    expect(w.text()).toContain('Mentions and replies')
  })

  it('stays off and explains when permission is denied', async () => {
    stubNotification('default', 'denied')
    const w = render()
    await w.get('[role="switch"]').trigger('click')
    await flushPromises()
    expect(loadPrefs(scope()).enabled).toBe(false)
    expect(w.get('[data-testid="notification-notice"]').text()).toContain('blocked')
    expect(w.get('[role="switch"]').attributes('disabled')).toBeDefined()
  })

  it('explains an unsupported browser', () => {
    vi.stubGlobal('Notification', undefined)
    delete (window as unknown as Record<string, unknown>).Notification
    const w = render()
    expect(w.get('[data-testid="notification-notice"]').text()).toContain('cannot show')
  })

  it('needs a known user', () => {
    stubNotification('granted')
    me.user = undefined
    const w = render()
    expect(w.get('[data-testid="notification-notice"]').text()).toContain('Sign in')
  })

  it('turns a kind off and notifications off without asking again', async () => {
    const N = stubNotification('granted')
    setPrefs(scope(), { ...DEFAULT_PREFS, enabled: true })
    const w = render()
    await w.get('input[type="checkbox"]').setValue(false)
    expect(loadPrefs(scope()).mentions).toBe(false)
    await w.get('[role="switch"]').trigger('click')
    expect(loadPrefs(scope()).enabled).toBe(false)
    expect(N.requestPermission).not.toHaveBeenCalled()
  })
  it('stays off and explains when the browser keeps no site data', async () => {
    const N = stubNotification('default', 'granted')
    const spy = vi.spyOn(localStorage, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    const w = render()
    await w.get('[role="switch"]').trigger('click')
    await flushPromises()
    spy.mockRestore()
    expect(N.requestPermission).not.toHaveBeenCalled()
    expect(loadPrefs(scope()).enabled).toBe(false)
    expect(w.get('[data-testid="notification-notice"]').text()).toContain('store data')
  })
})
