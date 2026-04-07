<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import { getSidebar, runAction } from '@/api'
import type { SidebarGroup, SidebarItem } from '@/types'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const route = useRoute()
const router = useRouter()

// Tracks which action items are currently in-flight (prevents double-click).
const actionInFlight = ref<Set<string>>(new Set())

// Sidebar data from API
const sidebarGroups = ref<SidebarGroup[]>([])
const sidebarAppName = ref('')

const appName = computed(() => sidebarAppName.value || schemaStore.app.name)

// Load sidebar data
async function loadSidebar() {
  try {
    const data = await getSidebar()
    sidebarAppName.value = data.app.name
    sidebarGroups.value = data.navigation
  } catch (err) {
    console.error('Failed to load sidebar:', err)
  }
}

// Keyboard shortcut for search
function handleKeydown(e: KeyboardEvent) {
  if (e.key === '/' && !['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement)?.tagName)) {
    e.preventDefault()
    router.push('/search')
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  loadSidebar()
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})

function isActive(href: string): boolean {
  return route.path === href || route.path.startsWith(href + '/')
}

function getIconEmoji(icon?: string): string {
  switch (icon) {
    case 'list': return '📋'
    case 'kanban': return '📊'
    case 'dashboard': return '🏠'
    case 'graph': return '🕸️'
    default: return '📄'
  }
}

async function handleAction(item: SidebarItem) {
  if (!item.action) return
  if (actionInFlight.value.has(item.action)) return

  actionInFlight.value.add(item.action)
  try {
    const response = await runAction(item.action)
    if (response?.message) {
      const type = response.message_type || 'success'
      uiStore[type](response.message)
    }
    if (response?.redirect) {
      router.push(response.redirect)
    }
  } catch (err: unknown) {
    let msg = 'Action failed'
    const e = err as { response?: { data?: { correlation_id?: string } } }
    const corrID = e?.response?.data?.correlation_id
    if (corrID) {
      msg = `Action failed (ref: ${corrID})`
    }
    uiStore.error(msg)
  } finally {
    actionInFlight.value.delete(item.action)
  }
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
      <template v-for="(group, index) in sidebarGroups" :key="index">
        <div v-if="group.group" class="nav-section">
          <div class="nav-section-title">{{ group.group }}</div>
          <template v-for="item in group.items" :key="item.label + (item.href || item.action || '')">
            <button
              v-if="item.action"
              type="button"
              class="nav-item nav-action"
              :aria-label="item.label"
              :disabled="actionInFlight.has(item.action)"
              @click="handleAction(item)"
            >
              <span class="nav-icon">{{ getIconEmoji(item.icon) }}</span>
              <span class="nav-label">{{ item.label }}</span>
            </button>
            <RouterLink
              v-else-if="item.href"
              :to="item.href"
              class="nav-item"
              :class="{ active: isActive(item.href) }"
            >
              <span class="nav-icon">{{ getIconEmoji(item.icon) }}</span>
              <span class="nav-label">{{ item.label }}</span>
              <span v-if="item.count !== undefined && !uiStore.sidebarCollapsed" class="nav-count">{{ item.count }}</span>
            </RouterLink>
          </template>
        </div>
        <template v-else>
          <template v-for="item in group.items" :key="item.label + (item.href || item.action || '')">
            <button
              v-if="item.action"
              type="button"
              class="nav-item nav-action"
              :aria-label="item.label"
              :disabled="actionInFlight.has(item.action)"
              @click="handleAction(item)"
            >
              <span class="nav-icon">{{ getIconEmoji(item.icon) }}</span>
              <span class="nav-label">{{ item.label }}</span>
            </button>
            <RouterLink
              v-else-if="item.href"
              :to="item.href"
              class="nav-item"
              :class="{ active: isActive(item.href) }"
            >
              <span class="nav-icon">{{ getIconEmoji(item.icon) }}</span>
              <span class="nav-label">{{ item.label }}</span>
              <span v-if="item.count !== undefined && !uiStore.sidebarCollapsed" class="nav-count">{{ item.count }}</span>
            </RouterLink>
          </template>
        </template>
      </template>
    </nav>

    </aside>
</template>

<style scoped>
.sidebar {
  width: 240px;
  height: calc(100vh - 24px); /* Account for status bar */
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
  flex: 1;
}

.nav-count {
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 500;
  min-width: 20px;
  text-align: center;
}

/* Action buttons in the sidebar — same look as RouterLink nav items */
.nav-action {
  width: 100%;
  background: none;
  border: none;
  color: inherit;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  font-size: inherit;
}

.nav-action:disabled {
  opacity: 0.5;
  cursor: wait;
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
