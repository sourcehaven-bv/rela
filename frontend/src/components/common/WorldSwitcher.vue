<script setup lang="ts">
import { computed } from 'vue'
import { useWorld } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores/schema'

/**
 * Selects the world the app reads in, written to `?world=` through
 * `setWorld`.
 *
 * Lists the declared worlds in schema.yaml order. A world the caller may not
 * select is shown disabled rather than hidden: world names are config, not a
 * secret, and a missing entry reads as a misconfiguration. Hidden when the
 * schema declares fewer than two worlds, since there is nothing to choose.
 */
const schemaStore = useSchemaStore()
const { world, setWorld } = useWorld()

const show = computed(() => schemaStore.worldOrder.length > 1)

const options = computed(() =>
  schemaStore.worldOrder.map((name) => ({
    name,
    readable: schemaStore.worlds.get(name)?.readable !== false,
  })),
)

function onChange(event: Event) {
  setWorld((event.target as HTMLSelectElement).value)
}
</script>

<template>
  <label v-if="show" class="world-switcher">
    <span class="world-label">World</span>
    <select
      class="world-select"
      data-testid="world-switcher"
      :value="world"
      @change="onChange"
    >
      <option
        v-for="o in options"
        :key="o.name"
        :value="o.name"
        :disabled="!o.readable"
      >
        {{ o.name }}
      </option>
    </select>
  </label>
</template>

<style scoped>
.world-switcher {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--rl-color-text-muted);
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
