<script setup lang="ts">
import { computed } from 'vue'
import RlDrawer from 'rela-components/components/overlay/RlDrawer.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlMultiSelect from 'rela-components/components/form/RlMultiSelect.vue'
import RlSegmentedControl from 'rela-components/components/data/RlSegmentedControl.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import ConfigSection from './ConfigSection.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { choiceListModel, formModel, propertiesOf } from '@/configure/models'
import { newMap, type TreeValue } from '@/configure/tree'

/**
 * One field of a form: how it is labelled, how wide it is, and, for a
 * choice-list field, which changes of value someone may make on this form.
 */
const props = defineProps<{ form: string; index: number | undefined }>()
const emit = defineEmits<{ close: [] }>()

const draft = useConfigDraftStore()

const model = computed(() => formModel(draft.currentDataEntry, props.form))
const field = computed(() =>
  props.index === undefined ? undefined : model.value?.fields[props.index]
)
const path = computed(() => ['forms', props.form, 'fields', props.index ?? -1])

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('screens', path.value, key, value)
}

const widths = [
  { value: '4', label: 'Third' },
  { value: '6', label: 'Half' },
  { value: '12', label: 'Full' },
]
const width = computed(() => String(field.value?.span ?? 12))

/** The options of the field's property, when it takes them from a choice list. */
const options = computed(() => {
  const m = model.value
  const f = field.value
  if (!m || !f) return []
  const p = propertiesOf(draft.currentSchema, m.entityType).find((x) => x.name === f.property)
  if (!p) return []
  if (p.choiceList)
    return (
      choiceListModel(draft.currentSchema, undefined, p.choiceList)?.options.map((o) => o.value) ??
      []
    )
  return p.inlineValues
})

function setTransition(from: string, to: string[]) {
  const current = { ...(field.value?.transitions ?? {}) }
  if (to.length) current[from] = to
  else delete current[from]
  const entries = Object.entries(current)
  setting('transitions', entries.length ? newMap(Object.fromEntries(entries)) : undefined)
}

function removeField() {
  if (props.index === undefined) return
  draft.removeAt('screens', ['forms', props.form, 'fields'], props.index)
  emit('close')
}
</script>

<template>
  <RlDrawer
    :title="`Field: ${field?.label || field?.property || ''}`"
    size="md"
    :open="field !== undefined"
    @close="emit('close')"
  >
    <div v-if="field" class="drawer-form" data-testid="config-field-drawer">
      <RlTextField
        :model-value="field.label"
        label="Label"
        :placeholder="field.property"
        hint="Leave empty to use the property's name."
        @update:model-value="setting('label', $event)"
      />
      <RlTextField
        :model-value="field.help"
        label="Help text"
        @update:model-value="setting('help', $event)"
      />
      <RlTextField
        :model-value="field.placeholder"
        label="Placeholder"
        @update:model-value="setting('placeholder', $event)"
      />
      <div>
        <RlText size="sm" weight="medium" as="div" class="drawer-form__label">Width</RlText>
        <RlSegmentedControl
          :model-value="width"
          label="Width"
          :options="widths"
          @update:model-value="setting('span', $event === '12' ? undefined : Number($event))"
        />
      </div>
      <RlCheckbox
        :model-value="field.hidden"
        label="Hidden"
        hint="The value is kept and set by defaults or automations."
        @update:model-value="setting('hidden', $event)"
      />
      <ConfigSection
        v-if="options.length"
        title="Allowed changes"
        description="From each value, which values someone may change it to on this form. Leave all empty to allow any change."
      >
        <RlMultiSelect
          v-for="from in options"
          :key="from"
          :model-value="field.transitions[from] ?? []"
          :label="`From ${from}`"
          :options="options.filter((o) => o !== from).map((o) => ({ value: o, label: o }))"
          @update:model-value="setTransition(from, $event)"
        />
      </ConfigSection>
    </div>
    <template #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeField"
        >Remove field</RlButton
      >
      <RlButton variant="primary" @click="emit('close')">Done</RlButton>
    </template>
  </RlDrawer>
</template>

<style scoped>
.drawer-form {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.drawer-form__label {
  margin-bottom: 6px;
}
</style>
