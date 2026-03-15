<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import type { NavigationEntry } from '@/types'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const route = useRoute()
const router = useRouter()

const appName = computed(() => schemaStore.app.name)
const navigation = computed(() => schemaStore.navigation)

// Keyboard shortcut for search
function handleKeydown(e: KeyboardEvent) {
  if (e.key === '/' && !['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement)?.tagName)) {
    e.preventDefault()
    router.push('/search')
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})

function getHref(entry: NavigationEntry): string {
  if (entry.list) return `/list/${entry.list}`
  if (entry.kanban) return `/kanban/${entry.kanban}`
  if (entry.dashboard) return '/'
  if (entry.graph) return '/graph'
  return '/'
}

function isActive(href: string): boolean {
  return route.path === href || route.path.startsWith(href + '/')
}

function getIcon(entry: NavigationEntry): string {
  if (entry.icon) return entry.icon
  if (entry.list) return '📋'
  if (entry.kanban) return '📊'
  if (entry.dashboard) return '🏠'
  if (entry.graph) return '🕸️'
  return '📄'
}
</script>

<template>
  <aside
    class="sidebar"
    :class="{ collapsed: uiStore.sidebarCollapsed, 'mobile-open': uiStore.sidebarMobileOpen }"
  >
    <div class="sidebar-header">
      <RouterLink to="/" class="logo">
        {{ appName }}
      </RouterLink>
      <button class="collapse-btn" @click="uiStore.toggleSidebar">
        {{ uiStore.sidebarCollapsed ? '→' : '←' }}
      </button>
    </div>

    <!-- Fixed top items: Search and Analysis -->
    <div class="sidebar-top-items">
      <RouterLink to="/search" class="nav-item" :class="{ active: route.path === '/search' }">
        <span class="nav-icon">🔍</span>
        <span class="nav-label">Search</span>
        <kbd v-if="!uiStore.sidebarCollapsed">/</kbd>
      </RouterLink>
      <RouterLink to="/analyze" class="nav-item" :class="{ active: route.path === '/analyze' }">
        <span class="nav-icon">⚠️</span>
        <span class="nav-label">Analysis</span>
      </RouterLink>
    </div>

    <nav class="sidebar-nav">
      <template v-for="(entry, index) in navigation" :key="index">
        <div v-if="entry.group" class="nav-section">
          <div class="nav-section-title">{{ entry.group }}</div>
          <RouterLink
            v-for="(item, itemIndex) in entry.items"
            :key="itemIndex"
            :to="getHref(item)"
            class="nav-item"
            :class="{ active: isActive(getHref(item)) }"
          >
            <span class="nav-icon">{{ getIcon(item) }}</span>
            <span class="nav-label">{{ item.label }}</span>
          </RouterLink>
        </div>
        <RouterLink
          v-else
          :to="getHref(entry)"
          class="nav-item"
          :class="{ active: isActive(getHref(entry)) }"
        >
          <span class="nav-icon">{{ getIcon(entry) }}</span>
          <span class="nav-label">{{ entry.label }}</span>
        </RouterLink>
      </template>
    </nav>

    <div class="sidebar-footer">
      <RouterLink to="/settings" class="settings-link" :class="{ active: route.path === '/settings' }">
        <span class="nav-icon">⚙️</span>
        <span class="nav-label">Settings</span>
      </RouterLink>
      <a href="/" class="version-switch" title="Switch to v1">
        v1 ↗
      </a>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 240px;
  height: 100vh;
  background: var(--sidebar-bg, #1a1a2e);
  color: var(--sidebar-text, #e8e8e8);
  display: flex;
  flex-direction: column;
  position: fixed;
  left: 0;
  top: 0;
  transition: width 0.2s ease;
  z-index: 100;
}

.sidebar.collapsed {
  width: 60px;
}

.sidebar.collapsed .nav-label,
.sidebar.collapsed .nav-section-title,
.sidebar.collapsed .logo {
  display: none;
}

.sidebar-header {
  padding: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.logo {
  font-weight: 600;
  font-size: 18px;
  color: inherit;
  text-decoration: none;
}

.collapse-btn {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  padding: 4px 8px;
  opacity: 0.7;
}

.collapse-btn:hover {
  opacity: 1;
}

.sidebar-top-items {
  padding: 8px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.sidebar-nav {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.nav-shortcut {
  margin-left: auto;
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
}

.nav-divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.1);
  margin: 8px 16px;
}

.nav-section {
  margin-bottom: 8px;
}

.nav-section-title {
  padding: 8px 16px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.6;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  color: inherit;
  text-decoration: none;
  transition: background 0.15s ease;
}

.nav-item:hover {
  background: rgba(255, 255, 255, 0.1);
}

.nav-item.active {
  background: rgba(255, 255, 255, 0.15);
  border-right: 3px solid var(--accent-color, #6366f1);
}

.nav-icon {
  width: 24px;
  margin-right: 12px;
  text-align: center;
}

.sidebar.collapsed .nav-icon {
  margin-right: 0;
}

.nav-label {
  font-size: 14px;
}

.sidebar-footer {
  padding: 12px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.settings-link {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  color: inherit;
  text-decoration: none;
  transition: background 0.15s ease;
}

.settings-link:hover {
  background: rgba(255, 255, 255, 0.1);
}

.settings-link.active {
  background: rgba(255, 255, 255, 0.15);
}

.settings-link .nav-icon {
  width: 24px;
  margin-right: 12px;
  text-align: center;
}

.sidebar.collapsed .settings-link .nav-label {
  display: none;
}

.sidebar.collapsed .settings-link .nav-icon {
  margin-right: 0;
}

.version-switch {
  font-size: 12px;
  color: inherit;
  opacity: 0.6;
  text-decoration: none;
  padding: 4px 16px;
}

.version-switch:hover {
  opacity: 1;
}

/* Mobile overlay */
@media (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
  }

  .sidebar.mobile-open {
    transform: translateX(0);
  }
}
</style>
