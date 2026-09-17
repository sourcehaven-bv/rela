<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick, useTemplateRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import { useScriptErrorStore } from '@/stores/scriptError'
import { renderDocument } from '@/api/documents'
import { useEvents } from '@/composables/useEvents'
import { createDocumentClickHandler } from '@/composables/useDocumentClicks'
import { renderMermaidDiagrams, renderPlantUMLDiagrams } from '@/utils/markdown'
import type { DocumentConfig } from '@/types'
import { getErrorMessage, getScriptError, shouldDropHeldContent } from '@/api/errors'
import PendingButton from '@/components/common/PendingButton.vue'
import DOMPurify from 'dompurify'
import { useDelayedPending } from '@/composables/useDelayedPending'
import { PENDING_TIMINGS } from '@/composables/pendingTimings'


const props = defineProps<{
  entityType: string
  entityId: string
}>()

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const scriptErrorStore = useScriptErrorStore()
const { on, off } = useEvents()

// Click handler for links inside the rendered document: intercepts
// internal links + enriches return_to with a #<closest-id> fragment so
// the form redirect scrolls back near where the user clicked.
const handleContentClick = createDocumentClickHandler(router)

// State
const selectedDoc = ref<string | null>(null)
const docContent = ref<string>('')
const loading = ref(false)

// Monotonic id of the most recently STARTED render. Renders are not serialized
// — an SSE burst or a tab switch mid-flight leaves two in the air — and they
// can complete out of order, so a response may only touch state if no newer
// render has started since. Without the fence a slow re-render of one document
// can land after a fast switch to another and paint it under the wrong tab;
// SSE re-renders pass refresh=true, which bypasses the server's render cache,
// so they are reliably the slow ones.
let renderGeneration = 0

// Cold-load only (the `!docContent` half): a re-render keeps the previous
// document on screen rather than blanking it — loadDocument only clears
// docContent when `cold`. The gate adds the other half: a render quicker
// than the threshold shows nothing at all.
const showBlockLoader = useDelayedPending(() => loading.value && !docContent.value, {
  delay: PENDING_TIMINGS.navDelayMs,
  minDuration: PENDING_TIMINGS.navMinDurationMs,
})

const isCached = ref(false)
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
    renderPlantUMLDiagrams(docBody.value, schemaStore.app?.plantuml_server_url)
  }
})

// Find documents that apply to this entity type.
//
// Standalone documents (no entity_type) are excluded by construction: their
// entity_type is undefined, which never equals a real type. They render at
// /document/:name from a navigation entry and have no entity to attach to.
//
// Documents gated by `permission:` ARE listed here, and render a 403 if the
// user lacks it. That matches the sidebar, which is principal-independent by
// design (docs/acl-security.md) — the config is not a secret, so we show what
// is configured and let the server answer authoritatively.
const availableDocuments = computed(() => {
  const docs: Array<{ name: string; config: DocumentConfig }> = []
  for (const [name, config] of schemaStore.documents) {
    if (config.entity_type && config.entity_type === props.entityType) {
      docs.push({ name, config })
    }
  }
  return docs
})

// Seed selectedDoc from the URL's ?doc= query so form-submit redirects
// land on the same tab the user was viewing, and bookmarks / shared
// links preserve panel state. Auto-select the first document otherwise.
watch(
  availableDocuments,
  (docs) => {
    if (docs.length === 0) return
    const fromQuery = typeof route.query.doc === 'string' ? route.query.doc : null
    if (fromQuery && docs.some((d) => d.name === fromQuery)) {
      selectedDoc.value = fromQuery
    } else if (!selectedDoc.value) {
      selectedDoc.value = docs[0].name
    }
  },
  { immediate: true }
)

// Keep ?doc= in sync with the selected tab without pushing history
// entries — we want Back to leave the entity page, not cycle through
// tab selections.
watch(selectedDoc, (name) => {
  if (!name || route.query.doc === name) return
  router.replace({ query: { ...route.query, doc: name } }).catch(() => {})
})

// Load document when selection changes
watch(
  [selectedDoc, () => props.entityId],
  async () => {
    if (selectedDoc.value && props.entityId) {
      await loadDocument(false, true)
    }
  },
  { immediate: true }
)

