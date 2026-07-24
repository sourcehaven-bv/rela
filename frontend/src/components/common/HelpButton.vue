<script setup lang="ts">
import { ref } from 'vue'
import HelpModal from '@/components/ui/HelpModal.vue'

/**
 * HelpButton — 44x44 icon button that opens a help popover or modal.
 *
 * Two modes:
 *   - :entity-type   — fetches /api/help/{entityType} and renders inside
 *                      HelpModal (the existing form-help component).
 *   - #content slot  — caller-provided static help body. Renders in a
 *                      lightweight modal so the same touch-friendly UX
 *                      applies on mobile and desktop.
 *
 * Touch target: 44x44 — meets the iOS HIG floor and replaces ad-hoc
 * 28px help buttons in views.
 */
defineProps<{
  entityType?: string
  entityLabel?: string
  title?: string
}>()

defineSlots<{
  content?(): unknown
}>()

const open = ref(false)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function handleOverlayClick(e: MouseEvent) {
  if (e.target === e.currentTarget) {
    close()
  }
}
</script>

<template>
  <button
    type="button"
    class="help-button"
    :title="title || 'Show help'"
    :aria-label="title || 'Show help'"
    :aria-expanded="open"
    @click="toggle"
  >
    ?
  </button>

  <!-- Entity-type help: defers to HelpModal which fetches markup. -->
  <HelpModal
    v-if="entityType"
    :open="open"
    :entity-type="entityType"
    :entity-label="entityLabel"
    @close="close"
  />

  <!-- Static help via #content slot. -->
  <Teleport v-if="!entityType && $slots.content" to="body">
    <div v-if="open" class="help-button__overlay" @click="handleOverlayClick">
      <div class="help-button__modal" role="dialog" aria-modal="true">
        <div class="help-button__header">
          <h3>{{ title || 'Help' }}</h3>
          <button class="help-button__close" :aria-label="'Close help'" @click="close">
            &times;
          </button>
        </div>
        <div class="help-button__body">
          <slot name="content" />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.help-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: 1px solid var(--border-color);
  background: var(--card-bg);
  color: var(--muted-text);
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.help-button:hover,
.help-button[aria-expanded='true'] {
  background: var(--hover-bg);
  color: var(--text-color);
  border-color: var(--accent-color);
}

.help-button__overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.help-button__modal {
  background: var(--card-bg);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
  max-width: 700px;
  width: 90%;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.help-button__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
}

.help-button__header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.help-button__close {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--muted-text);
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.help-button__close:hover {
  color: var(--text-color);
}

.help-button__body {
  padding: 20px;
  overflow-y: auto;
}
</style>
