import { describe, it, expect } from 'vitest'
import { leadWith, pageTabView, viewEntityTypes, type ViewConfigs } from './useViewEntityTypes'
import type { CalendarConfig, GanttConfig, KanbanConfig, PageTabView, SidebarPage } from '@/types'

const pipeline: KanbanConfig = {
  entity: 'deal',
  column_property: 'stage',
  card: { title: 'title' },
}

const agenda: CalendarConfig = {
  default_view: 'month',
  week_start: 'monday',
  day_start: '08:00',
  day_end: '18:00',
  max_events_per_day: 3,
  sources: [
    { entity: 'meeting', date: 'd', max_span: 0 },
    { entity: 'task', date: 'due', max_span: 0 },
    { entity: 'meeting', date: 'end', max_span: 0 },
  ],
}

const roadmap: GanttConfig = {
  hierarchy: ['contains'],
  multi_parent: 'first',
  on_cycle: 'prune',
  default_depth: 1,
  max_depth: 3,
  max_nodes: 100,
  sources: { project: {}, task: {} },
}

const configs: ViewConfigs = {
  getList: (id) => (id === 'orgs' ? { entity: 'org', columns: [] } : undefined),
  getKanban: (id) => (id === 'pipeline' ? pipeline : undefined),
  getCalendar: (id) => (id === 'agenda' ? agenda : undefined),
  getGantt: (id) => (id === 'roadmap' ? roadmap : undefined),
}

describe('viewEntityTypes', () => {
  it.each<[PageTabView, string, string[]]>([
    ['list', 'orgs', ['org']],
    ['kanban', 'pipeline', ['deal']],
    ['calendar', 'agenda', ['meeting', 'task']],
    ['gantt', 'roadmap', ['project', 'task']],
    ['list', 'nosuch', []],
    ['dashboard', 'orgs', []],
    ['document', 'orgs', []],
  ])('%s %s shows %j', (view, id, want) => {
    expect(viewEntityTypes(view, id, configs)).toEqual(want)
  })
})

describe('pageTabView', () => {
  const pages: Record<string, SidebarPage> = {
    crm: {
      label: 'CRM',
      tabs: [
        { id: 'orgs', label: 'Orgs', view: 'list', target: 'orgs' },
        { id: 'home', label: 'Home', view: 'dashboard' },
      ],
    },
    org: {
      label: 'Org',
      entity_type: 'org',
      tabs: [{ id: 'deals', label: 'Deals', view: 'kanban', target: 'pipeline' }],
    },
  }

  it('reads the open tab of a page', () => {
    expect(pageTabView({ page: 'crm', tab: 'orgs' }, pages)).toEqual({ view: 'list', id: 'orgs' })
  })

  it('reads the tab of an entity page', () => {
    expect(pageTabView({ page: 'org', entity: 'ORG-1', tab: 'deals' }, pages)).toEqual({
      view: 'kanban',
      id: 'pipeline',
    })
  })

  it('has no tab on the two-segment entity page route', () => {
    expect(pageTabView({ page: 'org', tab: 'ORG-1' }, pages)).toBeUndefined()
  })

  it.each([
    [{ page: 'crm', tab: 'home' }],
    [{ page: 'crm', tab: 'nosuch' }],
    [{ page: 'crm' }],
    [{ page: 'nosuch', tab: 'orgs' }],
  ])('has no view for %j', (params) => {
    expect(pageTabView(params, pages)).toBeUndefined()
  })
})

describe('leadWith', () => {
  const rows = ['a', 'b', 'c', 'd'].map((type) => ({ type }))

  it('leads with the given types in row order', () => {
    expect(leadWith(rows, ['d', 'b'])).toEqual({
      lead: [{ type: 'b' }, { type: 'd' }],
      rest: [{ type: 'a' }, { type: 'c' }],
    })
  })

  it('ignores types not among the rows', () => {
    expect(leadWith(rows, ['x'])).toEqual({ lead: [], rest: rows })
  })

  it('keeps every row when no types are given', () => {
    expect(leadWith(rows, [])).toEqual({ lead: [], rest: rows })
  })
})
