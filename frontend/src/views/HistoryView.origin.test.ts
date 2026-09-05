/**
 * Copy provenance in the version timeline (TKT-VQHPFK).
 *
 * The server records that a write was produced by a mechanism rather than
 * typed, and the timeline annotates the row. Three properties are load-bearing
 * and each has a mutation that these tests catch:
 *
 *   - The op stays what it is. Render the origin kind INSTEAD of the op badge
 *     and "a copy is still a create" stops being true on screen.
 *   - No origin renders nothing extra. Default the label to 'manual'/'direct'
 *     and every row is marked, which makes the copy marker meaningless.
 *   - A withheld source renders the rest. Gate the whole label on `source` and
 *     an ACL-gated source silently erases the copy fact too.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import HistoryView from './HistoryView.vue'
import { listVersions, getVersion } from '@/api/history'
import { getEntity } from '@/api/entities'
import type { VersionMeta } from '@/api/history'

const mockRouteQuery = ref<Record<string, unknown>>({})

vi.mock('vue-router', () => ({
  useRoute: () => ({
    params: { type: 'policy', id: 'POL-1' },
    get query() {
      return mockRouteQuery.value
    },
  }),
  // `replace` as well as `push`: useVersionSelectionSync publishes the
  // resolved base/target pair to the URL with replace, and a mock without it
  // fails the whole load with "router.replace is not a function".
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  // Navigable controls are real links since TKT-3CSZRG, and this mock replaces
  // the whole module, so it must supply RouterLink or the render throws.
  RouterLink: { template: '<a><slot /></a>' },
}))

vi.mock('@/api/history', async () => {
  const actual = await vi.importActual<typeof import('@/api/history')>('@/api/history')
  return { ...actual, listVersions: vi.fn(), getVersion: vi.fn(), restoreVersion: vi.fn() }
})

vi.mock('@/api/entities', async () => {
  const actual = await vi.importActual<typeof import('@/api/entities')>('@/api/entities')
  return { ...actual, getEntity: vi.fn() }
})

const mockList = vi.mocked(listVersions)
const mockGetVersion = vi.mocked(getVersion)
const mockGetEntity = vi.mocked(getEntity)

function meta(over: Partial<VersionMeta> = {}): VersionMeta {
  return {
    version: 1,
    op: 'update',
    type: 'policy',
    created_at: '2026-08-29T12:30:00Z',
    principal: { user: 'edith@example.com', tool: 'data-entry' },
    ...over,
  }
}

async function mountWith(versions: VersionMeta[], entity: Record<string, unknown> = {}) {
  mockList.mockResolvedValue({ versions, face: '', worldFaceAbsent: false })
  mockGetEntity.mockResolvedValue({
    id: 'POL-1',
    type: 'policy',
    content: '',
    properties: {},
    ...entity,
  } as never)
  mockGetVersion.mockResolvedValue({
    id: 'POL-1',
    version: 1,
    op: 'update',
    created_at: '2026-08-29T12:30:00Z',
    principal: { user: 'edith@example.com', tool: 'data-entry' },
    entity: { id: 'POL-1', type: 'policy', content: '', properties: {} },
  } as never)
  const w = mount(HistoryView, { global: { stubs: { Badge: true } } })
  await vi.waitFor(() => expect(w.find('.timeline-item').exists()).toBe(true))
  return w
}

describe('HistoryView copy provenance', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockRouteQuery.value = {}
  })

  it('renders the copy fact, source and definition, matching the CLI spelling', async () => {
    const w = await mountWith([
      meta({ origin: { kind: 'copy', source: 'POL-1@draft', definition: 'publish' } }),
    ])

    expect(w.find('.timeline-origin').text()).toBe('copy from POL-1@draft (publish)')
  })

  // The source entity id is gated server-side: a reader whose verdict does not
  // cover it simply gets no `source` key. That is a normal answer, so the copy
  // fact and the definition still render, with no placeholder and no error.
  it('omits a withheld source without implying missing data', async () => {
    const w = await mountWith([meta({ origin: { kind: 'copy', definition: 'publish' } })])

    const label = w.find('.timeline-origin')
    expect(label.text()).toBe('copy (publish)')
    expect(label.text()).not.toContain('from')
    expect(label.text()).not.toMatch(/unknown|hidden|unavailable|\?|—/)
  })

  // There is no `kind: 'manual'` on the wire. Absence IS the signal for a hand
  // edit, and the principal beside it already says who made it.
  it('renders nothing extra for a direct edit', async () => {
    const w = await mountWith([meta()])

    expect(w.find('.timeline-origin').exists()).toBe(false)
    expect(w.text()).not.toMatch(/manual|direct/i)
    // ...and the row still says who typed it.
    expect(w.find('.timeline-who').text()).toContain('edith@example.com')
  })

  // A copy genuinely IS a create or an update; provenance annotates that op
  // rather than replacing it.
  it('keeps the op badge alongside the provenance', async () => {
    const w = await mountWith([
      meta({ op: 'create', origin: { kind: 'copy', source: 'POL-1@draft' } }),
    ])

    expect(w.find('.timeline-badge').text()).toBe('create')
    expect(w.find('.timeline-origin').text()).toBe('copy from POL-1@draft')
  })

  it('marks only the copied rows in a mixed timeline', async () => {
    const w = await mountWith([
      meta({ version: 1, op: 'create' }),
      meta({ version: 2, origin: { kind: 'copy', source: 'POL-1@draft', definition: 'publish' } }),
      meta({ version: 3 }),
    ])

    const rows = w.findAll('.timeline-item')
    expect(rows).toHaveLength(3)
    expect(rows[0].find('.timeline-origin').exists()).toBe(false)
    expect(rows[1].find('.timeline-origin').text()).toBe('copy from POL-1@draft (publish)')
    expect(rows[2].find('.timeline-origin').exists()).toBe(false)
  })
})

// A restore is a WRITE to the BARE id (the restore route takes no face), so it
// is offered only while the timeline is the bare face's — which is what the
// served row's `_self` says, not the world. Under a world that resolved to the
// bare face, restoring puts back exactly what is shown; under one that served
// a NON-bare face, restoring would land that face's version on the bare one.
describe('HistoryView restore follows the face on screen', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockRouteQuery.value = {}
  })

  it('offers Restore in the default world (the control)', async () => {
    const w = await mountWith([meta({ op: 'update' })], {
      _actions: { update: true }, _self: '/api/v1/policys/POL-1',
    })
    expect(w.text()).toContain('Restore')
  })

  it('offers Restore under a world that served the BARE face', async () => {
    // The world is not what withdraws the affordance; the face is. A chain hit
    // on the bare face restores what it shows.
    mockRouteQuery.value = { world: 'editorial' }
    const w = await mountWith([meta({ op: 'update' })], {
      _actions: { update: true }, _self: '/api/v1/policys/POL-1',
    })
    expect(w.text()).toContain('Restore')
  })

  it('withdraws Restore while a NON-bare face is on screen', async () => {
    mockRouteQuery.value = { world: 'published' }
    const w = await mountWith([meta({ op: 'update' })], {
      _actions: { update: true }, _self: '/api/v1/policys/POL-1@published',
    })
    expect(w.text()).not.toContain('Restore')
  })
})
