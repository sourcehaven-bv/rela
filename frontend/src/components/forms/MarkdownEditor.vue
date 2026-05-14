<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import EasyMDE from 'easymde'
import 'easymde/dist/easymde.min.css'
import EntityPickerModal from './EntityPickerModal.vue'
import { insertEntityRef } from './insertEntityRef'

const props = defineProps<{
  modelValue: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
let editor: EasyMDE | null = null

// The entity-reference picker (TKT-I5NO) is owned by the editor — opened
// from the toolbar button, dismissed via emit('close') or after select.
const pickerOpen = ref(false)

onMounted(() => {
  if (!textareaRef.value) return

  // Inline SVG of FontAwesome Free 7's `circle-nodes` (solid). EasyMDE
  // bundles FA 4.7 which doesn't have this icon, so we pass the SVG via
  // `icon` (EasyMDE sets `button.innerHTML = options.icon`) instead of a
  // `className`. The connected-nodes glyph reads as "graph reference" —
  // matching what a backticked entity ID actually is: a typed edge into
  // the project graph. Path is the verbatim copy of FontAwesome's
  // svgs/solid/circle-nodes.svg (CC BY 4.0 — fontawesome.com/license/free).
  const entityRefIconSvg =
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" ' +
    'width="14" height="14" fill="currentColor" aria-hidden="true">' +
    '<path d="M418.4 157.9c35.3-8.3 61.6-40 61.6-77.9 0-44.2-35.8-80-80-80-43.4 0-78.7 34.5-80 ' +
    '77.5L136.2 151.1C121.7 136.8 101.9 128 80 128 35.8 128 0 163.8 0 208s35.8 80 80 80c12.2 0 ' +
    '23.8-2.7 34.1-7.6L259.7 407.8c-2.4 7.6-3.7 15.8-3.7 24.2 0 44.2 35.8 80 80 80s80-35.8 ' +
    '80-80c0-27.7-14-52.1-35.4-66.4l37.8-207.7zM156.3 232.2c2.2-6.9 3.5-14.2 3.7-21.7l183.8-73.5' +
    'c3.6 3.5 7.4 6.7 11.6 9.5L317.6 354.1c-5.5 1.3-10.8 3.1-15.8 5.5L156.3 232.2z"/></svg>'
  const entityRefButton: EasyMDE.ToolbarIcon = {
    name: 'entity-ref',
    action: () => {
      pickerOpen.value = true
    },
    className: 'entity-ref-button',
    icon: entityRefIconSvg,
    title: 'Insert entity reference',
  }

  editor = new EasyMDE({
    element: textareaRef.value,
    initialValue: props.modelValue,
    placeholder: props.placeholder || 'Markdown content...',
    spellChecker: false,
    autofocus: false,
    status: false,
    // Custom button: opens the entity-reference picker (TKT-I5NO).
    // Placed after the inline group with its own '|' separator so the
    // toolbar's visual rhythm stays intact (RR-91NT). The connected-
    // nodes glyph (FontAwesome 6 `circle-nodes`) reads as "graph
    // reference", matching what the inserted `\`<id>\`` actually is:
    // an edge into the project's entity graph. The icon is passed as
    // inline SVG because EasyMDE's bundled FontAwesome is 4.7 and
    // doesn't ship this glyph.
    //
    // Typed via a `const button: ToolbarIcon` declaration so the
    // surrounding string entries stay under EasyMDE's typed toolbar
    // union and the entire array still type-checks (RR-20PB).
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
      entityRefButton,
      '|',
      'preview',
      'side-by-side',
      'fullscreen',
      '|',
      'guide',
    ],
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

// Picker -> editor wiring.
//
// onPickerSelect is invoked with the entity ID; insertEntityRef wraps it
// in backticks and pads on adjacency. The helper no-ops if the editor was
// torn down while the picker was open (RR-032O).
//
// onPickerClose closes the modal and re-focuses the CodeMirror textarea
// on the NEXT tick — the picker's own watcher runs `previouslyFocused.focus()`
// first (which would land on the toolbar button), and we override after to
// land in the editor (RR-SKX3). Same path runs whether the close was
// triggered by a selection or by Esc/backdrop.
function onPickerSelect(id: string): void {
  insertEntityRef(editor, id)
}

function onPickerClose(): void {
  pickerOpen.value = false
  void nextTick(() => {
    editor?.codemirror.focus()
  })
}

onBeforeUnmount(() => {
  // Close the picker before tearing down EasyMDE so a late `select` event
  // can't fire against a null editor (RR-032O). The picker's own
  // onBeforeUnmount also aborts in-flight searches.
  pickerOpen.value = false
  if (editor) {
    editor.toTextArea()
    editor = null
  }
})
</script>

<template>
  <div class="markdown-editor">
    <textarea ref="textareaRef"/>
    <EntityPickerModal
      :open="pickerOpen"
      @select="onPickerSelect"
      @close="onPickerClose"
    />
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

/* The entity-reference button renders an inline SVG (FA6 circle-nodes)
   instead of a FontAwesome glyph. EasyMDE styles its <button> children
   as inline-block boxes sized for ~14px FA glyphs; the SVG inherits
   `currentColor` for fill so it picks up the same color rules as the
   font-icon buttons above. The flex centering keeps the SVG visually
   on the same baseline as its neighbors. */
.markdown-editor :deep(.editor-toolbar button.entity-ref-button) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
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
