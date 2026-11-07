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
 *
 * With `taskItemConversions` offered, each task item gets a convert button
 * while the body is open (TKT-3IBLIP).
 *
 * With `collab` set, the editor edits the body in a co-editing room
 * (TKT-CJL604). The caller sets it after `edit` and clears it when the room
 * ends; the editor is rebuilt on each change of mode, opening on the body
 * as the caller holds it then.
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlInlineEdit from 'rela-components/components/common/RlInlineEdit.vue'
import MarkdownEditor from '@/components/forms/milkdown/MilkdownEditor.vue'
import TaskItemConversions, {
  taskItemConvertLabel,
} from '@/components/forms/TaskItemConversions.vue'
import type { EntityRefResolver } from '@/utils/markdown'
import type { TaskItemConversionOffer } from '@/types'
import type { EditorCollab } from '@/composables/collab/collabRoom'

const props = defineProps<{
  content: string
  editable: boolean
  refResolver?: EntityRefResolver
  /** The entity whose body this is; required for task-item conversions. */
  source?: { id: string; face?: string }
  taskItemConversions?: TaskItemConversionOffer[]
  collab?: EditorCollab | null
}>()

const emit = defineEmits<{
  edit: []
  input: [content: string]
  done: []
}>()

const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null)
const inlineRef = ref<InstanceType<typeof RlInlineEdit> | null>(null)
const conversionsRef = ref<InstanceType<typeof TaskItemConversions> | null>(null)

const convertLabel = computed(() =>
  props.source ? taskItemConvertLabel(props.taskItemConversions) : ''
)

/** The conversion dialog is part of the edit: pressing it must not end it. */
function conversionDialogs(): Element[] {
  return conversionsRef.value?.panels() ?? []
}

// The editor owns the document while it is open. Feeding each save back in
// as `modelValue` would reset it under the cursor, so it opens on a snapshot.
const draft = ref('')
const { t } = useI18n()

function onEdit() {
  draft.value = props.content
  emit('edit')
}

// A room that ends while open leaves the caller holding the latest body,
// which the single-writer editor that replaces the room's opens on.
watch(
  () => props.collab,
  (next, prev) => {
    if (prev && !next) draft.value = props.content
  }
)

defineExpose({
  /** The open editor, or null. */
  editor: () => editorRef.value,
  /** Ends the edit as Escape would: flushes, then emits `done`. */
  close: () => inlineRef.value?.cancel(),
})

// Unmounted while open (the caller keys it per row): the editor's last
// keystrokes still go out, to the row the edit started on.
onBeforeUnmount(() => editorRef.value?.flush())

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
    ref="inlineRef"
    block
    trigger="explicit"
    :label="t('entity.bodyLabel')"
    :keep-open-within="conversionDialogs"
    @edit="onEdit"
    @commit="onLeave"
    @cancel="onLeave"
  >
    <template #read>
      <slot name="read" />
    </template>
    <template #edit>
      <div class="entity-body-editor" @keydown="onEditorKeydown">
        <MarkdownEditor
          :key="collab ? 'collab' : 'single'"
          ref="editorRef"
          :model-value="draft"
          :collab="collab ?? undefined"
          :ref-resolver="refResolver"
          :task-item-convert-label="convertLabel"
          @update:model-value="(v: string) => emit('input', v)"
          @task-item-convert="(target) => conversionsRef?.start(target)"
        />
      </div>
    </template>
  </RlInlineEdit>
  <slot v-else name="read" />
  <TaskItemConversions
    v-if="editable && convertLabel && source"
    ref="conversionsRef"
    :offers="taskItemConversions"
    :source="source"
    :editor="() => editorRef"
  />
</template>

<style scoped>
.entity-body-editor {
  width: 100%;
}
</style>
