import { describe, it, expect, beforeEach } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { collapsedStorageKey, filtersAdmit, groupRows, useListGrouping } from './useListGrouping'
import type { Entity, EntityType, ListGroupBy, ListResponse } from '@/types'

const taskType = {
  name: 'task',
  properties: {
    title: { type: 'string' },
    status: {
      type: 'enum',
      values: ['todo', 'doing', 'done'],
      labels: { todo: 'To do' },
    },
    owner: { type: 'string' },
    due: { type: 'date' },
    at: { type: 'datetime' },
  },
} as unknown as EntityType

function task(id: string, properties: Record<string, unknown>): Entity {
  return { id, type: 'task', properties: { title: id, ...properties }, relations: {} }
}

const now = new Date('2026-06-10T10:00:00Z')
const tz = 'Europe/Amsterdam'

function ids(sections: { id: string; items: Entity[] }[]) {
  return sections.map((s) => [s.id, s.items.map((e) => e.id)])
}

describe('groupRows by an enum', () => {
  const groupBy: ListGroupBy = {
    property: 'status',
    max_rows: 500,
    groups: [{ value: 'doing', label: 'In progress', color: 'green' }],
  }

  it('orders sections by the enum, not by the rows', () => {
    const rows = [
      task('T-1', { status: 'done' }),
      task('T-2', { status: 'todo' }),
      task('T-3', { status: 'done' }),
    ]
    const sections = groupRows(rows, groupBy, taskType, { now, tz })
    expect(ids(sections)).toEqual([
      ['value:todo', ['T-2']],
      ['value:doing', []],
      ['value:done', ['T-1', 'T-3']],
    ])
  })

  it('drops an empty section the filters rule out', () => {
    const sections = groupRows([task('T-1', { status: 'todo' })], groupBy, taskType, {
      now,
      tz,
      filterParams: { 'filter[status][ne]': 'done' },
    })
    expect(ids(sections)).toEqual([
      ['value:todo', ['T-1']],
      ['value:doing', []],
    ])
  })

  it('keeps a section that holds rows, whatever the filters say', () => {
    const sections = groupRows([task('T-1', { status: 'done' })], groupBy, taskType, {
      now,
      tz,
      filterParams: { 'filter[status][eq]': 'todo' },
    })
    expect(ids(sections)).toEqual([
      ['value:todo', []],
      ['value:done', ['T-1']],
    ])
  })

  it('titles a section from groups:, then the schema label, then the value', () => {
    const sections = groupRows([], groupBy, taskType, { now, tz })
    expect(sections.map((s) => [s.title, s.color])).toEqual([
      ['To do', undefined],
      ['In progress', 'green'],
      ['done', undefined],
    ])
  })

  it('keeps an empty group as a place to add to, and prefills its value', () => {
    const doing = groupRows([], groupBy, taskType, { now, tz }).find((s) => s.id === 'value:doing')
    expect(doing?.items).toEqual([])
    expect(doing?.prefill).toEqual({ status: 'doing' })
  })

  it('leaves empty groups out when asked', () => {
    const rows = [task('T-1', { status: 'done' })]
    expect(ids(groupRows(rows, groupBy, taskType, { now, tz, includeEmpty: false }))).toEqual([
      ['value:done', ['T-1']],
    ])
  })

  it('puts rows without a value in a trailing (none) section, only when there are some', () => {
    const rows = [task('T-1', {}), task('T-2', { status: 'todo' }), task('T-3', { status: '' })]
    const sections = groupRows(rows, groupBy, taskType, { now, tz })
    const last = sections[sections.length - 1]
    expect(last).toMatchObject({ id: 'none', title: '(none)' })
    expect(last.items.map((e) => e.id)).toEqual(['T-1', 'T-3'])
    expect(last.prefill).toBeUndefined()
    expect(groupRows([], groupBy, taskType, { now, tz }).some((s) => s.id === 'none')).toBe(false)
  })

  it('keeps a value the enum does not declare, after the declared ones', () => {
    const rows = [task('T-1', { status: 'blocked' })]
    const sections = groupRows(rows, groupBy, taskType, { now, tz })
    expect(sections.map((s) => s.id)).toEqual([
      'value:todo',
      'value:doing',
      'value:done',
      'value:blocked',
    ])
  })
})

describe('groupRows by a plain property', () => {
  it('makes one section per distinct value, in row order', () => {
    const rows = [
      task('T-1', { owner: 'ann' }),
      task('T-2', { owner: 'bob' }),
      task('T-3', { owner: 'ann' }),
    ]
    const sections = groupRows(rows, { property: 'owner', max_rows: 500 }, taskType, { now, tz })
    expect(ids(sections)).toEqual([
      ['value:ann', ['T-1', 'T-3']],
      ['value:bob', ['T-2']],
    ])
  })
})

