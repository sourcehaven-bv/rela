<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick, useTemplateRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import { useScriptErrorStore } from '@/stores/scriptError'
import { renderDocument } from '@/api/documents'
import { useEvents } from '@/composables/useEvents'
import { createDocumentClickHandler } from '@/composables/useDocumentClicks'
import { useBackTarget } from '@/composables/useBackTarget'
import { renderMermaidDiagrams } from '@/utils/markdown'
import { buildReturnTo } from '@/utils/returnPath'
import { getErrorMessage, getScriptError } from '@/api/errors'
import BackButton from '@/components/common/BackButton.vue'
import DOMPurify from 'dompurify'

const props = defineProps<{
  name: string
  entityId: string
}>()

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const scriptErrorStore = useScriptErrorStore()
const { on, off } = useEvents()

// State
const docContent = ref<string>('')
const loading = ref(true)
const isCached = ref(false)
const entityIds = ref<string[]>([])

// Sanitized content for safe rendering
const sanitizedContent = computed(() => DOMPurify.sanitize(docContent.value))

// Template ref to the rendered body element so we can run mermaid on it.
const docBody = useTemplateRef<HTMLElement>('docBody')

// Re-run mermaid rendering whenever the doc content is (re-)painted. The
// rela-server's document renderer emits <pre class="mermaid">…</pre>
// blocks that need mermaid.js to replace them with SVG.
watch(sanitizedContent, async () => {
  await nextTick()
  if (docBody.value) {
    await renderMermaidDiagrams(docBody.value)
  }
})

// Get document config
const docConfig = computed(() => schemaStore.documents.get(props.name))
const docTitle = computed(() => {
  if (docConfig.value?.title) return docConfig.value.title
  return props.name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
})

// Back affordance — follows return_to > from precedence. See TKT-JIEKC.
// Renders no button when neither is present (e.g. deep-linked arrival).
const backTarget = useBackTarget()

// Edit button config (opt-in per document via the `edit:` block in
// data-entry.yaml). Absent = no button. Server-side validation guarantees
// `form` references a real form and `label` is non-empty when the block
// is present.
const editConfig = computed(() => docConfig.value?.edit)

// editEntity navigates to the configured edit form for this entity. Unlike
// EntityDetail.vue's edit button (which relies on router.back() because the
// user got there via SPA navigation), DocumentView is deep-linkable, so we
// must thread `return_to` through the form so submit lands back here.
// Caller-side: the button is gated `v-if="editConfig"`, so editConfig.value
// is non-null when this fires.
function editEntity() {
  const cfg = editConfig.value!
  const returnTo = buildReturnTo(route.fullPath, ['refresh'])
  router.push({
    path: `/form/${cfg.form}/${props.entityId}`,
    query: returnTo ? { return_to: returnTo } : {},
  })
}

async function loadDocument(refresh = false) {
  loading.value = true
  docContent.value = ''

  try {
    // Pass the current location as return_to so form links inside the
    // rendered doc redirect back here on submit. Preserve user-meaningful
    // query state but drop render-only flags like ?refresh=true that
    // shouldn't round-trip.
    const returnTo = buildReturnTo(route.fullPath, ['refresh'])
    const result = await renderDocument(props.name, props.entityId, {
      refresh,
      returnTo,
    })
    docContent.value = result.html
    isCached.value = result.cached
    entityIds.value = result.entity_ids || []
  } catch (err: unknown) {
    const scriptErr = getScriptError(err)
    if (scriptErr) {
      scriptErrorStore.show(scriptErr)
    } else {
      uiStore.error(getErrorMessage(err, 'Failed to render document'))
    }
    docContent.value = ''
    entityIds.value = []
  } finally {
    loading.value = false
  }
}

// Click handler for links inside the rendered document: intercepts
// internal links + enriches return_to with a #<closest-id> fragment so
// the form redirect scrolls back near where the user clicked.
const handleContentClick = createDocumentClickHandler(router)

// Handle entity change events via centralized SSE. Type-scoped feed (no
// entity id, TKT-POT9GQ) → re-render the document on any entity change;
// the re-render is cheap and server-gated.
function handleEntityChange() {
  loadDocument(true)
}

// Load on mount and watch for prop changes
watch([() => props.name, () => props.entityId], () => {
  loadDocument()
}, { immediate: true })

onMounted(() => {
  on('entity:changed', handleEntityChange)
})

onUnmounted(() => {
  off('entity:changed', handleEntityChange)
})
</script>

<template>
  <div class="document-view">
    <header class="page-header">
      <div class="header-left">
        <BackButton v-if="backTarget" :target="backTarget" />
      </div>
      <h1>{{ docTitle }}: {{ entityId }}</h1>
      <div class="header-right">
        <button
          v-if="editConfig"
          class="btn btn-secondary"
          @click="editEntity"
        >
          {{ editConfig.label }}
        </button>
        <button
          class="btn btn-secondary"
          :disabled="loading"
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
      <div ref="docBody" class="document-body" @click="handleContentClick" v-html="sanitizedContent" />
    </div>

    <div v-else class="empty-state">
      <p>No document content available</p>
      <p class="muted">Document "{{ name }}" may not be configured or the entity "{{ entityId }}" may not exist.</p>
    </div>
  </div>
</template>

<style scoped>
.document-view {
  max-width: 1000px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  gap: 16px;
}

.page-header h1 {
  margin: 0;
  font-size: 24px;
  flex: 1;
  text-align: center;
}

.header-left,
.header-right {
  min-width: 120px;
}

.header-right {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* Below ~768px the 3-column flex layout collapses ungracefully (the title
 * wraps over 3 lines and the action buttons sit on the left under "Back").
 * Use flex-wrap + order to put title on its own row, then Back on the
 * left of the next row and Edit / Refresh on the right. */
@media (max-width: 768px) {
  .page-header {
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 8px;
  }

  /* Title takes the whole first row. */
  .page-header h1 {
    flex-basis: 100%;
    order: 0;
    font-size: 20px;
    text-align: left;
    line-height: 1.3;
  }

  .header-left {
    order: 1;
    min-width: 0;
  }

  .header-right {
    order: 2;
    flex: 1;
    min-width: 0;
    justify-content: flex-end;
  }

  /* Esc has no meaning on a touch device. */
  .header-left kbd {
    display: none;
  }
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
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
  padding: 96px 48px;
  gap: 16px;
  color: var(--muted-text);
  background: var(--card-bg);
  border-radius: 8px;
}

.muted {
  font-size: 14px;
  opacity: 0.7;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-color);
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
  background: var(--card-bg);
  border-radius: 8px;
  padding: 32px;
}

.cached-badge {
  position: absolute;
  top: 16px;
  right: 16px;
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
  font-size: 28px;
}

.document-body :deep(h2) {
  font-size: 22px;
}

.document-body :deep(h3) {
  font-size: 18px;
}

.document-body :deep(p) {
  margin: 0 0 16px;
}

.document-body :deep(ul),
.document-body :deep(ol) {
  margin: 0 0 16px;
  padding-left: 24px;
}

.document-body :deep(li) {
  margin-bottom: 6px;
}

.document-body :deep(pre) {
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 16px;
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
  margin: 16px 0;
}

.document-body :deep(th),
.document-body :deep(td) {
  padding: 10px 14px;
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
  margin: 32px 0;
}

.document-body :deep(blockquote) {
  margin: 16px 0;
  padding: 16px 20px;
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

kbd {
  padding: 2px 6px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 12px;
  font-family: inherit;
}
</style>
