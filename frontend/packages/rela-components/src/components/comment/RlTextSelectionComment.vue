<script setup lang="ts">
/**
 * Select-to-comment over a block of prose.
 *
 * Watches for a text selection inside `container` and floats a Comment button
 * beside it; accepting swaps the button for a composer that quotes the
 * selection.
 *
 * # The quote is the anchor
 *
 * The component reports the selected string plus the rendered text either
 * side of it, and nothing else. It does not try to describe a position in the
 * source: the app owns the source and can locate the quote in its own copy.
 * The surrounding context is what tells a repeated quote apart — "ne" selected
 * inside "done" has to resolve to the right "ne".
 *
 * # Blocking before the composer, not after
 *
 * Some selections cannot be anchored at all: one spanning two table cells is
 * the usual case. The caller sets `blockedReason` and the button is replaced
 * by the reason, so nobody writes a comment that was never going to save.
 *
 * # The selection stays painted
 *
 * Focusing the composer clears the page selection, so the text being
 * discussed would lose its highlight just as the user starts writing about
 * it. The range is kept and painted through the CSS Custom Highlight API
 * instead, which marks text without touching the DOM the caller rendered.
 * Where the API is missing, the quote in the composer is the fallback.
 */
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlButton from '../common/RlButton.vue'
import RlCommentComposerBox from './RlCommentComposerBox.vue'
import type { NewComment } from './types'

const props = withDefaults(
  defineProps<{
    /** The rendered element to watch. Selections outside it are ignored. */
    container: HTMLElement | null
    /**
     * Shortest selection that may be commented on, in characters. Counted in
     * code points, so an emoji counts once rather than twice.
     */
    minLength?: number
    /** How much rendered text to report either side of the selection. */
    contextLength?: number
    /** Why the current selection cannot be anchored, or null when it can. */
    blockedReason?: string | null
    /** The caller is still deciding whether this selection can be anchored. */
    checking?: boolean
    submitting?: boolean
  }>(),
  {
    minLength: 5,
    contextLength: 60,
    blockedReason: null,
    checking: false,
    submitting: false,
  },
)

const emit = defineEmits<{
  /** A new selection was made. The caller may verify it and set `blockedReason`. */
  select: [selection: { quote: string; prefix: string; suffix: string }]
  /** The selection was dropped, so any pending verification can be abandoned. */
  clear: []
  add: [comment: NewComment]
}>()

const quote = ref('')
const prefix = ref('')
const suffix = ref('')
const position = ref<{ top: number; left: number } | null>(null)
const composing = ref(false)
const composer = ref<InstanceType<typeof RlCommentComposerBox> | null>(null)

const blocked = computed(() => Boolean(props.blockedReason))

/** The name the stylesheet paints with `::highlight()`. Shared by every instance. */
const HIGHLIGHT_NAME = 'rl-text-selection-comment'

let range: Range | null = null
let painted: Range | null = null

/*
 * One `Highlight` per name, so instances add their own range to it rather
 * than replacing it: two open composers on one page both keep their text lit.
 */
function paint(target: Range) {
  if (typeof CSS === 'undefined' || !('highlights' in CSS)) return
  let highlight = CSS.highlights.get(HIGHLIGHT_NAME)
  if (!highlight) {
    highlight = new Highlight()
    CSS.highlights.set(HIGHLIGHT_NAME, highlight)
  }
  highlight.add(target)
  painted = target
}

function unpaint() {
  if (!painted) return
  const highlight = CSS.highlights.get(HIGHLIGHT_NAME)
  highlight?.delete(painted)
  if (highlight?.size === 0) CSS.highlights.delete(HIGHLIGHT_NAME)
  painted = null
}

function reset() {
  position.value = null
  composing.value = false
  quote.value = ''
  prefix.value = ''
  suffix.value = ''
  range = null
  unpaint()
  emit('clear')
}

/**
 * The rendered text immediately before and after the selection, taken from the
 * same coordinate space as the quote so the three agree.
 */
function contextAround(container: HTMLElement, range: Range) {
  const before = range.cloneRange()
  before.selectNodeContents(container)
  before.setEnd(range.startContainer, range.startOffset)

  const after = range.cloneRange()
  after.selectNodeContents(container)
  after.setStart(range.endContainer, range.endOffset)

  return {
    prefix: before.toString().slice(-props.contextLength),
    suffix: after.toString().slice(0, props.contextLength),
  }
}

