<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import EasyMDE from 'easymde'
import 'easymde/dist/easymde.min.css'

const props = defineProps<{
  modelValue: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
let editor: EasyMDE | null = null

onMounted(() => {
  if (!textareaRef.value) return

  editor = new EasyMDE({
    element: textareaRef.value,
    initialValue: props.modelValue,
    placeholder: props.placeholder || 'Markdown content...',
    spellChecker: false,
    autofocus: false,
    status: false,
    toolbar: [
      'bold',
      'italic',
      'heading',
      '|',
      'unordered-list',
      'ordered-list',
      '|',
      'link',
      'code',
      'quote',
      '|',
      'preview',
      'side-by-side',
      'fullscreen',
      '|',
      'guide',
    ] as EasyMDE.Options['toolbar'],
    minHeight: '200px',
  })

  editor.codemirror.on('change', () => {
    if (editor) {
      emit('update:modelValue', editor.value())
    }
  })
})

// Sync external changes to editor
watch(
  () => props.modelValue,
  (newValue) => {
    if (editor && editor.value() !== newValue) {
      editor.value(newValue)
    }
  }
)

onBeforeUnmount(() => {
  if (editor) {
    editor.toTextArea()
    editor = null
  }
})
</script>

<template>
  <div class="markdown-editor">
    <textarea ref="textareaRef"/>
  </div>
</template>

<style scoped>
.markdown-editor {
  width: 100%;
}

.markdown-editor :deep(.EasyMDEContainer) {
  width: 100%;
}

.markdown-editor :deep(.CodeMirror) {
  border: 1px solid var(--border-color);
  border-radius: 0 0 6px 6px;
  font-family: monospace;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.markdown-editor :deep(.CodeMirror-cursor) {
  border-left-color: var(--text-color);
}

.markdown-editor :deep(.CodeMirror-selected) {
  background: var(--accent-color) !important;
  opacity: 0.3;
}

.markdown-editor :deep(.CodeMirror-focused) {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.markdown-editor :deep(.editor-toolbar) {
  border: 1px solid var(--border-color);
  border-bottom: none;
  border-radius: 6px 6px 0 0;
  background: var(--card-bg);
}

.markdown-editor :deep(.editor-toolbar button) {
  color: var(--muted-text) !important;
}

.markdown-editor :deep(.editor-toolbar button:hover) {
  background: var(--hover-bg);
}

.markdown-editor :deep(.editor-toolbar button.active) {
  background: var(--border-color);
}

.markdown-editor :deep(.editor-toolbar i.separator) {
  border-left-color: var(--border-color);
  border-right-color: var(--border-color);
}

.markdown-editor :deep(.editor-preview) {
  padding: 16px;
  background: var(--card-bg);
  color: var(--text-color);
}

.markdown-editor :deep(.editor-preview-side) {
  border: 1px solid var(--border-color);
  border-left: none;
  background: var(--card-bg);
  color: var(--text-color);
}

/* CodeMirror line numbers and gutters */
.markdown-editor :deep(.CodeMirror-gutters) {
  background: var(--hover-bg);
  border-right-color: var(--border-color);
}

.markdown-editor :deep(.CodeMirror-linenumber) {
  color: var(--muted-text);
}

/* Fullscreen mode - ensure it covers everything including sidebar */
.markdown-editor :deep(.EasyMDEContainer.fullscreen) {
  z-index: 9999 !important;
}

.markdown-editor :deep(.editor-toolbar.fullscreen) {
  z-index: 9999 !important;
}

.markdown-editor :deep(.CodeMirror-fullscreen) {
  z-index: 9999 !important;
}

.markdown-editor :deep(.editor-preview-side.fullscreen) {
  z-index: 9999 !important;
}

/* Side-by-side fullscreen mode */
.markdown-editor :deep(.EasyMDEContainer.fullscreen .CodeMirror-sided) {
  z-index: 9999 !important;
}
</style>
