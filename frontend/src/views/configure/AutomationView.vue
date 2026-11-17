<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlMultiSelect from 'rela-components/components/form/RlMultiSelect.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import ConfigurePage from './ConfigurePage.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import LockedNote from '@/components/configure/LockedNote.vue'
import { useConfigDraftStore } from '@/configure/configDraft'
import {
  automations,
  automationTitle,
  entityTypeLabel,
  entityTypeNames,
  propertiesOf,
} from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import type { TreeValue } from '@/api/configure'
import { mapAt, newMap, remove } from '@/configure/tree'

/**
 * An automation: one flows.yaml trigger, then outputs in the order they run.
 * One that runs a script, or holds a setting Configure does not edit, is
 * shown read-only.
 */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()
const router = useRouter()
const { t } = useI18n()

const auto = computed(() => automations(draft.currentFlows).find((a) => a.name === props.name))
const path = computed(() => ['triggers', props.name])
const locked = computed(() => auto.value?.readOnly === true)
/** A trigger with a script is removed in flows.yaml; the save refuses it here. */
const holdsScript = computed(() => auto.value?.outputs.some((o) => o.kind === 'script') === true)

const typeOptions = computed(() =>
  entityTypeNames(draft.currentSchema).map((t) => ({
    value: t,
    label: entityTypeLabel(draft.currentSchema, t),
  }))
)
const propertyOptions = computed(() => {
  const type = auto.value?.entityTypes[0]
  const props = type ? propertiesOf(draft.currentSchema, type) : []
  return [
    { value: '', label: t('configureAutomations.anyProperty') },
    ...props.map((p) => ({ value: p.name, label: p.label })),
  ]
})

function trigger(key: string, value: TreeValue | undefined) {
  draft.setAt('flows', [...path.value, 'on'], key, value)
}

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('flows', path.value, key, value)
}

const rows = computed(() =>
  (auto.value?.outputs ?? []).map((o, i) => ({
    id: String(i),
    title: o.summary,
    index: i,
    locked: o.locked,
  }))
)

function move(row: { index: number }, to: number) {
  draft.moveAt('flows', [...path.value, 'outputs'], row.index, to)
}

function removeOutput(index: number) {
  draft.removeAt('flows', [...path.value, 'outputs'], index)
}

const setProperty = ref('')
const setValue = ref('')

function addSet() {
  if (!setProperty.value) return
  draft.appendAt(
    'flows',
    path.value,
    'outputs',
    newMap({ set: newMap({ properties: newMap({ [setProperty.value]: setValue.value }) }) })
  )
  setProperty.value = ''
  setValue.value = ''
}

function removeAutomation() {
  draft.edit('flows', (tree) => {
    const triggers = mapAt(tree, ['triggers'])
    if (triggers) remove(triggers, props.name)
  })
  void router.push(configureRoute.automations())
}
</script>

<template>
  <ConfigurePage
    :title="auto ? automationTitle(auto) : t('configureAutomations.one')"
    :back-label="t('configureAutomations.title')"
    :back-to="configureRoute.automations()"
  >
    <template v-if="auto && !holdsScript" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeAutomation">{{
        t('configureAutomations.remove')
      }}</RlButton>
    </template>
    <RlEmptyState v-if="!auto" :title="t('configureAutomations.notFound')" />
    <div v-else class="automation">
      <LockedNote v-if="locked">{{ t('configureAutomations.locked') }}</LockedNote>
      <ConfigSection :title="t('configureAutomations.one')">
        <div class="grid">
          <RlTextField :model-value="auto.name" :label="t('configureAutomations.name')" disabled />
          <RlTextField
            :model-value="auto.description"
            :label="t('configureAutomations.description')"
            :disabled="locked"
            @update:model-value="setting('description', $event || undefined)"
          />
        </div>
      </ConfigSection>
      <ConfigSection :title="t('configureAutomations.when')">
        <div class="grid grid--3">
          <RlMultiSelect
            :model-value="auto.entityTypes"
            :label="t('configureAutomations.recordOfType')"
            :options="typeOptions"
            :disabled="locked"
            @update:model-value="trigger('entity', $event)"
          />
          <RlSelect
            :model-value="auto.property"
            :label="t('configureAutomations.changes')"
            :options="propertyOptions"
            :disabled="locked || auto.created"
            @update:model-value="trigger('property', $event || undefined)"
          />
          <RlTextField
            :model-value="auto.becomes"
            :label="t('configureAutomations.to')"
            :disabled="locked || !auto.property"
            :hint="t('configureAutomations.anyChange')"
            @update:model-value="trigger('becomes', $event || undefined)"
          />
        </div>
        <RlCheckbox
          :model-value="auto.created"
          :label="t('configureAutomations.onCreated')"
          :disabled="locked"
          @update:model-value="trigger('created', $event || undefined)"
        />
        <RlCheckbox
          :model-value="auto.updated"
          :label="t('configureAutomations.onUpdated')"
          :disabled="locked"
          @update:model-value="trigger('updated', $event || undefined)"
        />
      </ConfigSection>
      <ConfigSection
        :title="t('configureAutomations.then')"
        :count="rows.length"
        :description="t('configureAutomations.thenHelp')"
      >
        <RlSortableList
          :items="rows"
          :label="t('configureAutomations.outputs')"
          :disabled="locked"
          @move="move"
        >
          <template #item="{ item, index: position }">
            <div class="output-row">
              <RlTag :label="String(position + 1)" />
              <RlText size="sm" weight="medium" class="output-row__title">{{ item.title }}</RlText>
              <RlTag v-if="item.locked" :label="t('configureAutomations.readOnly')" />
              <RlIconButton
                v-if="!locked"
                icon="delete"
                :label="t('configureAutomations.removeOutput', { name: item.title })"
                @click="removeOutput(item.index)"
              />
            </div>
          </template>
        </RlSortableList>
        <form v-if="!locked" class="add-output" @submit.prevent="addSet">
          <RlSelect
            v-model="setProperty"
            :label="t('configureAutomations.setProperty')"
            :options="propertyOptions.slice(1)"
            :placeholder="t('configureAutomations.chooseProperty')"
          />
          <RlTextField
            v-model="setValue"
            :label="t('configureAutomations.to')"
            :hint="t('configureAutomations.setHint', { placeholder: '{{today}}' })"
          />
          <RlButton
            variant="secondary"
            size="sm"
            icon="plus"
            type="submit"
            :disabled="!setProperty"
            >{{ t('configureAutomations.addOutput') }}</RlButton
          >
        </form>
      </ConfigSection>
    </div>
  </ConfigurePage>
</template>

<style scoped>
.automation {
  max-width: 760px;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-8);
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--rl-space-4);
}

.grid--3 {
  grid-template-columns: repeat(3, 1fr);
}

.output-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.output-row__title {
  flex: 1;
}

.add-output {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  align-items: end;
  gap: var(--rl-space-3);
}
</style>
