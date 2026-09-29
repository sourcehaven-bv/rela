<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import type { NavGroup, NavItem } from '../../types'
import RlIconButton from '../common/RlIconButton.vue'
import RlSearchBox from '../data/RlSearchBox.vue'
import RlText from '../common/RlText.vue'
import RlWorkspaceSwitcher from './RlWorkspaceSwitcher.vue'
import RlSidebarFooterLink from './RlSidebarFooterLink.vue'
import RlThemeToggle, { type ThemeChoice } from './RlThemeToggle.vue'
import RlSidebarGroup from './RlSidebarGroup.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    workspaceName: string
    /**
     * Whether the default switcher offers a menu. Off leaves the workspace
     * name as a plain label, for an app with one workspace or one space and
     * so nothing to switch to.
     *
     * Ignored when the `switcher` slot is filled, since the control is then
     * the consumer's.
     */
    workspaceMenu?: boolean
    groups: NavGroup[]
    activeId?: string
    /**
     * The id of the nav item whose panel is open, if any.
     *
     * Separate from `activeId` on purpose: `activeId` says where you are, and
     * a panel opened over the page is not somewhere you went. An item marked
     * `opensFlyout` reads this to report `aria-expanded` and to show that it
     * is the row the open panel came from.
     */
    flyoutId?: string | null
    footerLabel?: string
    /**
     * Count of items on screen past which the filter appears. A short sidebar
     * is read at a glance and a search box above it is one more thing in the
     * way, so the control earns its place only once scanning stops working.
     *
     * Set to 0 to always show it, or Infinity to never.
     */
    filterThreshold?: number
    /**
     * Items in one group past which it splits under its own subheadings.
     * Matched to `filterThreshold`, so a list that grows long gains the
     * filter and the subheadings together rather than one at a time.
     */
    groupThreshold?: number
    /**
     * Heading for the leftover items once a group splits under subheadings.
     * Settable because `groupThreshold` is: a consumer who lowers the
     * threshold summons this heading, and otherwise could not name or
     * translate the group it creates.
     */
    fallbackLabel?: string
    /**
     * Whether the footer carries the theme picker. On by default: the library
     * defines the three theme states, so an app that imports the sidebar gets
     * a working way to reach them without wiring one.
     *
     * Turn it off for an app that puts theme switching elsewhere, or replace
     * the whole band through the `footer` slot.
     */
    themeToggle?: boolean
    /**
     * The chosen theme, when the app stores the preference. Left unbound the
     * picker keeps the choice itself for the page.
     */
    theme?: ThemeChoice
  }>(),
  {
    workspaceMenu: true,
    flyoutId: null,
    footerLabel: 'Help docs',
    filterThreshold: 25,
    groupThreshold: 20,
    fallbackLabel: 'Other',
    themeToggle: true,
    theme: undefined,
  },
)

const emit = defineEmits<{
  select: [item: NavItem]
  /**
   * The collapse control was pressed. The sidebar has no collapsed state of
   * its own: it takes its width from `--rl-sidebar-width`, so collapsing is
   * done by rebinding that token. Setting `width` from outside also works,
   * but it overrides one rule and leaves the drawer's `min()` at the narrow
   * width reading the old value.
   *
   * An app that handles this event and changes nothing else collapses
   * nothing, with no error anywhere.
   */
  toggleCollapse: []
  workspaceClick: []
  groupMenu: [group: NavGroup]
  /** A group's add control was pressed; see `NavGroup.addLabel`. */
  groupAdd: [group: NavGroup]
  footerClick: []
  close: []
  /** The theme picker changed. Store this to make the choice outlast the page. */
  'update:theme': [value: ThemeChoice]
}>()

/** Every item, nested ones included, since children are navigable too. */
function countItems(items: NavItem[]): number {
  return items.reduce((n, item) => n + 1 + countItems(item.children ?? []), 0)
}

/*
 * What is actually on screen. Children of a collapsed parent are not shown,
 * so counting them would offer a filter for a list the user cannot see —
 * which is how a sidebar that fits comfortably ends up with a search box
 * above it, pushing the items it was meant to help find out of view.
 */
function countVisible(items: NavItem[]): number {
  return items.reduce(
    (n, item) => n + 1 + (item.expanded ? countVisible(item.children ?? []) : 0),
    0,
  )
}

const total = computed(() => countItems(props.groups.flatMap((group) => group.items)))
const visibleCount = computed(() => countVisible(props.groups.flatMap((group) => group.items)))
const filterable = computed(() => visibleCount.value > props.filterThreshold)

