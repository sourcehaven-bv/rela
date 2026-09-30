<script setup lang="ts">
/**
 * A comment affordance for one anchor, shown beside the thing it points at.
 *
 * Renders a small badge and, on click, a popover holding that anchor's thread
 * and a composer.
 *
 * # Where it goes
 *
 * Beside the field's LABEL, not its value. A field is a column on a grid, so
 * trailing the value puts the badge at a position that moves with the value's
 * length; on a shared row that reads as belonging to the next field along. The
 * label is a fixed point at any width.
 *
 * # Quiet until needed
 *
 * With no comments the badge is nearly invisible and appears on hover or
 * focus, so a form of twenty fields is not a form of twenty badges. It still
 * reveals on keyboard focus: a control that only appears on hover cannot be
 * reached without a mouse.
 */
import { computed, nextTick, ref } from 'vue'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'
import { useOverlayStack } from '../../composables/useOverlayStack'
import RlIcon from '../common/RlIcon.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlCommentThread from './RlCommentThread.vue'
import RlCommentComposerBox from './RlCommentComposerBox.vue'
import type { AnchoredComment, CommentAnchor, NewComment } from './types'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    anchor: CommentAnchor
    /** This anchor's comments, already filtered by the caller. */
    comments?: AnchoredComment[]
    /** Which edge of the badge the popover lines up with. */
    align?: 'start' | 'end'
    submitting?: boolean
  }>(),
  { comments: () => [], align: 'start', submitting: false },
)

const emit = defineEmits<{
  add: [comment: NewComment]
  edit: [id: string, body: string]
  toggleResolved: [id: string]
  remove: [id: string]
  open: []
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const composer = ref<InstanceType<typeof RlCommentComposerBox> | null>(null)

const { floatingStyles, containsTarget } = useAnchoredPanel(root, panel, {
  placement: computed(() => `bottom-${props.align}` as const),
  gap: 6,
})

// No scroll lock: a popover beside a field should not freeze the page.
const { push, pop, isTop } = useOverlayStack({ scrollLock: false })

const total = computed(() => props.comments.length)
const unresolvedCount = computed(() => props.comments.filter((c) => !c.resolved).length)
const allResolved = computed(() => total.value > 0 && unresolvedCount.value === 0)
const detached = computed(() => props.comments.some((c) => c.detached))

const anchorName = computed(() => props.anchor.label ?? props.anchor.ref)

const label = computed(() => {
  if (total.value === 0) return messages.commentOnAnchor({ anchor: anchorName.value })
  return messages.commentCountOnAnchor({
    count: total.value,
    noun: 'comment',
    anchor: anchorName.value,
  })
})

const state = computed(() => {
  if (total.value === 0) return 'empty'
  if (detached.value) return 'detached'
  return allResolved.value ? 'resolved' : 'active'
})

async function show() {
  open.value = true
  push()
  emit('open')
  await nextTick()
  composer.value?.focus()
}

function hide(restoreFocus = true) {
  if (!open.value) return
  open.value = false
  pop()
  if (restoreFocus) root.value?.querySelector<HTMLElement>('button')?.focus()
}

function toggle() {
  if (open.value) hide()
  else void show()
}

function onEscape() {
  if (isTop()) hide()
}

/*
 * The panel is teleported, so it is no longer a descendant of the badge:
 * `containsTarget` checks both, or every move into the popover would read as
 * a move out of the overlay and close it mid-edit.
 */
function onFocusOut(event: FocusEvent) {
  if (!containsTarget(root.value, event.relatedTarget as Node | null)) hide(false)
}

function onAdd(body: string) {
  emit('add', { anchor: props.anchor, body })
}
</script>

<template>
  <span ref="root" class="rl-comment-indicator" @focusout="onFocusOut" @keydown.escape="onEscape">
    <button
      type="button"
      class="rl-comment-indicator__badge"
      :class="`rl-comment-indicator__badge--${state}`"
      :aria-label="label"
      :aria-expanded="open"
      aria-haspopup="dialog"
      @click="toggle"
    >
      <RlIcon :name="allResolved ? 'check' : 'message-square'" :size="12" />
      <span v-if="total > 0">{{ total }}</span>
    </button>

    <!-- Teleported so a scrolling or clipping ancestor cannot cut it off. -->
    <Teleport to="body">
      <div
        v-if="open"
        ref="panel"
        class="rl-panel rl-panel--comment"
        role="dialog"
        :aria-label="label"
        :style="floatingStyles"
      >
        <header class="rl-comment-indicator__header">
          <span class="rl-comment-indicator__target">Comments on {{ anchorName }}</span>
          <RlIconButton icon="x" label="Close comments" :size="14" @click="hide()" />
        </header>

        <RlCommentThread
          v-if="total > 0"
          class="rl-comment-indicator__thread"
          :comments="comments"
          dense
          @edit="(id, text) => $emit('edit', id, text)"
          @toggle-resolved="(id) => $emit('toggleResolved', id)"
          @remove="(id) => $emit('remove', id)"
        />

        <RlCommentComposerBox
          ref="composer"
          :placeholder="total > 0 ? 'Reply...' : 'Add a comment...'"
          :submit-label="total > 0 ? 'Reply' : 'Comment'"
          :submitting="submitting"
          @submit="onAdd"
        />
      </div>
    </Teleport>
  </span>
</template>

<style scoped>
.rl-comment-indicator {
  position: relative;
  display: inline-flex;
  vertical-align: middle;
}

.rl-comment-indicator__badge {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding: 0 var(--rl-space-1);
  border: 1px solid transparent;
  border-radius: var(--rl-radius-pill);
  background: transparent;
  color: var(--rl-color-text-subtle);
  font: inherit;
  font-size: var(--rl-font-size-xs);
  font-weight: 600;
  line-height: 1.6;
  cursor: pointer;
}

.rl-comment-indicator__badge--active {
  background: var(--rl-color-accent-bg, var(--rl-color-bg-hover));
  border-color: var(--rl-color-border);
  color: var(--rl-color-accent);
}

.rl-comment-indicator__badge--resolved {
  background: var(--rl-color-success-bg);
  color: var(--rl-color-success-fg);
}

.rl-comment-indicator__badge--detached {
  background: var(--rl-color-warning-bg);
  color: var(--rl-color-warning-fg);
}

/*
 * An empty anchor stays out of the way until it is hovered or focused.
 * `:focus-visible` is not optional: without it the control is unreachable by
 * keyboard.
 *
 * To reveal it from the whole row instead of the badge alone, a caller adds
 * `.rl-comment-hover-group` to the row. That rule lives in styles/base.css,
 * because a scoped rule here could not match an ancestor the caller owns.
 */
.rl-comment-indicator__badge--empty { opacity: 0; }

.rl-comment-indicator:hover .rl-comment-indicator__badge--empty,
.rl-comment-indicator__badge--empty:focus-visible,
.rl-comment-indicator__badge--empty[aria-expanded='true'] {
  opacity: 1;
}

/* Touch has no hover, so the badge is always visible there. */
@media (hover: none) {
  .rl-comment-indicator__badge--empty { opacity: 1; }
}

.rl-comment-indicator__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-2);
  padding: var(--rl-space-2) var(--rl-space-3);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-comment-indicator__target {
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
}

.rl-comment-indicator__thread {
  max-height: 280px;
  overflow-y: auto;
}
</style>
