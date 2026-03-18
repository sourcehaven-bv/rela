<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { renderDocument } from '@/api/documents'
import type { DocumentConfig } from '@/types'

const props = defineProps<{
  entityType: string
  entityId: string
}>()

const schemaStore = useSchemaStore()
const uiStore = useUIStore()

// State
const selectedDoc = ref<string | null>(null)
const docContent = ref<string>('')
const loading = ref(false)
const isCached = ref(false)
const entityIds = ref<string[]>([]) // Entity IDs involved in current document
let eventSource: EventSource | null = null

// Find documents that apply to this entity type
const availableDocuments = computed(() => {
  const docs: Array<{ name: string; config: DocumentConfig }> = []
  for (const [name, config] of schemaStore.documents) {
    // Use entity_type for filtering if available, fallback to view for backward compatibility
    const targetType = config.entity_type || config.view
    if (targetType === props.entityType) {
      docs.push({ name, config })
    }
  }
  return docs
})

// Auto-select first document when available
watch(availableDocuments, (docs) => {
  if (docs.length > 0 && !selectedDoc.value) {
    selectedDoc.value = docs[0].name
  }
}, { immediate: true })

// Load document when selection changes
watch([selectedDoc, () => props.entityId], async () => {
  if (selectedDoc.value && props.entityId) {
    await loadDocument()
  }
}, { immediate: true })

async function loadDocument(refresh = false) {
  if (!selectedDoc.value) return

  console.log('[DocumentsPanel] loadDocument called, refresh:', refresh)
  loading.value = true
  docContent.value = ''

  try {
    const result = await renderDocument(selectedDoc.value, props.entityId, refresh)
    console.log('[DocumentsPanel] Document rendered, entity_ids:', result.entity_ids)
    docContent.value = result.html
    isCached.value = result.cached
    entityIds.value = result.entity_ids || []
    console.log('[DocumentsPanel] Updated entityIds:', entityIds.value)
  } catch (err) {
    console.error('Failed to render document:', err)
    uiStore.error('Failed to render document')
    docContent.value = ''
    entityIds.value = []
  } finally {
    loading.value = false
  }
}

// Get API base URL - prefer runtime injection (for tests) over build-time env var
function getApiBaseUrl(): string {
  // Check for runtime injection (used by e2e tests to bypass Vite proxy)
  if (typeof window !== 'undefined' && (window as { __RELA_API_BASE__?: string }).__RELA_API_BASE__) {
    return (window as { __RELA_API_BASE__?: string }).__RELA_API_BASE__!
  }
  // Fall back to build-time env var
  return import.meta.env.VITE_API_BASE || ''
}

// SSE subscription for live updates
function setupSSE() {
  if (eventSource) return // Already connected

  const baseUrl = getApiBaseUrl()
  const sseUrl = `${baseUrl}/api/v1/_events`
  console.log('[DocumentsPanel] Setting up SSE connection to:', sseUrl)
  console.log('[DocumentsPanel] Current entityIds:', entityIds.value)
  eventSource = new EventSource(sseUrl)

  eventSource.onopen = () => {
    console.log('[DocumentsPanel] SSE connection opened')
  }

  eventSource.onerror = (e) => {
    console.log('[DocumentsPanel] SSE connection error:', e)
  }

  // Handle entity change events
  const entityEvents = ['entity:created', 'entity:updated', 'entity:deleted']
  for (const eventType of entityEvents) {
    eventSource.addEventListener(eventType, (event: MessageEvent) => {
      console.log('[DocumentsPanel] SSE event received:', eventType, event.data)
      try {
        const data = JSON.parse(event.data) as { type?: string; id?: string }
        console.log('[DocumentsPanel] Parsed event data:', data)
        console.log('[DocumentsPanel] Current entityIds:', entityIds.value)
        console.log('[DocumentsPanel] ID match:', data.id && entityIds.value.includes(data.id))
        // Check if this entity is involved in our document
        if (data.id && entityIds.value.includes(data.id)) {
          console.log('[DocumentsPanel] Refreshing document due to entity change')
          // Refetch document (will re-render if stale)
          loadDocument(true)
        }
      } catch (err) {
        console.log('[DocumentsPanel] SSE parse error:', err)
      }
    })
  }
}