describe('groupRows by date buckets', () => {
  const groupBy: ListGroupBy = {
    property: 'due',
    buckets: 'relative',
    labels: { today: 'Vandaag' },
    max_rows: 500,
  }

  it('sorts rows into buckets, leaving out empty ones', () => {
    const rows = [
      task('T-1', { due: '2026-06-01' }),
      task('T-2', { due: '2026-06-10' }),
      task('T-3', { due: '2026-09-01' }),
      task('T-4', {}),
    ]
    const sections = groupRows(rows, groupBy, taskType, { now, tz })
    expect(ids(sections)).toEqual([
      ['bucket:overdue', ['T-1']],
      ['bucket:today', ['T-2']],
      ['bucket:later', ['T-3']],
      ['bucket:no_date', ['T-4']],
    ])
    expect(sections.map((s) => s.title)).toEqual(['Overdue', 'Vandaag', 'Later', 'No date'])
    expect(sections[0].color).toBe('red')
  })

  it('prefills the day for today and tomorrow on a date property', () => {
    const rows = [task('T-1', { due: '2026-06-10' }), task('T-2', { due: '2026-06-01' })]
    const sections = groupRows(rows, groupBy, taskType, { now, tz })
    expect(sections.find((s) => s.bucket === 'today')?.prefill).toEqual({ due: '2026-06-10' })
    expect(sections.find((s) => s.bucket === 'overdue')?.prefill).toBeUndefined()
  })

  it('prefills nothing on a datetime property, which needs a time too', () => {
    const rows = [task('T-1', { at: '2026-06-10T12:00:00Z' })]
    const sections = groupRows(
      rows,
      { property: 'at', buckets: 'relative', max_rows: 500 },
      taskType,
      { now, tz }
    )
    expect(sections[0].bucket).toBe('today')
    expect(sections[0].prefill).toBeUndefined()
  })
})

describe('useListGrouping', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  const byStatus: ListGroupBy = { property: 'status', max_rows: 2 }

  function setup(response: ListResponse<Entity>, groupBy: ListGroupBy | null = byStatus) {
    const scope = effectScope()
    const listId = ref('tasks')
    const grouping = scope.run(() =>
      useListGrouping({
        listId: () => listId.value,
        groupBy: () => groupBy ?? undefined,
        entityType: () => taskType,
        response: () => response,
      })
    )!
    return { grouping, listId, scope }
  }

  function response(
    rows: Entity[],
    meta: Partial<ListResponse<Entity>['meta']> = {}
  ): ListResponse<Entity> {
    return {
      data: rows,
      meta: { total: rows.length, page: 1, per_page: rows.length, has_more: false, ...meta },
    }
  }

  it('reports truncation when the server had rows past max_rows', () => {
    const rows = [task('T-1', { status: 'todo' }), task('T-2', { status: 'done' })]
    const { grouping, scope } = setup(response(rows, { total: 7, has_more: true }))
    expect(grouping.truncated.value).toBe(true)
    expect(grouping.total.value).toBe(7)
    scope.stop()
  })

  it('reports a set that fit as complete', () => {
    const { grouping, scope } = setup(response([task('T-1', { status: 'todo' })]))
    expect(grouping.truncated.value).toBe(false)
    scope.stop()
  })

  it('is not grouped and has no sections without group_by', () => {
    const { grouping, scope } = setup(response([task('T-1', {})], { has_more: true }), null)
    expect(grouping.grouped.value).toBe(false)
    expect(grouping.sections.value).toEqual([])
    expect(grouping.truncated.value).toBe(false)
    scope.stop()
  })

  it('remembers closed sections per list', async () => {
    const { grouping, listId, scope } = setup(response([task('T-1', { status: 'todo' })]))
    grouping.setCollapsed('value:done', true)
    expect(JSON.parse(localStorage.getItem(collapsedStorageKey('tasks'))!)).toEqual(['value:done'])
    expect(grouping.sections.value.find((s) => s.id === 'value:done')?.collapsed).toBe(true)

    listId.value = 'other'
    await nextTick()
    expect(grouping.sections.value.find((s) => s.id === 'value:done')?.collapsed).toBe(false)
    scope.stop()

    // A fresh mount reads the stored choice back.
    const again = setup(response([]))
    expect(again.grouping.sections.value.find((s) => s.id === 'value:done')?.collapsed).toBe(true)
    again.scope.stop()
  })

  it('survives a corrupt stored entry', () => {
    localStorage.setItem(collapsedStorageKey('tasks'), '{not json')
    const { grouping, scope } = setup(response([]))
    expect(grouping.sections.value.every((s) => !s.collapsed)).toBe(true)
    scope.stop()
  })
})

describe('filtersAdmit', () => {
  it.each([
    [undefined, 'done', true],
    [{}, 'done', true],
    [{ 'filter[status][ne]': 'done' }, 'done', false],
    [{ 'filter[status][ne]': 'done' }, 'todo', true],
    [{ 'filter[status]': 'todo' }, 'done', false],
    [{ 'filter[status][eq]': 'todo' }, 'todo', true],
    [{ 'filter[status][in]': 'todo,doing' }, 'done', false],
    [{ 'filter[status][in][]': ['todo', 'doing'] }, 'doing', true],
    [{ 'filter[status][gt]': 'm' }, 'done', true],
    [{ 'filter[other][ne]': 'done' }, 'done', true],
  ])('%j admits %s: %s', (params, value, want) => {
    expect(filtersAdmit(params, 'status', value)).toBe(want)
  })
})
