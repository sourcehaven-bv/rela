<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlSegmentedControl from 'rela-components/components/data/RlSegmentedControl.vue'
import { useWorld, worldQuery } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores/schema'

/**
 * Selects the world the app reads in, written to `?world=`.
 *
 * Lists the declared worlds in schema.yaml order, by their `label:`. A world
 * the caller may not select is shown disabled rather than left out: world
 * names are config, not a secret, and a missing entry reads as a
 * misconfiguration.
 *
 * ## What the operator controls
 *
 * - `app.world_switcher: none` renders nothing. Worlds are often wiring, not
 *   a choice a reader makes.
 * - `hidden: true` on a world leaves it out, unless it is the world on
 *   screen: the control never shows a world other than the one being read.
 *
 * ## Which control
 *
 * Two or three visible worlds render as a row of links, so every choice is
 * in view and each is a real link (middle click, new tab). More render as a
 * select. The count is taken from the VISIBLE worlds only, so the control
 * does not change shape when the reader lands on a hidden world; it then
 * shows one link more. Long labels truncate.
 *
 * The control has no visible label: the world banner says where the reader
 * is, and "World" is not a word the operator chose (TKT-5SZG2L). It renders
 * nothing when fewer than two worlds can be selected, since a control with
 * one usable option offers no choice. The exception is a reader on a world
 * they may not read: the control stays, as their way back.
 */
const { t } = useI18n()
const schemaStore = useSchemaStore()
const route = useRoute()
const { world, setWorld } = useWorld()

const current = computed(() => world.value || schemaStore.defaultWorld)

const visible = computed(() =>
  schemaStore.worldOrder.filter((name) => schemaStore.worlds.get(name)?.hidden !== true),
)

const options = computed(() =>
  schemaStore.worldOrder
    .filter((name) => schemaStore.worlds.get(name)?.hidden !== true || name === current.value)
    .map((name) => ({
      name,
      label: schemaStore.worldLabel(name),
      readable: schemaStore.worlds.get(name)?.readable !== false,
    })),
)

// A reader on a world they may not read still gets the way out, so the
// unreadable current world counts as a choice to leave.
const show = computed(() => {
  if (schemaStore.app.world_switcher === 'none') return false
  const readable = options.value.filter((o) => o.readable).length
  const currentUnreadable = options.value.some((o) => o.name === current.value && !o.readable)
  return readable > 1 || (currentUnreadable && readable > 0)
})

const segmented = computed(() => visible.value.length <= 3)

const segments = computed(() =>
  options.value.map((o) => ({
    value: o.name,
    label: o.label,
    disabled: !o.readable,
    as: RouterLink,
    // The current world links to the page as it is, so it keeps the page
    // number; another world resets it (see worldQuery).
    attrs: {
      to:
        o.name === current.value
          ? route.fullPath
          : { path: route.path, query: worldQuery(o.name, route.query, schemaStore.defaultWorld) },
    },
  })),
)

function onChange(event: Event) {
  setWorld((event.target as HTMLSelectElement).value)
}
</script>

<template>
  <div v-if="show && segmented" class="world-switcher" data-testid="world-switcher">
    <RlSegmentedControl
      :model-value="current"
      :options="segments"
      :label="t('switcher.worldAria')"
      size="sm"
    />
  </div>
  <div v-else-if="show" class="world-switcher">
    <select
      class="world-select"
      data-testid="world-switcher"
      :aria-label="t('switcher.worldAria')"
      :value="current"
      @change="onChange"
    >
      <option v-for="o in options" :key="o.name" :value="o.name" :disabled="!o.readable">
        {{ o.label }}
      </option>
    </select>
  </div>
</template>

<style scoped>
.world-switcher {
  display: flex;
  min-width: 0;
}

.world-switcher :deep(.rl-segmented) {
  max-width: 100%;
}

.world-select {
  flex: 1;
  min-width: 0;
  padding: 4px 6px;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-md);
  background: var(--rl-color-bg);
  color: var(--rl-color-text);
  font-size: 12px;
}
</style>
