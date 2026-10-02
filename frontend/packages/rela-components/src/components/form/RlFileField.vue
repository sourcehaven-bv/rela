<script setup lang="ts">
/**
 * File chooser with a drop zone.
 *
 * The native input is the control; the drop zone is a label pointing at it,
 * so clicking, tabbing and Enter all work without a keyboard handler.
 * Dragging is an addition on top, never the only way to attach a file.
 */
import { ref } from 'vue'
import RlFieldShell from './RlFieldShell.vue'
import RlIcon from '../common/RlIcon.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlText from '../common/RlText.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    modelValue?: File[]
    label: string
    /** Hides the label visually, keeping it for assistive technology. */
    labelHidden?: boolean
    hint?: string
    error?: string
    required?: boolean
    disabled?: boolean
    multiple?: boolean
    /** Native accept list, such as `image/*` or `.pdf,.docx`. */
    accept?: string
  }>(),
  { required: false, labelHidden: false, disabled: false, multiple: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: File[]] }>()

const dragging = ref(false)
const input = ref<HTMLInputElement | null>(null)

function setFiles(list: FileList | null) {
  if (!list) return
  const incoming = Array.from(list)
  emit('update:modelValue', props.multiple ? [...(props.modelValue ?? []), ...incoming] : incoming)
}

function onDrop(event: DragEvent) {
  dragging.value = false
  if (props.disabled) return
  setFiles(event.dataTransfer?.files ?? null)
}

function remove(index: number) {
  const next = [...(props.modelValue ?? [])]
  next.splice(index, 1)
  emit('update:modelValue', next)
  // The input keeps its own selection, so clear it or re-picking the same
  // file fires no change event.
  if (input.value) input.value.value = ''
}

/**
 * Rounded to one decimal; exact bytes are noise at this size.
 *
 * Through `useMessages` because the unit names and the decimal separator are
 * both localisable: `1,5 MB` is the correct spelling in most of Europe.
 */
function readableSize(bytes: number): string {
  return messages.fileSize({ bytes })
}
</script>

<template>
  <RlFieldShell
    v-slot="control"
    :label="label"
    :label-hidden="labelHidden"
    :hint="hint"
    :error="error"
    :required="required"
  >
    <label
      class="rl-file"
      :class="{ 'rl-file--dragging': dragging, 'rl-file--disabled': disabled }"
      @dragover.prevent="dragging = !disabled"
      @dragleave="dragging = false"
      @drop.prevent="onDrop"
    >
      <input
        :id="control.id"
        ref="input"
        class="rl-file__input"
        type="file"
        :multiple="multiple"
        :accept="accept"
        :disabled="disabled"
        :required="control.required"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @change="setFiles(($event.target as HTMLInputElement).files)"
      />
      <RlIcon name="upload" :size="20" class="rl-file__icon" aria-hidden="true" />
      <RlText size="sm" tone="muted">
        Drop {{ multiple ? 'files' : 'a file' }} here, or click to choose
      </RlText>
    </label>

    <ul v-if="modelValue?.length" class="rl-file__list">
      <li v-for="(file, index) in modelValue" :key="`${file.name}-${index}`" class="rl-file__item">
        <RlIcon name="file" :size="16" aria-hidden="true" />
        <RlText size="sm" truncate class="rl-file__name">{{ file.name }}</RlText>
        <RlText size="xs" tone="subtle">{{ readableSize(file.size) }}</RlText>
        <RlIconButton
          icon="x"
          :label="messages.removeFile({ name: file.name })"
          :size="14"
          tone="danger"
          @click="remove(index)"
        />
      </li>
    </ul>
  </RlFieldShell>
</template>

<style scoped>
.rl-file {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-5);
  border: 1px dashed var(--rl-color-border-strong);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-sunken);
  text-align: center;
  cursor: pointer;
  transition: border-color var(--rl-duration-fast) var(--rl-ease), background-color var(--rl-duration-fast) var(--rl-ease);
}

.rl-file:hover:not(.rl-file--disabled) { border-color: var(--rl-color-accent); }

.rl-file--dragging {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-bg-hover);
}

.rl-file--disabled { cursor: not-allowed; opacity: 0.5; }

/* Hidden but focusable: the ring is painted on the drop zone instead. */
.rl-file__input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}

.rl-file__input:focus-visible ~ * { pointer-events: none; }
.rl-file:has(.rl-file__input:focus-visible) {
  border-color: var(--rl-color-focus);
  box-shadow:
    0 0 0 var(--rl-focus-ring-gap) var(--rl-color-bg),
    0 0 0 calc(var(--rl-focus-ring-gap) + var(--rl-focus-ring-width)) var(--rl-color-focus);
}

.rl-file__icon { color: var(--rl-color-text-muted); }

.rl-file__list {
  margin: var(--rl-space-2) 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-1);
}

.rl-file__item {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-2);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text-muted);
}

.rl-file__name { flex: 1; min-width: 0; color: var(--rl-color-text); }
</style>
