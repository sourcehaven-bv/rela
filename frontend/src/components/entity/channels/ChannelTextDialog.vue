<script setup lang="ts">
/**
 * A dialog that asks for one line or paragraph of text: the reason for
 * removing a channel message, or the title of a new channel (TKT-91BK6F).
 *
 * The host mounts it under `v-if`, so its lifetime is the dialog's, as with
 * NewPileDialog. It does not send anything itself: `submit` hands the text
 * to the host, which runs the request, sets `busy` meanwhile, and passes a
 * refusal back in `error`, so the dialog stays open on a failure and the
 * user can fix the text or cancel.
 */
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'

const props = withDefaults(
  defineProps<{
    title: string
    description?: string
    label: string
    /** An empty value is refused in the dialog. */
    required?: boolean
    multiline?: boolean
    confirmLabel: string
    pendingLabel: string
    danger?: boolean
    busy?: boolean
    /** The server's refusal of the last submit. */
    error?: string
  }>(),
  {
    description: undefined,
    required: false,
    multiline: false,
    danger: false,
    busy: false,
    error: undefined,
  }
)

const emit = defineEmits<{ close: []; submit: [text: string] }>()
const { t } = useI18n()

const formId = `channel-text-dialog-${useId()}`
const text = ref('')
/** Set once the user tried to submit, so an empty field is not an error on open. */
const attempted = ref(false)

const missing = computed(() => props.required && text.value.trim() === '')
const fieldError = computed(() =>
  attempted.value && missing.value ? t('channels.required') : props.error
)

function submit() {
  attempted.value = true
  if (missing.value || props.busy) return
  emit('submit', text.value.trim())
}
</script>

<template>
  <RlModal :open="true" :title="title" size="sm" :persistent="busy" @close="emit('close')">
    <form :id="formId" class="channel-text-dialog" novalidate @submit.prevent="submit">
      <RlText v-if="description" as="p" tone="subtle" size="sm">{{ description }}</RlText>
      <RlTextarea
        v-if="multiline"
        v-model="text"
        :label="label"
        :required="required"
        :error="fieldError"
        :rows="3"
        data-testid="channel-text-dialog-field"
      />
      <RlTextField
        v-else
        v-model="text"
        :label="label"
        :required="required"
        :error="fieldError"
        autocomplete="off"
        data-testid="channel-text-dialog-field"
      />
    </form>
    <template #actions>
      <RlButton :disabled="busy" @click="emit('close')">{{ t('common.cancel') }}</RlButton>
      <RlButton
        variant="primary"
        :tone="danger ? 'danger' : 'default'"
        type="submit"
        :form="formId"
        :loading="busy"
        :pending-label="pendingLabel"
        data-testid="channel-text-dialog-submit"
      >
        {{ confirmLabel }}
      </RlButton>
    </template>
  </RlModal>
</template>

<style scoped>
.channel-text-dialog {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}
</style>
