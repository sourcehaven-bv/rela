<script setup lang="ts">
/**
 * The image of a `file` property marked `display: {image: true}`, as a small
 * thumbnail on a dense surface: a list cell, a table cell.
 *
 * The URL comes from the row's own value and address (`entityType` plus
 * `entityId`), so the cell asks the server for nothing the row did not
 * already carry. The browser loads the image itself: lazily, so a long list
 * fetches only the rows in view. The download route gates on its own, so the
 * thumbnail shows no more than the entity page does.
 *
 * Without the owning entity (a surface that is itself a control) there is no
 * URL, and the cell shows the file name as text. The thumbnail is not a link,
 * so a row that opens on click keeps doing so.
 */
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { attachmentFileUrl } from '@/api/attachments'
import { fileRefNames } from '@/utils/fileRefs'

const props = defineProps<WidgetProps>()

// An image property holds one file; a hand-edited list shows its first.
const name = computed(() => fileRefNames(props.modelValue)[0])

const src = computed(() => {
  if (!name.value || !props.entityType || !props.entityId || !props.propertyName) return undefined
  try {
    return attachmentFileUrl(props.entityType, props.entityId, props.propertyName, name.value)
  } catch {
    // An entity type with no registered plural (schema not loaded): show the
    // name rather than a broken image.
    return undefined
  }
})
</script>

<template>
  <img
    v-if="src"
    class="image-cell"
    :src="src"
    :alt="name"
    loading="lazy"
    decoding="async"
  />
  <span v-else-if="name" class="image-cell-name">{{ name }}</span>
</template>

<style scoped>
.image-cell {
  display: block;
  width: 32px;
  height: 32px;
  object-fit: contain;
  border-radius: 4px;
}

.image-cell-name {
  overflow: hidden;
  max-width: 24ch;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
