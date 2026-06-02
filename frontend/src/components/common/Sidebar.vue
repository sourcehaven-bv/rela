<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore, useGitStore } from '@/stores'
import { useScriptErrorStore } from '@/stores/scriptError'
import { getSidebar, runAction } from '@/api'
import { isCancelledFetch } from '@/composables/usePageData'
import { isScriptError } from '@/types/scriptError'
import type { SidebarGroup, SidebarItem } from '@/types'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const gitStore = useGitStore()
const scriptErrorStore = useScriptErrorStore()
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

// Load sidebar data
async function loadSidebar() {
  try {
    const data = await getSidebar()
    sidebarAppName.value = data.app.name
    sidebarGroups.value = data.navigation
    schemaStore.setLogoUrl(data.logoUrl ?? null)
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load sidebar:', err)
  }
}

// Keyboard shortcut for search.
//
// Defers to a list view's in-place search box when one is rendered — list
// views own their own search affordance now (TKT-603FQ), and jumping to the
// standalone /search page would surprise users mid-list. The fallback
// behavior (push /search) still applies on routes without a search box.
function handleKeydown(e: KeyboardEvent) {
  if (e.key !== '/') return
  if (['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement)?.tagName)) return
  if (document.querySelector('.entity-list .search-box')) return
  e.preventDefault()
  router.push('/search')
}

// Close mobile sidebar on route change
watch(() => route.path, () => {
  if (uiStore.sidebarMobileOpen) {
    uiStore.closeMobileSidebar()
  }
})

// Lock body scroll when mobile sidebar is open
watch(() => uiStore.sidebarMobileOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})

function handleKeydownAll(e: KeyboardEvent) {
  handleKeydown(e)
  if (e.key === 'Escape' && uiStore.sidebarMobileOpen) {
    uiStore.closeMobileSidebar()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydownAll)
  loadSidebar()
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydownAll)
  document.body.style.overflow = ''
})

function isActive(href: string): boolean {
  return route.path === href || route.path.startsWith(href + '/')
}

function getIconEmoji(icon?: string): string {
  switch (icon) {
    case 'list': return '📋'
    case 'kanban': return '📊'
    case 'dashboard': return '🏠'
    default: return '📄'
  }
}

async function handleAction(item: SidebarItem, ev?: Event) {
  if (!item.action) return
  if (actionInFlight.value.has(item.action)) return

  const triggerEl =
    ev && ev.currentTarget instanceof HTMLElement ? ev.currentTarget : null

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
    if (isScriptError(err)) {
      scriptErrorStore.show(err, triggerEl)
    } else {
      let msg = 'Action failed'
      const e = err as { response?: { data?: { correlation_id?: string } }; correlation_id?: string }
      const corrID = e?.response?.data?.correlation_id ?? e?.correlation_id
      if (corrID) {
        msg = `Action failed (ref: ${corrID})`
      }
      uiStore.error(msg)
    }
  } finally {
    actionInFlight.value.delete(item.action)
  }
}
</script>

<template>
  <aside
    id="main-sidebar"
    class="sidebar"
    :class="{ collapsed: uiStore.sidebarCollapsed, 'mobile-open': uiStore.sidebarMobileOpen }"
  >
    <div class="sidebar-header">
      <RouterLink to="/" class="logo" :aria-label="appName">
        <img v-if="logoUrl" :src="logoUrl" :alt="appName" class="logo-img" />
        <span v-else>{{ appName }}</span>
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
              @click="handleAction(item, $event)"
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
              @click="handleAction(item, $event)"
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

    <!-- Mobile-only footer: git status, settings, theme toggle -->
    <div class="sidebar-mobile-footer">
      <div v-if="gitStore.isAvailable" class="mobile-git-status" :class="gitStore.statusClass">
        <span class="mobile-git-dot"/>
        <span class="nav-label">{{ gitStore.branch }} · {{ gitStore.statusText }}</span>
      </div>
      <RouterLink to="/settings" class="nav-item" :class="{ active: route.path === '/settings' }">
        <span class="nav-icon">⚙️</span>
        <span class="nav-label">Settings</span>
      </RouterLink>
      <button
        v-if="!schemaStore.darkDisabled"
        class="nav-item nav-action"
        @click="uiStore.toggleDarkMode()"
      >
        <span class="nav-icon">{{ uiStore.isDark ? '☀️' : '🌙' }}</span>
        <span class="nav-label">{{ uiStore.isDark ? 'Light Mode' : 'Dark Mode' }}</span>
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="uiStore.sidebarMobileOpen"
        class="sidebar-backdrop"
        @click="uiStore.closeMobileSidebar()"
      />
    </Teleport>
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
  display: flex;
  align-items: center;
  min-width: 0;
}

.logo-img {
  max-height: 28px;
  max-width: 100%;
  object-fit: contain;
  display: block;
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

/* Mobile footer — hidden on desktop */
.sidebar-mobile-footer {
  display: none;
}

.mobile-git-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  font-size: 13px;
  opacity: 0.7;
}

.mobile-git-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}

.mobile-git-status.synced .mobile-git-dot {
  background: var(--success-color);
}

.mobile-git-status.changes .mobile-git-dot {
  background: var(--warning-color);
}

.mobile-git-status.conflict .mobile-git-dot {
  background: var(--error-color);
}

/* Mobile overlay */
@media (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
    height: 100vh;
    padding-top: env(safe-area-inset-top, 0px);
    transition: transform 0.25s ease;
  }

  /* When the mobile sidebar is open, the hamburger button overlays the
     sidebar header. Indent the header so the title isn't covered. */
  .sidebar.mobile-open .sidebar-header {
    padding-left: 60px;
  }

  .sidebar.mobile-open {
    transform: translateX(0);
  }

  .sidebar.collapsed {
    width: 240px;
  }

  .sidebar.collapsed .nav-label,
  .sidebar.collapsed .nav-section-title,
  .sidebar.collapsed .logo {
    display: unset;
  }

  .sidebar.collapsed .nav-icon {
    margin-right: 12px;
  }

  .collapse-btn {
    display: none;
  }

  .nav-item {
    padding: 12px 16px;
    min-height: 44px;
  }

  .sidebar-mobile-footer {
    display: block;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
    padding: 8px 0;
    margin-top: auto;
  }
}
</style>

<style>
/* Backdrop must be unscoped to work with Teleport */
.sidebar-backdrop {
  display: none;
}

@media (max-width: 768px) {
  .sidebar-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 99;
  }
}
</style>
