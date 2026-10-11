<script setup lang="ts">
/**
 * A list of changes, described in the reader's terms: "Added field Due",
 * "Changed Effort from Text to Choice list", "Removed option Parked".
 *
 * It is for any place a user is shown what did or will change: reviewing a
 * draft before saving it, one version of an entity against another, a bulk
 * edit before it runs, a data migration's effect. The rows are
 * `RlChangeItem`s, put in the default slot.
 *
 * It is deliberately not a diff of text. A change list names the thing that
 * changed and how, which is what a reader decides on; the serialized form
 * behind it (YAML, JSON, markdown source) is an implementation detail the
 * reader should not need to parse.
 *
 * A title is optional and names what the changes belong to, such as an entity
 * type or a form. Several lists in a row, one per thing, is the usual shape.
 */
import RlHeading from '../common/RlHeading.vue'
import RlCount from '../common/RlCount.vue'

withDefaults(
  defineProps<{
    /** What the changes belong to: `Ticket`, `Ticket form`. */
    title?: string
    /** Heading level for the title, which depends on where the list sits. */
    level?: 2 | 3 | 4
    /** Shows the number of changes beside the title. */
    count?: number
  }>(),
  { title: undefined, level: 3, count: undefined },
)

defineSlots<{ default?: () => unknown; actions?: () => unknown }>()
</script>

<template>
  <section class="rl-change-list">
    <header v-if="title || $slots.actions" class="rl-change-list__header">
      <RlHeading v-if="title" :level="level" size="md">{{ title }}</RlHeading>
      <RlCount v-if="count !== undefined" :value="count" />
      <div v-if="$slots.actions" class="rl-change-list__actions"><slot name="actions" /></div>
    </header>
    <ul class="rl-change-list__items">
      <slot />
    </ul>
  </section>
</template>

<style scoped>
.rl-change-list__header {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin-bottom: var(--rl-space-2);
}

.rl-change-list__actions {
  margin-left: auto;
}

.rl-change-list__items {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  overflow: hidden;
}
</style>
