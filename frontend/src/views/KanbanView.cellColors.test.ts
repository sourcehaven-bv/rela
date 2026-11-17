import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity } from '@/types'

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/risks' }),
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

function risk(id: string, impact: string, likelihood: string): Entity {
  return { id, type: 'risk', properties: { title: id, impact, likelihood }, relations: {} }
}

let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  localStorage.clear()
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('risk', 'risks')
  listAllEntitiesMock.mockReset().mockResolvedValue({
    data: [risk('R-1', 'high', 'likely')],
    meta: { total: 1, page: 1, per_page: 25, has_more: false },
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard() {
  const schema = useSchemaStore()
  schema.kanbans.set('risks', {
    entity: 'risk',
    title: 'Risks',
    column_property: 'impact',
    columns: [
      { value: 'low', label: 'Low impact' },
      { value: 'high', label: 'High impact' },
    ],
    swimlane_property: 'likelihood',
    swimlanes: [
      { value: 'likely', label: 'Likely' },
      { value: 'rare', label: 'Rare' },
    ],
    cell_colors: {
      high: {
        likely: { color: 'error', class: 'badge-red' },
        rare: { color: 'warning', label: 'Medium', class: 'badge-orange' },
      },
      low: { rare: { color: 'success', class: 'badge-green' } },
    },
    card: { title: 'title', fields: [] },
  } as never)
  schema.entityTypes.set('risk', {
    name: 'risk',
    properties: {
      title: { type: 'string' },
      impact: { type: 'enum', values: ['low', 'high'] },
      likelihood: { type: 'enum', values: ['rare', 'likely'] },
    },
  } as never)
  const wrapper = mount(KanbanView, {
    props: { id: 'risks' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('KanbanView cell colours', () => {
  it('paints each configured cell with its tokens and a text label', async () => {
    const wrapper = await mountBoard()

    const toned = wrapper.findAll('.rl-swimlane-column--toned')
    expect(toned).toHaveLength(3)
    // Lanes render in order (likely, rare); each lane's cells in column
    // order (low, high). The likely lane has only its high cell toned.
    const [likelyHigh, rareLow, rareHigh] = toned
    expect(likelyHigh.text()).toContain('Critical')
    expect(likelyHigh.text()).toContain('R-1')
    expect(likelyHigh.attributes('style')).toContain('--rl-swimlane-cell-bg: var(--rl-tag-red-bg)')
    expect(likelyHigh.attributes('style')).toContain('--rl-swimlane-cell-fg: var(--rl-tag-red-fg)')
    expect(rareLow.text()).toContain('Good')
    expect(rareHigh.text()).toContain('Medium')
  })

  it('marks the board toned, so plain cells keep the toned padding', async () => {
    const wrapper = await mountBoard()

    expect(wrapper.find('.rl-swimlane-board').classes()).toContain('rl-swimlane-board--toned')
  })

  it('drops the tone of a collapsed column', async () => {
    const wrapper = await mountBoard()

    await wrapper.get('button[aria-label="Collapse High impact"]').trigger('click')

    const toned = wrapper.findAll('.rl-swimlane-column--toned')
    expect(toned).toHaveLength(1)
    expect(toned[0].text()).toContain('Good')
    expect(wrapper.text()).not.toContain('Critical')
  })

  it('leaves a cell without a colour plain', async () => {
    const wrapper = await mountBoard()

    const cells = wrapper.findAll('.rl-swimlane-column')
    // likely/low is the first cell and has no colour.
    expect(cells[0].classes()).not.toContain('rl-swimlane-column--toned')
    expect(cells[0].attributes('style')).toBeUndefined()
  })
})
