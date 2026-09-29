<script setup lang="ts">
/**
 * Comment affordances for blocks that cannot be text-selected: images,
 * diagrams, embeds.
 *
 * Select-to-comment cannot reach them — there is no text to select — so a
 * button is placed over each one instead. The caller says which elements are
 * commentable and what key identifies each; the overlay measures them and
 * places a badge.
 *
 * # Why it re-measures
 *
 * Positions are absolute within the container's own positioning context, so
 * they survive page scroll but not a re-render, a resize, or an image
 * arriving late. A `ResizeObserver` on the container covers reflow, image
 * `load` covers the late arrival, and `scrollSelector` covers the case a
 * resize observer cannot see: a block inside its own scrolling wrapper, such
 * as a wide table, moves while the container does not.
 */
import { onBeforeUnmount, ref, watch } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlCommentThread from './RlCommentThread.vue'
import RlCommentComposerBox from './RlCommentComposerBox.vue'
import type { AnchoredComment, NewComment } from './types'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

/** One commentable block, as identified by the caller. */
export interface CommentableBlock {
  /** Stable across re-renders: the anchor ref, and the overlay's list key. */
  key: string
  el: HTMLElement
  /** What it is, for the button's label: "image", "diagram". */
  kind: string
}

const props = withDefaults(
  defineProps<{
    /** The element to scan. Needs a positioning context of its own. */
    container: HTMLElement | null
    blocks: CommentableBlock[]
    /** All comments on the record; matched to a block by anchor ref. */
    comments?: AnchoredComment[]
    /** Change this when the content is re-rendered, to force a re-measure. */
    renderKey?: unknown
    /**
     * Selector for scrolling wrappers between a block and the container. A
     * block inside one moves without the container resizing, so the badge
     * would drift off it and stay there.
     */
    scrollSelector?: string
    submitting?: boolean
  }>(),
  { comments: () => [], renderKey: () => 0, scrollSelector: '', submitting: false },
)

const emit = defineEmits<{
  add: [comment: NewComment]
  edit: [id: string, body: string]
  toggleResolved: [id: string]
  remove: [id: string]
}>()

interface PlacedBlock extends CommentableBlock {
  top: number
  left: number
  comments: AnchoredComment[]
}

const placed = ref<PlacedBlock[]>([])
const openKey = ref<string | null>(null)

let observer: ResizeObserver | null = null
let scrollers: HTMLElement[] = []
let lateImages: HTMLImageElement[] = []

function measure() {
  const host = props.container
  if (!host) {
    placed.value = []
    return
  }

  const hostRect = host.getBoundingClientRect()
  placed.value = props.blocks.map((block) => {
    const rect = block.el.getBoundingClientRect()
    return {
      ...block,
      top: rect.top - hostRect.top + 6,
      left: rect.left - hostRect.left + 6,
      comments: props.comments.filter((comment) => comment.anchor.ref === block.key),
    }
  })
}

function detachWatchers() {
  observer?.disconnect()
  observer = null
  for (const scroller of scrollers) scroller.removeEventListener('scroll', measure)
  scrollers = []
  for (const image of lateImages) image.removeEventListener('load', measure)
  lateImages = []
}

function attachWatchers() {
  detachWatchers()
  const host = props.container
  if (!host || typeof ResizeObserver === 'undefined') return

  observer = new ResizeObserver(measure)
  observer.observe(host)

  for (const image of host.querySelectorAll('img')) {
    if (!image.complete) {
      image.addEventListener('load', measure)
      lateImages.push(image)
    }
  }

  if (props.scrollSelector) {
    scrollers = [...host.querySelectorAll<HTMLElement>(props.scrollSelector)]
    for (const scroller of scrollers) {
      scroller.addEventListener('scroll', measure, { passive: true })
    }
  }
}

watch(
  () => [props.container, props.blocks, props.comments, props.renderKey] as const,
  async () => {
    // A tick, so the caller's own re-render has landed before anything is
    // measured against it.
    await Promise.resolve()
    measure()
    attachWatchers()
  },
  { immediate: true },
)

