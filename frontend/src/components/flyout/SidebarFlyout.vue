<script setup lang="ts">
/**
 * The panels a sidebar entry slides out over the page: the list, and beside
 * it the row the reader opened.
 *
 * The library draws the stack and owns Escape; rela owns what is open (see
 * useFlyout) and closes everything on navigation. That includes navigation
 * started from inside the flyout, such as a link in the detail panel or the
 * expand control: the flyout was a look at something on the way somewhere,
 * and once the reader has gone there it has done its job.
 */
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useFlyout } from '@/composables/useFlyout'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { entityDetailHref } from '@/utils/entityRoute'
import { entityRef } from '@/utils/entityRef'
import RlSlidePanelStack, { type SlidePanel } from 'rela-components/components/layout/RlSlidePanelStack.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import EntityDetail from '@/components/entity/EntityDetail.vue'
import FlyoutList from './FlyoutList.vue'

const route = useRoute()
const router = useRouter()
const flyout = useFlyout()

const panels = computed<SlidePanel[]>(() => {
  const result: SlidePanel[] = []
  const list = flyout.list.value
  if (!list) return result
  result.push({ id: 'list', title: list.title, size: 'md' })
  const entity = flyout.entity.value
  // The detail draws its own title, so the panel's stays for screen readers.
  if (entity) result.push({ id: 'entity', title: entityDisplayTitle(entity), size: 'lg', titleHidden: true })
  return result
})

/* Closing a panel closes what it opened, so the detail cannot outlive its list. */
function onClose(id: string) {
  if (id === 'list') flyout.close()
  else flyout.closeEntity()
}

function expand(id: string) {
  if (id === 'list') {
    const list = flyout.list.value
    if (list) void router.push(list.href)
    return
  }
  const entity = flyout.entity.value
  if (!entity) return
  const href = entityDetailHref({ id: entity.id, type: entity.type })
  if (href) void router.push(href)
}

watch(() => route.path, () => flyout.close())
</script>

<template>
  <RlSlidePanelStack :panels="panels" class="sidebar-flyout" manage-focus @close="onClose">
    <template #panel-actions="{ panel }">
      <RlIconButton
        icon="maximize-2"
        :label="panel.id === 'list' ? 'Open the full list' : 'Open the full page'"
        @click="expand(panel.id)"
      />
    </template>

    <template #panel="{ panel }">
      <FlyoutList
        v-if="panel.id === 'list' && flyout.list.value"
        :list-id="flyout.list.value.listId"
      />
      <EntityDetail
        v-else-if="panel.id === 'entity' && flyout.entity.value"
        :key="flyout.entity.value.id"
        class="sidebar-flyout__detail"
        :entity-type="flyout.entity.value.type"
        :entity-id="entityRef(flyout.entity.value)"
        hide-actions
      />
    </template>
  </RlSlidePanelStack>
</template>

<style scoped>
.sidebar-flyout__detail {
  padding: 0 var(--rl-space-5) var(--rl-space-5);
}
</style>
