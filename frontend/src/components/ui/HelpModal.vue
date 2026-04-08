<script setup lang="ts">
import { ref, watch } from 'vue'
import axios from 'axios'
import { isCancelledFetch } from '@/composables/usePageData'

const props = defineProps<{
  open: boolean
  entityType: string
  entityLabel?: string
}>()

const emit = defineEmits<{
  close: []
}>()

const loading = ref(false)
const error = ref<string | null>(null)
const helpContent = ref('')

async function loadHelp() {
  if (!props.entityType) return

  loading.value = true
  error.value = null

  try {
    const response = await axios.get(`/api/help/${props.entityType}`)
    helpContent.value = response.data
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load help:', err)
    error.value = 'Failed to load help content'
    helpContent.value = ''
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.open, props.entityType],
  ([isOpen]) => {
    if (isOpen) {
      loadHelp()
    }
  },
  { immediate: true }
)

function handleOverlayClick(e: MouseEvent) {
  if (e.target === e.currentTarget) {
    emit('close')
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="help-overlay" @click="handleOverlayClick">
      <div class="help-modal">
        <div class="help-header">
          <h3>{{ entityLabel || entityType }} Help</h3>
          <button class="close-btn" @click="emit('close')">&times;</button>
        </div>
        <div class="help-body">
          <div v-if="loading" class="loading-state">
            <div class="spinner"/>
            <span>Loading...</span>
          </div>
          <div v-else-if="error" class="error-state">
            {{ error }}
          </div>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div v-else class="help-content-wrapper" v-html="helpContent"/>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.help-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.help-modal {
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

.help-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.help-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--muted-text);
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color);
}

.help-body {
  padding: 20px;
  overflow-y: auto;
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px;
  color: var(--muted-text);
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border-color, #e2e8f0);
  border-top-color: var(--accent-color, #6366f1);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error-state {
  color: var(--error-color, #ef4444);
  text-align: center;
  padding: 40px;
}

/* Styles for help content from server */
.help-content-wrapper :deep(.help-content) {
  font-size: 14px;
  color: var(--text-color);
}

.help-content-wrapper :deep(.help-section) {
  margin-bottom: 24px;
}

.help-content-wrapper :deep(.help-section:last-child) {
  margin-bottom: 0;
}

.help-content-wrapper :deep(.help-entity-desc) {
  font-size: 15px;
  color: var(--text-color);
  line-height: 1.6;
}

.help-content-wrapper :deep(h4) {
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
  margin: 0 0 12px;
}

.help-content-wrapper :deep(.help-section-hint) {
  font-size: 12px;
  color: #94a3b8;
  margin: -8px 0 12px;
}

.help-content-wrapper :deep(.help-item) {
  padding: 10px 12px;
  background: var(--hover-bg);
  border-radius: 6px;
  margin-bottom: 8px;
}

.help-content-wrapper :deep(.help-item:last-child) {
  margin-bottom: 0;
}

.help-content-wrapper :deep(.help-item-header) {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.help-content-wrapper :deep(.help-item-header code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
}

.help-content-wrapper :deep(.help-item-meta) {
  font-size: 12px;
  color: var(--muted-text);
}

.help-content-wrapper :deep(.help-required) {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  background: #fef2f2;
  color: #dc2626;
  padding: 2px 6px;
  border-radius: 4px;
}

.help-content-wrapper :deep(.help-item-desc) {
  margin-top: 6px;
  font-size: 13px;
  color: var(--muted-text);
  line-height: 1.5;
}

.help-content-wrapper :deep(.help-empty) {
  text-align: center;
  color: #94a3b8;
  font-style: italic;
}
</style>
