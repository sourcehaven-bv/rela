<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import InlineCreateFormModal from '@/components/forms/InlineCreateFormModal.vue'
import { useSpaceStore } from '@/stores/space'
import type { Entity, SidebarCreate } from '@/types'

/**
 * The current space's Create menu (TKT-GNKR5H).
 *
 * The rows are the space's `create:` types that the principal may create and
 * that have a create form; the server filters them, so presence is the offer.
 * A row opens the create dialog in place. The new entity then opens on its
 * own page, in the current space.
 */
const space = useSpaceStore()
const router = useRouter()

const creating = ref<SidebarCreate | null>(null)

function onCreated(entity: Entity) {
  creating.value = null
  void router.push(`/entity/${entity.type}/${encodeURIComponent(entity.id)}`)
}
</script>

<template>
  <template v-if="space.create.length">
    <RlMenu align="end">
      <template #trigger="{ toggle, attrs }">
        <RlButton variant="primary" data-testid="space-create" v-bind="attrs" @click="toggle">
          Create
          <template #trailing><RlIcon name="chevron-down" :size="12" /></template>
        </RlButton>
      </template>
      <RlMenuItem v-for="row in space.create" :key="row.type" @click="creating = row">
        {{ row.label }}
      </RlMenuItem>
    </RlMenu>

    <InlineCreateFormModal
      v-if="creating"
      :show="true"
      :form-id="creating.form"
      :entity-type="creating.type"
      add-another
      @close="creating = null"
      @created="onCreated"
    />
  </template>
</template>