// cold: blank the view before fetching. Reserved for a switch to a DIFFERENT
// document (tab change or a new entity), where the old content is about to be
// wrong — keeping it on screen would show one document under another's tab.
//
// A re-render of the SAME document must not blank: docContent going empty
// drops the template past the delay-gated spinner into the empty-state branch
// (see the v-if chain below), which unmounts the rendered body, collapses the
// panel height and makes the browser clamp scroll to the top. That is the
// BUG-DJZTRF flash, and since the SSE feed re-renders on any entity write of
// any type, it fired constantly while nothing relevant had changed.
async function loadDocument(refresh = false, cold = false) {
  if (!selectedDoc.value) return

  const generation = ++renderGeneration
  loading.value = true
  if (cold) {
    docContent.value = ''
  }

  try {
    // Documents panel is embedded on the entity page; pass that path as
    // return_to so form links inside the rendered doc redirect back here.
    // Include the selected doc tab in the return URL so submit brings
    // the user back to the *same* document, not whichever one auto-
    // selects first.
    const returnTo =
      `/entity/${props.entityType}/${props.entityId}` +
      `?doc=${encodeURIComponent(selectedDoc.value)}`
    const result = await renderDocument(selectedDoc.value, props.entityId, {
      refresh,
      returnTo,
    })
    // A superseded render must not paint: its HTML belongs to the previously
    // selected document or to an older state of this one.
    if (generation !== renderGeneration) return

    // Assign only on a real change. Vue's ref equality check already makes an
    // identical assignment a no-op, so this guard is belt-and-braces rather
    // than the thing that prevents the repaint — keep it as the explicit
    // statement of intent (an SSE re-render normally returns byte-identical
    // HTML and must not repaint), and so a future switch to a non-primitive
    // content type cannot silently start re-patching the subtree.
    if (result.html !== docContent.value) {
      docContent.value = result.html
    }
    // Inside the fence and beside the assignment: the badge describes the
    // content on screen, so it must never outlive or precede it.
    isCached.value = result.cached
  } catch (err: unknown) {
    // A superseded render's failure is not the user's problem — the render
    // they are actually waiting on is still in flight.
    if (generation !== renderGeneration) return

    const scriptErr = getScriptError(err)
    if (scriptErr) {
      scriptErrorStore.show(scriptErr)
    } else {
      uiStore.error(getErrorMessage(err, 'Failed to render document'))
    }
    // Keep whatever is already rendered. The error is surfaced via the toast
    // or the script-error panel, so replacing a readable document with the
    // empty state on a transient failure loses the user's place for nothing.
    // A cold load has nothing to keep and correctly stays empty.
    //
    // A denial is the exception: content the principal may no longer read
    // must not stay painted (#1603, CONTROL-8-03). See shouldDropHeldContent's
    // godoc for which statuses count and why 404 is among them.
    //
    // isCached is deliberately NOT cleared alongside. The badge renders
    // inside the `v-else-if="docContent"` branch that this blanking already
    // unmounts, and every successful render reassigns it — so a stale `true`
    // has no path to the screen, and a test for one cannot fail.
    if (shouldDropHeldContent(err)) {
      docContent.value = ''
    }
  } finally {
    // Only the newest render owns the flag; an older one clearing it would
    // report "done" while the render the user is waiting on is still running,
    // re-enabling Refresh mid-flight.
    if (generation === renderGeneration) {
      loading.value = false
    }
  }
}

// Handle entity change events via centralized SSE. The feed is now
// type-scoped (no entity id, TKT-POT9GQ), so we re-render the document
// whenever any entity changes — a document can reference entities of any
// type, and the re-render is cheap and server-gated. The previous per-id
// match is no longer possible (and the id never reached the SPA anyway
// for entities the principal couldn't read).
function handleEntityChange() {
  loadDocument(true)
}

onMounted(() => {
  on('entity:changed', handleEntityChange)
})

onUnmounted(() => {
  off('entity:changed', handleEntityChange)
})

// An authored `title:` wins; otherwise the document id is shown as-is.
// Deriving a title from the id would be an English-only guess (DEC-6C1NAA).
function getDocTitle(name: string, config: DocumentConfig): string {
  return config.title || name
}
</script>

<template>
  <section v-if="availableDocuments.length > 0" class="documents-panel">
    <header class="panel-header">
      <h2>Documents</h2>
      <div class="header-controls">
        <select v-if="availableDocuments.length > 1" v-model="selectedDoc" class="doc-select">
          <option v-for="doc in availableDocuments" :key="doc.name" :value="doc.name">
            {{ getDocTitle(doc.name, doc.config) }}
          </option>
        </select>
        <PendingButton
          class="btn btn-sm btn-secondary"
          :pending="loading"
          label="Refresh"
          pending-label="Refreshing…"
          title="Refresh document"
          @click="loadDocument(true)"
        />
      </div>
    </header>

    <div v-if="showBlockLoader" class="loading-state">
      <div class="spinner" />
      <span>Rendering document...</span>
    </div>

    <div v-else-if="docContent" class="document-content">
      <div v-if="isCached" class="cached-badge">cached</div>
      <div
        ref="docBody"
        class="document-body md-body"
        @click="handleContentClick"
        v-html="sanitizedContent"
      />
    </div>

    <!-- Unlike DocumentView's, this empty state is not terminal: the tab
         selector and Refresh above stay rendered, because they come from
         config rather than content. After a denial (#1603) that leaves the
         user able to retry into another refusal. Deliberate — the panel is
         one section of an entity page, so hiding its chrome would misreport
         the document as unconfigured, and config names are not secret
         (docs/acl-security.md). -->
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
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
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

/* All rendered-markdown element styling (headings, paragraphs, lists, code,
   pre, blockquote, hr, img, links, tables) is shared across every markdown
   surface via the `.md-body` class on the `.document-body` container — see
   styles/markdown-content.css. */

/* Reduced motion. This is a SCOPED style, so styles/pending.css cannot
   reach .spinner-sm — a scoped selector carries a [data-v-*] attribute and
   outranks an unscoped rule. The suppression has to live beside the
   declaration. */
@media (prefers-reduced-motion: reduce) {
  .spinner-sm {
    animation: none;
  }
}
</style>
