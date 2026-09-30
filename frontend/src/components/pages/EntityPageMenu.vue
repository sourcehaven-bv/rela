<script setup lang="ts">
/**
 * The "⋯" menu beside an entity page's title: what can be done to the page's
 * entity itself, as opposed to the rows its tabs show.
 *
 * Details and Open are offered to every reader, since they only read. Delete
 * is offered when the server's `_actions` allow it; the delete itself
 * re-authorizes.
 */
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'

defineProps<{
  /** The entity's type label, for the menu's accessible name. */
  typeLabel: string
  canDelete: boolean
}>()

const emit = defineEmits<{
  details: []
  open: []
  delete: []
}>()
</script>

<template>
  <RlMenu align="start">
    <template #trigger="{ toggle, attrs }">
      <RlIconButton
        icon="ellipsis"
        :label="`${typeLabel} options`"
        data-testid="entity-page-menu"
        v-bind="attrs"
        @click="toggle"
      />
    </template>
    <RlMenuItem icon="panel" @click="emit('details')">Details</RlMenuItem>
    <RlMenuItem icon="external" @click="emit('open')">Open full page</RlMenuItem>
    <RlMenuItem v-if="canDelete" icon="delete" tone="danger" @click="emit('delete')">
      Delete
    </RlMenuItem>
  </RlMenu>
</template>
