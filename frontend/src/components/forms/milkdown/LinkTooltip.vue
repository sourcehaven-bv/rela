<script setup lang="ts">
/**
 * The panel shown on the link under the caret or the pointer.
 *
 * Presentational, like `MentionMenu` and the toolbar: it renders a target and
 * two actions and emits. Where it appears, and whether it appears at all, is
 * decided by the editor from ProseMirror state — this component holds no
 * editor reference and dispatches nothing.
 *
 * The href is rendered as TEXT, never as an `<a href>`. The value here has
 * been through `normalizeLinkUrl` when the user typed it, but a link parsed
 * from an existing file has not: bodies are opened as they are stored, so a
 * pre-existing `javascript:` URL reaches this panel intact. Showing it as text
 * means the panel cannot become the one navigable sink for it.
 *
 * Buttons preventDefault on mousedown for the same reason as the toolbar:
 * without it the selection collapses before the action runs, and the panel is
 * about the link the selection is in.
 */
defineProps<{
  /** The link's current target, displayed and never linkified. */
  href: string
}>()

const emit = defineEmits<{
  edit: []
  unlink: []
}>()
</script>

<template>
  <div class="link-tooltip" role="group" aria-label="Link">
    <span class="link-tooltip-href" :title="href">{{ href }}</span>
    <button
      type="button"
      class="link-tooltip-button"
      title="Edit link"
      aria-label="Edit link"
      @mousedown.prevent
      @click="emit('edit')"
    >
      Edit
    </button>
    <button
      type="button"
      class="link-tooltip-button"
      title="Remove link"
      aria-label="Remove link"
      @mousedown.prevent
      @click="emit('unlink')"
    >
      Remove
    </button>
  </div>
</template>

<style scoped>
.link-tooltip {
  display: flex;
  align-items: center;
  gap: var(--space-xs, 4px);
  padding: 4px 6px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm, 0 2px 8px rgba(0, 0, 0, 0.15));
  font-size: var(--font-size-sm);
  max-width: 360px;
}

/* The target can be arbitrarily long, so it truncates rather than letting the
   panel grow past the editor. The full value stays available as a tooltip. */
.link-tooltip-href {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-family: var(--font-mono, monospace);
  max-width: 220px;
}

.link-tooltip-button {
  flex: none;
  padding: 2px 8px;
  border: none;
  border-radius: var(--radius-sm, 4px);
  background: transparent;
  color: var(--accent-color);
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.link-tooltip-button:hover {
  background: var(--hover-bg, rgba(127, 127, 127, 0.12));
}

.link-tooltip-button:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}
</style>
