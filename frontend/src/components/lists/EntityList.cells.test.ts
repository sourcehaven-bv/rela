import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { defaultRegistry } from '@/widgets/registry'
import type { Entity, ListResponse } from '@/types'

// Cell rendering moved from a string-only path (formatCellValue interpolated
// as text) onto the widget registry in mode:'display' -- TKT-S9C14S. These
// tests pin the resulting behaviour at the two EntityList render sites
// (desktop <td> and mobile card), including the routings that are
// deliberately NOT the matching widget.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/things', name: 'list' }),
}))

describe('EntityList cell rendering via widgets', () => {
  const listId = 'things-list'
  const entityType = 'thing'

  function seedSchema(
    properties: Record<string, unknown>,
    columns: { property?: string; relation?: string; label?: string }[]
  ) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Things',
      entity: entityType,
      columns,
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Thing',
      properties,
    } as never)
  }

  function seedEntities(entities: Entity[]) {
    const response: ListResponse<Entity> = {
      data: entities,
      meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listEntitiesMock.mockResolvedValue(response)
  }

  async function mountList() {
    const wrapper = mount(EntityList, {
      props: { listId },
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'things')
    listEntitiesMock.mockReset()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders a string cell as plain text', async () => {
    seedSchema({ title: { type: 'string' } }, [{ property: 'title', label: 'Title' }])
    seedEntities([{ id: 'T-1', type: entityType, properties: { title: 'Hello' } }])
    const wrapper = await mountList()
    expect(wrapper.text()).toContain('Hello')
  })

  it('renders an enum cell as a Badge, as before the migration', async () => {
    seedSchema({ status: { type: 'enum', values: ['open', 'done'] } }, [
      { property: 'status', label: 'Status' },
    ])
    seedEntities([{ id: 'T-1', type: entityType, properties: { status: 'open' } }])
    const wrapper = await mountList()
    expect(wrapper.find('.badge').exists()).toBe(true)
    expect(wrapper.text()).toContain('open')
  })

  it('renders a list-valued enum as one Badge per value', async () => {
    seedSchema({ tags: { type: 'enum', values: ['a', 'b'], list: true } }, [
      { property: 'tags', label: 'Tags' },
    ])
    seedEntities([{ id: 'T-1', type: entityType, properties: { tags: ['a', 'b'] } }])
    const wrapper = await mountList()
    expect(wrapper.findAll('.badge').length).toBe(2)
  })

  it('keeps boolean cells as Yes/No text, not a checkbox (searchable/copy-pasteable)', async () => {
    seedSchema({ done: { type: 'boolean' } }, [{ property: 'done', label: 'Done' }])
    seedEntities([
      { id: 'T-1', type: entityType, properties: { done: true } },
      { id: 'T-2', type: entityType, properties: { done: false } },
    ])
    const wrapper = await mountList()
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('Yes')
    expect(rows[1].text()).toContain('No')
    // No rendered checkbox in the data cells (the select-all/row checkboxes
    // only exist when the list has actions, which this config does not).
    expect(wrapper.find('tbody input[type="checkbox"]').exists()).toBe(false)
  })

  it('renders a date cell formatted, not as the raw stored string', async () => {
    seedSchema({ due: { type: 'date' } }, [{ property: 'due', label: 'Due' }])
    seedEntities([{ id: 'T-1', type: entityType, properties: { due: '2026-03-04' } }])
    const wrapper = await mountList()
    expect(wrapper.text()).not.toContain('2026-03-04T')
    expect(wrapper.text()).toMatch(/2026/)
  })

  it('renders a relation cell as joined titles (unchanged string path)', async () => {
    seedSchema({ title: { type: 'string' } }, [
      { property: 'title', label: 'Title' },
      { relation: 'blocks', label: 'Blocks' },
    ])
    const response: ListResponse<Entity> = {
      data: [
        {
          id: 'T-1',
          type: entityType,
          properties: { title: 'A' },
          relations: { blocks: ['T-9'] },
        },
      ],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      // Display names come from the backend-computed `_title`, never
      // properties.title (BUG-1P88YM / entityDisplayTitle).
      included: {
        'T-9': {
          id: 'T-9',
          type: entityType,
          _title: 'Blocked one',
          properties: { title: 'Blocked one' },
        },
      },
    }
    listEntitiesMock.mockResolvedValue(response)
    const wrapper = await mountList()
    expect(wrapper.text()).toContain('Blocked one')
  })

  it('renders the lock marker for an inaccessible cell instead of a widget', async () => {
    seedSchema({ secret: { type: 'string' } }, [{ property: 'secret', label: 'Secret' }])
    seedEntities([
      {
        id: 'T-1',
        type: entityType,
        properties: {},
        inaccessible: [{ name: 'secret', reason: 'encrypted' }],
      } as unknown as Entity,
    ])
    const wrapper = await mountList()
    expect(wrapper.find('.inaccessible-cell').exists()).toBe(true)
  })

  it('does not warn when rendering any built-in property type', async () => {
    // A supportedPropertyTypes mismatch console.warns once per resolve; if
    // resolution leaked into the per-cell path this would fire per row.
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    seedSchema(
      {
        s: { type: 'string' },
        d: { type: 'date' },
        dt: { type: 'datetime' },
        i: { type: 'integer' },
        b: { type: 'boolean' },
        e: { type: 'enum', values: ['x'] },
        f: { type: 'file' },
      },
      [
        { property: 's' },
        { property: 'd' },
        { property: 'dt' },
        { property: 'i' },
        { property: 'b' },
        { property: 'e' },
        { property: 'f' },
      ]
    )
    seedEntities([
      {
        id: 'T-1',
        type: entityType,
        properties: {
          s: 'a',
          d: '2026-01-01',
          dt: '2026-01-01T10:00:00Z',
          i: 3,
          b: true,
          e: 'x',
          f: 'doc.pdf',
        },
      },
    ])
    await mountList()
    expect(warn).not.toHaveBeenCalled()
    warn.mockRestore()
  })

  // --- Regressions caught in review: list-ness must not erase the type's
  // formatter, and cells must stay quiet when empty. ---

  it('renders a list-valued date as joined text, not as enum badges', async () => {
    seedSchema({ d: { type: 'date', list: true } }, [{ property: 'd' }])
    seedEntities([
      { id: 'T-1', type: entityType, properties: { d: ['2026-01-01', '2026-02-02'] } },
    ])
    const wrapper = await mountList()
    const cell = wrapper.find('tbody td')
    expect(cell.find('.badge').exists()).toBe(false)
    expect(cell.text()).toContain('2026-01-01, 2026-02-02')
  })

  it('renders a list-valued rrule as its text form, not an em-dash', async () => {
    // Routing list-ness ahead of the type sent this to MultiSelectWidget,
    // which em-dashed it -- the value vanished entirely.
    seedSchema({ r: { type: 'rrule', list: true } }, [{ property: 'r' }])
    seedEntities([{ id: 'T-1', type: entityType, properties: { r: ['FREQ=DAILY'] } }])
    const wrapper = await mountList()
    const cell = wrapper.find('tbody td')
    expect(cell.text()).not.toContain('—')
    expect(cell.text()).toContain('every day')
  })

  it('leaves an empty list-valued cell blank, not an em-dash', async () => {
    // MultiSelectWidget renders '—' for an empty array (RR-UD2C), which is a
    // detail-view contract. formatCellValue documents the opposite for cells:
    // "blank table cells stay visually quiet".
    seedSchema({ tags: { type: 'enum', values: ['a'], list: true } }, [{ property: 'tags' }])
    seedEntities([{ id: 'T-1', type: entityType, properties: { tags: [] } }])
    const wrapper = await mountList()
    expect(wrapper.find('tbody td').text()).not.toContain('—')
  })

  it('still badges a non-empty list-valued enum', async () => {
    // The empty-array fix must not disable enum badging.
    seedSchema({ tags: { type: 'enum', values: ['a', 'b'], list: true } }, [{ property: 'tags' }])
    seedEntities([{ id: 'T-1', type: entityType, properties: { tags: ['a', 'b'] } }])
    const wrapper = await mountList()
    expect(wrapper.findAll('tbody .badge').length).toBe(2)
  })

  it('resolves each widget once per COLUMN, not once per cell (RR-UD2A)', async () => {
    // 50 rows x 3 columns = 150 cells. Resolution happens in a computed keyed
    // on the column set, so it must run 3 times. If this regresses to per-cell,
    // every supportedPropertyTypes mismatch also becomes one console.warn per
    // row per render.
    seedSchema(
      { a: { type: 'string' }, b: { type: 'date' }, c: { type: 'enum', values: ['x'] } },
      [{ property: 'a' }, { property: 'b' }, { property: 'c' }]
    )
    seedEntities(
      Array.from({ length: 50 }, (_, i) => ({
        id: `T-${i}`,
        type: entityType,
        properties: { a: 'v', b: '2026-01-01', c: 'x' },
      }))
    )
    const spy = vi.spyOn(defaultRegistry, 'resolveFromHint')
    const wrapper = await mountList()
    expect(wrapper.findAll('tbody tr').length).toBe(50)
    expect(spy).toHaveBeenCalledTimes(3)
    spy.mockRestore()
  })

  // --- The mobile card render site. Structurally a duplicate of the desktop
  // <td>, so it can silently drift; nothing covered it before. ---

  describe('mobile card render site', () => {
    let originalMatchMedia: typeof window.matchMedia

    beforeEach(() => {
      originalMatchMedia = window.matchMedia
      // isMobile is seeded from matchMedia at setup, so stub before mount.
      window.matchMedia = ((query: string) =>
        ({
          matches: query.includes('max-width: 768px'),
          media: query,
          addEventListener: () => {},
          removeEventListener: () => {},
        }) as unknown as MediaQueryList) as typeof window.matchMedia
    })

    afterEach(() => {
      window.matchMedia = originalMatchMedia
    })

    it('renders card fields through widgets, matching the desktop cell', async () => {
      seedSchema({ title: { type: 'string' }, status: { type: 'enum', values: ['open'] } }, [
        { property: 'title', label: 'Title' },
        { property: 'status', label: 'Status' },
      ])
      seedEntities([
        { id: 'T-1', type: entityType, properties: { title: 'Hello', status: 'open' } },
      ])
      const wrapper = await mountList()
      expect(wrapper.find('.mobile-card').exists()).toBe(true)
      // Column 0 is the card title; the enum is a badge in the field list.
      expect(wrapper.find('.mobile-card').text()).toContain('Hello')
      expect(wrapper.find('.mobile-card .badge').exists()).toBe(true)
    })

    it('keeps boolean card fields as Yes/No, matching desktop', async () => {
      seedSchema({ title: { type: 'string' }, done: { type: 'boolean' } }, [
        { property: 'title', label: 'Title' },
        { property: 'done', label: 'Done' },
      ])
      seedEntities([
        { id: 'T-1', type: entityType, properties: { title: 'A', done: false } },
      ])
      const wrapper = await mountList()
      expect(wrapper.find('.mobile-card').text()).toContain('No')
    })

    it('still hides a column whose value is empty (the emptiness predicate)', async () => {
      seedSchema({ title: { type: 'string' }, note: { type: 'string' } }, [
        { property: 'title', label: 'Title' },
        { property: 'note', label: 'Note' },
      ])
      seedEntities([{ id: 'T-1', type: entityType, properties: { title: 'A', note: '' } }])
      const wrapper = await mountList()
      expect(wrapper.find('.mobile-card').text()).not.toContain('Note')
    })

    it('keeps a false boolean visible rather than treating it as empty', async () => {
      seedSchema({ title: { type: 'string' }, done: { type: 'boolean' } }, [
        { property: 'title', label: 'Title' },
        { property: 'done', label: 'Done' },
      ])
      seedEntities([{ id: 'T-1', type: entityType, properties: { title: 'A', done: false } }])
      const wrapper = await mountList()
      expect(wrapper.find('.mobile-card').text()).toContain('Done')
    })
  })

  it('does not issue image requests for a file column (no FileWidget in cells)', async () => {
    seedSchema({ doc: { type: 'file' } }, [{ property: 'doc', label: 'Doc' }])
    seedEntities([{ id: 'T-1', type: entityType, properties: { doc: 'picture.png' } }])
    const wrapper = await mountList()
    // FileWidget's display branch renders <img> previews; routing file->text
    // keeps a table from issuing one image request per row.
    expect(wrapper.find('tbody img').exists()).toBe(false)
    expect(wrapper.text()).toContain('picture.png')
  })
})
