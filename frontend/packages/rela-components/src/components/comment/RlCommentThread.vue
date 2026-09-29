<script setup lang="ts">
/**
 * A list of comments on one anchor, with the per-comment actions.
 *
 * Shared by the panel, the indicator popover and the block overlay, so that
 * "resolve, edit, delete" looks and behaves the same wherever a thread is
 * shown. Editing is held here rather than in each caller: the edit box
 * replaces the comment body in place, so the state belongs with the row.
 */
import { ref, computed } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import type { AnchoredComment } from './types'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    comments: AnchoredComment[]
    /** Show which anchor each comment is on. Off inside a single-anchor popover. */
    showAnchor?: boolean
    /** Compact rows, for a popover rather than a full-width panel. */
    dense?: boolean
  }>(),
  { showAnchor: false, dense: false },
)

const emit = defineEmits<{
  edit: [id: string, body: string]
  toggleResolved: [id: string]
  remove: [id: string]
}>()

const editingId = ref<string | null>(null)
const draft = ref('')

/*
 * Unresolved first, so an active thread is not buried under settled remarks.
 * A stable sort keeps the caller's order within each group, which is normally
 * oldest to newest.
 */
const ordered = computed(() =>
  [...props.comments].sort((a, b) => Number(a.resolved ?? false) - Number(b.resolved ?? false)),
)

function anchorLabel(comment: AnchoredComment): string {
  const { label, ref: reference, kind } = comment.anchor
  return label ?? (kind === 'section' ? messages.sectionAnchor({ reference }) : reference)
}

function startEdit(comment: AnchoredComment) {
  editingId.value = comment.id
  draft.value = comment.body
}

function cancelEdit() {
  editingId.value = null
  draft.value = ''
}

function saveEdit(comment: AnchoredComment) {
  const body = draft.value.trim()
  if (!body) return
  emit('edit', comment.id, body)
  cancelEdit()
}
</script>

<template>
  <ul class="rl-comment-thread" :class="{ 'rl-comment-thread--dense': dense }">
    <li
      v-for="comment in ordered"
      :key="comment.id"
      class="rl-comment-thread__item"
      :class="{ 'rl-comment-thread__item--resolved': comment.resolved }"
    >
      <div class="rl-comment-thread__meta">
        <span v-if="showAnchor" class="rl-comment-thread__anchor">{{ anchorLabel(comment) }}</span>
        <span
          v-if="comment.detached"
          class="rl-comment-thread__detached"
          title="What this comment points at no longer exists"
        >
          detached
        </span>
        <span class="rl-comment-thread__author">{{ comment.author }}</span>
        <span class="rl-comment-thread__timestamp">{{ comment.timestamp }}</span>
      </div>

      <template v-if="editingId === comment.id">
        <textarea
          v-model="draft"
          class="rl-comment-thread__input"
          rows="3"
          aria-label="Edit comment"
          @keydown.escape="cancelEdit"
        />
        <RlButtonGroup align="start" class="rl-comment-thread__actions">
          <RlButton variant="ghost" size="sm" @click="cancelEdit">Cancel</RlButton>
          <RlButton variant="primary" size="sm" @click="saveEdit(comment)">Save</RlButton>
        </RlButtonGroup>
      </template>

      <template v-else>
        <p class="rl-comment-thread__body">{{ comment.body }}</p>
        <RlButtonGroup
          v-if="comment.editable || comment.deletable"
          align="start"
          class="rl-comment-thread__actions"
        >
          <RlButton
            v-if="comment.editable"
            variant="ghost"
            size="sm"
            @click="$emit('toggleResolved', comment.id)"
          >
            {{ comment.resolved ? 'Reopen' : 'Resolve' }}
          </RlButton>
          <RlButton v-if="comment.editable" variant="ghost" size="sm" @click="startEdit(comment)">
            Edit
          </RlButton>
          <RlButton
            v-if="comment.deletable"
            variant="ghost"
            size="sm"
            tone="danger"
            @click="$emit('remove', comment.id)"
          >
            Delete
          </RlButton>
        </RlButtonGroup>
      </template>
    </li>
  </ul>
</template>

<style scoped>
.rl-comment-thread {
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-comment-thread__item {
  padding: var(--rl-space-4) var(--rl-page-gutter-right) var(--rl-space-4) var(--rl-page-gutter-left);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-comment-thread__item:last-child { border-bottom: none; }

.rl-comment-thread--dense .rl-comment-thread__item {
  padding: var(--rl-space-3);
}

/* A settled remark stays legible: dimming it to the point of being hard to
 * read would make "resolved" mean "hidden", which it does not. */
.rl-comment-thread__item--resolved .rl-comment-thread__body {
  color: var(--rl-color-text-muted);
}

.rl-comment-thread__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--rl-space-2);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}

.rl-comment-thread__anchor {
  padding: 0 var(--rl-space-1);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}

.rl-comment-thread__detached {
  padding: 0 var(--rl-space-1);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-warning-bg);
  color: var(--rl-color-warning-fg);
  font-size: var(--rl-font-size-xs);
}

.rl-comment-thread__author { color: var(--rl-color-text-muted); }

.rl-comment-thread__body {
  margin: var(--rl-space-2) 0 0;
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-relaxed);
  color: var(--rl-color-text);
  /* Author-written text: keep their line breaks, and break a pasted URL
   * rather than letting it widen the panel. */
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.rl-comment-thread__actions { margin-top: var(--rl-space-2); }

.rl-comment-thread__input {
  display: block;
  width: 100%;
  margin-top: var(--rl-space-2);
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg);
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
  resize: vertical;
}

.rl-comment-thread__input:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -1px;
  border-color: var(--rl-color-accent);
}
</style>
