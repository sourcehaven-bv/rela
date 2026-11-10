<script setup lang="ts">
// MarkdownHelp renders author-written help (a field's `help`, a step's
// `description` and `details`) as sanitized markdown, so a help text can carry
// a link or a list. Plain text renders as one paragraph, as before.
import { computed } from 'vue'
import { renderMarkdown } from '@/utils/markdown'

const props = defineProps<{ text: string }>()

const html = computed(() => renderMarkdown(props.text))
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
  <div class="markdown-help" v-html="html" />
</template>

<style scoped>
.markdown-help :deep(p),
.markdown-help :deep(ul),
.markdown-help :deep(ol) {
  margin: 0;
}

.markdown-help :deep(* + p),
.markdown-help :deep(* + ul),
.markdown-help :deep(* + ol) {
  margin-top: 0.5em;
}

.markdown-help :deep(ul),
.markdown-help :deep(ol) {
  padding-left: 1.25em;
}
</style>
