<script setup lang="ts">
/**
 * A `file`-type property on a dense surface: a table or list cell, a card
 * field. Display only; edit mode is FileWidget's.
 *
 * The value is the stored property value, a list of storage paths. The cell
 * shows each file's name, never the path. One file renders as a paperclip and
 * its name; several render as a "N files" summary that expands to the names.
 * Every name links to its download when the cell knows the owning entity
 * (`entityType` plus `entityId`, the row's address). Without it the widget is
 * plain text with the names in the title: the caller passes no `entityId` on
 * a surface that is itself a control (a card, a calendar chip, a cell inside
 * a link), because a link or a <details> inside it is invalid HTML and its
 * clicks would fight the surface's. No previews: FileWidget's <img> previews would cost one request
 * per image per cell.
 *
 * The cell only ever holds a value the server already let this reader see,
 * and the download route gates on its own, so a link is no wider than the
 * entity page's.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Paperclip } from 'lucide-vue-next'
import type { WidgetProps } from './types'
import { attachmentFileUrl } from '@/api/attachments'
import { fileRefNames } from '@/utils/fileRefs'

const props = defineProps<WidgetProps>()
const { t } = useI18n()

interface CellFile {
  name: string
  href?: string
}

const files = computed<CellFile[]>(() =>
  fileRefNames(props.modelValue).map((name) => ({ name, href: hrefFor(name) }))
)

function hrefFor(name: string): string | undefined {
  if (!props.entityType || !props.entityId || !props.propertyName) return undefined
  try {
    return attachmentFileUrl(props.entityType, props.entityId, props.propertyName, name)
  } catch {
    // An entity type with no registered plural (schema not loaded): show the
    // name without a link rather than a broken one.
    return undefined
  }
}

// Links need every file's URL; without the owning entity there are none.
const linked = computed(() => files.value.every((f) => f.href !== undefined))
const summary = computed(() =>
  t('widgets.fileCell.count', { n: files.value.length }, files.value.length)
)
const allNames = computed(() => files.value.map((f) => f.name).join('\n'))
</script>

<template>
  <!-- With links: one linked name, or a "N files" list that opens to linked
       names. @click.stop: the cell may sit inside a row that opens on click;
       opening a file or the list must not also open the row. -->
  <span v-if="linked && files.length === 1" class="file-cell" @click.stop>
    <a
      class="file-cell-file"
      :href="files[0].href"
      :download="files[0].name"
      :title="files[0].name"
    >
      <Paperclip class="file-cell-icon" :size="14" aria-hidden="true" />
      <span class="file-cell-name">{{ files[0].name }}</span>
    </a>
  </span>
  <details v-else-if="linked && files.length > 1" class="file-cell file-cell-multi" @click.stop>
    <summary class="file-cell-file" :title="allNames">
      <Paperclip class="file-cell-icon" :size="14" aria-hidden="true" />
      <span class="file-cell-name">{{ summary }}</span>
    </summary>
    <ul class="file-cell-list">
      <li v-for="file in files" :key="file.name">
        <a class="file-cell-file" :href="file.href" :download="file.name" :title="file.name">
          <span class="file-cell-name">{{ file.name }}</span>
        </a>
      </li>
    </ul>
  </details>
  <!-- Without links (a card, a chip, a cell inside a link): plain text, so it
       adds no control inside the surface's own. The names are in the title. -->
  <span v-else-if="files.length" class="file-cell file-cell-file" :title="allNames">
    <Paperclip class="file-cell-icon" :size="14" aria-hidden="true" />
    <span class="file-cell-name">{{ files.length === 1 ? files[0].name : summary }}</span>
  </span>
</template>

<style scoped>
.file-cell {
  /* Positioned, so it paints above a list row's stretched row link. */
  position: relative;
  display: inline-block;
  max-width: 100%;
  vertical-align: middle;
}

.file-cell-file {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  min-width: 0;
}

a.file-cell-file {
  color: var(--rl-color-accent);
  text-decoration: none;
}

a.file-cell-file:hover .file-cell-name {
  text-decoration: underline;
}

.file-cell-multi summary {
  cursor: pointer;
  list-style: none;
}

.file-cell-multi summary::-webkit-details-marker {
  display: none;
}

.file-cell-icon {
  flex: none;
  color: var(--text-muted, #6b7280);
}

/* Long names truncate; the full name is in the title attribute. */
.file-cell-name {
  overflow: hidden;
  max-width: 24ch;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-cell-list {
  margin: 4px 0 0;
  padding-left: 18px;
}

.file-cell-list li {
  max-width: 100%;
}
</style>
