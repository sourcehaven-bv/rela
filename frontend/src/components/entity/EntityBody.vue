<script setup lang="ts">
/**
 * An entity's markdown body, edited where it is read.
 *
 * The read view is the caller's rendered body (slot `read`), which holds
 * links, task checkboxes, comment highlights and diagrams. The body keeps
 * every click, so selecting text never opens the editor; the edit starts
 * from the edit button, which stays in view while a long body scrolls.
 *
 * There is no commit step. Every change the editor reports goes to `input`,
 * and the caller saves it debounced, so what is on screen is already saved.
 * Leaving (clicking away, Escape) only returns to the read view, after
 * flushing the editor's own 200ms debounce so the last keystrokes are not
 * lost; `done` then tells the caller to write anything still pending.
 */
import { ref } from 'vue'
import RlInlineEdit from 'rela-components/components/common/RlInlineEdit.vue'
import MarkdownEditor from '@/components/forms/milkdown/MilkdownEditor.vue'
import type { EntityRefResolver } from '@/utils/markdown'

const props = defineProps<{
  content: string
  editable: boolean
  refResolver?: EntityRefResolver
}>()

const emit = defineEmits<{
  input: [content: string]
  done: []
}>()

const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null)

// The editor owns the document while it is open. Feeding each save back in
// as `modelValue` would reset it under the cursor, so it opens on a snapshot.
const draft = ref('')

function onLeave() {
  editorRef.value?.flush()
  emit('done')
}

/**
 * Enter is a newline in prose, never "done". Escape is left alone when the
 * editor already used it, such as to close its `@` menu.
 */
function onEditorKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || (event.key === 'Escape' && event.defaultPrevented)) {
    event.stopPropagation()
  }
}
</script>

<template>
  <RlInlineEdit
    v-if="editable"
    block
    trigger="explicit"
    label="Body"
    @edit="draft = props.content"
    @commit="onLeave"
    @cancel="onLeave"
  >
    <template #read>
      <slot name="read" />
    </template>
    <template #edit>
      <div class="entity-body-editor" @keydown="onEditorKeydown">
        <MarkdownEditor
          ref="editorRef"
          :model-value="draft"
          :ref-resolver="refResolver"
          @update:model-value="(v: string) => emit('input', v)"
        />
      </div>
    </template>
  </RlInlineEdit>
  <slot v-else name="read" />
</template>

<style scoped>
.entity-body-editor {
  width: 100%;
}
</style>
