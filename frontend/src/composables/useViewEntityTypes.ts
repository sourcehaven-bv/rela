import { computed } from 'vue'
import { useRoute, type RouteLocationNormalizedLoaded } from 'vue-router'
import { usePageStore } from '@/stores/pages'
import { useSchemaStore } from '@/stores/schema'
import { routeTabId } from '@/utils/pageContext'
import type {
  CalendarConfig,
  GanttConfig,
  KanbanConfig,
  ListConfig,
  PageTabView,
  SidebarPage,
} from '@/types'

/** The view configs a view's entity types are read from. */
export interface ViewConfigs {
  getList: (id: string) => ListConfig | undefined
  getKanban: (id: string) => KanbanConfig | undefined
  getCalendar: (id: string) => CalendarConfig | undefined
  getGantt: (id: string) => GanttConfig | undefined
}

/**
 * The entity types a view shows. A list and a board show one type. A
 * calendar and a gantt show the types of their sources. A gantt's sources are
 * a map, so their order means nothing. Any other view, and a view the config
 * does not declare, shows no known type.
 */
export function viewEntityTypes(view: PageTabView, id: string, configs: ViewConfigs): string[] {
  switch (view) {
    case 'list':
      return typesOf([configs.getList(id)?.entity])
    case 'kanban':
      return typesOf([configs.getKanban(id)?.entity])
    case 'calendar':
      return typesOf(configs.getCalendar(id)?.sources.map((s) => s.entity) ?? [])
    case 'gantt':
      return typesOf(Object.keys(configs.getGantt(id)?.sources ?? {}))
    default:
      return []
  }
}

function typesOf(types: Array<string | undefined>): string[] {
  return [...new Set(types.filter((t): t is string => !!t))]
}

/** The view and view id the open tab of a page shows, read from the route as PageView reads it. */
export function pageTabView(
  params: RouteLocationNormalizedLoaded['params'],
  pages: Record<string, SidebarPage>
): { view: PageTabView; id: string } | undefined {
  const page = pages[String(params.page)]
  const tabId = routeTabId(page, {
    tab: typeof params.tab === 'string' ? params.tab : undefined,
    entity: typeof params.entity === 'string' ? params.entity : undefined,
  })
  const tab = page?.tabs.find((t) => t.id === tabId)
  return tab?.target ? { view: tab.view, id: tab.target } : undefined
}

/* The standalone routes of the views that show known types, by route name. */
const routeViews: Record<string, PageTabView> = {
  list: 'list',
  kanban: 'kanban',
  calendar: 'calendar',
  gantt: 'gantt',
}

/**
 * The entity types the view on screen shows: a list, board, calendar or
 * gantt, on its own route or as the open tab of a page. Empty on every other
 * page. The space's Create menu puts these types first.
 */
export function useViewEntityTypes() {
  const route = useRoute()
  const schema = useSchemaStore()
  const pages = usePageStore()

  return computed((): string[] => {
    const name = String(route.name ?? '')
    const view = routeViews[name]
    const target =
      name === 'page' || name === 'entity-page'
        ? pageTabView(route.params, pages.pages)
        : view && { view, id: String(route.params.id ?? '') }
    return target ? viewEntityTypes(target.view, target.id, schema) : []
  })
}

/**
 * Splits menu rows into the rows of `types` and the rest. Both keep the
 * rows' own order, which is the config order. A view with several types
 * therefore leads with them in config order, not in the view's order.
 */
export function leadWith<T extends { type: string }>(
  rows: T[],
  types: string[]
): { lead: T[]; rest: T[] } {
  const wanted = new Set(types)
  return {
    lead: rows.filter((r) => wanted.has(r.type)),
    rest: rows.filter((r) => !wanted.has(r.type)),
  }
}
