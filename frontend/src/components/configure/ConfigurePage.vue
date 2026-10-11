<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import RlBackButton from 'rela-components/components/layout/RlBackButton.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import PageHeaderContent from '@/components/common/PageHeaderContent'
import { useConfigDraftStore } from '@/stores/configDraft'
import { problemsFor } from '@/configure/paths'

/**
 * The frame of every Configure screen: the page header with the draft's
 * count and the way to review it, a link back to the parent screen, and the
 * problems the last check found on this screen.
 */
const props = defineProps<{
  title: string
  /** The parent screen, for a screen that edits one item. */
  backLabel?: string
  backTo?: string
}>()

const draft = useConfigDraftStore()
const route = useRoute()

const problems = computed(() => problemsFor(draft.problems, route.path))
const countLabel = computed(() =>
  draft.count === 1 ? '1 unsaved change' : `${draft.count} unsaved changes`
)
const back = computed(() => (props.backTo ? { to: props.backTo } : undefined))
</script>

<template>
  <PageHeaderContent :title="title">
    <template #actions>
      <slot name="actions" />
      <template v-if="draft.count > 0">
        <RlText size="sm" tone="muted" data-testid="config-draft-count">{{ countLabel }}</RlText>
        <RlButton
          variant="primary"
          icon="check"
          data-testid="config-review"
          @click="draft.reviewOpen = true"
        >
          Review and save
        </RlButton>
      </template>
    </template>
  </PageHeaderContent>

  <div class="configure-page">
    <RlBackButton
      v-if="backLabel && back"
      :label="backLabel"
      :as="RouterLink"
      v-bind="back"
      class="configure-page__back"
    />
    <RlCallout
      v-for="(problem, i) in problems"
      :key="i"
      tone="danger"
      title="Fix before saving"
      data-testid="config-problem"
    >
      {{ problem.message }}
    </RlCallout>
    <slot />
  </div>
</template>

<style scoped>
.configure-page {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
  padding: var(--rl-space-6) var(--rl-page-gutter) 64px;
}

.configure-page__back {
  align-self: flex-start;
}
</style>
