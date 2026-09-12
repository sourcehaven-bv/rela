<script setup lang="ts">
/**
 * The `@` completion menu.
 *
 * Purely presentational: it renders the menu state and emits `pick` and
 * `hover`. Search and selection live in `useMentionMenu`, and positioning is
 * done by Milkdown's `SlashProvider` on the wrapping element, so this
 * component sets no coordinates of its own.
 *
 * Focus stays in the editor. The mousedown handler preventDefaults so clicking
 * a row does not pull focus out of ProseMirror, which would close the menu
 * before the click resolves.
 */
import { entityDisplayTitle } from '@/utils/entityDisplay'
import type { Entity } from '@/types'
import type { MentionMenuState } from './useMentionMenu'

const props = defineProps<{
  state: MentionMenuState
  minQueryLength: number
}>()

const emit = defineEmits<{
  pick: [index: number]
  hover: [index: number]
}>()

function titleOf(item: Entity): string {
  return entityDisplayTitle(item) || item.id
}
</script>

<template>
  <div class="mention-menu" @mousedown.prevent>
    <div v-if="props.state.loading" class="mention-menu-note">Searching…</div>
    <div v-else-if="props.state.errorMsg" class="mention-menu-note mention-menu-error">
      {{ props.state.errorMsg }}
    </div>
    <div v-else-if="props.state.query.length < props.minQueryLength" class="mention-menu-note">
      Type to search entities
    </div>
    <div v-else-if="props.state.items.length === 0" class="mention-menu-note">No matches</div>
    <ul v-else class="mention-menu-list" role="listbox">
      <li
        v-for="(item, idx) in props.state.items"
        :key="item.id"
        class="mention-menu-item"
        :class="{ 'is-highlighted': idx === props.state.highlightedIndex }"
        role="option"
        :aria-selected="idx === props.state.highlightedIndex"
        @click="emit('pick', idx)"
        @mousemove="emit('hover', idx)"
      >
        <span class="mention-menu-title">{{ titleOf(item) }}</span>
        <!-- The ID is shown here and nowhere else. It disambiguates two
             entities with the same title, and it is the label the editor
             falls back to when a title never resolves. The rendered view
             deliberately shows the title alone. -->
        <span class="mention-menu-id">{{ item.id }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.mention-menu {
  /* Positioning and show/hide belong to the anchor element the slash provider
     writes coordinates onto (see MilkdownEditor.vue); this only styles the
     panel itself. */
  min-width: 260px;
  max-width: 420px;
  max-height: 280px;
  overflow-y: auto;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: var(--card-bg);
  box-shadow: var(--shadow-lg);
  font-size: var(--font-size-base);
}

.mention-menu-list {
  margin: 0;
  padding: var(--space-2xs) 0;
  list-style: none;
}

.mention-menu-item {
  display: flex;
  gap: var(--space-sm);
  align-items: baseline;
  justify-content: space-between;
  padding: var(--space-xs) var(--space-md);
  cursor: pointer;
}

.mention-menu-item.is-highlighted {
  background: var(--hover-bg);
}

.mention-menu-title {
  overflow: hidden;
  color: var(--text-color);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mention-menu-id {
  flex-shrink: 0;
  color: var(--muted-text);
  font-size: var(--font-size-sm);
  font-family: monospace;
}

.mention-menu-note {
  padding: var(--space-sm) var(--space-md);
  color: var(--muted-text);
}

.mention-menu-error {
  /* `--error-color` is the token this theme actually defines; the fallback
     literal that used to stand in here was a light-theme red that never
     adapted to dark mode. */
  color: var(--error-color);
}
</style>
