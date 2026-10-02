<script setup lang="ts">
withDefaults(
  defineProps<{
    title?: string
    body?: string
    titlePlaceholder?: string
    bodyPlaceholder?: string
  }>(),
  {
    title: '',
    body: '',
    titlePlaceholder: 'Give your doc a title...',
    bodyPlaceholder: 'Click here to start writing',
  },
)

const emit = defineEmits<{ 'update:title': [value: string]; 'update:body': [value: string] }>()

function onInput(event: Event, field: 'title' | 'body') {
  const target = event.target as HTMLTextAreaElement
  if (field === 'title') emit('update:title', target.value)
  else emit('update:body', target.value)

  // Grow the field to fit its content rather than scrolling internally.
  target.style.height = 'auto'
  target.style.height = `${target.scrollHeight}px`
}
</script>

<template>
  <div class="rl-doc-editor">
    <textarea
      class="rl-doc-editor__title"
      rows="1"
      :value="title"
      :placeholder="titlePlaceholder"
      :aria-label="titlePlaceholder"
      @input="onInput($event, 'title')"
    />
    <textarea
      class="rl-doc-editor__body"
      rows="1"
      :value="body"
      :placeholder="bodyPlaceholder"
      :aria-label="bodyPlaceholder"
      @input="onInput($event, 'body')"
    />
  </div>
</template>

<style scoped>
.rl-doc-editor {
  max-width: 760px;
  margin: 0 auto;
  padding: var(--rl-space-8) var(--rl-page-gutter-right) var(--rl-space-8) var(--rl-page-gutter-left);
}

.rl-doc-editor__title,
.rl-doc-editor__body {
  display: block;
  width: 100%;
  border: none;
  background: transparent;
  font-family: inherit;
  color: var(--rl-color-text);
  resize: none;
  overflow: hidden;
}

.rl-doc-editor__title:focus,
.rl-doc-editor__body:focus { outline: none; }

.rl-doc-editor__title {
  font-size: clamp(28px, 7vw, 40px);
  font-weight: var(--rl-font-weight-normal);
  line-height: var(--rl-line-height-tight);
}

.rl-doc-editor__body {
  margin-top: var(--rl-space-6);
  font-size: var(--rl-font-size-lg);
  line-height: var(--rl-line-height-relaxed);
}

.rl-doc-editor__title::placeholder,
.rl-doc-editor__body::placeholder { color: var(--rl-color-text-subtle); }
</style>
