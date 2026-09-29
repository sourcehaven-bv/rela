<script setup lang="ts">
/** Section label above a group of sidebar items, with an optional add control and menu. */
import RlIconButton from '../common/RlIconButton.vue'

defineProps<{ label: string; showMenu?: boolean; addLabel?: string }>()
defineEmits<{ menu: []; add: [] }>()
</script>

<template>
  <div class="rl-sidebar-group-header">
    <span class="rl-sidebar-group-header__label">{{ label }}</span>
    <RlIconButton
      v-if="addLabel"
      class="rl-sidebar-group-header__add"
      icon="plus"
      :label="addLabel"
      :size="16"
      @click="$emit('add')"
    />
    <RlIconButton
      v-if="showMenu"
      icon="ellipsis"
      :label="`${label} options`"
      :size="16"
      @click="$emit('menu')"
    />
  </div>
</template>

<style scoped>
.rl-sidebar-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--rl-space-2);
  margin-bottom: var(--rl-space-1);
}

.rl-sidebar-group-header__label {
  flex: 1;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}

/*
 * Shown on hover or focus where there is a pointer to hover with, so a quiet
 * list of headings does not carry a row of plus signs. Without hover (touch)
 * it stays visible: it would otherwise be unreachable.
 */
@media (hover: hover) {
  .rl-sidebar-group-header__add { opacity: 0; }
  .rl-sidebar-group-header:hover .rl-sidebar-group-header__add,
  .rl-sidebar-group-header__add:focus-visible { opacity: 1; }
}

/*
 * In the rail the header becomes a rule. Unlike a nav item there is nothing
 * to keep for a screen reader by hiding it visually: the header labels the
 * group only in sighted reading order and is not any control's accessible
 * name, so it goes entirely and a line keeps the grouping the words carried.
 *
 * The menu goes with it. It acts on the group the label named, and a rail
 * offers no way to say which group that is.
 */
@container rl-sidebar (max-width: 120px) {
  .rl-sidebar-group-header {
    justify-content: center;
    padding: 0;
    margin: var(--rl-space-2) var(--rl-space-3) var(--rl-space-3);
    border-top: 1px solid var(--rl-color-border);
  }

  .rl-sidebar-group-header > * { display: none; }
}
</style>
