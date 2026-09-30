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
 */
import { computed, defineAsyncComponent, watchEffect, type Component } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useQuery } from '@pinia/colada'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import { isIconName } from 'rela-components/components/common/icons'
import type { ViewTab } from 'rela-components/components/layout/RlViewTabs.vue'
import { usePageFrame } from '@/composables/usePageHeader'
import { useEntityPageEditing } from '@/composables/useEntityPageEditing'
import { getEntity } from '@/api/entities'
import { shouldDropHeldContent } from '@/api/errors'
import { entityKeys } from '@/queries/entities'
import { usePageStore } from '@/stores/pages'
import { useSpaceStore } from '@/stores/space'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { pageTabPath } from '@/utils/pageContext'
import type { PageTabView, SidebarPageTab } from '@/types'

const props = defineProps<{
  page: string
  /** On an entity page reached at `/p/<page>/<entity>`, this is the entity. */
  tab?: string
  entity?: string
}>()

const router = useRouter()
const pageStore = usePageStore()
const spaceStore = useSpaceStore()
const frame = usePageFrame()

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
const tabId = computed(() => (anchorType.value && !props.entity ? undefined : props.tab))
const activeTab = computed(() => tabs.value.find((t) => t.id === tabId.value))

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
  void router.replace(tabHref(first.id))
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
    tabs: tabs.value.map(toViewTab),
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
  const key = tab.view === 'document' ? 'name' : 'id'
  const out: Record<string, unknown> = { [key]: tab.target }
  if (anchorId.value && tab.scope === 'relation') {
    out.pageScope = { page: props.page, tab: tab.id, entity: anchorId.value }
  } else if (anchorId.value && tab.scope === 'root') {
    out.root = anchorId.value
  }
  return out
})

/* An entity page with no entity to show: none named, or none this reader can see. */
const anchorMissing = computed(() => !!anchorType.value && (!anchorId.value || anchorGone.value))
</script>

<template>
  <div class="page-view">
    <RlEmptyState
      v-if="pageStore.loaded && pageConfig && anchorMissing"
      icon="warning"
      title="Not found"
      :description="
        anchorId
          ? `There is no ${pageConfig.label.toLowerCase()} “${anchorId}”, or you cannot open it.`
          : `Open a ${pageConfig.label.toLowerCase()} to see this page.`
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
      title="Page not found"
      :description="`There is no page “${page}” in the configuration.`"
    />
    <RlEmptyState
      v-else-if="pageStore.loaded && tabs.length === 0"
      title="Nothing to show"
      description="This page has no views you can open."
    />
  </div>
</template>

<style scoped>
.page-view {
  display: contents;
}
</style>
