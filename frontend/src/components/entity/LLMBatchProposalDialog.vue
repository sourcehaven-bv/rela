<script setup lang="ts">
/**
 * LLMBatchProposalDialog — review what a batch run of an `llm:` action
 * proposes for several entities (TKT-I2SRNWR).
 *
 * The run wrote nothing. Every change starts unticked: all the entities went
 * to the model in one request, so text in one of them can steer what it
 * proposed for another. The user ticks single changes, or a whole entity,
 * and the dialog applies them per entity through the ordinary write API,
 * skipping a property that changed since the proposal (planApply).
 *
 * Mount under `v-if`: the component's lifetime is the dialog's.
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalStack } from '@/composables/modalStack'
import { useSchemaStore, useUIStore } from '@/stores'
import { getEntity, updateEntity } from '@/api/entities'
import { getErrorMessage } from '@/api/errors'
import type { ActionBatch, ActionProposal } from '@/api/actions'
import { formatProposalValue, planApply } from './llmProposal'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlText from 'rela-components/components/common/RlText.vue'

const props = defineProps<{
  batch: ActionBatch
  /** Entity type of every proposal's entity. */
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

/** Selection key of one property change of one entity. */
function key(entity: string, property: string): string {
  return `${entity}\u0000${property}`
}

const selected = ref(new Set<string>())
// Changes already written, or skipped as stale, by an earlier apply.
const settled = ref(new Set<string>())
const busy = ref(false)
const failures = ref<string[]>([])

const proposals = computed(() => props.batch.proposals.filter((p) => p.set.length > 0))
const unchanged = computed(() => props.batch.proposals.length - proposals.value.length)
const nothingChecked = computed(() => selected.value.size === 0)

function propertyLabel(name: string): string {
  return schemaStore.getEntityType(props.entityType)?.properties?.[name]?.label || name
}

function heading(p: { entity: string; title?: string }): string {
  return p.title && p.title !== p.entity ? `${p.title} (${p.entity})` : p.entity
}

function open(p: ActionProposal): string[] {
  return p.set.map((c) => key(p.entity, c.property)).filter((k) => !settled.value.has(k))
}

function rowChecked(p: ActionProposal): boolean {
  const keys = open(p)
  return keys.length > 0 && keys.every((k) => selected.value.has(k))
}

function toggleRow(p: ActionProposal) {
  const on = !rowChecked(p)
  for (const k of open(p)) {
    if (on) selected.value.add(k)
    else selected.value.delete(k)
  }
}

function toggle(k: string) {
  if (!selected.value.delete(k)) selected.value.add(k)
}

function settle(k: string) {
  selected.value.delete(k)
  settled.value.add(k)
}

async function applyOne(p: ActionProposal, failed: string[]) {
  const chosen = new Set(
    p.set.filter((c) => selected.value.has(key(p.entity, c.property))).map((c) => c.property)
  )
  if (chosen.size === 0) return
  const fresh = await getEntity(props.entityType, p.entity)
  const plan = planApply(p, chosen, new Set(), fresh.properties ?? {})
  for (const prop of plan.stale) {
    failed.push(t('entity.llmBatch.stale', { entity: heading(p), property: propertyLabel(prop) }))
    settle(key(p.entity, prop))
  }
  if (Object.keys(plan.properties).length === 0) return
  await updateEntity(props.entityType, p.entity, { properties: plan.properties })
  for (const prop of Object.keys(plan.properties)) settle(key(p.entity, prop))
}

async function apply() {
  if (busy.value) return
  busy.value = true
  failures.value = []
  const failed: string[] = []
  try {
    for (const p of proposals.value) {
      try {
        await applyOne(p, failed)
      } catch (err) {
        failed.push(
          t('entity.llmBatch.saveFailed', {
            entity: heading(p),
            reason: getErrorMessage(err),
          })
        )
      }
    }
    emit('applied')
    if (failed.length > 0) {
      failures.value = failed
      return
    }
    uiStore.success(t('entity.llmProposal.applied', { label: props.label }))
    emit('close')
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
    panel-class="llm-batch"
    @close="emit('close')"
  >
    <RlText as="p" tone="muted">{{ t('entity.llmBatch.intro') }}</RlText>
    <RlText v-if="proposals.length === 0" as="p" tone="muted">
      {{ t('entity.llmProposal.empty') }}
    </RlText>

    <fieldset v-for="p in proposals" :key="p.entity" class="llm-batch-entity">
      <legend>
        <RlCheckbox
          :model-value="rowChecked(p)"
          :disabled="open(p).length === 0"
          :label="heading(p)"
          @update:model-value="toggleRow(p)"
        />
      </legend>
      <div v-for="c in p.set" :key="c.property" class="llm-batch-row">
        <RlCheckbox
          :model-value="selected.has(key(p.entity, c.property))"
          :disabled="settled.has(key(p.entity, c.property))"
          :label="propertyLabel(c.property)"
          @update:model-value="toggle(key(p.entity, c.property))"
        />
        <span class="llm-batch-change">
          <del>{{ formatProposalValue(c.current) }}</del>
          <ins>{{ formatProposalValue(c.proposed) }}</ins>
        </span>
      </div>
    </fieldset>

    <RlText v-if="unchanged > 0" as="p" size="sm" tone="muted">
      {{ t('entity.llmBatch.unchanged', { count: unchanged }, unchanged) }}
    </RlText>
    <RlText v-if="batch.skipped > 0" as="p" size="sm" tone="muted">
      {{ t('entity.llmBatch.skipped', { count: batch.skipped }, batch.skipped) }}
    </RlText>
    <ul v-if="batch.failed.length" class="llm-batch-notes">
      <li v-for="f in batch.failed" :key="f.entity">
        {{ t('entity.llmBatch.failed', { entity: heading(f), reason: f.reason }) }}
      </li>
    </ul>
    <ul v-if="failures.length" class="llm-batch-failures" role="alert">
      <li v-for="(f, i) in failures" :key="i">{{ f }}</li>
    </ul>
    <RlText v-if="proposals[0]?.model" as="p" size="sm" tone="muted">
      {{ t('entity.llmProposal.model', { model: proposals[0].model }) }}
    </RlText>

    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" @click="emit('close')">{{ t('common.cancel') }}</RlButton>
        <RlButton
          v-if="proposals.length"
          variant="primary"
          :disabled="busy || nothingChecked"
          @click="apply"
        >
          {{ t('entity.llmProposal.apply') }}
        </RlButton>
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
.llm-batch-entity {
  border: none;
  border-top: 1px solid var(--rl-color-border);
  margin: var(--space-md) 0 0;
  padding: var(--space-sm) 0 0;
}
.llm-batch-entity legend {
  font-weight: 600;
}
.llm-batch-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  align-items: baseline;
  padding-left: var(--space-lg);
}
.llm-batch-change {
  display: inline-flex;
  gap: var(--space-2xs);
}
.llm-batch-change ins::before {
  content: '→ ';
  color: var(--rl-color-text-muted);
}
.llm-batch-change del {
  color: var(--rl-color-text-muted);
}
.llm-batch-notes {
  color: var(--rl-color-text-muted);
}
.llm-batch-failures {
  color: var(--rl-color-danger);
}
</style>
