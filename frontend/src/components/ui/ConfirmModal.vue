<script setup lang="ts">
/**
 * Reusable confirm dialog, now a thin adapter over `RlConfirmDialog`.
 *
 * The props are rela's and unchanged, because `useConfirm()` is the only
 * caller and its host state shape is what App.vue passes in. What is no
 * longer rela's: the scrim, the focus trap, the scroll lock, the overlay
 * stack, Escape, and focusing Cancel on open. `RlConfirmDialog` does all of
 * it, including the one this component never had — a Tab trap.
 *
 * ## Two deliberate behaviour changes
 *
 * **The busy label is now gated.** This component swapped `Confirm` for
 * `Confirm…` the instant `busy` went true, which is the exact shape rela's
 * own rule forbids ("never wire an indicator straight to a boolean" — a 40ms
 * request flashes for a few frames). `RlConfirmDialog` runs it through
 * `useDelayedPending`, and both projects independently picked the same
 * 500ms/400ms timings, so this is rela's documented rule finally being
 * applied here rather than a library preference overriding it.
 *
 * **The confirm button is `aria-disabled` while busy, not natively
 * disabled.** Native `disabled` drops focus to `<body>` mid-interaction and
 * strands a keyboard user on the control they just pressed; RR-R5VL59 says
 * the primary action takes `aria-disabled` for exactly that reason. Cancel
 * keeps native `disabled` (via `persistent`), because "cancel an in-flight
 * action" has no defined meaning here.
 *
 * ## What is NOT carried over
 *
 * The default slot. `RlConfirmDialog` renders its `description` as text, and
 * the only production mount (App.vue, driven by `useConfirm`) passes
 * `message`. A slot would need markup support in the library; nothing asks
 * for it.
 */
import { computed } from 'vue'
import { useModalStack } from '@/composables/modalStack'
import RlConfirmDialog from 'rela-components/components/common/RlConfirmDialog.vue'

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

// rela's stack is a separate registry from the library's: `isAnyModalOpen()`
// suppresses global keyboard shortcuts, and a dialog registered only with the
// library's overlay stack is invisible to it.
useModalStack(computed(() => props.open))

// `danger` is rela's word for the same thing the library calls a tone.
const tone = computed(() => (props.danger ? 'danger' : 'default'))
</script>

<template>
  <RlConfirmDialog
    class="rela-confirm"
    :open="open"
    :title="title"
    :description="message"
    :confirm-label="confirmLabel"
    :cancel-label="cancelLabel"
    :tone="tone"
    :confirming="busy"
    @confirm="emit('confirm')"
    @cancel="emit('cancel')"
  />
</template>

<style scoped>
/*
 * Preserve authored line breaks so a caller can present a list (e.g. "these
 * fields will be cleared") without it collapsing into one run-on paragraph.
 * `pre-line` keeps newlines but still wraps long lines and collapses the
 * incidental indentation of a template literal.
 *
 * `:deep` because the paragraph is RlConfirmDialog's. This reaches it through
 * the class above, which lands because RlConfirmDialog's root is a single
 * element (RlModal), not the <Teleport> that swallows a class on RlModal
 * itself.
 */
.rela-confirm :deep(.rl-confirm-dialog__description) {
  white-space: pre-line;
}
</style>
