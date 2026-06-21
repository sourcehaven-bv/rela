<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { getErrorMessage } from '@/api'
import { useKeyboardShortcuts, shortcutsModalOpen, paletteOpen, useEvents } from '@/composables'
import { useConfirmHost } from '@/composables/useConfirm'
import Sidebar from '@/components/common/Sidebar.vue'
import StatusBar from '@/components/common/StatusBar.vue'
import Toast from '@/components/common/Toast.vue'
import ScriptErrorDialog from '@/components/common/ScriptErrorDialog.vue'
import KeyboardShortcutsModal from '@/components/ui/KeyboardShortcutsModal.vue'
import CommandPaletteModal from '@/components/ui/CommandPaletteModal.vue'
import ConfirmModal from '@/components/ui/ConfirmModal.vue'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const loading = ref(true)
const error = ref<string | null>(null)

// Initialize global keyboard shortcuts
useKeyboardShortcuts()

// Initialize SSE connection for real-time updates
useEvents()

// Single global confirm modal — driven by useConfirm() from anywhere.
const { state: confirmState, onConfirmEvent, onCancelEvent } = useConfirmHost()

// useConfirmHost re-throws errors from onConfirm callbacks so the modal stays
// open with busy cleared (caller has already surfaced the error via toast).
// Don't let those become unhandled-rejection warnings at the modal boundary.
function handleConfirm() {
  onConfirmEvent().catch(() => {})
}

onMounted(async () => {
  try {
    await schemaStore.load()
  } catch (err) {
    error.value = getErrorMessage(err, 'Failed to load application')
    uiStore.error(error.value)
  } finally {
    loading.value = false
  }
})

// Apply palette CSS variables when schema loads, theme toggles, or
// the saved palette changes (e.g. after the user clicks Save Palette
// in Settings — schemaStore.reload() rewrites paletteLight/paletteDark/
// darkDisabled, this watch picks it up and re-applies inline styles
// to <html> so the change is visible immediately on the current
// screen).
//
// When the project palette has dark disabled (Regular mode), we
// always render the light palette regardless of the user's global
// dark toggle, AND we strip the `dark` class from <html> so any
// dark-mode CSS rules don't apply. The toggle button is hidden in
// the status bar in this case.
watch(
  [
    () => schemaStore.loaded,
    () => uiStore.darkMode,
    () => schemaStore.paletteLight,
    () => schemaStore.paletteDark,
    () => schemaStore.darkDisabled,
  ],
  () => {
    if (!schemaStore.loaded) return

    // Regular-mode project: force-render as light, no html.dark class.
    if (schemaStore.darkDisabled) {
      document.documentElement.classList.remove('dark')
      if (Object.keys(schemaStore.paletteLight).length > 0) {
        uiStore.applyPalette(schemaStore.paletteLight)
      }
      return
    }

    // Light+Dark project: respect the user's global toggle.
    const palette = uiStore.darkMode ? schemaStore.paletteDark : schemaStore.paletteLight
    if (Object.keys(palette).length > 0) {
      uiStore.applyPalette(palette)
    }
  },
  { immediate: true }
)
</script>

<template>
  <div v-if="loading" class="loading-screen">
    <div class="spinner"/>
    <p>Loading...</p>
  </div>

  <div v-else-if="error" class="error-screen">
    <h1>Error</h1>
    <p>{{ error }}</p>
    <button @click="schemaStore.reload()">Retry</button>
  </div>

  <div v-else class="app-layout">
    <Sidebar />
    <button
      class="mobile-menu-btn"
      :aria-expanded="uiStore.sidebarMobileOpen"
      aria-label="Toggle navigation"
      aria-controls="main-sidebar"
      @click="uiStore.sidebarMobileOpen ? uiStore.closeMobileSidebar() : uiStore.openMobileSidebar()"
    >
      ☰
    </button>
    <main class="main-content" :class="{ 'sidebar-collapsed': uiStore.sidebarCollapsed }">
      <RouterView />
    </main>
    <StatusBar />
    <Toast />
    <ScriptErrorDialog />
    <KeyboardShortcutsModal
      :open="shortcutsModalOpen"
      @close="shortcutsModalOpen = false"
    />
  </div>

  <!-- Mounted unconditionally so Cmd+K works during schema loading and on
       the error screen, mirroring the ConfirmModal hoist below. -->
  <CommandPaletteModal :open="paletteOpen" @close="paletteOpen = false" />

  <!-- Mounted unconditionally (outside the loading/error/loaded branches) so
       any caller of useConfirm() resolves to a rendered modal even during
       schema loading or on the error screen. Without this hoist, callers
       would deadlock on a forever-pending promise. -->
  <ConfirmModal
    :open="confirmState.open"
    :title="confirmState.title"
    :message="confirmState.message"
    :confirm-label="confirmState.confirmLabel"
    :cancel-label="confirmState.cancelLabel"
    :busy="confirmState.busy"
    :danger="confirmState.danger"
    @confirm="handleConfirm"
    @cancel="onCancelEvent"
  />
