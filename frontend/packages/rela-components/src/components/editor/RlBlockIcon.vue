<script setup lang="ts">
/**
 * Inline SVG glyphs for the editor toolbar.
 *
 * Inline paths rather than an icon font: the editor draws a set of glyphs no
 * general icon set covers (table row and column operations especially), and
 * the geometry is hand-written so there is no licence to carry.
 *
 * Separate from `RlIcon`, whose paths are the app-chrome set. The geometry
 * lives in `editorIcons.ts` so a caller with no component to mount can build
 * the same glyph through the DOM API.
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
