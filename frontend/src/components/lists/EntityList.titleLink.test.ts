import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'

// A title column the list marks with `link:` reads as a link, in the link
// colour (BUG-6WQ1U8S). Without `link:` the title stays row text. The colour
// itself is CSS, which Vitest does not apply; this pins the class it hangs on.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/invoices', name: 'list' }),
  RouterLink: { props: ['to'], template: '<a href="#"><slot /></a>' },
}))

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  listEntitiesMock.mockReset().mockResolvedValue({
    data: [
      {
        id: 'INV-1',
        type: 'invoice',
        title: 'HM021-0001',
        properties: { number: 'HM021-0001', total: 10 },
      },
    ],
    meta: { total: 1, page: 1, per_page: 25, has_more: false },
    included: {},
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountInvoices(link: boolean) {
  const schemaStore = useSchemaStore()
  schemaStore.lists.set('invoices', {
    id: 'invoices',
    title: 'Invoices',
    entity: 'invoice',
    columns: [
      { property: 'number', label: 'Number', ...(link ? { link: 'detail' } : {}) },
      { property: 'total', label: 'Total' },
    ],
  } as never)
  schemaStore.entityTypes.set('invoice', {
    name: 'invoice',
    label: 'Invoice',
    properties: {
      number: { type: 'string', values: null },
      total: { type: 'decimal', values: null },
    },
  } as never)
  const wrapper = mount(withPageHeader(EntityList), {
    props: { listId: 'invoices' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('EntityList title link', () => {
  it('marks the title as a link when the column sets link:', async () => {
    const wrapper = await mountInvoices(true)
    const title = wrapper.find('.rl-table-row__primary a')
    expect(title.exists()).toBe(true)
    expect(title.text()).toContain('HM021-0001')
    expect(title.classes()).toContain('entity-list__title-link')
    wrapper.unmount()
  })

  it('leaves the title as row text without link:', async () => {
    const wrapper = await mountInvoices(false)
    const title = wrapper.find('.rl-table-row__primary a')
    expect(title.exists()).toBe(true)
    expect(title.classes()).not.toContain('entity-list__title-link')
    wrapper.unmount()
  })
})
