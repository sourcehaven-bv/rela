import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'

import RemotesView from './RemotesView.vue'
import { ApiError } from '@/api/errors'
import type { RemoteSummary } from '@/api/remotes'

const listRemotes = vi.hoisted(() => vi.fn())
vi.mock('@/api/remotes', async (orig) => ({
  ...(await orig<typeof import('@/api/remotes')>()),
  listRemotes,
}))

function remote(over: Partial<RemoteSummary> = {}): RemoteSummary {
  return {
    name: 'cat',
    url: 'https://cat.test',
    credential_kind: 'env',
    push: true,
    conflict: 'latest',
    max_clock_skew_seconds: 300,
    superseded_retention_seconds: 0,
    types: [],
    running: false,
    failures: 0,
    linked: 4,
    conflicts: { total: 0, by_reason: {} },
    ...over,
  }
}

function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }],
  })
  return mount(RemotesView, { global: { plugins: [router] } })
}

describe('RemotesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listRemotes.mockReset()
  })

  it('shows each remote with its state and gated counts', async () => {
    listRemotes.mockResolvedValue([
      remote({
        last_run: '2026-11-11T10:00:00Z',
        conflicts: { total: 3, by_reason: { both_changed: 2, uncertain: 1 } },
      }),
      remote({ name: 'dog', failures: 2, next_retry: '2026-11-11T11:00:00Z', push: false }),
      remote({ name: 'eel', credential_kind: '' }),
    ])
    const w = mountView()
    await flushPromises()

    const rows = w.findAll('[data-testid="remote-row"]')
    expect(rows).toHaveLength(3)
    expect(rows[0].text()).toContain('cat')
    expect(rows[0].text()).toContain('OK')
    expect(rows[0].text()).toContain('Both changed: 2')
    expect(rows[0].text()).toContain('Uncertain create: 1')
    expect(rows[0].find('a[href="/remotes/cat"]').exists()).toBe(true)
    expect(rows[1].text()).toContain('Failed 2 times in a row')
    expect(rows[1].text()).toContain('Next try')
    expect(rows[1].text()).toContain('Off')
    expect(rows[2].text()).toContain('Not run yet')
    expect(rows[2].text()).not.toContain('Credential')
  })

  it('says so when there are no remotes', async () => {
    listRemotes.mockResolvedValue([])
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('No remotes are set up')
  })

  it('names remote:sync when the permission is missing', async () => {
    listRemotes.mockRejectedValue(
      new ApiError('Forbidden', {
        kind: 'http',
        status: 403,
        problem: {
          type: 'https://rela.dev/errors/permission_required',
          title: 'Forbidden',
          status: 403,
        },
        original: null,
      })
    )
    const w = mountView()
    await flushPromises()
    expect(w.find('[data-testid="remotes-error"]').text()).toContain('remote:sync')
  })

  it('keeps the server message on another failure', async () => {
    listRemotes.mockRejectedValue(
      new ApiError('Boom', { kind: 'http', status: 500, original: null })
    )
    const w = mountView()
    await flushPromises()
    expect(w.find('[data-testid="remotes-error"]').text()).toContain('Boom')
  })
})
