<script setup lang="ts">
/**
 * A page of `pages:`: several views of one subject as tabs (TKT-ITQ0HL).
 *
 * The page owns the header's title and tab bar; the active tab mounts the
 * same view its standalone route mounts, which still owns the header's
 * actions and tools and its own query state (sort, filters, `?selected=`).
 * Tabs are addressed by the URL, `/p/<page>/<tab>`, so a bookmark, a reload
 * and the browser's back button all land on the same tab.
 *
 * Which tabs this principal sees comes from the sidebar payload. `_config`
 * serves every tab to everyone, so it cannot answer that.
 *
 * An entity page (`entity_type:`) shows one entity, the anchor, at
 * `/p/<page>/<entity>/<tab>`. Its header shows the anchor's title and badge,
 * and each tab narrows its view to the anchor: a list or board to the rows
 * the anchor reaches over the tab's relation, a timeline to the anchor's
 * subtree. The two-segment form `/p/<page>/<entity>` opens the first tab.
 *
 * A page with `mode: steps` (TKT-35BY3PE) draws its tabs as numbered steps
 * with Back and Next instead of a tab bar, and shows each step's prompt.
 * `?present=1` presents it: the shell hides the sidebar and the next-action
 * card, and PageDown and PageUp move between steps. The flag is carried from
 * step to step and does nothing on a page with tabs.
 */
import {
  computed,
  defineAsyncComponent,
  onBeforeUnmount,
  onMounted,
  watchEffect,
  type Component,
} from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { intlLocale } from '@/i18n/locale'
import { useQuery, useQueryCache } from '@pinia/colada'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import { isIconName } from 'rela-components/components/common/icons'
import type { ViewTab } from 'rela-components/components/layout/RlViewTabs.vue'
import { usePageFrame } from '@/composables/usePageHeader'
import { usePresentation } from '@/composables/usePresentation'
import { useDetailPanelOutlet } from '@/composables/useDetailPanel'
import { isAnyModalOpen } from '@/composables/modalStack'
import PageSteps from '@/components/pages/PageSteps.vue'
import { useEntityPageEditing } from '@/composables/useEntityPageEditing'
import { getEntity } from '@/api/entities'
import { shouldDropHeldContent } from '@/api/errors'
import { liveEntityPath } from '@/api/live'
import { useLiveSubscription } from '@/composables/useLiveSubscription'
import { entityKeys } from '@/queries/entities'
import { usePageStore } from '@/stores/pages'
import { useSpaceStore } from '@/stores/space'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { entityRef } from '@/utils/entityRef'
import { isInputFocused } from '@/utils/dom'
import { pageTabPath, routeTabId } from '@/utils/pageContext'
import type { PageTabView, SidebarPageTab } from '@/types'

const props = defineProps<{
  page: string
  /** On an entity page reached at `/p/<page>/<entity>`, this is the entity. */
  tab?: string
  entity?: string
}>()

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const pageStore = usePageStore()
const spaceStore = useSpaceStore()
const frame = usePageFrame()
const detailPanel = useDetailPanelOutlet()

/* The standalone views, loaded on demand as their own routes load them. */
const views: Record<PageTabView, Component> = {
  list: defineAsyncComponent(() => import('@/views/ListView.vue')),
  kanban: defineAsyncComponent(() => import('@/views/KanbanView.vue')),
  calendar: defineAsyncComponent(() => import('@/views/CalendarView.vue')),
  gantt: defineAsyncComponent(() => import('@/views/GanttView.vue')),
  document: defineAsyncComponent(() => import('@/views/DocumentView.vue')),
  dashboard: defineAsyncComponent(() => import('@/views/DashboardView.vue')),
}

/*
 * Where the detail panel sits against each kind of view. A board, a calendar
 * and a timeline need their full width, so the panel lies over them; a list
 * shares the row with it.
 */
const panelModes: Record<PageTabView, 'inline' | 'overlay'> = {
  list: 'inline',
  kanban: 'overlay',
  calendar: 'overlay',
  gantt: 'overlay',
  document: 'overlay',
  dashboard: 'overlay',
}

const pageConfig = computed(() => pageStore.pages[props.page])
const tabs = computed(() => pageConfig.value?.tabs ?? [])
const anchorType = computed(() => pageConfig.value?.entity_type)

/*
 * The route's segments, read for the kind of page. On an entity page the
 * two-segment route puts the entity where a plain page has its tab.
 */
