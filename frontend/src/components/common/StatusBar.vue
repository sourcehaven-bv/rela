<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useGitStore, useSchemaStore, useUIStore } from '@/stores'
import { shortcutsModalOpen } from '@/composables/useKeyboardShortcuts'

const gitStore = useGitStore()
const uiStore = useUIStore()
const schemaStore = useSchemaStore()
const route = useRoute()
const router = useRouter()

// Initial fetch - SSE handles subsequent updates
onMounted(() => {
  gitStore.fetchStatus().catch(() => {
    // Errors are already handled by the store
  })
})

async function handleSync() {
  try {
    const result = await gitStore.sync()
    if (result.conflict_files && result.conflict_files.length > 0) {
      router.push('/conflicts')
    }
  } catch {
    // Error is already captured in store
  }
}
</script>

<template>
  <footer class="status-bar">
    <!-- Left side: Git status -->
    <div class="status-left">
      <div v-if="gitStore.isAvailable" class="git-status" :class="gitStore.statusClass">
        <div class="status-item" :title="gitStore.syncing ? 'Syncing...' : 'Click to sync'" @click="handleSync">
          <span class="git-branch">{{ gitStore.branch }}</span>
          <span class="git-dot"/>
          <span class="git-status-text">{{ gitStore.statusText }}</span>
        </div>
        <RouterLink
          v-if="gitStore.hasConflicts"
          to="/conflicts"
          class="status-item status-warning"
          title="Resolve conflicts"
        >
          Resolve Conflicts
        </RouterLink>
      </div>
    </div>

    <!-- Right side: Theme, Settings, and shortcuts -->
    <div class="status-right">
      <!--
        Hide the dark/light toggle when the project's palette is in
        Regular mode — there's only one set of colors so the toggle
        would be a no-op (and confusing).
      -->
      <button
        v-if="!schemaStore.darkDisabled"
        class="status-item theme-toggle"
        :title="uiStore.isDark ? 'Switch to light mode' : 'Switch to dark mode'"
        @click="uiStore.toggleDarkMode()"
      >
        <span v-if="uiStore.isDark" class="theme-icon">☀️</span>
        <span v-else class="theme-icon">🌙</span>
      </button>
      <RouterLink
        to="/settings"
        class="status-item"
        :class="{ active: route.path === '/settings' }"
      >
        Settings
      </RouterLink>
      <button
        class="status-item"
        title="Keyboard shortcuts"
        @click="shortcutsModalOpen = true"
      >
        <kbd>?</kbd> Shortcuts
      </button>
    </div>
  </footer>
</template>

<style scoped>
.status-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 24px;
  background: var(--sidebar-bg, #1a1a2e);
  color: var(--sidebar-text, #e8e8e8);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px;
  font-size: 12px;
  z-index: 200;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.status-left,
.status-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  color: inherit;
  text-decoration: none;
  background: none;
  border: none;
  cursor: pointer;
  opacity: 0.8;
  transition: all 0.15s ease;
  border-radius: 2px;
  font-size: 12px;
  height: 20px;
}

.status-item:hover {
  opacity: 1;
  background: rgba(255, 255, 255, 0.1);
}

.status-item.active {
  opacity: 1;
  background: rgba(255, 255, 255, 0.15);
}

.status-warning {
  color: var(--warning-color, #f59e0b);
}

.git-status {
  display: flex;
  align-items: center;
  gap: 4px;
}

.git-branch {
  font-weight: 500;
}

.git-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.git-status.synced .git-dot {
  background: #10b981;
}

.git-status.changes .git-dot {
  background: #f59e0b;
}

.git-status.conflict .git-dot {
  background: #ef4444;
}

.git-status-text {
  opacity: 0.7;
}

.status-bar kbd {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  padding: 1px 4px;
  font-size: 10px;
  color: rgba(255, 255, 255, 0.6);
}

.theme-toggle {
  padding: 2px 6px;
}

.theme-icon {
  font-size: 14px;
  line-height: 1;
}
</style>