function cleanupSSE() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

onMounted(() => {
  setupSSE()
})

onUnmounted(() => {
  cleanupSSE()
})

function getDocTitle(name: string, config: DocumentConfig): string {
  return config.title || name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
}
</script>

<template>
  <section v-if="availableDocuments.length > 0" class="documents-panel">
    <header class="panel-header">
      <h2>Documents</h2>
      <div class="header-controls">
        <select
          v-if="availableDocuments.length > 1"
          v-model="selectedDoc"
          class="doc-select"
        >
          <option
            v-for="doc in availableDocuments"
            :key="doc.name"
            :value="doc.name"
          >
            {{ getDocTitle(doc.name, doc.config) }}
          </option>
        </select>
        <button
          class="btn btn-sm btn-secondary"
          :disabled="loading"
          @click="loadDocument(true)"
          title="Refresh document"
        >
          <span v-if="loading" class="spinner-sm" />
          <span v-else>Refresh</span>
        </button>
      </div>
    </header>

    <div v-if="loading && !docContent" class="loading-state">
      <div class="spinner" />
      <span>Rendering document...</span>
    </div>

    <div v-else-if="docContent" class="document-content">
      <div v-if="isCached" class="cached-badge">cached</div>
      <div class="document-body" v-html="docContent" />
    </div>

    <div v-else class="empty-state">
      <p>No document content available</p>
    </div>
  </section>
</template>

<style scoped>
.documents-panel {
  background: white;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  margin-bottom: 24px;
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  color: #374151;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.doc-select {
  padding: 6px 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  background: white;
  font-size: 14px;
  cursor: pointer;
}

.doc-select:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

.btn-secondary {
  background: var(--border-color, #e2e8f0);
  color: var(--text-color, #1e293b);
}

.btn-secondary:hover:not(:disabled) {
  background: #cbd5e1;
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: #64748b;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.spinner-sm {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.document-content {
  position: relative;
  padding: 24px;
}

.cached-badge {
  position: absolute;
  top: 12px;
  right: 12px;
  padding: 2px 8px;
  background: #f1f5f9;
  border-radius: 4px;
  font-size: 11px;
  color: #64748b;
  text-transform: uppercase;
}

.document-body {
  font-size: 15px;
  line-height: 1.7;
  color: #374151;
}

/* Style injected HTML content */
.document-body :deep(h1),
.document-body :deep(h2),
.document-body :deep(h3) {
  margin: 24px 0 12px;
  color: #1e293b;
}

.document-body :deep(h1:first-child),
.document-body :deep(h2:first-child),
.document-body :deep(h3:first-child) {
  margin-top: 0;
}

.document-body :deep(h1) {
  font-size: 24px;
}

.document-body :deep(h2) {
  font-size: 20px;
}

.document-body :deep(h3) {
  font-size: 16px;
}

.document-body :deep(p) {
  margin: 0 0 12px;
}

.document-body :deep(ul),
.document-body :deep(ol) {
  margin: 0 0 12px;
  padding-left: 24px;
}

.document-body :deep(li) {
  margin-bottom: 4px;
}

.document-body :deep(pre) {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 12px;
  overflow-x: auto;
  font-size: 13px;
}

.document-body :deep(code) {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

.document-body :deep(pre code) {
  background: none;
  padding: 0;
}

.document-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0;
}

.document-body :deep(th),
.document-body :deep(td) {
  padding: 8px 12px;
  text-align: left;
  border: 1px solid #e2e8f0;
}

.document-body :deep(th) {
  background: #f8fafc;
  font-weight: 600;
}

.document-body :deep(hr) {
  border: none;
  border-top: 1px solid #e2e8f0;
  margin: 24px 0;
}

.document-body :deep(blockquote) {
  margin: 12px 0;
  padding: 12px 16px;
  background: #f8fafc;
  border-left: 4px solid var(--accent-color);
  color: #64748b;
}

.document-body :deep(img) {
  max-width: 100%;
  height: auto;
}

.document-body :deep(a) {
  color: var(--accent-color);
  text-decoration: none;
}

.document-body :deep(a:hover) {
  text-decoration: underline;
}
</style>
