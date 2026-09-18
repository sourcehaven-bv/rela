<script setup lang="ts">
/**
 * Inline SVG glyphs for the editor toolbar.
 *
 * Inline paths rather than an icon font, matching the reasoning already
 * recorded on the entity-reference button: the editor ships no font at all,
 * which matters for the sandboxed app editor where every byte is inlined.
 *
 * The geometry lives in `editorIcons.ts` because the sandboxed app editor
 * draws the same glyphs without Vue. Rendering both from one table is what
 * keeps them identical.
 */
import { computed } from 'vue'
import { ICON_SVG_ATTRS, ICON_PARTS } from './editorIcons'

const props = defineProps<{ name: string }>()

const parts = computed(() => ICON_PARTS[props.name] ?? [])
</script>

<template>
  <svg v-bind="ICON_SVG_ATTRS">
    <!-- `text` is set on the numeral glyphs only, so the shape tags render as
         empty elements rather than carrying a stray text node. -->
    <template v-for="(part, i) in parts" :key="i">
      <component :is="part.tag" v-if="part.text === undefined" v-bind="part.attrs" />
      <component :is="part.tag" v-else v-bind="part.attrs">{{ part.text }}</component>
    </template>
  </svg>
</template>
