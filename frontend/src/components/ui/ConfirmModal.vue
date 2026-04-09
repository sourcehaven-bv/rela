<script setup lang="ts">
/**
 * Reusable confirm dialog.
 *
 * - Focus lands on Cancel on open (safer default, mirrors window.confirm
 *   convention). Previously-focused element is restored on close.
 * - Escape and overlay click emit `cancel`; `busy` suppresses both.
 * - Registers with the modal stack while open so other keyboard handlers
 *   know to stand down.
 * - Uses global .modal-overlay / .modal / .modal-actions / .btn styles from
 *   App.vue, so no scoped CSS block is needed.
 */
import { computed, nextTick, ref, watch } from 'vue'
import { useModalStack } from '@/composables/modalStack'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    message?: string
    confirmLabel?: string
    cancelLabel?: string
    busy?: boolean
    danger?: boolean
  }>(),
  {
    message: '',
    confirmLabel: 'Confirm',
    cancelLabel: 'Cancel',
    busy: false,
    danger: false,
  }
)

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const cancelButtonRef = ref<HTMLButtonElement | null>(null)
const previouslyFocused = ref<HTMLElement | null>(null)

// Unique ID per instance for aria-labelledby. Teleport places the markup at
// document.body, so a static ID would collide if two instances ever mounted
// at once.
const titleId = `confirm-modal-title-${Math.random().toString(36).slice(2, 10)}`

const busyConfirmLabel = computed(() =>
  props.busy ? `${props.confirmLabel}\u2026` : props.confirmLabel
)

useModalStack(computed(() => props.open))

// Focus Cancel on open; restore previous focus on close. Standard WAI-ARIA
// dialog pattern — without this, closing the modal leaves focus on <body>
// which breaks keyboard navigation and screen readers.
watch(
  () => props.open,
  async (isOpen, wasOpen) => {
    if (isOpen && !wasOpen) {
      previouslyFocused.value = (document.activeElement as HTMLElement) ?? null
      await nextTick()
      cancelButtonRef.value?.focus()
    } else if (!isOpen && wasOpen) {
      previouslyFocused.value?.focus()
      previouslyFocused.value = null
    }
  }
)

function handleOverlayClick(e: MouseEvent) {
  if (e.target !== e.currentTarget) return
  if (props.busy) return
  emit('cancel')
}

// Escape closes the modal. stopPropagation prevents global Escape handlers
// (e.g. useKeyboardShortcuts) from also firing.
function handleKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (props.busy) return
  e.stopPropagation()
  emit('cancel')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="modal-overlay"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      tabindex="-1"
      @click="handleOverlayClick"
      @keydown="handleKeydown"
    >
      <div class="modal">
        <h3 :id="titleId">{{ title }}</h3>
        <p v-if="$slots.default || message">
          <slot>{{ message }}</slot>
        </p>
        <div class="modal-actions">
          <button
            ref="cancelButtonRef"
            class="btn btn-secondary"
            :disabled="busy"
            @click="emit('cancel')"
          >
            {{ cancelLabel }}
          </button>
          <button
            class="btn"
            :class="danger ? 'btn-danger' : 'btn-primary'"
            :disabled="busy"
            @click="emit('confirm')"
          >
            {{ busyConfirmLabel }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
