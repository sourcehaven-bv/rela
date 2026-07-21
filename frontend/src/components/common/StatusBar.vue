<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useGitStore, useSchemaStore, useUIStore } from '@/stores'
import { shortcutsModalOpen } from '@/composables/useKeyboardShortcuts'
import { renderMarkdown } from '@/utils/markdown'

const gitStore = useGitStore()
const uiStore = useUIStore()
const schemaStore = useSchemaStore()
const route = useRoute()
const router = useRouter()

// Global "About" help: the deployment description (data-entry app.description,
// falling back to the metamodel's top-level description on the server). The
// button is shown only when there is a description to show (TKT-DUQBD0).
const aboutOpen = ref(false)
const appName = computed(() => schemaStore.app?.name || 'rela')
const appDescription = computed(() => schemaStore.aboutDescription?.trim() || '')
// The description is authored as markdown (in data-entry.yaml or the metamodel);
// render it so *emphasis*, lists, etc. display. renderMarkdown sanitizes.
const appDescriptionHtml = computed(() => renderMarkdown(appDescription.value))

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
      <button
        v-if="appDescription"
        class="status-item about-btn"
        title="About this app"
        @click="aboutOpen = true"
      >
        <span class="about-icon">ⓘ</span> <span class="about-text">About</span>
      </button>
      <RouterLink
        to="/settings"
        class="status-item"
        :class="{ active: route.path === '/settings' }"
      >
        Settings
      </RouterLink>
      <button
        class="status-item shortcuts-btn"
        title="Keyboard shortcuts"
        @click="shortcutsModalOpen = true"
      >
        <kbd>?</kbd> <span class="shortcuts-text">Shortcuts</span>
      </button>
    </div>

    <!-- About overlay: the deployment description. Teleported so it isn't
         clipped by the fixed status bar. -->
    <Teleport to="body">
      <div v-if="aboutOpen" class="about-overlay" @click.self="aboutOpen = false">
        <div class="about-modal">
          <div class="about-header">
            <h3>{{ appName }}</h3>
            <button class="about-close" @click="aboutOpen = false">&times;</button>
          </div>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div class="about-body" v-html="appDescriptionHtml"/>
        </div>
      </div>
    </Teleport>
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

.about-icon {
  font-size: 13px;
  line-height: 1;
}

.about-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.about-modal {
  background: var(--card-bg);
  color: var(--text-color);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
  max-width: 560px;
  width: 90%;
  max-height: 80vh;
  overflow: auto;
  padding: 0 0 20px;
}

.about-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.about-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.about-close {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--muted-text);
  cursor: pointer;
  line-height: 1;
}

.about-body {
  padding: 16px 20px 0;
  font-size: 14px;
  line-height: 1.6;
}

.about-body :deep(p) {
  margin: 0 0 10px;
}

.about-body :deep(p:last-child) {
  margin-bottom: 0;
}

@media (max-width: 768px) {
  .status-bar {
    display: none;
  }
}
</style>
