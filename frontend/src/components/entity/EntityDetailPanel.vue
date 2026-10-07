<script setup lang="ts">
/**
 * An entity's detail rendered beside the list it was opened from.
 *
 * A panel is NOT the canonical item view. The full page at
 * `/entity/:type/:id` stays the shareable address of the entity itself; this
 * is a preview of one row of one list, addressed by that list's URL plus
 * `?selected=`. The toolbar's expand control is the bridge between the two.
 *
 * The header's action buttons are hidden (`hideActions`): stacked inside a
 * 720px column they crowd the title. The same actions, from the same checks,
 * are the toolbar's "⋯" menu instead.
 *
 * An owned row is shown here as itself, with a link to its owner: following
 * the owner would navigate away from the list the panel belongs to.
 */
import RlDetailPanel from 'rela-components/components/task/RlDetailPanel.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import { ref } from 'vue'
import EntityDetail from './EntityDetail.vue'
import EntityActionsMenu from './EntityActionsMenu.vue'
import type { EntityAction } from './entityActions'

defineProps<{
  entityType: string
  entityId: string
}>()

const emit = defineEmits<{
  close: []
  expand: []
}>()

const actions = ref<EntityAction[]>([])
</script>

<template>
  <RlDetailPanel
    class="entity-detail-panel"
    show-close
    show-link
    :show-navigation="false"
    :show-attach="false"
    @close="emit('close')"
    @expand="emit('expand')"
  >
    <!--
      On a phone the panel is the whole screen, so a close control reads as a
      back step. The library hides its prev/next there for the same reason.
    -->
    <template #leading>
      <RlIconButton
        class="rl-phone-only"
        icon="arrow-left"
        label="Back to list"
        @click="emit('close')"
      />
    </template>

    <template #menu>
      <EntityActionsMenu :actions="actions" />
    </template>

    <EntityDetail
      :entity-type="entityType"
      :entity-id="entityId"
      hide-actions
      :follow-owner="false"
      @actions="actions = $event"
    />
  </RlDetailPanel>
</template>
