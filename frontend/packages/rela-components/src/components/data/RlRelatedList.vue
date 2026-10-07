<script setup lang="ts" generic="T extends RelatedItem">
/**
 * A titled list of records linked to the one on screen, with a count and a
 * way to add one. Subtasks are one use; the list does not know what the
 * link means, so the same component shows a decision's requirements or a
 * risk's measures.
 *
 * Rows are `RlRelatedRow` unless the `row` slot draws its own, for a caller
 * whose rows need more than a glyph, a title and a few values.
 */
import type { RelatedItem } from './types'
import RlHeading from '../common/RlHeading.vue'
import RlText from '../common/RlText.vue'
import RlButton from '../common/RlButton.vue'
import RlRelatedRow from './RlRelatedRow.vue'

withDefaults(
  defineProps<{
    title: string
    items: T[]
    /**
     * The add button's label, such as "Add subtask". Absent means no button:
     * a list that cannot take a new record must not offer to.
     */
    addLabel?: string
    /**
     * What to say when there are no items. Absent hides the whole list when
     * it is empty, unless it offers to add one.
     */
    emptyLabel?: string
    /** The heading's place in the document outline. */
    level?: 2 | 3 | 4 | 5 | 6
  }>(),
  { level: 2 },
)

const emit = defineEmits<{
  select: [item: T]
  add: []
}>()
</script>

<template>
  <section v-if="items.length || emptyLabel || addLabel" class="rl-related-list">
    <RlHeading
      :level="level"
      size="md"
      weight="normal"
      tone="muted"
      line-height="normal"
      class="rl-related-list__title"
    >
      {{ title }} <RlText size="md" tone="subtle">&middot; {{ items.length }}</RlText>
    </RlHeading>

    <div v-if="items.length" class="rl-related-list__rows">
      <template v-for="item in items" :key="item.id">
        <slot name="row" :item="item">
          <RlRelatedRow :item="item" @select="emit('select', $event)" />
        </slot>
      </template>
    </div>
    <RlText v-else-if="emptyLabel" as="p" tone="subtle" class="rl-related-list__empty">{{ emptyLabel }}</RlText>

    <RlButton
      v-if="addLabel"
      variant="secondary"
      size="sm"
      class="rl-related-list__add"
      @click="emit('add')"
    >
      {{ addLabel }}
    </RlButton>
  </section>
</template>

<style scoped>
.rl-related-list__title { margin-bottom: var(--rl-space-3); }

.rl-related-list__rows { border-top: 1px solid var(--rl-color-border); }

.rl-related-list__empty { padding: var(--rl-space-2) 0; }

.rl-related-list__add { margin-top: var(--rl-space-3); }
</style>
