<script setup lang="ts">
/**
 * LLMProposalDialog — review what an `llm:` action proposes (TKT-4USH1HO).
 *
 * The action wrote nothing. The user checks the changes they accept, and the
 * dialog applies them through the ordinary write APIs, so the user's own ACL,
 * validation and audit apply. Before writing, it reads the entity again and
 * skips any property that changed since the proposal was made.
 *
 * Mount under `v-if`: the component's lifetime is the dialog's.
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalStack } from '@/composables/modalStack'
import { useSchemaStore, useUIStore } from '@/stores'
import { createRelation, getEntity, updateEntity } from '@/api/entities'
import { getErrorMessage } from '@/api/errors'
import type { ActionProposal } from '@/api/actions'
import { formatProposalValue, planApply, relationKey } from './llmProposal'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlText from 'rela-components/components/common/RlText.vue'

const props = defineProps<{
  proposal: ActionProposal
  /** Entity type of proposal.entity. */
  entityType: string
  /** Action label, for the title. */
  label: string
}>()

const emit = defineEmits<{
  close: []
  applied: []
}>()

const { t } = useI18n()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
useModalStack(computed(() => true))

// A free-text value starts unticked. The model read entity data that other
// people wrote, so such a value may carry text injected through it; the user
// opts in to it after reading it. Only values the type constrains to a
// closed or structured set (enums, numbers, booleans, dates) and candidate
// ids start ticked. Anything else starts unticked, including a custom type
// whose only check is a pattern: an email or URL is still text the model
// chose.
const CONSTRAINED_TYPES = new Set(['enum', 'integer', 'decimal', 'boolean', 'date', 'datetime'])
const selectedSet = ref(
  new Set(props.proposal.set.filter((c) => !isFreeText(c.property)).map((c) => c.property))
)
const selectedRelate = ref(new Set(props.proposal.relate.map(relationKey)))
// Items already written, or skipped as stale, by an earlier apply. They are
// unchecked and locked, so applying again never sends them twice.
const settledSet = ref(new Set<string>())
const settledRelate = ref(new Set<string>())
const busy = ref(false)
const failures = ref<string[]>([])

const empty = computed(() => props.proposal.set.length === 0 && props.proposal.relate.length === 0)
const nothingChecked = computed(
  () => selectedSet.value.size === 0 && selectedRelate.value.size === 0
)

function isFreeText(name: string): boolean {
  const def = schemaStore.getEntityType(props.entityType)?.properties?.[name]
  if (!def) return true
  return !def.values?.length && !CONSTRAINED_TYPES.has(def.type)
}

function propertyLabel(name: string): string {
  return schemaStore.getEntityType(props.entityType)?.properties?.[name]?.label || name
}

function relationLabel(name: string): string {
  return schemaStore.getRelationType(name)?.label || name
}

function toggle(set: Set<string>, key: string) {
  if (!set.delete(key)) set.add(key)
}

function settle(selected: Set<string>, settled: Set<string>, key: string) {
  selected.delete(key)
  settled.add(key)
}

async function apply() {
  if (busy.value) return
  busy.value = true
  failures.value = []
  try {
    const fresh = await getEntity(props.entityType, props.proposal.entity)
    const plan = planApply(
      props.proposal,
      selectedSet.value,
      selectedRelate.value,
      fresh.properties ?? {}
    )
    const failed: string[] = []
    for (const p of plan.stale) {
      failed.push(t('entity.llmProposal.stale', { property: propertyLabel(p) }))
      settle(selectedSet.value, settledSet.value, p)
    }
    if (Object.keys(plan.properties).length > 0) {
      try {
        await updateEntity(props.entityType, props.proposal.entity, { properties: plan.properties })
        for (const p of Object.keys(plan.properties)) settle(selectedSet.value, settledSet.value, p)
      } catch (err) {
        failed.push(getErrorMessage(err, t('entity.llmProposal.saveFailed')))
      }
    }
    for (const r of plan.relations) {
      try {
        await createRelation(props.entityType, props.proposal.entity, r.relation, r.to)
        settle(selectedRelate.value, settledRelate.value, relationKey(r))
      } catch (err) {
        failed.push(
          t('entity.llmProposal.linkFailed', {
            relation: relationLabel(r.relation),
            title: r.title,
            reason: getErrorMessage(err),
          })
        )
      }
    }
    if (failed.length > 0) {
      failures.value = failed
      emit('applied')
      return
    }
    uiStore.success(t('entity.llmProposal.applied', { label: props.label }))
    emit('applied')
    emit('close')
  } catch (err) {
    failures.value = [getErrorMessage(err, t('entity.llmProposal.saveFailed'))]
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <RlModal
    :open="true"
    :title="t('entity.llmProposal.title', { label })"
    size="lg"
    :layer="900"
    panel-class="llm-proposal"
    @close="emit('close')"
  >
    <RlText v-if="empty" as="p" tone="muted">{{ t('entity.llmProposal.empty') }}</RlText>
    <template v-else>
      <RlText as="p" tone="muted">{{ t('entity.llmProposal.intro') }}</RlText>

      <fieldset v-if="proposal.set.length" class="llm-proposal-group">
        <legend>{{ t('entity.llmProposal.properties') }}</legend>
        <div v-for="c in proposal.set" :key="c.property" class="llm-proposal-row">
          <RlCheckbox
            :model-value="selectedSet.has(c.property)"
            :disabled="settledSet.has(c.property)"
            :label="propertyLabel(c.property)"
            @update:model-value="toggle(selectedSet, c.property)"
          />
          <span class="llm-proposal-change">
            <del>{{ formatProposalValue(c.current) }}</del>
            <ins>{{ formatProposalValue(c.proposed) }}</ins>
          </span>
        </div>
      </fieldset>

      <fieldset v-if="proposal.relate.length" class="llm-proposal-group">
        <legend>{{ t('entity.llmProposal.relations') }}</legend>
        <RlCheckbox
          v-for="r in proposal.relate"
          :key="relationKey(r)"
          :model-value="selectedRelate.has(relationKey(r))"
          :disabled="settledRelate.has(relationKey(r))"
          :label="`${relationLabel(r.relation)}: ${r.to_title} (${r.to})`"
          @update:model-value="toggle(selectedRelate, relationKey(r))"
        />
      </fieldset>

      <ul v-if="failures.length" class="llm-proposal-failures" role="alert">
        <li v-for="(f, i) in failures" :key="i">{{ f }}</li>
      </ul>
      <RlText v-if="proposal.model" as="p" size="sm" tone="muted">
        {{ t('entity.llmProposal.model', { model: proposal.model }) }}
      </RlText>
    </template>

    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" @click="emit('close')">{{ t('common.cancel') }}</RlButton>
        <RlButton v-if="!empty" variant="primary" :disabled="busy || nothingChecked" @click="apply">
          {{ t('entity.llmProposal.apply') }}
        </RlButton>
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
.llm-proposal-group {
  border: none;
  margin: var(--space-md) 0;
  padding: 0;
}
.llm-proposal-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  align-items: baseline;
}
.llm-proposal-change {
  display: inline-flex;
  gap: var(--space-2xs);
}
.llm-proposal-change ins::before {
  content: '→ ';
  color: var(--rl-color-text-muted);
}
.llm-proposal-change del {
  color: var(--rl-color-text-muted);
}
.llm-proposal-failures {
  color: var(--rl-color-danger);
}
</style>
