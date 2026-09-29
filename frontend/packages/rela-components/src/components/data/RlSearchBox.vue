<script setup lang="ts">
/**
 * Search field with a clear button and debounced input.
 *
 * Debouncing is here rather than at the call site so every search in the
 * product waits the same amount before firing. Clearing and Enter bypass the
 * wait: both are deliberate acts, and making the user wait after them feels
 * broken.
 */
import { onBeforeUnmount, ref, watch } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlIconButton from '../common/RlIconButton.vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
    placeholder?: string
    /** Milliseconds to wait after the last keystroke. */
    debounce?: number
    size?: 'sm' | 'md'
    disabled?: boolean
    /** Announced result count, such as `12 results`. */
    resultText?: string
  }>(),
  {
    modelValue: '',
    label: 'Search',
    placeholder: 'Search',
    debounce: 250,
    size: 'md',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** Fires after the debounce, or at once on clear and Enter. */
  search: [value: string]
}>()

const input = ref<HTMLInputElement | null>(null)
const draft = ref(props.modelValue)
let timer: ReturnType<typeof setTimeout> | undefined

/*
 * An external change only wins when the user is not mid-word. Otherwise a
 * slow response arriving late would overwrite what they have since typed.
 */
watch(
  () => props.modelValue,
  (value) => {
    if (!timer) draft.value = value
  },
)

function commit(value: string) {
  clearTimeout(timer)
  timer = undefined
  emit('update:modelValue', value)
  emit('search', value)
}

function onInput(event: Event) {
  const value = (event.target as HTMLInputElement).value
  draft.value = value
  clearTimeout(timer)
  timer = setTimeout(() => commit(value), props.debounce)
}

function clear() {
  draft.value = ''
  commit('')
  // Focus goes back to the field, not nowhere, so typing can continue.
  input.value?.focus()
}

onBeforeUnmount(() => clearTimeout(timer))

/*
 * A search field is often summoned rather than always present: a toolbar
 * shows a Search button, and the field appears when it is pressed. Revealing
 * a field without putting the caret in it makes the press take two steps, and
 * the caller cannot reach the input itself to do it, since it is internal.
 */
defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <div class="rl-search" :class="`rl-search--${size}`">
    <RlIcon name="search" :size="16" class="rl-search__icon" aria-hidden="true" />

    <input
      ref="input"
      type="search"
      class="rl-control rl-search__input"
      :class="size === 'sm' && 'rl-control--sm'"
      :value="draft"
      :placeholder="placeholder"
      :aria-label="label"
      :disabled="disabled"
      @input="onInput"
      @keydown.enter.prevent="commit(draft)"
      @keydown.escape="clear"
    />

    <RlIconButton
      v-if="draft"
      icon="x"
      label="Clear search"
      :size="14"
      class="rl-search__clear"
      @click="clear"
    />

    <!--
      The count is announced politely when it changes, so a screen reader user
      learns how many results their query found without hunting for the number.
    -->
    <span v-if="resultText" class="rl-visually-hidden" role="status">{{ resultText }}</span>
  </div>
</template>

<style scoped>
.rl-search {
  position: relative;
  display: flex;
  align-items: center;
}

.rl-search__icon {
  position: absolute;
  left: var(--rl-space-3);
  color: var(--rl-color-text-muted);
  pointer-events: none;
}

.rl-search__input {
  padding-left: var(--rl-space-8);
  padding-right: var(--rl-space-8);
}

/* The native clear affordance is hidden; the button replaces it. */
.rl-search__input::-webkit-search-cancel-button { display: none; }

.rl-search__clear {
  position: absolute;
  right: var(--rl-space-2);
}

.rl-search--sm .rl-search__icon { left: var(--rl-space-2); }
.rl-search--sm .rl-search__input { padding-left: var(--rl-space-6); }
</style>
