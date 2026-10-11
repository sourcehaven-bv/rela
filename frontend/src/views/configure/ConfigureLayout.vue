<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { ApiError } from '@/api/errors'
import { useConfigureSnapshot } from '@/queries/configure'
import { useConfigDraftStore } from '@/stores/configDraft'
import { useUIStore } from '@/stores'
import { isConfigIncomplete, takeConfigSaved } from '@/configure/saveGate'
import { useDiscardDraft } from '@/configure/useDiscardDraft'
import ReviewDrawer from '@/components/configure/ReviewDrawer.vue'

/**
 * The Configure space (TKT-F5NGMG). Loads the configuration once, hands it
 * to the draft store, and keeps the review drawer mounted across every
 * screen, since a draft spans them all.
 */
const draft = useConfigDraftStore()
const ui = useUIStore()
const { data, error, isPending } = useConfigureSnapshot()
const discardStale = useDiscardDraft()

watch(
  data,
  (snapshot) => {
    if (snapshot && snapshot !== draft.base) draft.load(snapshot)
  },
  { immediate: true }
)

onMounted(() => {
  if (takeConfigSaved()) ui.success('Configuration saved')
  // A save whose migration did not finish reloads this tab too; ask again.
  if (isConfigIncomplete()) draft.reviewOpen = true
})

/** Why the space cannot be shown, in words for the person reading it. */
const failure = computed(() => {
  const err = error.value
  if (!err) return ''
  if (err instanceof ApiError && err.status === 404) {
    return 'Configuring in the app is not turned on for this server.'
  }
  if (err instanceof ApiError && err.status === 403) {
    return 'You do not have permission to configure this app.'
  }
  return 'The configuration could not be loaded.'
})
</script>

<template>
  <RlStatusRegion v-if="failure" tone="error" data-testid="config-unavailable">
    {{ failure }}
  </RlStatusRegion>
  <RlStatusRegion v-else-if="isPending || !draft.loaded">Loading…</RlStatusRegion>
  <template v-else>
    <div v-if="draft.stale" class="configure-stale">
      <RlCallout tone="warning" title="The configuration changed" data-testid="config-stale">
        Someone saved a change since your draft was started, so it can no longer be saved. Discard
        your draft to start again from the current configuration.
        <div class="configure-stale__actions">
          <RlButton variant="secondary" size="sm" @click="discardStale">Discard my draft</RlButton>
        </div>
      </RlCallout>
    </div>
    <RouterView />
    <ReviewDrawer />
  </template>
</template>

<style scoped>
.configure-stale {
  padding: var(--rl-space-4) var(--rl-page-gutter) 0;
}

.configure-stale__actions {
  margin-top: var(--rl-space-3);
}
</style>
