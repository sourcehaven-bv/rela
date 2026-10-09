<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import { getSidebar, runAction } from '@/api'
import { isCancelledFetch } from '@/composables/usePageData'
import { useActionFeedback } from '@/composables/useActionFeedback'
import { useEvents } from '@/composables/useEvents'
import type { Entity, SidebarCreate, SidebarGroup, SidebarItem } from '@/types'
import type { NavGroup, NavItem } from 'rela-components/types'
import type { ThemeChoice } from 'rela-components/components/layout/RlThemeToggle.vue'
import RlThemeToggle from 'rela-components/components/layout/RlThemeToggle.vue'
import SidebarFooter from './SidebarFooter.vue'
import { isInputFocused } from '@/utils/dom'
import RlSidebar from 'rela-components/components/layout/RlSidebar.vue'
import RlSidebarGroup from 'rela-components/components/layout/RlSidebarGroup.vue'
import ProjectSwitcher from './ProjectSwitcher.vue'
import SpaceSwitcher from './SpaceSwitcher.vue'
import WorldSwitcher from './WorldSwitcher.vue'
import AccountMenu from './AccountMenu.vue'
import InlineCreateFormModal from '@/components/forms/InlineCreateFormModal.vue'
import { spaceOf, stripSpace, useSpaceStore, withSpace } from '@/stores/space'
import { usePageStore } from '@/stores/pages'
import {
  activeNavId,
  expandEntityEntries,
  expandGeneratedItems,
  PILE_NAV_PREFIX,
  PILES_GROUP_ID,
  pilesNavGroup,
  toNavGroups,
} from './sidebarNav'
import { usePiles } from '@/composables/usePiles'
import NewPileDialog from '@/components/piles/NewPileDialog.vue'
import { useNavStatus } from '@/composables/useNavStatus'
import { useNavItems } from '@/composables/useNavItems'
import { useNavEntities } from '@/composables/useNavEntities'
import { useFlyout } from '@/composables/useFlyout'
import { shouldDeferToBrowser } from '@/utils/openIntent'
import { apiUrl } from '@/api/base'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { reportResult, reportError } = useActionFeedback()
const spaceStore = useSpaceStore()
const pageStore = usePageStore()
const route = useRoute()
const router = useRouter()

// Tracks which action items are currently in-flight (prevents double-click).
const actionInFlight = ref<Set<string>>(new Set())

// Sidebar data from API
const sidebarGroups = ref<SidebarGroup[]>([])
const sidebarAppName = ref('')

const appName = computed(() => sidebarAppName.value || schemaStore.app.name)

// Logo lives on the schema store so SettingsView can update it after
// upload/remove without a sidebar refetch.
const logoUrl = computed(() => schemaStore.logoUrl)

/*
 * The pinned entries, the config groups and the custom apps, as one nav model.
 *
 * All three were separate blocks of template before, each repeating the
 * link-or-button branch. They are the same shape, so they are built as one
 * list and the library renders them; only their PLACEMENT differs, and the
 * pinned pair keeps its own slot.
 */
const appGroups = computed<SidebarGroup[]>(() => {
  // Custom apps (sandboxed-iframe extensions). The label falls back to title,
  // then the id, so an app with no metadata still gets a usable entry.
  const apps = Array.from(schemaStore.apps.entries()).map(([id, app]) => ({
    label: app.label || app.title || id,
    href: `/app/${id}`,
    icon: 'apps',
  }))
  return apps.length ? [{ group: 'Apps', items: apps }] : []
})

const flyout = useFlyout()

/*
 * A plain click on an `open: flyout` entry slides its list out instead of
 * navigating, and a second one puts it away. Every other click is the link's.
 */
function onFlyoutClick(item: SidebarItem, id: string, event: MouseEvent) {
  if (shouldDeferToBrowser(event) || !item.flyout || !item.href) return
  event.preventDefault()
  event.stopPropagation()
  flyout.toggle({ navId: id, title: item.label, listId: item.flyout.list, href: item.href })
}

// Counts are fetched only when some entry declares `status:` rules.
const hasStatusRules = computed(() =>
  sidebarGroups.value.some((group) => group.items.some((item) => item.status_key))
)
const navStatus = useNavStatus(hasStatusRules)

// Entries of generated groups are fetched only when some group declares
// `items_from:`; they are per principal, so the sidebar carries none.
const hasGeneratedGroups = computed(() => sidebarGroups.value.some((group) => group.items_key))
const navItems = useNavItems(hasGeneratedGroups)
const navEntities = useNavEntities(sidebarGroups)

/*
 * Every nav href in the current space (TKT-GNKR5H).
 *
 * The router would redirect an unprefixed path anyway, but a real prefixed
 * href keeps a modified click and the active-row match in the space.
 */
function inSpace(groups: SidebarGroup[]): SidebarGroup[] {
  if (!spaceStore.enabled) return groups
  return groups.map((group) => ({
    ...group,
    items: group.items.map((item) =>
      item.href ? { ...item, href: spaceStore.href(item.href) } : item
    ),
  }))
}

