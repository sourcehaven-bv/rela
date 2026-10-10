import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RelatedSectionRows from './RelatedSectionRows.vue'
import { useSchemaStore } from '@/stores/schema'
import type { ViewEntity } from '@/api'

// Renders `to` as a real href so a test can assert the destination.
vi.mock('vue-router', () => ({
  RouterLink: {
    props: ['to'],
    template: '<a :href="href"><slot /></a>',
    computed: {
      href(this: { to: { path: string; hash?: string; query?: Record<string, string> } }) {
        const qs = new URLSearchParams(this.to.query ?? {}).toString()
        return `${this.to.path}${qs ? `?${qs}` : ''}${this.to.hash ?? ''}`
      },
    },
  },
}))

function row(over: Partial<ViewEntity> & Pick<ViewEntity, 'id'>): ViewEntity {
  return { type: 'task', title: over.id, hasContent: false, ...over }
}

describe('RelatedSectionRows', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useSchemaStore().entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { due: { type: 'date' }, done: { type: 'boolean' } },
    } as never)
  })

  it("links an owned row to its owner's page, anchored at the row", () => {
    const w = mount(RelatedSectionRows, {
      props: {
        entities: [
          row({
            id: 'TASK-2',
            _owner: { id: 'TASK-1', type: 'task', title: 'Plan', relation: 'subtask' },
          }),
        ],
        world: 'nl',
      },
    })
    expect(w.find('a').attributes('href')).toBe('/entity/task/TASK-1?world=nl#TASK-2')
  })

  it("renders a row owned by the page's entity as static text", () => {
    const owned = row({
      id: 'TASK-2',
      _owner: { id: 'TASK-1', type: 'task', title: 'Plan', relation: 'subtask' },
    })
    const other = row({
      id: 'TASK-4',
      _owner: { id: 'TASK-9', type: 'task', title: 'Other', relation: 'subtask' },
    })
    const w = mount(RelatedSectionRows, {
      props: { entities: [owned, other], pageEntity: 'TASK-1' },
    })
    const rows = w.findAll('.rl-related-row')
    expect(rows[0].find('a').exists()).toBe(false)
    expect(rows[0].find('button').exists()).toBe(false)
    expect(rows[0].find('span.rl-related-row__title').text()).toBe('TASK-2')
    // A row owned by another entity still links to that owner.
    expect(rows[1].find('a').attributes('href')).toBe('/entity/task/TASK-9#TASK-4')
  })

  it('links a row without an owner to its own page', () => {
    const w = mount(RelatedSectionRows, { props: { entities: [row({ id: 'TASK-3' })] } })
    expect(w.find('a').attributes('href')).toBe('/entity/task/TASK-3')
  })

  it('anchors each row at its entity id', () => {
    const w = mount(RelatedSectionRows, { props: { entities: [row({ id: 'TASK-3' })] } })
    expect(w.find('#TASK-3').exists()).toBe(true)
  })

  it('shows fields as text and leaves out empty and unreadable ones', () => {
    const w = mount(RelatedSectionRows, {
      props: {
        entities: [
          row({
            id: 'TASK-3',
            title: 'Book venue',
            _props: { done: true, due: '' },
            fields: [
              { property: 'done', label: 'Done', values: ['true'] },
              { property: 'due', label: 'Due', values: [] },
              { property: 'secret', label: 'Secret', inaccessible: true, values: ['x'] },
            ],
          }),
        ],
      },
    })
    const meta = w.find('.rl-related-row__meta')
    expect(meta.text()).toBe('Done Yes')
    expect(w.text()).toContain('Book venue')
    expect(w.text()).not.toContain('Secret')
  })
})