onBeforeUnmount(detachWatchers)

function toggle(block: PlacedBlock) {
  openKey.value = openKey.value === block.key ? null : block.key
}

function add(block: PlacedBlock, body: string) {
  emit('add', {
    anchor: { kind: 'block', ref: block.key, label: block.kind },
    body,
  })
  openKey.value = null
}

function label(block: PlacedBlock): string {
  const count = block.comments.length
  if (count === 0) return messages.commentOnBlock({ kind: block.kind })
  return messages.commentCountOnBlock({ count, noun: 'comment', kind: block.kind })
}
</script>

<template>
  <!-- The overlay never takes pointer events itself, only its buttons do, so
       an image underneath stays clickable and text stays selectable. -->
  <div class="rl-block-comment-overlay" @keydown.escape="openKey = null">
    <div
      v-for="block in placed"
      :key="block.key"
      class="rl-block-comment-overlay__anchor"
      :style="{ top: `${block.top}px`, left: `${block.left}px` }"
    >
      <button
        type="button"
        class="rl-block-comment-overlay__badge"
        :class="{ 'rl-block-comment-overlay__badge--has': block.comments.length > 0 }"
        :aria-label="label(block)"
        :aria-expanded="openKey === block.key"
        aria-haspopup="dialog"
        @click="toggle(block)"
      >
        <RlIcon name="message-square" :size="12" />
        <span v-if="block.comments.length > 0">{{ block.comments.length }}</span>
      </button>

      <div
        v-if="openKey === block.key"
        class="rl-block-comment-overlay__panel"
        role="dialog"
        :aria-label="label(block)"
      >
        <p class="rl-block-comment-overlay__target">On this {{ block.kind }}</p>

        <!-- The existing remarks, so this reads as a thread: a block has no
             highlight to click, so there is nowhere else they could appear. -->
        <RlCommentThread
          v-if="block.comments.length > 0"
          class="rl-block-comment-overlay__thread"
          :comments="block.comments"
          dense
          @edit="(id, text) => $emit('edit', id, text)"
          @toggle-resolved="(id) => $emit('toggleResolved', id)"
          @remove="(id) => $emit('remove', id)"
        />

        <RlCommentComposerBox
          cancellable
          :submitting="submitting"
          @submit="(body) => add(block, body)"
          @cancel="openKey = null"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.rl-block-comment-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.rl-block-comment-overlay__anchor {
  position: absolute;
  pointer-events: auto;
}

.rl-block-comment-overlay__badge {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding: var(--rl-space-1) var(--rl-space-2);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg);
  color: var(--rl-color-text-subtle);
  font: inherit;
  font-size: var(--rl-font-size-xs);
  font-weight: 600;
  line-height: 1.4;
  box-shadow: var(--rl-shadow-sm);
  opacity: 0.8;
  cursor: pointer;
}

.rl-block-comment-overlay__badge:hover,
.rl-block-comment-overlay__badge:focus-visible {
  opacity: 1;
  border-color: var(--rl-color-accent);
}

.rl-block-comment-overlay__badge--has {
  opacity: 1;
  border-color: var(--rl-color-accent);
  color: var(--rl-color-accent);
}

.rl-block-comment-overlay__panel {
  position: absolute;
  top: calc(100% + var(--rl-space-1));
  left: 0;
  z-index: var(--rl-z-menu);
  width: 320px;
  max-width: calc(100vw - 2 * var(--rl-space-4));
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
  box-shadow: var(--rl-shadow-lg);
  overflow: hidden;
}

.rl-block-comment-overlay__target {
  margin: 0;
  padding: var(--rl-space-2) var(--rl-space-3);
  border-bottom: 1px solid var(--rl-color-border);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
}

.rl-block-comment-overlay__thread {
  max-height: 200px;
  overflow-y: auto;
}
</style>
