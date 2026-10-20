<script setup lang="ts">
/**
 * The channels on one entity, grouped by channel type.
 *
 * It lists only what the app passes, which is what the server let this user
 * read: there are no locked rows and no unread counts. A channel type with
 * `cardinality: many` gets a create button when the user may create one; a
 * `one` type has its single channel or nothing.
 *
 * The state filter is the caller's (`v-model:filter`), because the server
 * filters the list; this component only offers the choice.
 */
import RlButton from '../common/RlButton.vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import RlSegmentedControl from '../data/RlSegmentedControl.vue'
import type { ChannelGroupView } from './types'

defineProps<{
  groups: ChannelGroupView[]
  /** The key of the open channel. */
  activeKey?: string
  /** Which channels the list holds; omit to hide the filter. */
  filter?: 'open' | 'all'
}>()

const emit = defineEmits<{
  select: [key: string]
  create: [type: string]
  'update:filter': [filter: 'open' | 'all']
}>()

const filterOptions = [
  { value: 'open', label: 'Open' },
  { value: 'all', label: 'All' },
]
</script>

<template>
  <div class="rl-entity-channels">
    <RlSegmentedControl
      v-if="filter"
      class="rl-entity-channels__filter"
      label="Show channels"
      size="sm"
      :model-value="filter"
      :options="filterOptions"
      @update:model-value="(v: string) => emit('update:filter', v === 'all' ? 'all' : 'open')"
    />

    <section v-for="g in groups" :key="g.type" class="rl-entity-channels__group" :aria-label="g.label">
      <div class="rl-entity-channels__head">
        <h3>{{ g.label }}</h3>
        <RlButton
          v-if="g.cardinality === 'many' && g.canCreate"
          variant="ghost"
          size="sm"
          icon="plus"
          @click="emit('create', g.type)"
        >
          {{ g.createLabel || 'New channel' }}
        </RlButton>
      </div>
      <ul v-if="g.channels.length" class="rl-entity-channels__list">
        <li v-for="c in g.channels" :key="c.key">
          <button
            type="button"
            class="rl-entity-channels__row"
            :class="{
              'rl-entity-channels__row--active': c.key === activeKey,
              'rl-entity-channels__row--inactive': c.state !== 'open',
            }"
            :aria-current="c.key === activeKey ? 'true' : undefined"
            @click="emit('select', c.key)"
          >
            <RlIcon :name="c.state === 'open' ? 'discussion' : c.state === 'closed' ? 'lock' : 'archive'" :size="16" />
            <span class="rl-entity-channels__main">
              <span class="rl-entity-channels__title">{{ c.title }}</span>
              <span v-if="c.summary" class="rl-entity-channels__summary">{{ c.summary }}</span>
            </span>
            <RlTag v-if="c.state !== 'open'" :label="c.state === 'closed' ? 'Closed' : 'Archived'" color="grey" />
          </button>
        </li>
      </ul>
      <p v-else class="rl-entity-channels__empty">No channels yet.</p>
    </section>
  </div>
</template>

<style scoped>
.rl-entity-channels {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
  color: var(--rl-color-text);
  font-size: var(--rl-font-size-sm);
}

.rl-entity-channels__filter { align-self: flex-start; }

.rl-entity-channels__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-2);
  margin-bottom: var(--rl-space-2);
}

.rl-entity-channels__head h3 {
  margin: 0;
  font-size: var(--rl-font-size-sm);
  font-weight: var(--rl-font-weight-semibold);
}

.rl-entity-channels__list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  overflow: hidden;
}

.rl-entity-channels__list li + li { border-top: 1px solid var(--rl-color-border); }

.rl-entity-channels__row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  width: 100%;
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.rl-entity-channels__row:hover { background: var(--rl-color-bg-hover); }

.rl-entity-channels__row:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-entity-channels__row--active { background: var(--rl-color-bg-selected); }
.rl-entity-channels__row--inactive { color: var(--rl-color-text-muted); }

.rl-entity-channels__main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}

.rl-entity-channels__title {
  overflow: hidden;
  font-weight: var(--rl-font-weight-semibold);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-entity-channels__summary {
  overflow: hidden;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-entity-channels__empty {
  margin: 0;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}
</style>