const anchorId = computed(() => (anchorType.value ? (props.entity ?? props.tab) : undefined))
const tabId = computed(() => routeTabId(pageConfig.value, props))
const activeTab = computed(() => tabs.value.find((t) => t.id === tabId.value))
const steps = computed(() => pageConfig.value?.mode === 'steps')

function tabHref(id: string): string {
  return spaceStore.href(pageTabPath({ page: props.page, tab: id, entity: anchorId.value }))
}

/*
 * `/p/<page>` and an unknown tab id both open the first tab this principal
 * may see. A replace, so the bare URL does not stay in history as a step
 * that only redirects. An entity page without an entity has no first tab to
 * open.
 */
watchEffect(() => {
  const first = tabs.value[0]
  if (!pageStore.loaded || !first || activeTab.value) return
  if (anchorType.value && !anchorId.value) return
  // A steps page keeps `?present=1` and the world, so a link that presents
  // the bare page opens its first step presented.
  void router.replace(
    steps.value ? stepHref(first.id, route.query.present === '1') : tabHref(first.id)
  )
})

/*
 * The anchor, read like any entity: a hidden one and a missing one are the
 * same 404. Keyed like the entity's detail, so an SSE change to it refetches
 * the title and badge. A refetch that is refused drops what was held.
 */
const anchorQuery = useQuery({
  key: () => entityKeys.detail(anchorType.value ?? '', anchorId.value ?? ''),
  query: () => getEntity(anchorType.value as string, anchorId.value as string),
  enabled: () => !!anchorType.value && !!anchorId.value,
})
const queryCache = useQueryCache()
// Live view (TKT-HJ5Z9Y) of the anchor, for changes entity:changed misses,
// such as losing access to it.
useLiveSubscription(
  'page-anchor',
  () =>
    anchorType.value && anchorId.value ? liveEntityPath(anchorType.value, anchorId.value) : null,
  () =>
    void queryCache.invalidateQueries({
      key: entityKeys.detail(anchorType.value ?? '', anchorId.value ?? ''),
      exact: true,
    })
)
const anchorGone = computed(() => shouldDropHeldContent(anchorQuery.error.value))
const anchor = computed(() => (anchorGone.value ? undefined : anchorQuery.data.value))

/* The title, the badge and the menu beside them edit the anchor itself. */
const editing = useEntityPageEditing({
  entity: anchor,
  entityType: anchorType,
  badgeProperty: computed(() => pageConfig.value?.badge),
})

function toViewTab(tab: SidebarPageTab): ViewTab {
  return {
    id: tab.id,
    label: tab.label,
    ...(tab.icon && isIconName(tab.icon) ? { icon: tab.icon } : {}),
    panelMode: panelModes[tab.view],
    // A real link, so middle-click and "open in new tab" work.
    as: RouterLink,
    attrs: { to: tabHref(tab.id) },
  }
}

/*
 * Switching tabs is a push, so back returns to the previous tab. The new
 * URL carries no query: sort, filters and selection belong to the tab they
 * were made on.
 */
function select(id: string) {
  if (id !== props.tab) void router.push(tabHref(id))
}

watchEffect(() => {
  const page = pageConfig.value
  const active = activeTab.value
  if (!page || !active) {
    frame.clear()
    return
  }
  if (anchorType.value && (anchorGone.value || !anchorId.value)) {
    frame.clear()
    return
  }
  // An entity page is titled by its entity. The page label stands in while
  // the entity loads.
  const title = anchor.value ? entityDisplayTitle(anchor.value) : page.label
  frame.show({
    title,
    badge: editing.badge.value,
    menu: editing.menu.value,
    // A steps page draws its own step bar instead of the tab bar.
    tabs: steps.value ? [] : tabs.value.map(toViewTab),
    active: active.id,
    select,
  })
})

/*
 * The props the active tab's view takes, the same its own route passes. A
 * tab of an entity page adds its narrowing to the anchor.
 */
const viewProps = computed((): Record<string, unknown> => {
  const tab = activeTab.value
  if (!tab?.target) return {}
  const key = tab.view === 'document' || tab.view === 'dashboard' ? 'name' : 'id'
  const out: Record<string, unknown> = { [key]: tab.target }
  if (anchorId.value && tab.scope === 'relation') {
    out.pageScope = {
      page: props.page,
      tab: tab.id,
      entity: anchorId.value,
      ...(anchor.value ? { ref: entityRef(anchor.value) } : {}),
    }
  } else if (anchorId.value && tab.scope === 'root') {
    out.root = anchorId.value
  }
  return out
})

