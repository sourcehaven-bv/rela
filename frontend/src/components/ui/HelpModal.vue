<script setup lang="ts">
import { ref, watch, nextTick, toRef, useTemplateRef } from 'vue'
import { useModalStack } from '@/composables/modalStack'
// RlModal brings Escape, the focus trap, focus restore and the scroll lock,
// none of which this dialog had.
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import axios from 'axios'
import DOMPurify from 'dompurify'
import { isCancelledFetch } from '@/composables/usePageData'
import { renderMermaidDiagrams } from '@/utils/markdown'
import { apiUrl } from '@/api/base'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'

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
const contentBody = useTemplateRef<HTMLElement>('contentBody')

// Monotonic request token: switching the help target (or a fast open→close→open)
// starts a new loadHelp before the previous fetch resolves. Each run captures its
// token and bails the moment a newer run has started, so a stale response never
// overwrites content or renders its mermaid against the current entity's DOM.
let loadToken = 0

async function loadHelp() {
  if (!props.entityType) return

  const myToken = ++loadToken
  loading.value = true
  error.value = null

  try {
    const response = await axios.get(apiUrl(`/api/help/${props.entityType}`))
    if (myToken !== loadToken) return // superseded by a newer load
    // The server renders metamodel descriptions with goldmark in unsafe
    // mode (raw HTML passes through), so sanitize at the sink like every
    // other v-html consumer (utils/markdown.ts, DocumentView).
    helpContent.value = DOMPurify.sanitize(response.data)
  } catch (err) {
    if (myToken !== loadToken) return // superseded
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load help:', err)
    error.value = 'Failed to load help content'
    helpContent.value = ''
  } finally {
    if (myToken === loadToken) loading.value = false
  }

  // Render the Lifecycle section's <pre class="mermaid"> state diagrams to SVG.
  // Must run AFTER loading is false so the v-else content div (contentBody) is
  // actually mounted — mermaid can't find blocks in an unrendered subtree.
  // Mirrors DocumentView's post-paint render. Re-check the token after nextTick
  // so a superseded run never renders against the newer entity's DOM.
  if (!error.value) {
    await nextTick()
    if (myToken === loadToken && contentBody.value) {
      await renderMermaidDiagrams(contentBody.value)
    }
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

// rela's separate registry: it is what stops the global shortcut handler
// acting while this is up.
useModalStack(toRef(props, 'open'))
</script>

<template>
  <RlModal
    :open="open"
    :title="`${entityLabel || entityType} Help`"
    size="lg"
    @close="emit('close')"
  >
    <RlStatusRegion v-if="loading">Loading...</RlStatusRegion>
    <RlStatusRegion v-else-if="error" tone="error">{{ error }}</RlStatusRegion>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-else ref="contentBody" class="help-content-wrapper" v-html="helpContent"/>
  </RlModal>
</template>

<style scoped>
/* The overlay, panel, title row, close button and body padding are RlModal's. */

/* Styles for help content from server */
.help-content-wrapper :deep(.help-content) {
  font-size: 14px;
  color: var(--rl-color-text);
}

.help-content-wrapper :deep(.help-section) {
  margin-bottom: 24px;
}

.help-content-wrapper :deep(.help-section:last-child) {
  margin-bottom: 0;
}

.help-content-wrapper :deep(.help-entity-desc) {
  font-size: 15px;
  color: var(--rl-color-text);
  line-height: 1.6;
}

.help-content-wrapper :deep(h4) {
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--rl-color-text-muted);
  margin: 0 0 12px;
}

.help-content-wrapper :deep(.help-section-hint) {
  font-size: 12px;
  color: #94a3b8;
  margin: -8px 0 12px;
}

.help-content-wrapper :deep(.help-item) {
  padding: 10px 12px;
  background: var(--rl-color-bg-hover);
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
  color: var(--rl-color-text);
}

.help-content-wrapper :deep(.help-item-meta) {
  font-size: 12px;
  color: var(--rl-color-text-muted);
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
  color: var(--rl-color-text-muted);
  line-height: 1.5;
}

.help-content-wrapper :deep(.help-empty) {
  text-align: center;
  color: #94a3b8;
  font-style: italic;
}

/* Server-rendered help tables (Properties / Relations / Values / Lifecycle). */
.help-content-wrapper :deep(.help-table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 24px;
  font-size: 13px;
}

.help-content-wrapper :deep(.help-table th) {
  text-align: left;
  padding: 6px 10px;
  border-bottom: 2px solid var(--rl-color-border, #e2e8f0);
  color: var(--rl-color-text-muted);
  font-weight: 600;
  white-space: nowrap;
}

.help-content-wrapper :deep(.help-table td) {
  padding: 6px 10px;
  border-bottom: 1px solid var(--rl-color-border, #e2e8f0);
  vertical-align: top;
  line-height: 1.5;
}

.help-content-wrapper :deep(.help-table tr:last-child td) {
  border-bottom: none;
}

.help-content-wrapper :deep(.help-table tbody tr:hover) {
  background: var(--rl-color-bg-hover);
}

.help-content-wrapper :deep(.help-table code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: var(--rl-color-text);
}

/* Markdown prose (simpleMarkdownToHTML) inside a cell wraps in <p>; strip its
   default margins so a one-line description sits flush in the row. */
.help-content-wrapper :deep(.help-table td p) {
  margin: 0;
}

/* Entity intro paragraph + section headings spacing. */
.help-content-wrapper :deep(.entity-description) {
  font-size: 14px;
  line-height: 1.6;
  margin-bottom: 20px;
}

.help-content-wrapper :deep(.entity-description p:first-child) {
  margin-top: 0;
}

.help-content-wrapper :deep(h4) {
  margin-top: 4px;
}

/* Mermaid lifecycle diagram — centered with breathing room. */
.help-content-wrapper :deep(.mermaid-diagram) {
  display: flex;
  justify-content: center;
  margin: 0 0 20px;
}
</style>