const shownGroups = computed(() =>
  inSpace([
    ...expandGeneratedItems(expandEntityEntries(sidebarGroups.value, navEntities.rowsFor), navItems.itemsFor),
    ...appGroups.value,
  ])
)

const configNavGroups = computed(() =>
  toNavGroups(shownGroups.value, RouterLink, onFlyoutClick, navStatus.statusFor)
)

// The user's piles (TKT-K3RJLH), first among the groups, since they are the
// user's own and not part of any space's configured navigation.
const piles = usePiles()
const creatingPile = ref(false)

const navGroups = computed(() =>
  piles.available.value ? [pilesNavGroup(piles.piles.value), ...configNavGroups.value] : configNavGroups.value
)

/*
 * The "+" on a generated group's heading: the create dialog for the group's
 * list, then the new entity's page. toNavGroups maps one to one, so the
 * library's group finds its source by position.
 */
const creating = ref<{ offer: SidebarCreate; page?: string } | null>(null)

function onGroupAdd(group: NavGroup) {
  if (group.id === PILES_GROUP_ID) {
    creatingPile.value = true
    return
  }
  const source = shownGroups.value[configNavGroups.value.findIndex((g) => g.id === group.id)]
  if (source?.items_create) creating.value = { offer: source.items_create, page: source.items_page }
}

function onCreated(entity: Entity) {
  const page = creating.value?.page
  creating.value = null
  const id = encodeURIComponent(entity.id)
  void router.push(spaceStore.href(page ? `/p/${page}/${id}` : `/entity/${entity.type}/${id}`))
}

const pinnedGroups = computed(() =>
  toNavGroups(
    inSpace([
      {
        items: [
          { label: 'Search', href: '/search', icon: 'search' },
          { label: 'Analysis', href: '/analyze', icon: 'warning' },
        ],
      },
    ]),
    RouterLink
  )
)

/*
 * The highlighted row, resolved across the pinned entries as well as the nav.
 * They are rendered in separate slots but form one selection: two active rows
 * at once would say the user is in two places.
 */
const activeId = computed(
  () => activeNavId(pinnedGroups.value, route.path) ?? activeNavId(navGroups.value, route.path)
)

/*
 * The theme choice, stored by rela rather than by the control.
 *
 * Left unbound the picker keeps the choice for the page and forgets it on
 * reload. rela's store already persists the same three values under the
 * `theme` key, so this binds straight onto it with no translation. The
 * picker is rendered from the #footer slot rather than by RlSidebar itself,
 * because rela replaces the whole band (see the slot).
 */
const theme = computed<ThemeChoice>({
  get: () => uiStore.themeMode,
  set: (value) => uiStore.setThemeMode(value),
})

// Load sidebar data. With spaces, the server resolves the route's space: an
// unknown or hidden one falls back to the first the principal may enter, and
// the route follows it.
// Drops a response overtaken by a later load, so two quick config reloads
// keep the newer navigation.
let sidebarRequest = 0

async function loadSidebar() {
  const request = ++sidebarRequest
  try {
    // Before the initial navigation resolves the route reads `/`, which
    // would send a deep link to the first space.
    await router.isReady()
    const data = await getSidebar(spaceOf(route.path))
    if (request !== sidebarRequest) return
    spaceStore.set(data)
    pageStore.set(data.pages)
    if (data.space && spaceOf(route.path) !== data.space) {
      await router.replace(withSpace(stripSpace(route.fullPath), data.space))
    }
    sidebarAppName.value = data.app.name
    sidebarGroups.value = data.navigation
    schemaStore.setLogoUrl(data.logoUrl ?? null)
    // Principal-scoped inline-create offers ride on this payload; see
    // SidebarData.inline_create for why the sidebar carries them.
    schemaStore.setInlineCreate(data.inline_create ?? {})
    // So is whether this principal may use piles (TKT-K3RJLH).
    schemaStore.setPiles(data)
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load sidebar:', err)
  }
}

// Keyboard shortcut for search.
//
// Defers to a search box that is already on screen rather than jumping to the
// standalone /search page: a list view owns its own search affordance
// (TKT-603FQ), and so does the library sidebar's own filter, which appears
// once the nav is long enough. Taking focus away from either would surprise
// the user mid-typing. The fallback (push /search) still applies elsewhere.
function handleKeydown(e: KeyboardEvent) {
  if (e.key !== '/') return
  if (isInputFocused()) return
  if (document.querySelector('.entity-list .search-box')) return
  if (document.querySelector('.rl-sidebar .rl-search-box input')) return
  e.preventDefault()
  router.push('/search')
}

// A different space has a different navigation and Create menu.
watch(
  () => spaceOf(route.path),
  (next) => {
    if (spaceStore.enabled && next !== spaceStore.current) void loadSidebar()
  }
)

// Close mobile sidebar on route change
watch(
  () => route.path,
  () => {
    if (uiStore.sidebarMobileOpen) {
      uiStore.closeMobileSidebar()
    }
  }
)

