<script setup lang="ts">
/**
 * One group of sidebar items, with the behaviour that depends on how many
 * there are.
 *
 * A group of five is a list you read. A group of forty is one you hunt
 * through, and the same markup serves both badly: subheadings on a short list
 * spend vertical space organising something nobody was struggling with, and
 * their absence on a long one leaves an undifferentiated wall.
 *
 * So the group looks at its own size. Below the threshold it renders exactly
 * as it always did, a header and a flat list. Above it, and only if the data
 * says how, it splits its items under subheadings. Nothing is configured at
 * the call site: the same group grows the affordance when it needs it and
 * loses it again when it does not.
 */
import { computed } from 'vue'
import type { NavGroup, NavItem } from '../../types'
import RlSidebarGroupHeader from './RlSidebarGroupHeader.vue'
import RlSidebarNavItem from './RlSidebarNavItem.vue'

const props = withDefaults(
  defineProps<{
    group: NavGroup
    activeId?: string
    /** The id of the item whose panel is open. See `NavItem.opensFlyout`. */
    flyoutId?: string | null
    /**
     * Item count past which the group splits under its subheadings. Matched
     * to the sidebar's filter threshold, so a list that grows long gains both
     * at once rather than arriving as two separate surprises.
     *
     * Set to Infinity to keep a group flat however long it gets.
     */
    groupThreshold?: number
    /** Heading for items with no subgroup of their own. */
    fallbackLabel?: string
  }>(),
  { groupThreshold: 20, fallbackLabel: 'Other' },
)

const emit = defineEmits<{ select: [item: NavItem]; menu: []; add: [] }>()

/**
 * Whether to split. Both conditions matter: a long group with no subgroups
 * declared has nothing to split by, and a short one does not need to.
 */
const grouped = computed(
  () =>
    (props.group.subgroups?.length ?? 0) > 0 &&
    props.group.items.length > props.groupThreshold,
)

/**
 * The items under each subheading, in the order the subgroups were declared.
 *
 * Empty subgroups are dropped rather than shown as bare headings, and
 * anything left over collects under one trailing fallback. Leaving the
 * remainder out would hide items the user can still reach by other means,
 * which is worse than an untidy last heading.
 */
const sections = computed(() => {
  const subgroups = props.group.subgroups ?? []
  const known = new Set(subgroups.map((subgroup) => subgroup.id))

  const named = subgroups
    .map((subgroup) => ({
      id: subgroup.id,
      label: subgroup.label,
      items: props.group.items.filter((item) => item.groupId === subgroup.id),
    }))
    .filter((section) => section.items.length > 0)

  const rest = props.group.items.filter(
    (item) => !item.groupId || !known.has(item.groupId),
  )

  return rest.length > 0
    ? [...named, { id: '__rest', label: props.fallbackLabel, items: rest }]
    : named
})
</script>

<template>
  <section class="rl-sidebar-group">
    <RlSidebarGroupHeader
      v-if="group.label"
      class="rl-sidebar-group__header"
      :label="group.label"
      :show-menu="group.showMenu"
      :add-label="group.addLabel"
      @menu="emit('menu')"
      @add="emit('add')"
    />

    <template v-if="grouped">
      <div v-for="section in sections" :key="section.id" class="rl-sidebar-group__section">
        <!--
          A subheading, not a group header: it names a slice of one list
          rather than a list of its own, so it is quieter than the header
          above it and carries no menu.
        -->
        <h3 class="rl-sidebar-group__subheading">{{ section.label }}</h3>

        <RlSidebarNavItem
          v-for="item in section.items"
          :key="item.id"
          :item="item"
          :active-id="activeId"
          :flyout-id="flyoutId"
          @select="emit('select', $event)"
        />
      </div>
    </template>

    <RlSidebarNavItem
      v-for="item in group.items"
      v-else
      :key="item.id"
      :item="item"
      :active-id="activeId"
      :flyout-id="flyoutId"
      @select="emit('select', $event)"
    />
  </section>
</template>

<style scoped>
/*
 * Sticks while its own items scroll past, so a long group is never a list of
 * names with nothing saying what they are a list of. The spread paints the
 * nav's padding in the same colour, so items travelling underneath are
 * covered rather than showing in the gutter beside the label.
 *
 * The spread is clipped at the bottom edge. Below the header it would paint
 * over the top of the first item, which sits closer than the spread's width.
 */
.rl-sidebar-group__header {
  position: sticky;
  top: 0;
  /* Box-sized to the offset its subheadings park at, so the two agree. */
  box-sizing: border-box;
  min-height: var(--rl-sidebar-subheading-offset);
  z-index: var(--rl-z-sticky-raised);
  background: var(--rl-color-bg-sunken);
  box-shadow: 0 0 0 var(--rl-space-3) var(--rl-color-bg-sunken);
  clip-path: inset(calc(-1 * var(--rl-space-3)) calc(-1 * var(--rl-space-3)) 0);
}

.rl-sidebar-group__section + .rl-sidebar-group__section {
  margin-top: var(--rl-space-4);
}

/*
 * Below the group header, so the two stack rather than overlap when a long
 * section scrolls under both.
 */
.rl-sidebar-group__subheading {
  position: sticky;
  top: var(--rl-sidebar-subheading-offset);
  z-index: var(--rl-z-sticky);
  margin: 0 0 var(--rl-space-1);
  /*
   * Padded to meet the group header exactly where it parks. Without the top
   * padding an item shows in the sliver between the two as it scrolls under,
   * since the subheading's own box starts below the header's bottom edge.
   * The spread is clipped at the bottom for the reason the header gives.
   */
  padding: var(--rl-space-2) var(--rl-space-2) var(--rl-space-1);
  background: var(--rl-color-bg-sunken);
  box-shadow: 0 0 0 var(--rl-space-3) var(--rl-color-bg-sunken);
  clip-path: inset(calc(-1 * var(--rl-space-3)) calc(-1 * var(--rl-space-3)) 0);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--rl-color-text-subtle);
}
</style>