const query = ref('')

/*
 * A parent stays when one of its children matches, so a match is never
 * orphaned from the group it belongs to. The matching children are kept and
 * forced open, because a filtered tree that hides its own results behind a
 * collapsed caret is worse than no filter at all.
 */
function filterItems(items: NavItem[], needle: string): NavItem[] {
  return items.flatMap((item) => {
    const children = filterItems(item.children ?? [], needle)
    const hit = item.label.toLowerCase().includes(needle)
    if (!hit && children.length === 0) return []
    return [{ ...item, children: children.length ? children : undefined, expanded: children.length > 0 || item.expanded }]
  })
}

const visibleGroups = computed(() => {
  const needle = query.value.trim().toLowerCase()
  if (!needle) return props.groups
  return props.groups
    .map((group) => ({ ...group, items: filterItems(group.items, needle) }))
    .filter((group) => group.items.length > 0)
})

const matchCount = computed(() =>
  countItems(visibleGroups.value.flatMap((group) => group.items)),
)

const empty = computed(() => query.value.trim().length > 0 && matchCount.value === 0)

const nav = ref<HTMLElement | null>(null)

/*
 * Bring the current page's item into view. Without this a long sidebar opens
 * scrolled to the top, so the one item that says where you are is the one you
 * cannot see. `nearest` keeps it still when the item is already visible,
 * rather than yanking the list on every navigation.
 */
function revealActive() {
  if (!props.activeId || !nav.value) return
  const active = nav.value.querySelector('.rl-nav-item--active')
  active?.scrollIntoView({ block: 'nearest' })
}

onMounted(() => nextTick(revealActive))
watch(() => props.activeId, () => nextTick(revealActive))
</script>

<template>
  <aside class="rl-sidebar" aria-label="Sidebar">
    <header class="rl-sidebar__header">
      <!--
        The switcher position, not the switcher itself. A consumer whose
        project picker fetches its own list and renders its own menu replaces
        the control here and keeps the header's layout, the collapse button
        and the close button.
      -->
      <slot name="switcher">
        <RlWorkspaceSwitcher
          :name="workspaceName"
          :menu="workspaceMenu"
          @click="emit('workspaceClick')"
        >
          <template v-if="$slots.logo" #logo><slot name="logo" /></template>
        </RlWorkspaceSwitcher>
      </slot>

      <RlIconButton
        class="rl-sidebar__collapse"
        icon="panel-left"
        label="Collapse sidebar"
        @click="emit('toggleCollapse')"
      />
      <RlIconButton
        class="rl-sidebar__close"
        icon="x"
        label="Close navigation"
        @click="emit('close')"
      />
    </header>

    <!--
      Appears on its own once the list is too long to scan. Outside the
      scrolling nav, so filtering never scrolls the control out of reach.
    -->
    <div v-if="filterable" class="rl-sidebar__filter">
      <RlSearchBox
        v-model="query"
        size="sm"
        label="Filter navigation"
        placeholder="Filter"
        :result-text="query ? messages.resultCount({ shown: matchCount, total }) : undefined"
      />
    </div>

    <!--
      Fixed entries that are not part of the navigation data: a search entry,
      a link to an analysis view. Outside the filter and outside the scrolling
      nav on purpose. A pinned item the filter could hide is not pinned, one
      that scrolls away is not either, and one that gained a group heading
      would be claiming to be a category it is not.
    -->
    <div v-if="$slots.pinned" class="rl-sidebar__pinned">
      <slot name="pinned" />
    </div>

    <nav ref="nav" class="rl-sidebar__nav" aria-label="Main">
      <!--
        Each group decides for itself whether it is long enough to need
        subheadings, so the sidebar hands it the data and stays out of it.
        Filtering deliberately drops a group below that threshold, which is
        correct: a filtered list is short again and reads as a plain one.
      -->
      <RlSidebarGroup
        v-for="group in visibleGroups"
        :key="group.id"
        class="rl-sidebar__group"
        :group="group"
        :active-id="activeId"
        :flyout-id="flyoutId"
        :group-threshold="groupThreshold"
        :fallback-label="fallbackLabel"
        @select="emit('select', $event)"
        @menu="emit('groupMenu', group)"
        @add="emit('groupAdd', group)"
      />

      <RlText v-if="empty" size="sm" tone="subtle" class="rl-sidebar__empty">
        {{ messages.noMatches({ query }) }}
      </RlText>
    </nav>

    <!--
      Chrome rather than navigation: a build indicator, a settings link, a
      theme toggle. One label cannot express that, so the slot takes the
      whole band and the single link stays the default.
    -->
    <footer class="rl-sidebar__footer">
      <slot name="footer">
        <RlSidebarFooterLink :label="footerLabel" @click="emit('footerClick')" />
        <RlThemeToggle
          v-if="themeToggle"
          class="rl-sidebar__theme"
          :model-value="theme"
          @update:model-value="emit('update:theme', $event)"
        />
      </slot>
    </footer>
  </aside>
