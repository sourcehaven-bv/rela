<script setup lang="ts">
/**
 * The compact composer inside a comment popover or overlay.
 *
 * Separate from `RlCommentComposer`, which is the full-width box at the foot
 * of a detail panel: this one is denser, offers Cancel only when the caller
 * wants it, and posts on Cmd/Ctrl+Enter because plain Enter has to stay
 * available for line breaks in a box this small.
 */
import { ref, useTemplateRef } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import RlKbd from '../data/RlKbd.vue'

withDefaults(
  defineProps<{
    placeholder?: string
    submitLabel?: string
    /** Offer a Cancel button, for a composer the caller can dismiss. */
    cancellable?: boolean
    submitting?: boolean
    /** The shortcut shown in the hint, for a caller that maps a different one. */
    submitShortcut?: string
  }>(),
  {
    placeholder: 'Add a comment...',
    submitLabel: 'Comment',
    cancellable: false,
    submitting: false,
    submitShortcut: 'Ctrl+Enter',
  },
)

const emit = defineEmits<{ submit: [body: string]; cancel: [] }>()

const body = ref('')
const input = useTemplateRef<HTMLTextAreaElement>('input')

function submit() {
  const trimmed = body.value.trim()
  if (!trimmed) return
  emit('submit', trimmed)
  body.value = ''
}

function cancel() {
  body.value = ''
  emit('cancel')
}

/** Lets a popover put the caret in the box as it opens. */
defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <form class="rl-comment-composer-box" @submit.prevent="submit">
    <slot name="quote" />

    <textarea
      ref="input"
      v-model="body"
      class="rl-comment-composer-box__input"
      rows="3"
      :placeholder="placeholder"
      aria-label="Comment body"
      @keydown.meta.enter.prevent="submit"
      @keydown.ctrl.enter.prevent="submit"
    />

    <div class="rl-comment-composer-box__actions">
      <span class="rl-comment-composer-box__hint">
        <RlKbd :keys="submitShortcut" size="sm" /> to post
      </span>
      <RlButtonGroup>
        <RlButton v-if="cancellable" variant="ghost" size="sm" @click="cancel">Cancel</RlButton>
        <RlButton
          variant="primary"
          size="sm"
          type="submit"
          :disabled="!body.trim() || submitting"
          :loading="submitting"
          loading-label="Posting comment"
        >
          {{ submitLabel }}
        </RlButton>
      </RlButtonGroup>
    </div>
  </form>
</template>

<style scoped>
.rl-comment-composer-box {
  padding: var(--rl-space-3);
  border-top: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
}

.rl-comment-composer-box__input {
  display: block;
  width: 100%;
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg);
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
  resize: vertical;
}

.rl-comment-composer-box__input::placeholder { color: var(--rl-color-text-subtle); }

.rl-comment-composer-box__input:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -1px;
  border-color: var(--rl-color-accent);
}

.rl-comment-composer-box__actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin-top: var(--rl-space-2);
}

.rl-comment-composer-box__hint {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  margin-right: auto;
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}
</style>
