import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'

import RlPagination from 'rela-components/components/data/RlPagination.vue'
import RemoteConflictsView from './RemoteConflictsView.vue'
import { ApiError } from '@/api/errors'
import type { RemoteConflictPage, RemoteEntry } from '@/api/remotes'
import { useUIStore } from '@/stores/ui'

/**
 * A remote's conflict list (TKT-0FMYPEZ): a control shows only when the
 * entry's `_actions` allows it, keeping the local side of an uncertain create
 * asks first, and a refusal is a toast.
 */

const api = vi.hoisted(() => ({
  listRemoteConflicts: vi.fn(),
  resolveRemoteEntry: vi.fn(),
  dismissRemoteEntry: vi.fn(),
  linkRemoteEntry: vi.fn(),
}))
vi.mock('@/api/remotes', async (orig) => ({
  ...(await orig<typeof import('@/api/remotes')>()),
  ...api,
}))

const confirm = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm }) }))

const NONE = { resolve_local: false, resolve_remote: false, dismiss: false, link: false }

function entry(over: Partial<RemoteEntry> = {}): RemoteEntry {
  return {
    local: 'TKT-1',
    type: 'ticket',
    address: 'requirement/REQ-9',
    field: 'title',
    reason: 'both_changed',
    rule: 'latest',
    since: '2026-11-11T10:00:00Z',
    seen: '2026-11-11T10:00:00Z',
    remote_value: 'Theirs',
    _actions: { ...NONE, resolve_local: true, resolve_remote: true },
    ...over,
  }
}

function pageOf(data: RemoteEntry[], total = data.length): RemoteConflictPage {
  return { data, total, page: 1, per_page: 25, counts: {} }
}

function httpError(status: number, code?: string, message = 'Boom') {
  return new ApiError(message, {
    kind: 'http',
    status,
    problem: code ? { type: `https://rela.dev/errors/${code}`, title: message, status } : undefined,
    original: null,
  })
}

async function mountView(data: RemoteEntry[], total?: number) {
  api.listRemoteConflicts.mockResolvedValue(pageOf(data, total))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }],
  })
  const w = mount(RemoteConflictsView, { props: { name: 'cat' }, global: { plugins: [router] } })
  await flushPromises()
  return w
}

