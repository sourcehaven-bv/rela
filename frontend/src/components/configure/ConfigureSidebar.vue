<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import RlSidebar from 'rela-components/components/layout/RlSidebar.vue'
import RlThemeToggle, {
  type ThemeChoice,
} from 'rela-components/components/layout/RlThemeToggle.vue'
import type { NavGroup } from 'rela-components/types'
import SpaceSwitcher from '@/components/common/SpaceSwitcher.vue'
import AccountMenu from '@/components/common/AccountMenu.vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { configureRoute } from '@/configure/routes'

/**
 * The Configure space's sidebar (TKT-F5NGMG): the data model and the screens,
 * one entry per thing an operator can change. It replaces the app's sidebar
 * while a Configure screen is open; the header's switcher leads back.
 */
const route = useRoute()
const uiStore = useUIStore()
const schemaStore = useSchemaStore()

const appName = computed(() => schemaStore.app.name)

// Bound to the same stored choice as the app's sidebar, so it carries across.
const theme = computed<ThemeChoice>({
  get: () => uiStore.themeMode,
  set: (value) => uiStore.setThemeMode(value),
})

function item(id: string, label: string, icon: string, to: string) {
  return { id, label, icon, as: RouterLink, attrs: { to } }
}

const groups: NavGroup[] = [
  {
    id: 'model',
    label: 'Data model',
    items: [
      item('entity-types', 'Entity types', 'boxes', configureRoute.entityTypes()),
      item('choice-lists', 'Choice lists', 'list', configureRoute.choiceLists()),
      item('relations', 'Relations', 'network', configureRoute.relations()),
      item('rules', 'Rules', 'shield-check', configureRoute.rules()),
      item('automations', 'Automations', 'zap', configureRoute.automations()),
    ],
  },
  {
    id: 'screens',
    label: 'Screens',
    items: [
      item('navigation', 'Navigation', 'menu', configureRoute.navigation()),
      item('forms', 'Forms', 'document', configureRoute.forms()),
      item('lists', 'Lists', 'table', configureRoute.lists()),
      item('boards', 'Boards', 'kanban', configureRoute.boards()),
      item('dashboard', 'Dashboard', 'dashboard', configureRoute.dashboard()),
    ],
  },
]

/** The section of the current path: `/configure/forms/edit` is `forms`. */
const activeId = computed(() => route.path.split('/')[2] ?? 'entity-types')
</script>

<template>
  <RlSidebar
    id="main-sidebar"
    class="sidebar"
    :class="{ collapsed: uiStore.sidebarCollapsed }"
    workspace-name="Configure"
    :groups="groups"
    :active-id="activeId"
    @toggle-collapse="uiStore.toggleSidebar"
    @close="uiStore.closeMobileSidebar"
  >
    <template #switcher>
      <SpaceSwitcher :app-name="appName" configuring />
    </template>
    <template #footer>
      <AccountMenu />
      <RlThemeToggle v-if="!schemaStore.darkDisabled" v-model="theme" class="sidebar-theme" />
    </template>
  </RlSidebar>
</template>

<style scoped>
.sidebar-theme {
  margin-top: var(--rl-space-2);
}

.sidebar.collapsed {
  --rl-sidebar-width: 60px;
}
</style>