</template>

<style>
/* Theme tokens (:root / :root.dark) live in src/styles/tokens.css, imported
   from main.ts — they are a shared source so custom apps can serve the same
   values. See tokens.css. */

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  font-family: 'Open Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background: var(--bg-color);
  color: var(--text-color);
  line-height: 1.5;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.app-layout {
  display: flex;
  min-height: 100vh;
}

.main-content {
  flex: 1;
  margin-left: 240px;
  padding: 24px;
  padding-bottom: 48px; /* Account for status bar */
  transition: margin-left 0.2s ease;
}

.main-content.sidebar-collapsed {
  margin-left: 60px;
}

.loading-screen,
.error-screen {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  gap: 16px;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error-screen h1 {
  color: var(--error-color);
}

.error-screen button {
  padding: 8px 16px;
  background: var(--accent-color);
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

/* Mobile hamburger button — visible only on small screens */
.mobile-menu-btn {
  display: none;
  position: fixed;
  top: 8px;
  left: 8px;
  z-index: 101;
  width: 44px;
  height: 44px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  font-size: 20px;
  line-height: 1;
  color: var(--text-color);
  cursor: pointer;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

@media (max-width: 768px) {
  .mobile-menu-btn {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .main-content {
    margin-left: 0;
    padding: 16px;
    padding-top: 60px; /* Space for hamburger button */
    padding-bottom: 16px; /* No status bar on mobile */
  }

  .main-content.sidebar-collapsed {
    margin-left: 0;
  }

  .page-header {
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 16px;
  }

  .page-header h1 {
    font-size: 20px;
  }

  .header-actions {
    flex-wrap: wrap;
    gap: 8px;
  }

  /* Hide keyboard shortcut hints on mobile — !important needed to
     override scoped component styles that set display: inline-flex */
  kbd {
    display: none !important;
  }

  /* EasyMDE toolbar responsive */
  .EasyMDEContainer .editor-toolbar {
    overflow-x: auto;
    flex-wrap: nowrap;
  }
}

@media (max-width: 480px) {
  .main-content {
    padding: 12px;
    padding-top: 56px;
    padding-bottom: 12px;
  }

  .modal {
    padding: 16px;
    width: 95%;
  }
}

/* Keyboard shortcut hints */
kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-bottom-width: 2px;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
  color: var(--muted-text);
  line-height: 1;
  vertical-align: middle;
}

kbd + kbd {
  margin-left: 2px;
}

.btn kbd,
button kbd {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
  color: rgba(255, 255, 255, 0.8);
  font-size: 10px;
  height: 16px;
  min-width: 16px;
  margin-left: 4px;
}

.btn-secondary kbd {
  background: var(--bg-color);
  border-color: var(--border-color);
  color: var(--muted-text);
}

.sidebar kbd {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
  color: rgba(255, 255, 255, 0.4);
}

/* ==========================================================================
   Shared Button Utilities
   ========================================================================== */

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s ease;
  text-decoration: none;
  white-space: nowrap;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

.btn-primary {
  background: var(--accent-color);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  filter: brightness(1.1);
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover:not(:disabled) {
  filter: brightness(0.95);
}

.btn-danger {
  background: var(--error-color);
  color: white;
}

.btn-danger:hover:not(:disabled) {
  filter: brightness(0.9);
}

.btn-ghost {
  background: transparent;
  color: var(--text-color);
}

.btn-ghost:hover:not(:disabled) {
  background: var(--hover-bg);
}

/* ==========================================================================
   Shared Loading States
   ========================================================================== */

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: var(--muted-text);
}

.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: var(--muted-text);
}

/* ==========================================================================
   Shared Modal Styles
   ========================================================================== */

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 24px;
  max-width: 500px;
  width: 90%;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2);
}

.modal h3 {
  margin: 0 0 12px;
  color: var(--text-color);
}

.modal p {
  margin: 0 0 24px;
  color: var(--muted-text);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* ==========================================================================
   Page Header Utility
   ========================================================================== */

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 12px;
}
</style>
