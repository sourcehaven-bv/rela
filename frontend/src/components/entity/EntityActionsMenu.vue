<script setup lang="ts">
/**
 * An entity's actions behind a "⋯" button, in sections. See entityActions.ts
 * for where the list comes from.
 */
import { computed } from 'vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import { groupEntityActions, type EntityAction } from './entityActions'

const props = withDefaults(
  defineProps<{
    actions: EntityAction[]
    label?: string
    align?: 'start' | 'end'
  }>(),
  { label: 'More actions', align: 'end' }
)

const sections = computed(() => groupEntityActions(props.actions))
</script>

<template>
  <RlMenu v-if="actions.length" :align="align">
    <template #trigger="{ toggle, attrs }">
      <RlIconButton
        icon="ellipsis"
        :label="label"
        data-testid="entity-actions-menu"
        v-bind="attrs"
        @click="toggle"
      />
    </template>
    <template v-for="(section, index) in sections" :key="section[0].group">
      <RlMenuSeparator v-if="index > 0" />
      <RlMenuItem
        v-for="action in section"
        :key="action.id"
        :data-action="action.id"
        :icon="action.icon"
        :tone="action.tone"
        :shortcut="action.shortcut"
        :disabled="action.disabled"
        :href="action.href"
        @click="action.run?.()"
      >
        {{ action.label }}
      </RlMenuItem>
    </template>
  </RlMenu>
</template>
