<script setup lang="ts">
/**
 * An entity's heading, edited where it sits when the title is writable.
 *
 * The heading stays an `h1` in both states and the control goes inside it, so
 * the document outline does not change while editing. A heading cannot sit
 * inside the edit trigger, which is a button.
 *
 * `editable` is false for a derived title (a templated `display_property`),
 * and for one the principal may not write. Both read as a plain heading.
 */
import { ref } from 'vue'
import RlInlineEdit from 'rela-components/components/common/RlInlineEdit.vue'

const props = defineProps<{
  /** What the heading shows: the server's `_title`, which falls back to the ID. */
  title: string
  /** The raw value of the title property, which is what the control edits. */
  value: string
  editable: boolean
  /** A required title cannot be cleared; an empty commit is dropped. */
  required?: boolean
}>()

const emit = defineEmits<{
  commit: [value: string]
}>()

const draft = ref('')

function onCommit() {
  const next = draft.value.trim()
  if (next === props.value) return
  if (next === '' && props.required) return
  emit('commit', next)
}
</script>

<template>
  <h1 class="entity-title text-wrap-anywhere">
    <RlInlineEdit v-if="editable" block label="Title" @edit="draft = value" @commit="onCommit">
      <template #read>{{ title }}</template>
      <template #edit>
        <input
          v-model="draft"
          type="text"
          class="rl-control entity-title-input"
          aria-label="Title"
        />
      </template>
    </RlInlineEdit>
    <template v-else>{{ title }}</template>
  </h1>
</template>

<style scoped>
/* The control takes the heading's type, so the swap does not change the size
   of the words being edited. */
.entity-title-input {
  width: 100%;
  font: inherit;
  letter-spacing: inherit;
}
</style>
