<script setup lang="ts">
/**
 * The frame every Configure mockup sits in: the Configure space's sidebar, a
 * page header, and the draft bar that leads to the review step.
 *
 * Edits collect in one draft across every screen, because a single change
 * (removing a status value) can touch the data model, a form and a board at
 * once. Nothing is written until the draft is reviewed and saved.
 *
 * A fixture for the mockup stories, not part of the library's surface.
 */
import { ref } from 'vue'
import RlAppShell from '../components/layout/RlAppShell.vue'
import RlSidebar from '../components/layout/RlSidebar.vue'
import RlPageHeader from '../components/layout/RlPageHeader.vue'
import RlBackButton from '../components/layout/RlBackButton.vue'
import RlButton from '../components/common/RlButton.vue'
import RlText from '../components/common/RlText.vue'
import RlToastHost from '../components/feedback/RlToastHost.vue'
import { useToasts } from '../components/feedback/useToasts'
import RlConfigReview from './RlConfigReview.vue'
import { configNavGroups } from './config'

const props = withDefaults(
  defineProps<{
    activeId: string
    title: string
    /** A parent screen to go back to, for a screen that edits one item. */
    backLabel?: string
    /** The Storybook story id of that parent screen. */
    backTo?: string
    /** Unsaved changes in the draft. */
    draftCount?: number
    /** Opens the review step on load, for the story that shows it. */
    reviewOpen?: boolean
  }>(),
  { backLabel: undefined, backTo: undefined, draftCount: 5, reviewOpen: false },
)

const navOpen = ref(false)
const sidebarWidth = ref(260)
const reviewing = ref(props.reviewOpen)
const { toasts, dismiss, success } = useToasts()

function saved(summary: string) {
  reviewing.value = false
  success('Configuration saved', summary)
}
</script>

<template>
  <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
    <template #sidebar>
      <RlSidebar
        workspace-name="Configure"
        workspace-menu
        :groups="configNavGroups"
        :active-id="activeId"
        @close="navOpen = false"
      />
    </template>

    <div v-if="backLabel && backTo" style="padding: 16px var(--rl-page-gutter) 0">
      <!-- The stories render in Storybook's iframe, so the link opens the parent story in the whole window. -->
      <RlBackButton :label="backLabel" :href="'/?path=/story/' + backTo" target="_top" />
    </div>
    <RlPageHeader :title="title" :show-star="false" :show-menu="false" @open-nav="navOpen = true">
      <template v-if="$slots.tabs" #tabs><slot name="tabs" /></template>
      <template #actions>
        <slot name="actions" />
        <template v-if="draftCount > 0">
          <RlText size="sm" tone="muted">{{ draftCount }} unsaved changes</RlText>
          <RlButton variant="primary" icon="check" @click="reviewing = true">Review and save</RlButton>
        </template>
      </template>
    </RlPageHeader>

    <div style="padding: 24px var(--rl-page-gutter) 64px">
      <slot />
    </div>

    <RlConfigReview :open="reviewing" @close="reviewing = false" @save="saved" @discard="reviewing = false" />
    <RlToastHost :toasts="toasts" @dismiss="dismiss" />
  </RlAppShell>
</template>