// Lock body scroll when mobile sidebar is open
watch(
  () => uiStore.sidebarMobileOpen,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
  }
)

function handleKeydownAll(e: KeyboardEvent) {
  handleKeydown(e)
  if (e.key === 'Escape' && uiStore.sidebarMobileOpen) {
    uiStore.closeMobileSidebar()
  }
}

// A `refresh` is sent when data-entry.yaml reloads. Refetching keeps an
// `entities:` entry from requesting a scope the new config removed, which
// would otherwise show "Could not load" until a full page reload.
const { on: onEvent } = useEvents()
onEvent('refresh', loadSidebar)

onMounted(() => {
  document.addEventListener('keydown', handleKeydownAll)
  loadSidebar()
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydownAll)
  document.body.style.overflow = ''
})

/*
 * A nav row was chosen.
 *
 * A link navigates by itself — it is a real `RouterLink`, so the browser owns
 * modifier-click and middle-click. Only an action needs handling here, and it
 * is matched back by the id the mapping minted.
 */
function onSelect(navItem: NavItem) {
  if (navItem.id.startsWith(PILE_NAV_PREFIX)) {
    const pileId = navItem.id.slice(PILE_NAV_PREFIX.length)
    flyout.togglePile({ navId: navItem.id, title: navItem.label, pileId })
    return
  }
  const action = navItem.id.startsWith('action:') ? navItem.id.slice('action:'.length) : undefined
  if (!action) return
  const item = allItems.value.find((candidate) => candidate.action === action)
  if (item) handleAction(item)
}

const allItems = computed<SidebarItem[]>(() =>
  [...sidebarGroups.value, ...appGroups.value].flatMap((group) => group.items)
)

async function handleAction(item: SidebarItem, ev?: Event) {
  if (!item.action) return
  if (actionInFlight.value.has(item.action)) return

  const triggerEl = ev && ev.currentTarget instanceof HTMLElement ? ev.currentTarget : null

  actionInFlight.value.add(item.action)
  try {
    const response = await runAction(item.action)
    reportResult(response)
    if (response?.redirect) {
      router.push(response.redirect)
    }
  } catch (err: unknown) {
    reportError(err, triggerEl)
  } finally {
    actionInFlight.value.delete(item.action)
  }
}
</script>

<template>
  <RlSidebar
    id="main-sidebar"
    class="sidebar"
    :class="{ collapsed: uiStore.sidebarCollapsed }"
    :workspace-name="appName"
    :groups="navGroups"
    :active-id="activeId"
    :flyout-id="flyout.openNavId.value"
    @select="onSelect"
    @group-add="onGroupAdd"
    @toggle-collapse="uiStore.toggleSidebar"
    @close="uiStore.closeMobileSidebar"
  >
    <!--
      The project picker fetches its own list and renders its own menu, so it
      replaces the control and keeps the header's layout around it. Each
      switcher renders only when there is more than one choice.
    -->
    <template #switcher>
      <ProjectSwitcher />
      <SpaceSwitcher :app-name="appName">
        <template v-if="logoUrl" #logo>
          <img :src="apiUrl(logoUrl)" :alt="appName" class="logo-img" />
        </template>
      </SpaceSwitcher>
      <WorldSwitcher />
    </template>

    <!--
      Search and Analysis sit above the config-driven nav and never scroll out
      of reach, which is why they are a slot rather than a first group.
    -->
    <template #pinned>
      <RlSidebarGroup
        v-for="group in pinnedGroups"
        :key="group.id"
        :group="group"
        :active-id="activeId"
      />
    </template>

    <!--
      The footer is rela's chrome band: git status, the next-action chip,
      Settings, About, Shortcuts. It replaces the library's single default
      link, so the theme picker has to be re-rendered here — the slot takes
      the whole band, toggle included.
    -->
    <template #footer>
      <SidebarFooter />
      <AccountMenu />
      <RlThemeToggle v-if="!schemaStore.darkDisabled" v-model="theme" class="sidebar-theme" />
    </template>
  </RlSidebar>
  <InlineCreateFormModal
    v-if="creating"
    :show="true"
    :form-id="creating.offer.form"
    :entity-type="creating.offer.type"
    @close="creating = null"
    @created="onCreated"
  />
  <NewPileDialog v-if="creatingPile" @close="creatingPile = false" />
</template>

<style scoped>
/* Matches the library's own `.rl-sidebar__theme`: below the links, not beside
   them — the rail is too narrow for a row. */
.sidebar-theme {
  margin-top: var(--rl-space-2);
}

/*
 * The collapsed rail. The library owns the sidebar's width through
 * `--rl-sidebar-width` and has no collapsed state of its own — it emits
 * `toggle-collapse` and leaves the decision here — so narrowing the rail means
 * rebinding that variable rather than setting `width` directly, which the
 * library's own rule would otherwise win.
 */
.sidebar.collapsed {
  --rl-sidebar-width: 60px;
}

.logo-img {
  max-height: 28px;
  max-width: 100%;
  object-fit: contain;
}
</style>