function onSelectionChange() {
  // A selection made while the composer is open must not move the anchor out
  // from under it: the user is writing about the text they already picked.
  if (composing.value) return

  const container = props.container
  const selection = typeof window === 'undefined' ? null : window.getSelection()
  if (!container || !selection || selection.isCollapsed || selection.rangeCount === 0) {
    if (position.value) reset()
    return
  }

  const selected = selection.getRangeAt(0)
  if (!container.contains(selected.commonAncestorContainer)) {
    if (position.value) reset()
    return
  }

  const text = selection.toString().trim()
  if ([...text].length < props.minLength) {
    if (position.value) reset()
    return
  }

  const rect = selected.getBoundingClientRect()
  const host = container.getBoundingClientRect()
  const context = contextAround(container, selected)

  // A copy: the live range belongs to the selection and moves with it.
  range = selected.cloneRange()
  quote.value = text
  prefix.value = context.prefix
  suffix.value = context.suffix
  // Offsets are relative to the container's own positioning context, so they
  // survive page scroll. The caller gives the container `position: relative`.
  position.value = {
    top: rect.bottom - host.top + 6,
    left: Math.max(0, rect.left - host.left),
  }

  emit('select', { quote: text, prefix: context.prefix, suffix: context.suffix })
}

async function startComposing() {
  composing.value = true
  if (range) paint(range)
  await nextTick()
  composer.value?.focus()
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') reset()
}

function add(body: string) {
  emit('add', {
    anchor: { kind: 'text', ref: '', quote: quote.value },
    body,
  })
  reset()
  if (typeof window !== 'undefined') window.getSelection()?.removeAllRanges()
}

/*
 * Listening only while there is a container to listen for: a `selectionchange`
 * handler is document-wide, so an unmounted or idle instance would still run
 * on every drag anywhere on the page.
 */
watch(
  () => props.container,
  (container) => {
    if (typeof document === 'undefined') return
    document.removeEventListener('selectionchange', onSelectionChange)
    document.removeEventListener('keydown', onKeydown)
    if (!container) {
      if (position.value) reset()
      return
    }
    document.addEventListener('selectionchange', onSelectionChange)
    document.addEventListener('keydown', onKeydown)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  unpaint()
  if (typeof document === 'undefined') return
  document.removeEventListener('selectionchange', onSelectionChange)
  document.removeEventListener('keydown', onKeydown)
})

/** Lets a caller dismiss the affordance, after saving elsewhere for instance. */
defineExpose({ reset })
</script>

<template>
  <!-- `mousedown.prevent` keeps the selection alive: clicking the button would
       otherwise collapse the very selection it is about to comment on. -->
  <div
    v-if="position"
    class="rl-text-selection-comment"
    :style="{ top: `${position.top}px`, left: `${position.left}px` }"
    @mousedown.prevent
  >
    <p v-if="!composing && blocked" class="rl-text-selection-comment__blocked">
      <RlIcon name="warning" :size="14" />
      {{ blockedReason }}
    </p>

    <RlButton
      v-else-if="!composing"
      variant="secondary"
      size="sm"
      icon="message-square"
      :disabled="checking"
      @click="startComposing"
    >
      {{ checking ? 'Checking...' : 'Comment' }}
    </RlButton>

    <div v-else class="rl-text-selection-comment__panel">
      <RlCommentComposerBox
        ref="composer"
        cancellable
        :submitting="submitting"
        @submit="add"
        @cancel="reset"
      >
        <!-- The quote, so the composer still says what it is about when the
             painted range has scrolled out of view. -->
        <template #quote>
          <blockquote class="rl-text-selection-comment__quote">{{ quote }}</blockquote>
        </template>
      </RlCommentComposerBox>
    </div>
  </div>
</template>

<style scoped>
.rl-text-selection-comment {
  position: absolute;
  z-index: var(--rl-z-menu);
}

.rl-text-selection-comment__blocked {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin: 0;
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-warning-bg);
  color: var(--rl-color-warning-fg);
  font-size: var(--rl-font-size-sm);
  box-shadow: var(--rl-shadow-md);
}

.rl-text-selection-comment__panel {
  width: 320px;
  max-width: calc(100vw - 2 * var(--rl-space-4));
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
  box-shadow: var(--rl-shadow-lg);
  overflow: hidden;
}

/* The composer sits alone in the panel, so it needs no divider of its own. */
.rl-text-selection-comment__panel :deep(.rl-comment-composer-box) {
  border-top: none;
  background: var(--rl-color-bg);
}

.rl-text-selection-comment__quote {
  margin: 0 0 var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-3);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-sunken);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-sm);
  font-style: italic;
  /* Two lines of the quote is enough to identify it; more pushes the box
   * down the page and away from what it points at. */
  max-height: 3.4em;
  overflow: hidden;
}

/* Quote marks, so the excerpt reads as quoted text rather than as a field. */
.rl-text-selection-comment__quote::before { content: '\201C'; }
.rl-text-selection-comment__quote::after { content: '\201D'; }
</style>

<!--
  Unscoped: the painted range lives in the caller's prose, which carries none
  of this component's scope attributes. The system selection colours make it
  look like the selection it replaces.
-->
<style>
::highlight(rl-text-selection-comment) {
  background-color: Highlight;
  color: HighlightText;
}
</style>
