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
import { computed, type DeepReadonly } from 'vue'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import type { Entity } from '@/types'
import type { MentionMenuState } from './useMentionMenu'

const props = defineProps<{
  /**
   * Readonly so the controller's own type binds directly: the menu is rendered
   * from state it must never mutate, and laundering the readonly away at the
   * call site is what a cast there was hiding.
   */
  state: DeepReadonly<MentionMenuState>
  minQueryLength: number
  /**
   * Which combined-list row is highlighted, resolved by the controller.
   *
   * Passed in rather than read off `state` because the highlight is stored as
   * an IDENTITY and resolved against the live rows; a position kept in state
   * could address a row that had changed underneath it.
   */
  highlightedIndex: number
}>()

const emit = defineEmits<{
  pick: [index: number]
  hover: [index: number]
}>()

function titleOf(item: DeepReadonly<Entity>): string {
  return entityDisplayTitle(item) || item.id
}

/**
 * The highlight addresses types and entities as one sequence, so an entity's
 * row index is offset by however many type rows precede it.
 */
const typeCount = computed(() => props.state.typeItems.length)
function entityIndex(idx: number): number {
  return typeCount.value + idx
}

/**
 * A per-instance id prefix, so two editors on one page cannot mint the same
 * option ids (`aria-activedescendant` resolves against the whole document).
 */
const uid = `mention-menu-${Math.random().toString(36).slice(2, 8)}`

/** The DOM id of the row at `index` in the combined list. */
function optionId(index: number): string {
  return `${uid}-opt-${index}`
}

/**
 * The option `aria-activedescendant` points at.
 *
 * Without this a screen reader announces nothing as the highlight moves: the
 * rows are not focused (focus stays in the editor), so the only way to convey
 * the active row is to name it from the listbox.
 */
const activeOptionId = computed(() =>
  props.highlightedIndex >= 0 ? optionId(props.highlightedIndex) : undefined
)

/** True when the panel has nothing to show and a note should stand in. */
const isEmpty = computed(() => props.state.typeItems.length === 0 && props.state.items.length === 0)
</script>

<template>
  <div
    class="mention-menu"
    role="listbox"
    aria-label="Entity reference suggestions"
    :aria-activedescendant="activeOptionId"
    @mousedown.prevent
  >
    <!-- The scope chip. Shown above everything, including while loading, so
         the user can always see which type the results are drawn from. It is
         menu state rather than document text, so clearing it writes nothing to
         the editor. -->
    <div v-if="props.state.selectedType" class="mention-menu-chip-row">
      <span class="mention-menu-chip">{{ props.state.selectedType }}</span>
      <span class="mention-menu-chip-hint">Backspace to clear</span>
    </div>

    <!-- Types are ranked and rendered even while a search is in flight: they
         come from the already-loaded schema, so blanking them behind the
         spinner would make the section flicker on every keystroke. -->
    <template v-if="props.state.typeItems.length > 0">
      <div :id="`${uid}-types-label`" class="mention-menu-section">Types</div>
      <ul
        class="mention-menu-list"
        role="group"
        :aria-labelledby="`${uid}-types-label`"
        data-section="types"
      >
        <li
          v-for="(name, idx) in props.state.typeItems"
          :id="optionId(idx)"
          :key="`type:${name}`"
          class="mention-menu-item"
          :class="{ 'is-highlighted': idx === props.highlightedIndex }"
          role="option"
          :aria-selected="idx === props.highlightedIndex"
          @click="emit('pick', idx)"
          @mousemove="emit('hover', idx)"
        >
          <span class="mention-menu-title">{{ name }}</span>
          <span class="mention-menu-id">type</span>
        </li>
      </ul>
    </template>

    <div v-if="props.state.loading || props.state.errorMsg">
      <div v-if="typeCount > 0" class="mention-menu-section">Entities</div>
      <div v-if="props.state.loading" class="mention-menu-note">Searching…</div>
      <div v-else class="mention-menu-note mention-menu-error">{{ props.state.errorMsg }}</div>
    </div>
    <div
      v-else-if="props.state.query.length < props.minQueryLength && isEmpty"
      class="mention-menu-note"
    >
      Type to search entities
    </div>
    <div v-else-if="isEmpty" class="mention-menu-note">No matches</div>
    <template v-else-if="props.state.items.length > 0">
      <div v-if="typeCount > 0" :id="`${uid}-entities-label`" class="mention-menu-section">
        Entities
      </div>
      <ul
        class="mention-menu-list"
        role="group"
        :aria-labelledby="typeCount > 0 ? `${uid}-entities-label` : undefined"
        :aria-label="typeCount > 0 ? undefined : 'Entities'"
        data-section="entities"
      >
        <li
          v-for="(item, idx) in props.state.items"
          :id="optionId(entityIndex(idx))"
          :key="`entity:${item.id}:${idx}`"
          class="mention-menu-item"
          :class="{ 'is-highlighted': entityIndex(idx) === props.highlightedIndex }"
          role="option"
          :aria-selected="entityIndex(idx) === props.highlightedIndex"
          @click="emit('pick', entityIndex(idx))"
          @mousemove="emit('hover', entityIndex(idx))"
        >
          <span class="mention-menu-title">{{ titleOf(item) }}</span>
          <!-- The ID is shown here and nowhere else. It disambiguates two
               entities with the same title, and it is the label the editor
               falls back to when a title never resolves. The rendered view
               deliberately shows the title alone. -->
          <span class="mention-menu-id">{{ item.id }}</span>
        </li>
      </ul>
    </template>
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

.mention-menu-section {
  padding: var(--space-2xs) var(--space-md);
  color: var(--muted-text);
  font-size: var(--font-size-sm);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.mention-menu-section:not(:first-child) {
  border-top: 1px solid var(--border-color);
  margin-top: var(--space-2xs);
  padding-top: var(--space-xs);
}

.mention-menu-chip-row {
  display: flex;
  gap: var(--space-sm);
  align-items: baseline;
  padding: var(--space-xs) var(--space-md);
  border-bottom: 1px solid var(--border-color);
}

.mention-menu-chip {
  padding: 0 var(--space-xs);
  border-radius: var(--radius-sm);
  background: var(--hover-bg);
  color: var(--text-color);
  font-size: var(--font-size-sm);
  font-family: monospace;
}

.mention-menu-chip-hint {
  color: var(--muted-text);
  font-size: var(--font-size-sm);
}

.mention-menu-error {
  /* `--error-color` is the token this theme actually defines; the fallback
     literal that used to stand in here was a light-theme red that never
     adapted to dark mode. */
  color: var(--error-color);
}
</style>
