<script setup lang="ts">
/**
 * Debounced text input used as the free-text search box on list views.
 *
 * Mirrors the keystroke debounce strategy in FilterBar.vue (250ms): emits
 * `update:modelValue` after the user stops typing, plus immediately on clear
 * and on Enter. External value changes (URL deep-link, programmatic) replace
 * the local buffer unless a debounce is in flight — in which case the user's
 * in-progress text wins.
 */
import { ref, watch, onBeforeUnmount } from 'vue'

const TEXT_DEBOUNCE_MS = 250

const props = defineProps<{
  modelValue: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const local = ref(props.modelValue)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.modelValue,
  (v) => {
    if (debounceTimer !== null) return
    local.value = v
  },
)

function flushDebounce() {
  if (debounceTimer === null) return
  clearTimeout(debounceTimer)
  debounceTimer = null
}

function emitNow() {
  flushDebounce()
  emit('update:modelValue', local.value)
}

function handleInput() {
  flushDebounce()
  debounceTimer = setTimeout(() => {
    debounceTimer = null
    emit('update:modelValue', local.value)
  }, TEXT_DEBOUNCE_MS)
}

function handleClear() {
  local.value = ''
  emitNow()
  inputRef.value?.focus()
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (local.value) {
      e.preventDefault()
      handleClear()
    } else {
      inputRef.value?.blur()
    }
  }
}

function focus() {
  inputRef.value?.focus()
  inputRef.value?.select()
}

onBeforeUnmount(() => {
  flushDebounce()
})

defineExpose({ focus })
</script>

<template>
  <div class="search-box">
    <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
         stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="11" cy="11" r="8"/>
      <line x1="21" y1="21" x2="16.65" y2="16.65"/>
    </svg>
    <input
      ref="inputRef"
      v-model="local"
      type="search"
      :placeholder="placeholder || 'Search...'"
      @input="handleInput"
      @keyup.enter="emitNow"
      @keydown="handleKeydown"
    />
    <button
      v-if="local"
      type="button"
      class="clear-btn"
      title="Clear search"
      @click="handleClear"
    >
      &times;
    </button>
  </div>
</template>

<style scoped>
.search-box {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
}

.search-icon {
  position: absolute;
  left: 12px;
  width: 16px;
  height: 16px;
  color: var(--muted-text);
  pointer-events: none;
}

input {
  flex: 1;
  height: 36px;
  padding: 0 36px 0 36px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

input:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

input::-webkit-search-cancel-button,
input::-webkit-search-decoration {
  -webkit-appearance: none;
}

.clear-btn {
  position: absolute;
  right: 8px;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  background: var(--hover-bg);
  border: none;
  border-radius: 50%;
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  color: var(--muted-text);
}

.clear-btn:hover {
  background: var(--border-color);
  color: var(--text-color);
}
</style>
