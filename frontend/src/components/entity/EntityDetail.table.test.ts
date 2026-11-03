// `display: table` cells render through the dense routing a list cell uses
// (BUG-UI5TT6). They used to badge every typed value, so a title or a date
// appeared as a gray, capitalized pill.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity, EntityType } from '@/types'
import type { ViewCell, ViewColumn, ViewResponse, ViewRow } from '@/api'
import { _setEntityPluralForTest } from '@/api/entities'

const fetchViewMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: () => Promise.resolve([]),
}))
vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRoute: () => ({
    query: {},
    params: {},
    path: '/entity/project/P-1',
    fullPath: '/entity/project/P-1',
  }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}))

const taak = {
  name: 'taak',
  label: 'Taak',
  properties: {
    titel: { type: 'string' },
    status: { type: 'enum', values: ['todo', 'done'] },
    labels: { type: 'enum', values: ['a', 'b', 'c'], list: true },
    notities: { type: 'string', list: true },
    vervaldatum: { type: 'date' },
    klaar: { type: 'boolean' },
    bijlagen: { type: 'file', max: 5 },
  },
} as unknown as EntityType

/** A one-row table section: one column and one cell per entry. */
function view(
  cols: { column: ViewColumn; cell: ViewCell }[],
  row: Partial<ViewRow> = {}
): ViewResponse {
  const entry: Entity = { id: 'P-1', type: 'project', properties: { titel: 'Project' } }
  return {
    entry,
    sections: [
      {
        heading: 'Taken',
        sectionId: 'taken',
        display: 'table',
        isEmpty: false,
        isGrouped: false,
        hasContent: false,
        columns: cols.map((c) => c.column),
        rows: [{ entityId: 'T-1', entityType: 'taak', cells: cols.map((c) => c.cell), ...row }],
      },
    ],
  }
}

