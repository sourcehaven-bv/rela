<script setup lang="ts">
/**
 * One attached file: a kind-coloured icon (or a thumbnail), the name, and a
 * line with the kind and what a click does.
 *
 * With an `href` the card is a download link; without one it is a button that
 * emits `click`. The remove button sits beside that link rather than inside
 * it, since a control inside a link is invalid and unreachable by keyboard.
 */
import { computed } from 'vue'
import type { Attachment, AttachmentKind } from '../../types'
import type { IconName } from '../common/icons'
import RlIcon from '../common/RlIcon.vue'
import RlIconButton from '../common/RlIconButton.vue'

const props = withDefaults(
  defineProps<{
    attachment: Attachment
    /** Shows a remove button. */
    removable?: boolean
    /** Disables the remove button, for example while a write is running. */
    busy?: boolean
  }>(),
  { removable: false, busy: false },
)
const emit = defineEmits<{ click: [attachment: Attachment]; remove: [attachment: Attachment] }>()

const kindLabel: Record<AttachmentKind, string> = {
  pdf: 'PDF',
  doc: 'DOC',
  sheet: 'SHEET',
  image: 'IMAGE',
  other: 'FILE',
}

const kindIcon: Record<AttachmentKind, IconName> = {
  pdf: 'file',
  doc: 'document',
  sheet: 'spreadsheet',
  image: 'image',
  other: 'file',
}

const action = computed(() => props.attachment.action ?? (props.attachment.href ? 'Download' : undefined))
</script>

<template>
  <div class="rl-attachment-card">
    <component
      :is="attachment.href ? 'a' : 'button'"
      class="rl-attachment-card__main"
      v-bind="attachment.href ? { href: attachment.href, download: attachment.name } : { type: 'button' }"
      @click="!attachment.href && emit('click', attachment)"
    >
      <img
        v-if="attachment.preview"
        :src="attachment.preview"
        alt=""
        class="rl-attachment-card__icon rl-attachment-card__preview"
      />
      <span v-else class="rl-attachment-card__icon" :class="`rl-attachment-card__icon--${attachment.kind}`">
        <RlIcon :name="kindIcon[attachment.kind]" :size="22" />
      </span>
      <span class="rl-attachment-card__body">
        <span class="rl-attachment-card__name" :title="attachment.name">{{ attachment.name }}</span>
        <span class="rl-attachment-card__meta">
          {{ kindLabel[attachment.kind] }}<template v-if="action"> &middot; {{ action }}</template>
        </span>
      </span>
    </component>
    <RlIconButton
      v-if="removable"
      class="rl-attachment-card__remove"
      icon="x"
      tone="danger"
      :size="16"
      :label="`Remove ${attachment.name}`"
      :disabled="busy"
      @click="emit('remove', attachment)"
    />
  </div>
</template>

<style scoped>
.rl-attachment-card {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  min-width: min(240px, 100%);
  flex: 1 1 240px;
  max-width: 100%;
  padding-right: var(--rl-space-2);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-sm);
  transition: background-color var(--rl-duration-fast) var(--rl-ease);
}
.rl-attachment-card:hover { background: var(--rl-color-bg-sunken); }

.rl-attachment-card__main {
  display: flex;
  flex: 1;
  align-items: center;
  gap: var(--rl-space-4);
  min-width: 0;
  padding: var(--rl-space-3);
  border: none;
  border-radius: inherit;
  background: none;
  color: inherit;
  font-family: inherit;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

/*
 * The remove button waits for the pointer, so a stray click on a list of
 * files cannot hit it. Keyboard focus shows it too. A touch screen has no
 * hover, so there it stays visible.
 */
@media (hover: hover) {
  .rl-attachment-card__remove { opacity: 0; }
  .rl-attachment-card:hover .rl-attachment-card__remove,
  .rl-attachment-card:focus-within .rl-attachment-card__remove { opacity: 1; }
}

.rl-attachment-card__main:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

.rl-attachment-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  flex: none;
  border-radius: var(--rl-radius-md);
  color: var(--rl-color-text-inverse);
}

.rl-attachment-card__preview { object-fit: cover; }

.rl-attachment-card__icon--pdf   { background: var(--rl-color-file-pdf); }
.rl-attachment-card__icon--doc   { background: var(--rl-color-file-doc); }
.rl-attachment-card__icon--sheet { background: var(--rl-color-file-sheet); }
.rl-attachment-card__icon--image { background: var(--rl-color-file-image); }
.rl-attachment-card__icon--other { background: var(--rl-color-file-other); }

.rl-attachment-card__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.rl-attachment-card__name {
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-attachment-card__meta {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}
</style>
