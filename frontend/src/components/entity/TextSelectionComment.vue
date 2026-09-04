<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, useTemplateRef } from 'vue'
import { useUIStore } from '@/stores'
import { addComment } from '@/api/comments'
import { getErrorMessage } from '@/api/errors'

/**
 * Select-to-comment for an entity's markdown body (TKT-FIO205 stage 2).
 *
 * Watches for a text selection inside the body element and offers a floating
 * "Comment" button; accepting opens a small composer anchored to the selection.
 *
 * # Only the quote crosses the wire
 *
 * The browser has RENDERED text; the anchor lives in SOURCE coordinates. Rather
 * than map between them here — which would mean re-implementing the server's
 * render↔source mapping in TypeScript — the client sends the selected string
 * and the server locates it in its own copy of the body. That also means a
 * client cannot describe context the entity does not contain.
 *
 * `quote_index` disambiguates a selection whose text occurs more than once: we
 * count occurrences of the quote in the rendered body up to the selection, which
 * matches source order because the renderer preserves it.
 */
const props = defineProps<{
  entityType: string
  entityId: string
  /** The rendered body element to watch for selections. */
  container: HTMLElement | null
}>()

const emit = defineEmits<{ added: [] }>()

const uiStore = useUIStore()

/** Minimum selection length, mirroring the server's MinQuoteRunes. */
const MIN_QUOTE_RUNES = 5

const quote = ref('')
const quoteIndex = ref(0)
const anchorPos = ref<{ top: number; left: number } | null>(null)
const composing = ref(false)
const body = ref('')
const submitting = ref(false)
const composerRef = useTemplateRef<HTMLTextAreaElement>('composer')

function reset() {
  anchorPos.value = null
  composing.value = false
  quote.value = ''
  quoteIndex.value = 0
  body.value = ''
}

/**
 * Counts how many times `text` appears in the container before `range`, so the
 * server can anchor to the occurrence the user actually selected.
 */
function occurrenceOf(container: HTMLElement, range: Range, text: string): number {
  const before = range.cloneRange()
  before.selectNodeContents(container)
  before.setEnd(range.startContainer, range.startOffset)
  const prefix = before.toString()

  let count = 0
  let idx = prefix.indexOf(text)
  while (idx !== -1) {
    count++
    idx = prefix.indexOf(text, idx + text.length)
  }
  return count
}

function onSelectionChange() {
  // A selection made while the composer is open must not move the anchor from
  // under it — the user is typing about the text they already picked.
  if (composing.value) return

  const container = props.container
  const sel = window.getSelection()
  if (!container || !sel || sel.isCollapsed || sel.rangeCount === 0) {
    anchorPos.value = null
    return
  }

  const range = sel.getRangeAt(0)
  if (!container.contains(range.commonAncestorContainer)) {
    anchorPos.value = null
    return
  }

  const text = sel.toString().trim()
  if ([...text].length < MIN_QUOTE_RUNES) {
    anchorPos.value = null
    return
  }

  const rect = range.getBoundingClientRect()
  const host = container.getBoundingClientRect()
  quote.value = text
  quoteIndex.value = occurrenceOf(container, range, text)
  anchorPos.value = {
    top: rect.bottom - host.top + 6,
    left: Math.max(0, rect.left - host.left),
  }
}

async function startComposing() {
  composing.value = true
  await nextTick()
  composerRef.value?.focus()
}

async function submit() {
  const text = body.value.trim()
  if (!text || submitting.value) return
  submitting.value = true
  try {
    await addComment(props.entityType, props.entityId, {
      anchor: { kind: 'text', ref: '', quote: quote.value, quote_index: quoteIndex.value },
      body: text,
    })
    reset()
    window.getSelection()?.removeAllRanges()
    emit('added')
  } catch (err) {
    // The most likely failure is a stale tab: the body changed since it was
    // rendered, so the quote no longer exists server-side. The message says so.
    uiStore.error(getErrorMessage(err))
  } finally {
    submitting.value = false
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    reset()
  }
}

onMounted(() => {
  document.addEventListener('selectionchange', onSelectionChange)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('selectionchange', onSelectionChange)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div
    v-if="anchorPos"
    class="tsc"
    :style="{ top: `${anchorPos.top}px`, left: `${anchorPos.left}px` }"
    @mousedown.prevent
  >
    <button v-if="!composing" type="button" class="tsc-btn" @click="startComposing">
      <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
        <path d="M8 1a7 7 0 0 0-6.1 10.4L1 15l3.8-.9A7 7 0 1 0 8 1Z" />
      </svg>
      Comment
    </button>

    <form v-else class="tsc-form" @submit.prevent="submit">
      <blockquote class="tsc-quote">{{ quote }}</blockquote>
      <textarea
        ref="composer"
        v-model="body"
        class="tsc-input"
        rows="3"
        placeholder="Add a comment…"
        aria-label="Comment body"
        @keydown.meta.enter="submit"
        @keydown.ctrl.enter="submit"
      />
      <div class="tsc-actions">
        <span class="tsc-hint">⌘↵ to post</span>
        <button type="button" class="tsc-cancel" @click="reset">Cancel</button>
        <button type="submit" class="tsc-submit" :disabled="submitting || !body.trim()">
          {{ submitting ? 'Adding…' : 'Comment' }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.tsc {
  position: absolute;
  z-index: 25;
}

.tsc-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md, 6px);
  background: var(--card-bg);
  color: var(--text-color);
  box-shadow: var(--shadow-lg, 0 4px 12px rgb(0 0 0 / 12%));
  cursor: pointer;
  font: 600 var(--font-size-sm) / 1 inherit;
}
.tsc-btn svg {
  width: 12px;
  height: 12px;
  color: var(--accent-color);
}
.tsc-btn:hover {
  border-color: var(--accent-color);
}

.tsc-form {
  width: 320px;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 8px);
  background: var(--card-bg);
  box-shadow: var(--shadow-lg, 0 10px 30px rgb(0 0 0 / 16%));
}

/* The quoted text, so the composer states what is being commented on — the
 * selection highlight is lost the moment focus moves to the textarea. */
.tsc-quote {
  margin: 0 0 8px;
  padding: 4px 8px;
  border-left: 3px solid var(--accent-color);
  background: var(--bg-color);
  color: var(--muted-text);
  font-size: var(--font-size-sm);
  font-style: italic;
  max-height: 3.6em;
  overflow: hidden;
}

.tsc-input {
  width: 100%;
  padding: 6px 8px;
  resize: vertical;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm, 5px);
  background: var(--input-bg);
  color: var(--text-color);
  font: inherit;
  font-size: var(--font-size-base);
}
.tsc-input:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

.tsc-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 7px;
}
.tsc-hint {
  margin-right: auto;
  font-size: var(--font-size-sm);
  color: var(--muted-text);
}
.tsc-cancel,
.tsc-submit {
  padding: 4px 11px;
  border-radius: var(--radius-sm, 5px);
  font: 600 var(--font-size-sm) / 1.4 inherit;
  cursor: pointer;
}
.tsc-cancel {
  border: 1px solid var(--border-color);
  background: var(--bg-color);
  color: var(--text-color);
}
.tsc-submit {
  border: 1px solid var(--accent-color);
  background: var(--accent-color);
  color: #fff;
}
.tsc-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
