<script setup lang="ts">
import { ref, watch, onBeforeUnmount, nextTick } from 'vue'
import { useUIStore } from '@/stores'
import { addComment, type Comment } from '@/api/comments'
import { getErrorMessage } from '@/api/errors'
import { findCommentableBlocks, type CommentableBlock } from '@/utils/blockAnchor'

/**
 * Comment affordances for body blocks that cannot be text-selected
 * (TKT-FIO205): images, mermaid and PlantUML diagrams.
 *
 * Select-to-comment cannot reach them — there is no text to select — so this
 * places a button over each block. The anchor is the block's SOURCE markdown
 * (`![alt](url)`, a fence body), which means it rides the existing `text`
 * anchor kind rather than needing a new one: re-resolution and the detached
 * flag come for free.
 */
const props = defineProps<{
  entityType: string
  entityId: string
  /** The rendered body element to scan. */
  container: HTMLElement | null
  /** Re-scan whenever this changes — the body was re-rendered. */
  renderKey: unknown
  /** All comments on this entity, to show which blocks already have one. */
  comments: Comment[]
}>()

const emit = defineEmits<{ added: [] }>()

const uiStore = useUIStore()

interface Placed extends CommentableBlock {
  top: number
  left: number
  /** Comments already anchored to this block's source. */
  existing: Comment[]
}

const blocks = ref<Placed[]>([])
const composingFor = ref<string | null>(null)
const body = ref('')
const submitting = ref(false)

/**
 * Re-measures every commentable block.
 *
 * Positions are absolute within the body's own positioning context, so they
 * survive page scroll — but not a re-render or a resize, which is why this
 * re-runs on renderKey and on the observers below.
 */
function scan() {
  const host = props.container
  if (!host) {
    blocks.value = []
    return
  }
  const source = host.dataset.commentSource || ''
  const hostRect = host.getBoundingClientRect()

  blocks.value = findCommentableBlocks(host, source).map((b) => {
    const r = b.el.getBoundingClientRect()
    return {
      ...b,
      top: r.top - hostRect.top + 6,
      left: r.left - hostRect.left + 6,
      existing: props.comments.filter(
        (c) => c.anchor.kind === 'text' && c.anchor.quote === b.quote
      ),
    }
  })
}

/**
 * Images load asynchronously and diagrams are replaced after render, so a
 * single scan measures the wrong geometry. Re-measure as the body settles.
 */
let observer: ResizeObserver | null = null

