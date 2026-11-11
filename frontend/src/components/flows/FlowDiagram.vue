<script setup lang="ts">
/**
 * A flow's Mermaid state diagram, drawn by the same renderer as documents
 * (renderMermaidDiagrams). The source goes in as text, never as HTML, and
 * the renderer replaces the block with a sanitized SVG.
 *
 * The block is keyed on the source: the renderer swaps the `<pre>` out of
 * the DOM, so a new source needs a fresh element rather than a patch of
 * the old one.
 */
import { nextTick, onMounted, useTemplateRef, watch } from 'vue'
import { renderMermaidDiagrams } from '@/utils/markdown'

const props = defineProps<{ source: string }>()

const host = useTemplateRef<HTMLElement>('host')

async function draw() {
  await nextTick()
  if (host.value) await renderMermaidDiagrams(host.value)
}

onMounted(draw)
watch(() => props.source, draw)
</script>

<template>
  <div ref="host" class="flow-diagram" data-testid="flow-diagram">
    <div :key="source">
      <pre class="mermaid">{{ source }}</pre>
    </div>
  </div>
</template>

<style scoped>
.flow-diagram {
  overflow-x: auto;
}

.flow-diagram :deep(svg) {
  max-width: 100%;
  height: auto;
}
</style>