</template>

<style scoped>
.rl-sidebar {
  /*
   * Named so a row's status marker can answer the rail width itself. The
   * sidebar has no collapsed state to pass down: the app rebinds
   * `--rl-sidebar-width` and the rail is just a narrow sidebar.
   */
  container: rl-sidebar / inline-size;
  display: flex;
  flex-direction: column;
  width: var(--rl-sidebar-width);
  flex: none;
  height: 100%;
  border-right: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
}

.rl-sidebar__header {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-3) var(--rl-space-3) var(--rl-space-2);
}

.rl-sidebar__filter { padding: 0 var(--rl-space-3) var(--rl-space-2); }

.rl-sidebar__nav {
  flex: 1;
  overflow-y: auto;
  padding: 0 var(--rl-space-3);
}

/*
 * Shares the nav's horizontal padding so the rows line up with the groups
 * below, and is separated from them by the gap that divides one group from
 * the next: pinned entries read as their own band rather than as the first
 * group's unlabelled members.
 */
.rl-sidebar__pinned {
  flex: none;
  padding: 0 var(--rl-space-3);
  margin-bottom: var(--rl-space-6);
}

.rl-sidebar__group + .rl-sidebar__group { margin-top: var(--rl-space-6); }

.rl-sidebar__empty { display: block; padding: var(--rl-space-2); }

.rl-sidebar__footer { padding: var(--rl-space-3); }

/* Below the link rather than beside it: the rail is too narrow for a row. */
.rl-sidebar__theme { margin-top: var(--rl-space-2); }

/* The close button belongs to the drawer; the collapse button to the desktop rail. */
.rl-sidebar__close { display: none; }

@media (max-width: 767px) {
  .rl-sidebar {
    width: min(var(--rl-sidebar-width), 84vw);
    /*
     * Fills whatever holds it rather than the viewport. As a drawer that
     * container is inset for the home indicator, and measuring the full
     * viewport here would push the footer back under it.
     */
    height: 100%;
    max-height: 100dvh;
  }

  .rl-sidebar__collapse { display: none; }
  .rl-sidebar__close { display: flex; }
}

/*
 * The rail. The parts that carry text answer this width themselves, but the
 * column around them has to make room first: the gutters here were sized for
 * a label, so at a rail width the rows still measured wider than the sidebar
 * and the nav grew a horizontal scrollbar under them.
 *
 * The collapse control stays, because it is the only way back to the full
 * sidebar. Its `panel-left` glyph already reads as "the panel", and the
 * button's label is unchanged for a screen reader; only the header's second
 * column goes, so the one control centres in the rail.
 */
@container rl-sidebar (max-width: 120px) {
  .rl-sidebar__nav {
    padding-inline: var(--rl-space-2);
    overflow-x: hidden;
  }

  .rl-sidebar__header,
  .rl-sidebar__footer { padding-inline: var(--rl-space-2); }

  .rl-sidebar__header { justify-content: center; }
}

/*
 * The sidebar spans the full height on every width: as a rail beside the
 * content on a desktop, as a drawer off the left edge on a phone. Either
 * way its own top and bottom bands are the ones against the screen edge, so
 * they carry the status bar and the home indicator, and the whole column
 * carries the notch on the left.
 *
 * Through the `--rl-sidebar-safe-*` indirection rather than the raw insets,
 * so a container that has already inset this sidebar can zero them and stop
 * the padding being applied twice. The app shell's drawer does exactly that.
 */
.rl-sidebar { padding-left: var(--rl-sidebar-safe-left); }
.rl-sidebar__header { padding-top: calc(var(--rl-space-3) + var(--rl-sidebar-safe-top)); }
.rl-sidebar__footer { padding-bottom: calc(var(--rl-space-3) + var(--rl-sidebar-safe-bottom)); }
</style>
