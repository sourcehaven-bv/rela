import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import AccountMenu from './AccountMenu.vue'
import type { MeResponse } from '@/api'

const getMeMock = vi.fn<() => Promise<MeResponse>>()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getMe: () => getMeMock(),
}))

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: routerPush,
    resolve: (path: string) => ({ href: path }),
  }),
  useRoute: () => ({ path: '/' }),
}))

let wrapper: VueWrapper | undefined

async function open(me: MeResponse | Error) {
  if (me instanceof Error) getMeMock.mockRejectedValue(me)
  else getMeMock.mockResolvedValue(me)
  const pinia = createPinia()
  setActivePinia(pinia)
  wrapper = mount(AccountMenu, { attachTo: document.body, global: { plugins: [pinia, PiniaColada] } })
  await flushPromises()
  const trigger = wrapper.find('[data-testid="account-menu"]')
  if (trigger.exists()) {
    await trigger.trigger('click')
    await flushPromises()
  }
  return wrapper
}

function rows(): { label: string; href: string | null }[] {
  return Array.from(document.body.querySelectorAll('[role="menuitem"]')).map((el) => ({
    label: el.textContent?.trim() ?? '',
    href: el.getAttribute('href'),
  }))
}

describe('AccountMenu', () => {
  beforeEach(() => {
    getMeMock.mockReset()
    routerPush.mockReset()
  })
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    document.body.innerHTML = ''
  })

  it('shows the person, the org and the proxy links', async () => {
    const w = await open({
      user: 'PERS-1',
      email: 'ada@example.com',
      org: { id: 'o1', slug: 'acme', name: 'Acme' },
      person: { type: 'person', id: 'PERS-1', title: 'Ada Lovelace' },
      links: {
        account: '/pratique/admin/account',
        switch_org: '/pratique/auth/select-tenant',
        admin: '/pratique/admin/',
        sign_out: '/pratique/auth/logout',
      },
    })

    expect(w.find('.account-trigger__name').text()).toBe('Ada Lovelace')
    expect(w.find('.account-trigger__org').text()).toBe('Acme')
    expect(document.body.querySelector('.rl-menu-section')?.textContent).toContain('ada@example.com')
    expect(rows()).toEqual([
      { label: 'Profile', href: '/entity/person/PERS-1' },
      { label: 'Account', href: '/pratique/admin/account' },
      { label: 'Switch org', href: '/pratique/auth/select-tenant' },
      { label: 'Admin', href: '/pratique/admin/' },
      { label: 'Sign out', href: '/pratique/auth/logout' },
    ])
  })

  it('falls back to the email, then the user id, with no person', async () => {
    const w = await open({ user: 'e2e@example.com' })
    expect(w.find('.account-trigger__name').text()).toBe('e2e@example.com')
    expect(w.find('.account-trigger__org').exists()).toBe(false)
    // Nothing to offer but who you are: no rows, no stray separators.
    expect(rows()).toEqual([])
    expect(document.body.querySelector('[role="separator"]')).toBeNull()
  })

  it('opens the profile in the SPA on a plain click', async () => {
    await open({ user: 'PERS-1', person: { type: 'person', id: 'PERS-1', title: 'Ada' } })
    const profile = document.body.querySelector<HTMLAnchorElement>('[role="menuitem"]')!
    profile.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }))
    expect(routerPush).toHaveBeenCalledWith('/entity/person/PERS-1')
  })

  it('renders nothing when the identity cannot be fetched', async () => {
    const w = await open(new Error('404'))
    expect(w.find('[data-testid="account-menu"]').exists()).toBe(false)
  })
})
