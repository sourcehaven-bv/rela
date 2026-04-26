<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'

import { useScriptErrorStore } from '../../stores/scriptError'

import ScriptErrorPanel from './ScriptErrorPanel.vue'

const store = useScriptErrorStore()
const closeBtn = ref<HTMLButtonElement | null>(null)

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape' && store.current) {
    e.preventDefault()
    store.dismiss()
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
})

// Move focus to the close button when the dialog opens. Restoring focus to
// the trigger element is the store's job so it survives this component
// being remounted.
watch(
  () => store.current,
  (val) => {
    if (val) {
      // Wait for the DOM to render the dialog before focusing.
      requestAnimationFrame(() => closeBtn.value?.focus())
    }
  },
)
</script>

<template>
  <div
    v-if="store.current"
    class="modal-overlay"
    role="alertdialog"
    aria-modal="true"
    aria-labelledby="script-error-title"
    @click.self="store.dismiss()"
  >
    <div class="modal script-error-modal">
      <div class="se-modal-header">
        <h3 id="script-error-title">Script error</h3>
        <button
          ref="closeBtn"
          type="button"
          class="se-close"
          aria-label="Close"
          @click="store.dismiss()"
        >
          ×
        </button>
      </div>
      <ScriptErrorPanel :error="store.current" />
    </div>
  </div>
</template>

<style scoped>
.script-error-modal {
  max-width: 720px;
  width: 90%;
  max-height: 85vh;
  overflow: auto;
}

.se-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.se-modal-header h3 {
  margin: 0;
}

.se-close {
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  font-size: 24px;
  line-height: 1;
  color: var(--muted-text);
  cursor: pointer;
  border-radius: 4px;
}

.se-close:hover {
  background: var(--card-bg);
  color: var(--text-color);
}
</style>
