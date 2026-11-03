<script setup lang="ts">
import { ref } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

withDefaults(
  defineProps<{
    placeholder?: string
    label?: string
    submitLabel?: string
    cancelLabel?: string
    /** Blocks submission and shows the pending state on the submit button. */
    submitting?: boolean
  }>(),
  {
    placeholder: undefined,
    label: undefined,
    submitLabel: undefined,
    cancelLabel: undefined,
    submitting: false,
  },
)

const emit = defineEmits<{ submit: [body: string]; cancel: [] }>()
const body = ref('')

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
</script>

<template>
  <form class="rl-comment-composer" @submit.prevent="submit">
    <textarea
      v-model="body"
      class="rl-comment-composer__input"
      rows="1"
      :placeholder="placeholder ?? messages.chromeText({ key: 'addCommentPlaceholder' })"
      :aria-label="label ?? messages.chromeText({ key: 'addCommentLabel' })"
      @keydown.enter.exact.prevent="submit"
      @keydown.escape="cancel"
    />

    <!-- The actions stay out of the way until there is something to act on. -->
    <RlButtonGroup v-if="body.trim()" class="rl-comment-composer__actions">
      <RlButton variant="ghost" size="sm" @click="cancel">{{
        cancelLabel ?? messages.chromeText({ key: 'cancel' })
      }}</RlButton>
      <RlButton
        variant="primary"
        size="sm"
        type="submit"
        :loading="submitting"
        :loading-label="messages.chromeText({ key: 'postingComment' })"
      >
        {{ submitLabel ?? messages.chromeText({ key: 'commentAction' }) }}
      </RlButton>
    </RlButtonGroup>
  </form>
</template>

<style scoped>
.rl-comment-composer {
  padding: var(--rl-space-3) var(--rl-page-gutter-right) var(--rl-space-5) var(--rl-page-gutter-left);
  background: var(--rl-color-bg);
  border-top: 1px solid var(--rl-color-border);
}

.rl-comment-composer__input {
  display: block;
  width: 100%;
  padding: var(--rl-space-3) var(--rl-space-4);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text);
  resize: vertical;
}

.rl-comment-composer__input::placeholder { color: var(--rl-color-text-subtle); }

.rl-comment-composer__input:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -1px;
  border-color: var(--rl-color-accent);
}

.rl-comment-composer__actions { margin-top: var(--rl-space-2); }
</style>
