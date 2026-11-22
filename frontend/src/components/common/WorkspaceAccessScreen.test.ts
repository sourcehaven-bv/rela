import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { i18n } from '@/i18n'
import WorkspaceAccessScreen from './WorkspaceAccessScreen.vue'
import type { AccessRefusal } from '@/api/workspaceAccess'

function render(refusal: AccessRefusal, extra: Record<string, unknown> = {}) {
  const location = { assign: vi.fn(), reload: vi.fn() }
  const wrapper = mount(WorkspaceAccessScreen, {
    props: { refusal, location, ...extra },
    global: { plugins: [i18n] },
  })
  return { wrapper, location }
}

describe('WorkspaceAccessScreen', () => {
  it.each([
    ['suspended', 'This workspace is suspended'],
    ['archived', 'This workspace is archived'],
  ] as const)(
    'says the workspace is %s, says who to ask, and offers no retry',
    (refusal, title) => {
      const { wrapper } = render(refusal)
      expect(wrapper.text()).toContain(title)
      expect(wrapper.text()).toContain(
        "contact the person who manages your organisation's subscription"
      )
      expect(wrapper.text()).not.toMatch(/tenant|organization/i)
      expect(wrapper.findAll('button')).toHaveLength(0)
    }
  )

  it('shows an ended session as a dialog over the app', () => {
    const { wrapper } = render('session_ended')
    const dialog = wrapper.get('[role="alertdialog"]')
    expect(dialog.attributes('aria-modal')).toBe('true')
    expect(dialog.text()).toContain('Your session has ended')
  })

  it('sends "Sign in again" to the sign-in page the server named', async () => {
    const { wrapper, location } = render('session_ended', {
      loginUrl: '/auth/login?rd=%2Ftickets',
    })
    await wrapper.get('button').trigger('click')
    expect(location.assign).toHaveBeenCalledWith('/auth/login?rd=%2Ftickets')
    expect(location.reload).not.toHaveBeenCalled()
  })

  it('reloads the page to sign in when the server named no sign-in page', async () => {
    const { wrapper, location } = render('session_ended')
    await wrapper.get('button').trigger('click')
    expect(location.reload).toHaveBeenCalled()
    expect(location.assign).not.toHaveBeenCalled()
  })

  it("shows the server's explanation when it cannot identify anyone", async () => {
    const { wrapper, location } = render('no_identity', {
      detail: 'no user identity: set auth.header',
    })
    expect(wrapper.text()).toContain('This server cannot identify you')
    expect(wrapper.text()).toContain('no user identity: set auth.header')
    expect(wrapper.text()).not.toContain('Sign in again')
    await wrapper.get('button').trigger('click')
    expect(location.reload).toHaveBeenCalled()
  })

  it('offers to try again for a workspace that may still appear', async () => {
    const { wrapper, location } = render('unknown_workspace')
    expect(wrapper.text()).toContain('This workspace is not available')
    await wrapper.get('button').trigger('click')
    expect(location.reload).toHaveBeenCalled()
  })

  const exits = {
    signOutUrl: '/auth/logout',
    accountUrl: 'https://account.example.com/',
  }

  it.each(['suspended', 'archived', 'unknown_workspace'] as const)(
    'offers the account portal and sign out on a %s workspace',
    (refusal) => {
      const { wrapper } = render(refusal, exits)
      const links = wrapper.findAll('a')
      expect(links.map((a) => [a.text(), a.attributes('href')])).toEqual([
        ['Go to your account', exits.accountUrl],
        ['Sign out', exits.signOutUrl],
      ])
    }
  )

  it('offers no account links when the server cannot identify anyone', () => {
    const { wrapper } = render('no_identity', { ...exits, detail: 'set auth.header' })
    expect(wrapper.findAll('a')).toHaveLength(0)
  })

  it('shows the configured contact text instead of the default', () => {
    const contact = 'Email support@example.com'
    const { wrapper } = render('suspended', { contact })
    expect(wrapper.text()).toContain(contact)
    expect(wrapper.text()).not.toContain('manages your organisation')
  })

  it('shows the logo, or else the app name', () => {
    const named = render('archived', { appName: 'Workspace' }).wrapper
    expect(named.find('img').exists()).toBe(false)
    expect(named.get('.access-brand').text()).toBe('Workspace')

    const logo = '/brand/logo.svg'
    const img = render('archived', { appName: 'Workspace', logo }).wrapper.get('img')
    expect(img.attributes('src')).toBe(logo)
    expect(img.attributes('alt')).toBe('Workspace')
  })
})
