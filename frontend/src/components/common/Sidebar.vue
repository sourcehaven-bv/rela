<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import type { NavigationEntry } from '@/types'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const route = useRoute()

const appName = computed(() => schemaStore.app.name)
const navigation = computed(() => schemaStore.navigation)

function isActive(href: string): boolean {
  return route.path === href || route.path.startsWith(href + '/')
}

function getIcon(entry: NavigationEntry): string {
  if (entry.icon) return entry.icon
  if (entry.href?.startsWith('/list/')) return '📋'
  if (entry.href?.startsWith('/kanban/')) return '📊'
  if (entry.href?.startsWith('/dashboard')) return '🏠'
  if (entry.href?.startsWith('/search')) return '🔍'
  if (entry.href?.startsWith('/graph')) return '🕸️'
  if (entry.href?.startsWith('/analyze')) return '⚠️'
  if (entry.href?.startsWith('/settings')) return '⚙️'
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

    <nav class="sidebar-nav">
      <template v-for="(entry, index) in navigation" :key="index">
        <div v-if="entry.type === 'divider'" class="nav-divider"></div>

        <div v-else-if="entry.type === 'section'" class="nav-section">
          <div class="nav-section-title">{{ entry.label }}</div>
          <template v-if="entry.items">
            <RouterLink
              v-for="(item, itemIndex) in entry.items"
              :key="itemIndex"
              :to="item.href || '/'"
              class="nav-item"
              :class="{ active: isActive(item.href || '') }"
            >
              <span class="nav-icon">{{ getIcon(item) }}</span>
              <span class="nav-label">{{ item.label }}</span>
            </RouterLink>
          </template>
        </div>

        <RouterLink
          v-else-if="entry.type === 'link' && entry.href"
          :to="entry.href"
          class="nav-item"
          :class="{ active: isActive(entry.href) }"
        >
          <span class="nav-icon">{{ getIcon(entry) }}</span>
          <span class="nav-label">{{ entry.label }}</span>
        </RouterLink>
      </template>
    </nav>

    <div class="sidebar-footer">
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

.sidebar-nav {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
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
  padding: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.version-switch {
  font-size: 12px;
  color: inherit;
  opacity: 0.6;
  text-decoration: none;
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