function observe() {
  observer?.disconnect()
  const host = props.container
  if (!host || typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(() => scan())
  observer.observe(host)
  for (const img of host.querySelectorAll('img')) {
    if (!img.complete) img.addEventListener('load', scan, { once: true })
  }
}

watch(
  () => [props.container, props.renderKey, props.comments] as const,
  async () => {
    await nextTick()
    scan()
    observe()
  },
  { immediate: true, deep: false }
)

onBeforeUnmount(() => observer?.disconnect())

function startComposing(b: Placed) {
  composingFor.value = b.quote
  body.value = ''
}

function cancel() {
  composingFor.value = null
  body.value = ''
}

async function submit(b: Placed) {
  const text = body.value.trim()
  if (!text || submitting.value) return
  submitting.value = true
  try {
    await addComment(props.entityType, props.entityId, {
      // The block's source markdown IS the quote — the ordinary text path from
      // here, so drift handling needs no special case.
      anchor: { kind: 'text', ref: '', quote: b.quote },
      body: text,
    })
    cancel()
    emit('added')
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="bco">
    <div
      v-for="b in blocks"
      :key="b.quote"
      class="bco-anchor"
      :style="{ top: `${b.top}px`, left: `${b.left}px` }"
    >
      <button
        type="button"
        class="bco-btn"
        :class="{ 'bco-btn--has': b.existing.length > 0 }"
        :title="
          b.existing.length > 0
            ? `${b.existing.length} comment(s) on this ${b.kind}`
            : `Comment on this ${b.kind}`
        "
        @click.stop="startComposing(b)"
      >
        <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
          <path d="M8 1a7 7 0 0 0-6.1 10.4L1 15l3.8-.9A7 7 0 1 0 8 1Z" />
        </svg>
        <span v-if="b.existing.length > 0">{{ b.existing.length }}</span>
      </button>

      <form
        v-if="composingFor === b.quote"
        class="bco-form"
        @submit.prevent="submit(b)"
        @click.stop
      >
        <p class="bco-target">On this {{ b.kind }}</p>

        <!-- Existing remarks, so the composer is a thread rather than a
             write-only box — the block has no highlight to click. -->
        <ul v-if="b.existing.length > 0" class="bco-list">
          <li v-for="c in b.existing" :key="c.id" class="bco-cmt">
            <b>{{ c.author }}</b>
            <p>{{ c.body }}</p>
          </li>
        </ul>

        <textarea
          v-model="body"
          class="bco-input"
          rows="3"
          placeholder="Add a comment…"
          aria-label="Comment body"
          @keydown.meta.enter="submit(b)"
          @keydown.ctrl.enter="submit(b)"
        />
        <div class="bco-actions">
          <span class="bco-hint">⌘↵ to post</span>
          <button type="button" class="bco-cancel" @click="cancel">Cancel</button>
          <button type="submit" class="bco-submit" :disabled="submitting || !body.trim()">
            {{ submitting ? 'Adding…' : 'Comment' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
/* The overlay itself never intercepts pointer events — only its buttons do —
 * so an image stays clickable and text under it stays selectable. */
.bco {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.bco-anchor {
  position: absolute;
  pointer-events: auto;
}

.bco-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 7px;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: var(--card-bg);
  color: var(--muted-text);
  box-shadow: var(--shadow-sm, 0 1px 3px rgb(0 0 0 / 12%));
  cursor: pointer;
  font: 600 var(--font-size-sm) / 1 inherit;
  opacity: 0.75;
  transition: opacity 0.12s;
}
.bco-btn:hover,
.bco-btn:focus-visible {
  opacity: 1;
  border-color: var(--accent-color);
}
.bco-btn svg {
  width: 12px;
  height: 12px;
}
.bco-btn--has {
  opacity: 1;
  background: var(--comment-highlight);
  border-color: var(--comment-highlight);
  color: var(--text-color);
}
.bco-btn:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

.bco-form {
  position: absolute;
  top: 28px;
  left: 0;
  z-index: 27;
  width: 320px;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 8px);
  background: var(--card-bg);
  box-shadow: var(--shadow-lg, 0 10px 30px rgb(0 0 0 / 16%));
}

.bco-target {
  margin: 0 0 8px;
  font-size: var(--font-size-sm);
  color: var(--muted-text);
}

.bco-list {
  list-style: none;
  margin: 0 0 8px;
  padding: 0;
  max-height: 160px;
  overflow-y: auto;
}
.bco-cmt {
  padding: 6px 0;
  border-bottom: 1px solid var(--border-color);
  font-size: var(--font-size-sm);
}
.bco-cmt:last-child {
  border-bottom: 0;
}
.bco-cmt p {
  margin: 2px 0 0;
  color: var(--text-color);
  white-space: pre-wrap;
}

.bco-input {
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
.bco-input:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

.bco-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 7px;
}
.bco-hint {
  margin-right: auto;
  font-size: var(--font-size-sm);
  color: var(--muted-text);
}
.bco-cancel,
.bco-submit {
  padding: 4px 11px;
  border-radius: var(--radius-sm, 5px);
  font: 600 var(--font-size-sm) / 1.4 inherit;
  cursor: pointer;
}
.bco-cancel {
  border: 1px solid var(--border-color);
  background: var(--bg-color);
  color: var(--text-color);
}
.bco-submit {
  border: 1px solid var(--accent-color);
  background: var(--accent-color);
  color: #fff;
}
.bco-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
