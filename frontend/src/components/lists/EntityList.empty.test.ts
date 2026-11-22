import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'

// The empty list names the type by its plural label from the schema. A type
// without one gets a text without the type, never the type name with an "s"
// ("No persons found.").

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/people', name: 'list' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('person', 'people')
  listEntitiesMock.mockReset().mockResolvedValue({
    data: [],
    meta: { total: 0, page: 1, per_page: 25, has_more: false },
    included: {},
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountPeople(labelPlural?: string, emptyMessage?: string) {
  const schemaStore = useSchemaStore()
  schemaStore.lists.set('people', {
    id: 'people',
    title: 'People',
    entity: 'person',
    ...(emptyMessage === undefined ? {} : { empty_message: emptyMessage }),
    columns: [{ property: 'name', label: 'Name' }],
  } as never)
  schemaStore.entityTypes.set('person', {
    name: 'person',
    label: 'Person',
    plural: 'people',
    ...(labelPlural === undefined ? {} : { label_plural: labelPlural }),
    properties: { name: { type: 'string', values: null } },
  } as never)
  const wrapper = mount(withPageHeader(EntityList), {
    props: { listId: 'people' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

async function emptyText(labelPlural?: string, emptyMessage?: string): Promise<string> {
  const wrapper = await mountPeople(labelPlural, emptyMessage)
  const text = wrapper.find('.empty-state').text()
  wrapper.unmount()
  return text
}

async function placeholder(labelPlural?: string): Promise<string | undefined> {
  const wrapper = await mountPeople(labelPlural)
  const value = document
    .querySelector('input[type="search"], .search-box input')
    ?.getAttribute('placeholder')
  wrapper.unmount()
  return value ?? undefined
}

describe('EntityList empty state', () => {
  it('names the type by its plural label', async () => {
    expect(await emptyText('People')).toContain('No People found.')
  })

  it('leaves the type out when the schema has no plural label', async () => {
    const text = await emptyText()
    expect(text).toContain('Nothing found.')
    expect(text).not.toContain('persons')
  })

  it('treats a blank plural label as none', async () => {
    expect(await emptyText('  ')).toContain('Nothing found.')
  })

  it("uses the list's own empty_message when set", async () => {
    const text = await emptyText('People', 'Nobody has signed up yet.')
    expect(text).toContain('Nobody has signed up yet.')
    expect(text).not.toContain('No People found.')
  })

  it('names the type by its plural label in the search box', async () => {
    expect(await placeholder('People')).toBe('Search People...')
    expect(await placeholder()).toBe('Search...')
  })
})
