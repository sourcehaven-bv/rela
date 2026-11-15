<script setup lang="ts">
/**
 * A step that walks the rows of its list one at a time (TKT-M07C97Y): the
 * row's own detail view, its position among the list's rows, and links to
 * the previous and next row.
 *
 * The page owns the URLs and reads the rows; this component only draws
 * them. The page also passes the row's detail view in the default slot,
 * which is shown only in the `ready` state. Each control is a real link, so a row can be opened in a new tab
 * and the browser's back button returns to the previous row.
 */
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import type { StepWalkState } from '@/types'

/**
 * The part of an entity position this component reads. It is declared
 * here, so drawing a walk does not depend on the API layer.
 */
interface WalkPosition {
  prev: { id: string } | null
  next: { id: string } | null
  current: number
  total: number
}

defineProps<{
  state: StepWalkState
  /** The row on screen, from `?item=`. */
  item?: string
  position?: WalkPosition
  /** The URL of row `id` on this step, keeping present mode as it is; without an id, the first row. */
  href: (id?: string) => string
}>()

const { t } = useI18n()
</script>

<template>
  <div class="page-walk">
    <RlEmptyState
      v-if="state === 'empty'"
      :title="t('page.steps.walk.empty')"
      :description="t('page.steps.walk.emptyDescription')"
    />
    <RlEmptyState
      v-else-if="state === 'missing'"
      icon="warning"
      :title="t('page.steps.walk.missing')"
      :description="t('page.steps.walk.missingDescription', { id: item ?? '' })"
    >
      <template #actions>
        <RlButton :as="RouterLink" :to="href()" variant="secondary" size="sm">
          {{ t('page.steps.walk.first') }}
        </RlButton>
      </template>
    </RlEmptyState>
    <RlEmptyState
      v-else-if="state === 'error'"
      icon="warning"
      :title="t('page.steps.walk.error')"
      :description="t('page.steps.walk.errorDescription')"
    />
    <template v-else-if="item">
      <nav class="page-walk__nav" :aria-label="t('page.steps.walk.label')">
        <RlButton
          v-if="position?.prev"
          :as="RouterLink"
          :to="href(position.prev.id)"
          variant="secondary"
          size="sm"
          icon="arrow-left"
          data-testid="walk-prev"
        >
          {{ t('page.steps.walk.prev') }}
        </RlButton>
        <RlButton
          v-else
          variant="secondary"
          size="sm"
          icon="arrow-left"
          disabled
          data-testid="walk-prev"
        >
          {{ t('page.steps.walk.prev') }}
        </RlButton>
        <span
          v-if="position"
          class="page-walk__position"
          data-testid="walk-position"
          aria-live="polite"
        >
          {{ t('page.steps.walk.position', { current: position.current, total: position.total }) }}
        </span>
        <RlButton
          v-if="position?.next"
          :as="RouterLink"
          :to="href(position.next.id)"
          variant="secondary"
          size="sm"
          data-testid="walk-next"
        >
          {{ t('page.steps.walk.next') }}
        </RlButton>
        <RlButton v-else variant="secondary" size="sm" disabled data-testid="walk-next">
          {{ t('page.steps.walk.next') }}
        </RlButton>
      </nav>
      <!--
        Shown once the list has confirmed the row is one of its own, so a
        row from elsewhere never flashes up.
      -->
      <slot v-if="state === 'ready'" />
    </template>
  </div>
</template>

<style scoped>
.page-walk {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.page-walk__nav {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-sm) var(--space-lg);
}

.page-walk__position {
  color: var(--rl-color-text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
