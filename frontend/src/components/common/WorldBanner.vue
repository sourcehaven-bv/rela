<script setup lang="ts">
/**
 * The banner a world-scoped surface shows above its content: an optional
 * announcement (the `label`), a note about what the world changes here (the
 * default slot), and — where one exists — the way to the face that takes
 * writes (the `actions` slot).
 *
 * Shared by EntityList and EntityDetail so the surfaces read as one feature.
 * Each supplies its own note because a world changes different things per
 * surface: rows filtered on a list, writes refused on a detail. The caller
 * owns the `v-if`; this component only lays the claim out, and the caller
 * knows when the claim would be made over an error state (EntityList's
 * `!loadError` gate) or over a page that is NOT read-only (EntityDetail's
 * `worldAbsent` variant, which uses `variant="absent"`).
 *
 * The label is optional because the announcement is operator config
 * (`banner:` on the world): "DRAFT — not in force" on an editorial world,
 * nothing on a language world where "you are reading Dutch" is noise to
 * someone who chose Dutch. The note is the caller's and not configurable —
 * it states facts about the request that hold whatever the operator declares.
 */
defineProps<{
  /** The announcement. Empty renders no label, only the note. */
  label?: string
  /**
   * `absent` marks the warning-coloured case: the world served NOTHING for
   * this entity and the page shows the default face instead. It carries the
   * same colour WorldBadge uses for a substitute, because it is the same
   * statement at page scale.
   */
  variant?: 'absent'
}>()
</script>

<template>
  <div class="world-banner" :class="{ 'world-banner--absent': variant === 'absent' }">
    <span v-if="label" class="world-banner__label">{{ label }}</span>
    <span class="world-banner__note"><slot /></span>
    <div v-if="$slots.actions" class="world-banner__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.world-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-xs) var(--space-md);
  margin-bottom: var(--space-md);
  padding: var(--space-sm) var(--space-md);
  background: color-mix(in srgb, var(--accent-color) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent-color) 30%, transparent);
  border-radius: var(--radius-md);
}

.world-banner--absent {
  border-color: color-mix(in srgb, var(--warning-color) 60%, transparent);
  background: color-mix(in srgb, var(--warning-color) 12%, transparent);
}

.world-banner__label {
  font-size: var(--font-size-base);
  color: var(--text-color);
}

/* Grows to take the row's remaining width, and wraps under the label when
   the row is too narrow for both — so a long note (the list's) reads as its
   own line while a short one (the detail's) sits beside the label. */
.world-banner__note {
  flex: 1 1 24rem;
  font-size: var(--font-size-sm);
  color: var(--muted-text);
}

.world-banner__actions {
  margin-left: auto;
}
</style>