describe('EntityDetail table section cells', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    fetchViewMock.mockReset()
    const schema = useSchemaStore()
    schema.entityTypes.set('project', {
      name: 'project',
      label: 'Project',
      properties: {},
    } as never)
    schema.entityTypes.set('taak', taak)
    schema.loaded = true
    _setEntityPluralForTest('taak', 'taken')
  })

  async function cells(cols: { column: ViewColumn; cell: ViewCell }[], row?: Partial<ViewRow>) {
    fetchViewMock.mockResolvedValue(view(cols, row))
    const wrapper = mount(EntityDetail, {
      props: { entityType: 'project', entityId: 'P-1' },
      global: {
        plugins: [pinia, PiniaColada],
        stubs: { DocumentsPanel: true, CommentsPanel: true },
      },
    })
    await flushPromises()
    return wrapper.findAll('table.data-table tbody tr td')
  }

  it('renders a title as its link text, without a badge', async () => {
    const title = 'Bewijs van competentie'
    const [td] = await cells([
      {
        column: { property: 'titel', link: 'detail' },
        cell: { values: [title], propType: 'string', widget: 'text', link: '/entity/taak/T-1' },
      },
    ])
    expect(td.find('a').text()).toBe(title)
    expect(td.find('.badge').exists()).toBe(false)
  })

  it('badges an enum, and every value of a list enum', async () => {
    const [status, labels] = await cells([
      {
        column: { property: 'status' },
        cell: { values: ['todo'], propType: 'enum', widget: 'select' },
      },
      {
        column: { property: 'labels' },
        cell: { values: ['a', 'b', 'c'], propType: 'enum', widget: 'multi-select' },
      },
    ])
    expect(status.findAll('.badge')).toHaveLength(1)
    expect(labels.findAll('.badge')).toHaveLength(3)
  })

  it('keeps every value of a non-enum list', async () => {
    const values = ['een', 'twee']
    const [td] = await cells([
      { column: { property: 'notities' }, cell: { values, propType: 'string', widget: 'text' } },
    ])
    expect(td.text()).toBe(values.join(', '))
  })

  it('formats a date without a badge', async () => {
    const [td] = await cells([
      {
        column: { property: 'vervaldatum' },
        cell: { values: ['2026-09-26'], propType: 'date', widget: 'date' },
      },
    ])
    expect(td.find('.badge').exists()).toBe(false)
    expect(td.text()).toMatch(/2026/)
    expect(td.text()).not.toBe('2026-09-26')
  })

  it('renders a boolean as Yes/No text, not a checkbox', async () => {
    const [yes, no] = await cells([
      {
        column: { property: 'klaar' },
        cell: { values: ['true'], propType: 'boolean', widget: 'checkbox' },
      },
      {
        column: { property: 'klaar' },
        cell: { values: ['false'], propType: 'boolean', widget: 'checkbox' },
      },
    ])
    expect(yes.find('input').exists()).toBe(false)
    expect(yes.text()).toBe('Yes')
    expect(no.text()).toBe('No')
  })

  it('keeps every value of an undeclared property', async () => {
    const values = ['x', 'y']
    const [td] = await cells([
      { column: { property: 'onbekend' }, cell: { values, widget: 'text' } },
    ])
    expect(td.text()).toBe(values.join(', '))
  })

  it('keeps the joined string for a relation column', async () => {
    const values = ['Jan', 'Piet']
    const [td] = await cells([{ column: { relation: 'toegewezen' }, cell: { values } }])
    expect(td.text()).toBe(values.join(', '))
  })

  it('leaves an empty cell blank', async () => {
    const [status, date] = await cells([
      { column: { property: 'status' }, cell: { values: [], propType: 'enum', widget: 'select' } },
      {
        column: { property: 'vervaldatum' },
        cell: { values: [], propType: 'date', widget: 'date' },
      },
    ])
    expect(status.text()).toBe('')
    expect(date.text()).toBe('')
  })

  // A file column shows the file's name and links to its download, the way
  // the entity page does, never the storage path.
  describe('file cells', () => {
    const path = (name: string) => `attachments/T-1/bijlagen/c54fb91819f593e9/${name}`

    it('shows one file as its name, linked to the download', async () => {
      const name = 'hersteltest-q2.pdf'
      const [td] = await cells([
        { column: { property: 'bijlagen' }, cell: { values: [path(name)], widget: 'file' } },
      ])
      const link = td.find('a.file-cell-file')
      expect(td.text()).toBe(name)
      expect(td.text()).not.toContain('attachments/')
      expect(link.attributes('href')).toBe(`/api/v1/taken/T-1/_attachments/bijlagen/${name}`)
      expect(link.attributes('title')).toBe(name)
      expect(td.find('img').exists()).toBe(false)
    })

    it('links to the face the row was served in', async () => {
      const name = 'a.pdf'
      const [td] = await cells(
        [{ column: { property: 'bijlagen' }, cell: { values: [path(name)], widget: 'file' } }],
        { _self: '/api/v1/taken/T-1%40published' }
      )
      expect(td.find('a.file-cell-file').attributes('href')).toBe(
        `/api/v1/taken/${encodeURIComponent('T-1@published')}/_attachments/bijlagen/${name}`
      )
    })

    it('summarizes several files and lists every name', async () => {
      const names = ['b.pdf', 'a.docx']
      const [td] = await cells([
        { column: { property: 'bijlagen' }, cell: { values: names.map(path), widget: 'file' } },
      ])
      const summary = td.find('summary')
      expect(summary.text()).toBe(`${names.length} files`)
      expect(summary.attributes('title')).toBe([...names].sort().join('\n'))
      expect(td.findAll('li a').map((a) => a.text())).toEqual([...names].sort())
    })

    it('shows the name without a nested link in a linked column', async () => {
      const name = 'a.pdf'
      const [td] = await cells([
        {
          column: { property: 'bijlagen', link: 'detail' },
          cell: { values: [path(name)], widget: 'file', link: '/entity/taak/T-1' },
        },
      ])
      expect(td.findAll('a')).toHaveLength(1)
      expect(td.find('a').attributes('href')).toBe('/entity/taak/T-1')
      expect(td.text()).toBe(name)
    })

    it('leaves an empty file cell blank', async () => {
      const [td] = await cells([
        { column: { property: 'bijlagen' }, cell: { values: [], widget: 'file' } },
      ])
      expect(td.text()).toBe('')
    })
  })
})