/* An entity page with no entity to show: none named, or none this reader can see. */
const anchorMissing = computed(() => !!anchorType.value && (!anchorId.value || anchorGone.value))

/*
 * Steps and present mode. Presenting is read from the route on every
 * change, never stored, so leaving `?present=1` by any route (Exit, the
 * browser's back button, a link) gives the shell back.
 */
const presenting = computed(
  () => steps.value && route.query.present === '1' && !!activeTab.value && !anchorMissing.value
)
usePresentation(() => presenting.value)

/*
 * A step link keeps present mode and the world, so the room goes on seeing
 * the same data. Like a tab, it drops the rest: sort, filters and selection
 * belong to the step they were made on.
 */
function stepHref(id: string, present = presenting.value): string {
  const query = new URLSearchParams()
  const world = route.query.world
  if (typeof world === 'string' && world) query.set('world', world)
  if (present) query.set('present', '1')
  const qs = query.toString()
  return qs ? `${tabHref(id)}?${qs}` : tabHref(id)
}

function goStep(id: string) {
  void router.push(stepHref(id))
}

/* Entering or leaving present mode shows the same view, so the query stays. */
function setPresenting(on: boolean) {
  const query = { ...route.query }
  if (on) query.present = '1'
  else delete query.present
  void router.push({ path: route.path, query })
}

/*
 * PageDown and PageUp step through a presented page, as a presentation
 * clicker sends them. Only while presenting: elsewhere the keys scroll a
 * long list as they always do. Not while typing, in a dialog, or with the
 * detail panel open, where the keys scroll what the panel shows.
 */
function onStepKey(event: KeyboardEvent) {
  if (!presenting.value || event.defaultPrevented) return
  if (event.key !== 'PageDown' && event.key !== 'PageUp') return
  if (isInputFocused() || isAnyModalOpen() || detailPanel.open.value) return
  const i = tabs.value.findIndex((t) => t.id === activeTab.value?.id)
  const target = tabs.value[event.key === 'PageDown' ? i + 1 : i - 1]
  event.preventDefault()
  if (i >= 0 && target) goStep(target.id)
}
onMounted(() => window.addEventListener('keydown', onStepKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onStepKey))

/*
 * The tab on screen, published for the space's Create menu, which sits
 * outside the page and links a new row to the anchor. Only once the anchor
 * has loaded: a link names the face on screen. Until then `opening` tells
 * the menu an entity page is on its way.
 */
watchEffect(() => {
  const tab = activeTab.value
  const current =
    anchorId.value && anchor.value && tab
      ? { page: props.page, tab: tab.id, entity: anchorId.value, ref: entityRef(anchor.value) }
      : undefined
  pageStore.current = current
  pageStore.opening = !!anchorId.value && !current && !anchorGone.value
})
onBeforeUnmount(() => {
  pageStore.current = undefined
  pageStore.opening = false
})
</script>

<template>
  <div class="page-view">
    <PageSteps
      v-if="steps && activeTab && !anchorMissing"
      class="page-view__steps"
      :steps="tabs"
      :active="activeTab.id"
      :presenting="presenting"
      :href="stepHref"
      @go="goStep"
      @present="setPresenting"
    />
    <RlEmptyState
      v-if="pageStore.loaded && pageConfig && anchorMissing"
      icon="warning"
      :title="t('page.notFound')"
      :description="
        anchorId
          ? t('page.anchorMissing', {
              type: pageConfig.label.toLocaleLowerCase(intlLocale()),
              id: anchorId,
            })
          : t('page.anchorNone', { type: pageConfig.label.toLocaleLowerCase(intlLocale()) })
      "
    />
    <component
      :is="views[activeTab.view]"
      v-else-if="activeTab"
      :key="`${page}/${anchorId ?? ''}/${activeTab.id}`"
      v-bind="viewProps"
    />
    <RlEmptyState
      v-else-if="pageStore.loaded && !pageConfig"
      icon="warning"
      :title="t('page.pageNotFound')"
      :description="t('page.pageNotConfigured', { page })"
    />
    <RlEmptyState
      v-else-if="pageStore.loaded && tabs.length === 0"
      :title="t('page.nothingToShow')"
      :description="t('page.noViews')"
    />
  </div>
</template>

<style scoped>
.page-view {
  display: contents;
}

.page-view__steps {
  flex: none;
}
</style>
