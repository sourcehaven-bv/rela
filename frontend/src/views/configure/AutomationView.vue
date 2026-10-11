<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
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
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import LockedNote from '@/components/configure/LockedNote.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import {
  automations,
  automationTitle,
  entityTypeLabel,
  entityTypeNames,
  propertiesOf,
} from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { newMap, type TreeValue } from '@/configure/tree'

/**
 * An automation: one trigger, then actions in the order they run. One that
 * runs a script, or an action Configure does not edit, is shown read-only.
 */
const props = defineProps<{ index: number }>()
const draft = useConfigDraftStore()
const router = useRouter()

const auto = computed(() => automations(draft.currentSchema).find((a) => a.index === props.index))
const path = computed(() => ['automations', props.index])
const locked = computed(() => auto.value?.readOnly === true)

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
    { value: '', label: 'Any property' },
    ...props.map((p) => ({ value: p.name, label: p.label })),
  ]
})

function trigger(key: string, value: TreeValue | undefined) {
  draft.setAt('schema', [...path.value, 'on'], key, value)
}

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('schema', path.value, key, value)
}

const rows = computed(() =>
  (auto.value?.actions ?? []).map((a, i) => ({
    id: String(i),
    title: a.summary,
    index: i,
    locked: a.locked,
  }))
)

function move(row: { index: number }, to: number) {
  draft.moveAt('schema', [...path.value, 'do'], row.index, to)
}

function removeAction(index: number) {
  draft.removeAt('schema', [...path.value, 'do'], index)
}

const setProperty = ref('')
const setValue = ref('')

function addSet() {
  if (!setProperty.value) return
  draft.appendAt(
    'schema',
    path.value,
    'do',
    newMap({ set: setProperty.value, value: setValue.value })
  )
  setProperty.value = ''
  setValue.value = ''
}

function removeAutomation() {
  draft.removeAt('schema', ['automations'], props.index)
  void router.push(configureRoute.automations())
}
</script>

<template>
  <ConfigurePage
    :title="auto ? automationTitle(auto) : 'Automation'"
    back-label="Automations"
    :back-to="configureRoute.automations()"
  >
    <template v-if="auto" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeAutomation"
        >Remove</RlButton
      >
    </template>
    <RlEmptyState v-if="!auto" title="No such automation" />
    <div v-else class="automation">
      <LockedNote v-if="locked">
        This automation runs a script or an action that is changed in the project's files. It is
        shown here so you can see what it does.
      </LockedNote>
      <ConfigSection title="Automation">
        <div class="grid">
          <RlTextField
            :model-value="auto.name"
            label="Name"
            required
            :disabled="locked"
            @update:model-value="setting('name', $event)"
          />
          <RlTextField
            :model-value="auto.description"
            label="Description"
            :disabled="locked"
            @update:model-value="setting('description', $event)"
          />
        </div>
      </ConfigSection>
      <ConfigSection title="When">
        <div class="grid grid--3">
          <RlMultiSelect
            :model-value="auto.entityTypes"
            label="A record of type"
            :options="typeOptions"
            :disabled="locked"
            @update:model-value="trigger('entity', $event)"
          />
          <RlSelect
            :model-value="auto.property"
            label="Changes"
            :options="propertyOptions"
            :disabled="locked || auto.created"
            @update:model-value="trigger('property', $event || undefined)"
          />
          <RlTextField
            :model-value="auto.becomes"
            label="To"
            :disabled="locked || !auto.property"
            hint="Leave empty for any change."
            @update:model-value="trigger('becomes', $event)"
          />
        </div>
        <RlCheckbox
          :model-value="auto.created"
          label="When a record is created"
          :disabled="locked"
          @update:model-value="trigger('created', $event || undefined)"
        />
      </ConfigSection>
      <ConfigSection
        title="Then"
        :count="rows.length"
        description="The actions run in the order shown."
      >
        <RlSortableList
          :items="rows"
          label="Actions, in the order they run"
          :disabled="locked"
          @move="move"
        >
          <template #item="{ item, index: position }">
            <div class="action-row">
              <RlTag :label="String(position + 1)" />
              <RlText size="sm" weight="medium" class="action-row__title">{{ item.title }}</RlText>
              <RlTag v-if="item.locked" label="Read only" />
              <RlIconButton
                v-if="!locked"
                icon="delete"
                :label="`Remove ${item.title}`"
                @click="removeAction(item.index)"
              />
            </div>
          </template>
        </RlSortableList>
        <form v-if="!locked" class="add-action" @submit.prevent="addSet">
          <RlSelect
            v-model="setProperty"
            label="Set a property"
            :options="propertyOptions.slice(1)"
            placeholder="Choose a property"
          />
          <RlTextField v-model="setValue" label="To" hint="Text, or {{today}} for the date." />
          <RlButton variant="secondary" size="sm" icon="plus" type="submit" :disabled="!setProperty"
            >Add action</RlButton
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

.action-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.action-row__title {
  flex: 1;
}

.add-action {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  align-items: end;
  gap: var(--rl-space-3);
}
</style>
