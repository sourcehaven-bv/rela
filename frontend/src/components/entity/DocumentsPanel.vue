<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick, useTemplateRef } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { renderDocument } from '@/api/documents'
import { useEvents } from '@/composables/useEvents'
import { renderMermaidDiagrams } from '@/utils/markdown'
import type { DocumentConfig } from '@/types'
import DOMPurify from 'dompurify'

const props = defineProps<{
  entityType: string
  entityId: string
}>()

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { on, off } = useEvents()

// State
const selectedDoc = ref<string | null>(null)
const docContent = ref<string>('')
const loading = ref(false)
const isCached = ref(false)
const entityIds = ref<string[]>([]) // Entity IDs involved in current document
const docBody = useTemplateRef<HTMLElement>('docBody')

// Sanitized content for safe rendering
const sanitizedContent = computed(() => DOMPurify.sanitize(docContent.value))

// Re-run mermaid rendering whenever the doc content is (re-)painted. The
// rela-server's document renderer emits <pre class="mermaid">…</pre>
// blocks that need mermaid.js to replace them with SVG.
watch(sanitizedContent, async () => {
  await nextTick()
  if (docBody.value) {
    await renderMermaidDiagrams(docBody.value)
  }
})

// Find documents that apply to this entity type
const availableDocuments = computed(() => {
  const docs: Array<{ name: string; config: DocumentConfig }> = []
  for (const [name, config] of schemaStore.documents) {
    const targetType = config.entity_type
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

  loading.value = true
  docContent.value = ''

  try {
    const result = await renderDocument(selectedDoc.value, props.entityId, refresh)
    docContent.value = result.html
    isCached.value = result.cached
    entityIds.value = result.entity_ids || []
  } catch {
    uiStore.error('Failed to render document')
    docContent.value = ''
    entityIds.value = []
  } finally {
    loading.value = false
  }
}

// Handle entity change events via centralized SSE
function handleEntityChange(data: { id?: string }) {
  if (data.id && entityIds.value.includes(data.id)) {
    loadDocument(true)
  }
}

onMounted(() => {
  on('entity:created', handleEntityChange)
  on('entity:updated', handleEntityChange)
  on('entity:deleted', handleEntityChange)
})

onUnmounted(() => {
  off('entity:created', handleEntityChange)
  off('entity:updated', handleEntityChange)
  off('entity:deleted', handleEntityChange)
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
          title="Refresh document"
          @click="loadDocument(true)"
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
      <div ref="docBody" class="document-body" v-html="sanitizedContent" />
    </div>

    <div v-else class="empty-state">
      <p>No document content available</p>
    </div>
  </section>
</template>

<style scoped>
.documents-panel {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  margin-bottom: 24px;
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  color: var(--text-color);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.doc-select {
  padding: 6px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--input-bg);
  color: var(--text-color);
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
  filter: brightness(0.9);
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: var(--muted-text);
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
  background: var(--hover-bg);
  border-radius: 4px;
  font-size: 11px;
  color: var(--muted-text);
  text-transform: uppercase;
}

.document-body {
  font-size: 15px;
  line-height: 1.7;
  color: var(--text-color);
}

/* Style injected HTML content */
.document-body :deep(h1),
.document-body :deep(h2),
.document-body :deep(h3) {
  margin: 24px 0 12px;
  color: var(--text-color);
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
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 12px;
  overflow-x: auto;
  font-size: 13px;
}

.document-body :deep(code) {
  background: var(--hover-bg);
  color: var(--text-color);
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
  border: 1px solid var(--border-color);
}

.document-body :deep(th) {
  background: var(--hover-bg);
  font-weight: 600;
}

.document-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--border-color);
  margin: 24px 0;
}

.document-body :deep(blockquote) {
  margin: 12px 0;
  padding: 12px 16px;
  background: var(--hover-bg);
  border-left: 4px solid var(--accent-color);
  color: var(--muted-text);
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
