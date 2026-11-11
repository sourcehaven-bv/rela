<script setup lang="ts">
/**
 * A composer field for a `file` property: a button that opens the file
 * picker, and the picked files, each with a remove button.
 *
 * The property's `max` is the most files it holds (unset means one). With
 * one, a new pick replaces the file; with more, picks add to the list. The
 * picker offers the property's `accept` types as a hint only. The field
 * does not refuse a pick: counting and sizes are the app's to check, and
 * the type is the server's, which sniffs the content. Their messages come
 * back through `error` like any other field's.
 *
 * The button carries the control id, so the label and the error point at
 * it, and the composer's error focus lands on it. The native input is
 * hidden and comes after the button, so a lookup for the first control in
 * the field finds the button.
 */
import { computed, useTemplateRef } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlFieldShell from '../form/RlFieldShell.vue'
import type { ResolvedChannelField } from './channelFields'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    field: ResolvedChannelField
    files: File[]
    error?: string
  }>(),
  { error: undefined },
)

const emit = defineEmits<{
  'update:files': [files: File[]]
}>()

const messages = useMessages()
const input = useTemplateRef<HTMLInputElement>('input')

const p = computed(() => props.field.property)
const multiple = computed(() => (p.value.max ?? 1) > 1)

/** The picker's filter. `default-safe` names the server's preset, not a type. */
const accept = computed(() => {
  const list = (p.value.accept ?? []).filter((a) => a && a !== 'default-safe')
  return list.length ? list.join(',') : undefined
})

const label = computed(() =>
  props.field.placeholder ?? messages.channelText({ key: multiple.value ? 'chooseFiles' : 'chooseFile' }),
)

function open() {
  input.value?.click()
}

function onChange() {
  const el = input.value
  if (!el) return
  const picked = Array.from(el.files ?? [])
  // Cleared, so picking the same file again after removing it fires change.
  el.value = ''
  if (!picked.length) return
  emit('update:files', multiple.value ? [...props.files, ...picked] : picked.slice(0, 1))
}

function remove(i: number) {
  emit(
    'update:files',
    props.files.filter((_, j) => j !== i),
  )
}
</script>

<template>
  <RlFieldShell v-slot="control" :label="field.label" :hint="field.help" :error="error" :required="p.required">
    <div class="rl-channel-file-field">
      <RlButton
        :id="control.id"
        variant="secondary"
        size="sm"
        icon="paperclip"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @click="open"
      >
        {{ label }}
      </RlButton>
      <input
        ref="input"
        type="file"
        class="rl-channel-file-field__input"
        tabindex="-1"
        aria-hidden="true"
        :multiple="multiple"
        :accept="accept"
        @change="onChange"
      />
      <ul v-if="files.length" class="rl-channel-file-field__list">
        <li v-for="(f, i) in files" :key="`${i}:${f.name}`" class="rl-channel-file-field__file">
          <span class="rl-channel-file-field__name">{{ f.name }}</span>
          <RlIconButton
            icon="x"
            :size="14"
            :label="messages.channelRemoveFile({ name: f.name })"
            @click="remove(i)"
          />
        </li>
      </ul>
    </div>
  </RlFieldShell>
</template>

<style scoped>
.rl-channel-file-field {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-channel-file-field__input {
  display: none;
}

.rl-channel-file-field__list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-channel-file-field__file {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  max-width: 100%;
  padding: 0 var(--rl-space-1) 0 var(--rl-space-2);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  font-size: var(--rl-font-size-xs);
}

.rl-channel-file-field__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
