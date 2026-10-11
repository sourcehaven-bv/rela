<script setup lang="ts">
import { computed, watch } from 'vue'
import { RouterLink } from 'vue-router'
import RlDrawer from 'rela-components/components/overlay/RlDrawer.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlChangeList from 'rela-components/components/data/RlChangeList.vue'
import RlChangeItem from 'rela-components/components/data/RlChangeItem.vue'
import RlHeading from 'rela-components/components/common/RlHeading.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { useConfirm } from '@/composables/useConfirm'
import { problemRoute } from '@/configure/paths'
import { describeStep, mappingOptions, mappingQuestion, useReview } from '@/configure/useReview'
import { useDiscardDraft } from '@/configure/useDiscardDraft'
import type { ConfigProblem } from '@/api/configure'

/**
 * The review step (TKT-F5NGMG): every unsaved change in the reader's terms,
 * what the server found wrong with the draft, the data migration saving
 * writes, and Save.
 *
 * Opening it asks the server to check the draft. A removed option still in
 * use comes back as a question (where do those records go?), answered here;
 * each answer checks the draft again. Save is offered only once the check
 * finds nothing left to resolve.
 */
const draft = useConfigDraftStore()
const review = useReview()
const { confirm } = useConfirm()
const discardDraft = useDiscardDraft()

// Check on opening, and again whenever the draft changes while open: an
// answered question, or an edit made in another tab.
watch(
  () => [draft.reviewOpen, draft.revision] as const,
  ([open]) => {
    if (open && !review.incomplete.value) void review.check()
  }
)

function close() {
  draft.reviewOpen = false
}

function chooseMapping(problem: ConfigProblem, to: string) {
  if (!problem.entity_type || !problem.property || !problem.value) return
  draft.setValueMapping({
    entity_type: problem.entity_type,
    property: problem.property,
    from: problem.value,
    to,
  })
}

/** The answer already chosen for a removed value, kept while the check reruns. */
function chosen(problem: ConfigProblem): string {
  if (!problem.entity_type || !problem.property || !problem.value) return ''
  return draft.mappingFor(problem.entity_type, problem.property, problem.value)?.to ?? ''
}

async function discard() {
  const ok = await confirm({
    title: 'Discard all changes?',
    message: 'Every unsaved change on every Configure screen is dropped.',
    confirmLabel: 'Discard',
    danger: true,
  })
  if (!ok) return
  await discardDraft()
}

const steps = computed(() =>
  (review.migration.value?.steps ?? []).map((s) => describeStep(draft.currentSchema, s))
)
const saveLabel = computed(() => {
  const n = review.records.value
  if (!steps.value.length) return 'Save'
  return n === 1
    ? 'Save and migrate 1 record'
    : `Save and migrate ${n.toLocaleString('en')} records`
})
const blockedReason = computed(() => {
  if (draft.stale || review.conflict.value) return 'The configuration changed since you started.'
  if (review.checking.value || !review.checked.value) return 'Checking the draft…'
  if (review.problems.value.length) return 'Resolve the problems above first.'
  return ''
})
</script>

<template>
  <RlDrawer title="Review changes" size="lg" :open="draft.reviewOpen" @close="close">
    <div class="review" data-testid="config-review-drawer">
      <RlCallout v-if="review.incomplete.value" tone="warning" title="The migration did not finish">
        The new configuration is saved, but some records were not updated yet. Finish the migration
        to update them.
      </RlCallout>

      <RlCallout v-if="review.failure.value" tone="danger" data-testid="config-review-failure">
        {{ review.failure.value }}
      </RlCallout>

      <RlText v-if="!draft.changes.length && !review.incomplete.value" tone="muted">
        There are no unsaved changes.
      </RlText>

      <RlChangeList
        v-for="group in draft.changes"
        :key="group.key"
        :title="group.title"
        :count="group.items.length"
      >
        <RlChangeItem
          v-for="(item, i) in group.items"
          :key="i"
          :kind="item.kind"
          :label="item.label"
          :detail="item.detail"
          :before="item.before"
          :after="item.after"
          :as="RouterLink"
          :attrs="{ to: item.to, onClick: close }"
        />
      </RlChangeList>

      <section
        v-if="review.valueProblems.value.length || review.otherProblems.value.length"
        class="review__section"
        aria-labelledby="review-problems"
      >
        <RlHeading id="review-problems" :level="3" size="md">To resolve</RlHeading>
        <RlSelect
          v-for="p in review.valueProblems.value"
          :key="`${p.entity_type}.${p.property}.${p.value}`"
          :model-value="chosen(p)"
          :label="mappingQuestion(draft.currentSchema, p)"
          :options="mappingOptions(draft.currentSchema, p)"
          placeholder="Choose a value"
          data-testid="config-value-mapping"
          @update:model-value="chooseMapping(p, $event)"
        />
        <RlCallout
          v-for="(p, i) in review.otherProblems.value"
          :key="i"
          tone="danger"
          data-testid="config-review-problem"
        >
          {{ p.message }}
          <RouterLink :to="problemRoute(p)" class="review__go" @click="close"
            >Go to the screen</RouterLink
          >
        </RlCallout>
      </section>

      <section v-if="steps.length" class="review__section" aria-labelledby="review-migration">
        <div class="review__intro">
          <RlHeading id="review-migration" :level="3" size="md">Data migration</RlHeading>
          <RlText size="sm" tone="muted" as="p" class="review__para">
            Some changes do not fit the records as they are. Saving also updates those records once
            and keeps a record of it in the migration history.
          </RlText>
        </div>
        <RlChangeList
          title="Steps"
          :level="4"
          :count="steps.length"
          data-testid="config-migration-steps"
        >
          <RlChangeItem
            v-for="(s, i) in steps"
            :key="i"
            :kind="s.kind"
            :label="s.label"
            :detail="s.detail"
            :before="s.before"
            :after="s.after"
          />
        </RlChangeList>
        <RlTextField
          :model-value="draft.migrationTitle"
          label="Name in the migration history"
          :placeholder="review.migration.value?.title"
          hint="Says why the records changed, for whoever reads the history later."
          @update:model-value="draft.setMigrationTitle($event)"
        />
      </section>
    </div>

    <template #actions>
      <RlText
        v-if="blockedReason && !review.incomplete.value"
        size="sm"
        tone="muted"
        class="review__reason"
      >
        {{ blockedReason }}
      </RlText>
      <RlButtonGroup>
        <template v-if="review.incomplete.value">
          <RlButton
            variant="primary"
            :loading="review.saving.value"
            pending-label="Finishing…"
            @click="review.finishMigration"
          >
            Finish migration
          </RlButton>
        </template>
        <template v-else>
          <RlButton v-if="draft.hasDraft" variant="ghost" tone="danger" @click="discard"
            >Discard all</RlButton
          >
          <RlButton
            variant="primary"
            :disabled="!review.canSave.value"
            :loading="review.saving.value"
            pending-label="Saving…"
            data-testid="config-save"
            @click="review.save"
          >
            {{ saveLabel }}
          </RlButton>
        </template>
      </RlButtonGroup>
    </template>
  </RlDrawer>
</template>

<style scoped>
.review {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-6);
}

.review__section {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
  padding-top: var(--rl-space-6);
  border-top: 1px solid var(--rl-color-border);
}

.review__intro {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.review__para {
  margin: 0;
}

.review__go {
  display: inline-block;
  margin-left: var(--rl-space-2);
}

.review__reason {
  margin-right: auto;
}
</style>