describe('RemoteConflictsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const fn of Object.values(api)) fn.mockReset()
    confirm.mockReset()
    api.resolveRemoteEntry.mockResolvedValue({ changed: 1 })
    api.dismissRemoteEntry.mockResolvedValue({ changed: 1 })
    api.linkRemoteEntry.mockResolvedValue({ changed: 1 })
  })

  it('shows the entry with its reason, cause and fallback', async () => {
    const w = await mountView([
      entry({ fallback: 'same_time' }),
      entry({
        local: 'TKT-2',
        field: undefined,
        fields: ['title', 'status'],
        reason: 'refused',
        cause: 'refused',
        status: 422,
        remote_value: undefined,
        _actions: { ...NONE, resolve_local: true },
      }),
    ])
    const rows = w.findAll('[data-testid="remote-entry"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('Both changed')
    expect(rows[0].text()).toContain('both sides changed at the same time')
    expect(rows[0].text()).toContain('Theirs')
    expect(rows[0].find('a[href="/entity/ticket/TKT-1"]').exists()).toBe(true)
    expect(rows[1].text()).toContain('Whole row')
    expect(rows[1].text()).toContain('title, status')
    expect(rows[1].text()).toContain('The remote refused the write')
    expect(rows[1].text()).toContain('HTTP 422')
  })

  it('links no row for a relation entry', async () => {
    const w = await mountView([entry({ kind: 'relation', local: 'A--traces--B', type: 'traces' })])
    expect(w.find('[data-testid="remote-entry"] a').exists()).toBe(false)
    expect(w.text()).toContain('A--traces--B')
  })

  it('marks a superseded value that was not kept', async () => {
    const w = await mountView([
      entry({
        reason: 'superseded',
        superseded: {
          winner: 'remote',
          lost_value_kept: false,
          local_updated: '2026-11-11T09:00:00Z',
          remote_updated: '2026-11-11T10:00:00Z',
        },
        _actions: { ...NONE, dismiss: true },
      }),
    ])
    expect(w.find('[data-testid="remote-entry-not-kept"]').exists()).toBe(true)
    expect(w.text()).toContain('The remote value was taken')
  })

  it('shows a kept superseded value', async () => {
    const w = await mountView([
      entry({
        reason: 'superseded',
        superseded: {
          winner: 'remote',
          local_value: 'Mine',
          lost_value_kept: true,
          local_updated: '2026-11-11T09:00:00Z',
          remote_updated: '2026-11-11T10:00:00Z',
        },
        _actions: { ...NONE, dismiss: true },
      }),
    ])
    expect(w.text()).toContain('Overwritten value: Mine')
    expect(w.find('[data-testid="remote-entry-not-kept"]').exists()).toBe(false)
  })

  it('says a large value is too large to show', async () => {
    const w = await mountView([entry({ remote_value: undefined, remote_value_omitted: true })])
    expect(w.text()).toContain('Too large to show')
  })

  it.each([
    { name: 'resolve_local', testid: 'remote-keep-local' },
    { name: 'resolve_remote', testid: 'remote-take-remote' },
    { name: 'dismiss', testid: 'remote-dismiss' },
    { name: 'link', testid: 'remote-link-open' },
  ] as const)('shows $testid only when $name is allowed', async ({ name, testid }) => {
    const on = await mountView([entry({ _actions: { ...NONE, [name]: true } })])
    expect(on.find(`[data-testid="${testid}"]`).exists()).toBe(true)
    const off = await mountView([entry({ _actions: { ...NONE } })])
    expect(off.find(`[data-testid="${testid}"]`).exists()).toBe(false)
  })

  it('takes the remote value and reloads', async () => {
    const w = await mountView([entry()])
    await w.find('[data-testid="remote-take-remote"]').trigger('click')
    await flushPromises()
    expect(api.resolveRemoteEntry).toHaveBeenCalledWith(
      'cat',
      { local: 'TKT-1', field: 'title' },
      'remote'
    )
    expect(api.listRemoteConflicts).toHaveBeenCalledTimes(2)
  })

  it('keeps a both-changed local value without asking', async () => {
    const w = await mountView([entry()])
    await w.find('[data-testid="remote-keep-local"]').trigger('click')
    await flushPromises()
    expect(confirm).not.toHaveBeenCalled()
    expect(api.resolveRemoteEntry).toHaveBeenCalledWith(
      'cat',
      { local: 'TKT-1', field: 'title' },
      'local'
    )
  })

  it('asks before sending an uncertain create again', async () => {
    const uncertain = entry({
      reason: 'uncertain',
      address: undefined,
      field: undefined,
      fields: ['title'],
      remote_value: undefined,
      _actions: { ...NONE, resolve_local: true, link: true },
    })
    confirm.mockResolvedValue(false)
    const w = await mountView([uncertain])
    await w.find('[data-testid="remote-keep-local"]').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalledOnce()
    expect(api.resolveRemoteEntry).not.toHaveBeenCalled()

    confirm.mockResolvedValue(true)
    await w.find('[data-testid="remote-keep-local"]').trigger('click')
    await flushPromises()
    expect(api.resolveRemoteEntry).toHaveBeenCalledWith(
      'cat',
      { local: 'TKT-1', field: undefined },
      'local'
    )
  })

  it('links an uncertain create to a remote row', async () => {
    const w = await mountView([
      entry({
        reason: 'uncertain',
        field: undefined,
        address: undefined,
        _actions: { ...NONE, link: true },
      }),
    ])
    await w.find('[data-testid="remote-link-open"]').trigger('click')
    await w.find('[data-testid="remote-link-address"] input').setValue(' requirement/REQ-7 ')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(api.linkRemoteEntry).toHaveBeenCalledWith('cat', 'TKT-1', 'requirement/REQ-7')
  })

  it('dismisses a superseded entry', async () => {
    const w = await mountView([
      entry({ reason: 'superseded', _actions: { ...NONE, dismiss: true } }),
    ])
    await w.find('[data-testid="remote-dismiss"]').trigger('click')
    await flushPromises()
    expect(api.dismissRemoteEntry).toHaveBeenCalledWith('cat', { local: 'TKT-1', field: 'title' })
  })

  it.each([
    { name: 'a busy remote', err: httpError(409, 'remote_busy'), want: 'A sync is running' },
    { name: 'a refusal', err: httpError(403, 'forbidden', 'Not allowed'), want: 'Not allowed' },
  ])('toasts $name', async ({ err, want }) => {
    const w = await mountView([entry()])
    const ui = useUIStore()
    const spy = vi.spyOn(ui, 'error')
    api.resolveRemoteEntry.mockRejectedValue(err)
    await w.find('[data-testid="remote-take-remote"]').trigger('click')
    await flushPromises()
    expect(spy).toHaveBeenCalledWith(expect.stringContaining(want))
  })

  it('says an unknown remote does not exist', async () => {
    api.listRemoteConflicts.mockRejectedValue(httpError(404, 'not_found'))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }],
    })
    const w = mount(RemoteConflictsView, { props: { name: 'nope' }, global: { plugins: [router] } })
    await flushPromises()
    expect(w.find('[data-testid="remote-conflicts-error"]').text()).toContain(
      'There is no remote named nope'
    )
  })

  it('drops the held list when a reload is refused', async () => {
    const w = await mountView([entry()])
    api.listRemoteConflicts.mockRejectedValue(httpError(403, 'permission_required', 'Forbidden'))
    await w.find('[data-testid="remote-take-remote"]').trigger('click')
    await flushPromises()
    expect(w.findAll('[data-testid="remote-entry"]')).toHaveLength(0)
    expect(w.find('[data-testid="remote-conflicts-error"]').text()).toContain('remote:sync')
  })

  it('pages', async () => {
    const w = await mountView([entry()], 60)
    expect(w.text()).toContain('60 entries')
  })

  it('steps back when an action empties the last page', async () => {
    const w = await mountView([entry()], 26)
    api.listRemoteConflicts.mockResolvedValue(pageOf([entry()], 26))
    w.findComponent(RlPagination).vm.$emit('update:page', 2)
    await flushPromises()
    expect(api.listRemoteConflicts).toHaveBeenLastCalledWith('cat', 2, 25, expect.anything())
    api.listRemoteConflicts.mockResolvedValueOnce(pageOf([], 25))
    api.listRemoteConflicts.mockResolvedValueOnce(pageOf([entry()], 25))
    await w.find('[data-testid="remote-take-remote"]').trigger('click')
    await flushPromises()
    expect(api.listRemoteConflicts).toHaveBeenLastCalledWith('cat', 1, 25, expect.anything())
    expect(w.findAll('[data-testid="remote-entry"]')).toHaveLength(1)
  })
})
