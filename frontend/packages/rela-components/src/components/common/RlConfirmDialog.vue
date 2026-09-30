<script setup lang="ts">
/**
 * Modal confirmation for an action that cannot be undone.
 *
 * A destructive control should not act on a single click, so this pairs with
 * the `danger` tone on RlButton: the button opens the dialog and the dialog
 * carries the actual Delete.
 *
 * Built on RlModal, so the scrim, focus trap, scroll lock and overlay stack
 * are the shared ones rather than a second copy.
 */
import { nextTick, ref, watch } from 'vue'
import RlModal from '../overlay/RlModal.vue'
import RlButton from './RlButton.vue'
import RlButtonGroup from './RlButtonGroup.vue'
import RlText from './RlText.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    confirmLabel?: string
    cancelLabel?: string
    /** `danger` for deletions; `default` for a neutral confirmation. */
    tone?: 'default' | 'danger'
    /** Shows the pending state on the confirm button. */
    confirming?: boolean
  }>(),
  {
    confirmLabel: 'Delete',
    cancelLabel: 'Cancel',
    tone: 'danger',
    confirming: false,
  },
)

const emit = defineEmits<{ confirm: []; cancel: [] }>()

const cancelButton = ref<InstanceType<typeof RlButton> | null>(null)

function cancel() {
  if (props.confirming) return
  emit('cancel')
}

/*
 * Focus lands on Cancel rather than on the first control, so a stray Enter or
 * Space confirms nothing. RlModal focuses the first control by default, which
 * here is the close button, so this moves it afterwards.
 */
watch(
  () => props.open,
  async (open) => {
    if (!open) return
    await nextTick()
    cancelButton.value?.$el?.focus()
  },
  { immediate: true },
)
</script>

<template>
  <RlModal
    :open="open"
    :title="title"
    size="sm"
    role="alertdialog"
    :show-close="false"
    :persistent="confirming"
    @close="cancel"
  >
    <RlText v-if="description" as="p" size="md" tone="muted" class="rl-confirm-dialog__description">
      {{ description }}
    </RlText>

    <template #actions>
      <RlButtonGroup class="rl-confirm-dialog__actions" :stacked="false">
        <RlButton ref="cancelButton" variant="secondary" @click="cancel">
          {{ cancelLabel }}
        </RlButton>
        <RlButton
          variant="primary"
          :tone="tone"
          :loading="confirming"
          :loading-label="`${confirmLabel} in progress`"
          @click="emit('confirm')"
        >
          {{ confirmLabel }}
        </RlButton>
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
.rl-confirm-dialog__description {
  margin: 0;
  line-height: var(--rl-line-height-normal);
}

@media (max-width: 767px) {
  /* Side by side would crowd the two labels on a narrow screen. */
  .rl-confirm-dialog__actions {
    flex-direction: column-reverse;
  }
  .rl-confirm-dialog__actions :deep(.rl-button) { width: 100%; }
}
</style>
