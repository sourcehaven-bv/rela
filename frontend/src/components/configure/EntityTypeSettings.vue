<script setup lang="ts">
import { computed } from 'vue'
import RlHeading from 'rela-components/components/common/RlHeading.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlRadioGroup from 'rela-components/components/form/RlRadioGroup.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlOptionSelect from 'rela-components/components/form/RlOptionSelect.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeModel } from '@/configure/models'
import { ENTITY_COLORS, entityColorName } from '@/configure/names'
import { getIn, isMap, setOrRemove, type TreeValue } from '@/configure/tree'

/** The settings that describe an entity type itself, beside its properties. */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()

const model = computed(() => entityTypeModel(draft.currentSchema, props.name))

function setting(key: string, value: TreeValue | undefined) {
  draft.edit('schema', (tree) => {
    const entity = getIn(tree, ['entities', props.name])
    if (isMap(entity)) setOrRemove(entity, key, value)
  })
}

const idOptions = [
  {
    value: 'short',
    label: 'Short code',
    description: 'A prefix and six random characters: TKT-8UCV32',
  },
  { value: 'sequential', label: 'Counting up', description: 'A prefix and a number: INV-1, INV-2' },
  {
    value: 'manual',
    label: 'Chosen by hand',
    description: 'The author types the ID, such as a package name',
  },
]

const colorOptions = computed(() => {
  const names = ENTITY_COLORS.map((c) => c.name)
  return colorName.value === 'custom' ? [...names, 'custom'] : names
})
const colorName = computed(() =>
  entityColorName(model.value?.color ?? '', model.value?.borderColor ?? '')
)
const swatch = (name: string) =>
  ENTITY_COLORS.find((c) => c.name === name) ?? {
    color: model.value?.color,
    border: model.value?.borderColor,
  }

function setColor(name: string) {
  const preset = ENTITY_COLORS.find((c) => c.name === name)
  if (!preset) return
  draft.edit('schema', (tree) => {
    const entity = getIn(tree, ['entities', props.name])
    if (!isMap(entity)) return
    setOrRemove(entity, 'color', preset.color)
    setOrRemove(entity, 'border_color', preset.border)
  })
}

const titleOptions = computed(() => [
  { value: '', label: 'Title' },
  ...(model.value?.properties ?? []).map((p) => ({ value: p.name, label: p.label })),
])
</script>

<template>
  <aside v-if="model" class="settings" data-testid="config-entity-settings">
    <RlHeading :level="2" size="md">Settings</RlHeading>
    <RlTextField
      :model-value="model.label"
      label="Name"
      required
      @update:model-value="setting('label', $event)"
    />
    <RlTextField
      :model-value="model.plural === model.label ? '' : model.plural"
      label="Plural name"
      :placeholder="model.label"
      @update:model-value="setting('label_plural', $event)"
    />
    <RlRadioGroup
      :model-value="model.idType"
      label="IDs"
      :options="idOptions"
      @update:model-value="setting('id_type', $event === 'short' ? undefined : $event)"
    />
    <RlTextField
      v-if="model.idType !== 'manual'"
      :model-value="model.idPrefix"
      label="ID prefix"
      hint="Existing IDs keep their prefix."
      @update:model-value="setting('id_prefix', $event)"
    />
    <div>
      <RlText size="sm" weight="medium" as="div" class="settings__label">Colour</RlText>
      <RlOptionSelect
        :model-value="colorName"
        :options="colorOptions"
        label="Colour"
        placeholder="No colour"
        @update:model-value="setColor"
      >
        <template #option="{ value }">
          <span class="swatch">
            <span
              class="swatch__dot"
              :style="{ background: swatch(value).color, borderColor: swatch(value).border }"
            />
            {{ value === 'custom' ? 'Custom' : value }}
          </span>
        </template>
      </RlOptionSelect>
    </div>
    <RlSelect
      :model-value="model.displayProperty"
      label="Records are called by"
      :options="titleOptions"
      hint="The property shown as a record's title."
      @update:model-value="setting('display_property', $event)"
    />
    <RlTextarea
      :model-value="model.description"
      label="Description"
      :rows="3"
      @update:model-value="setting('description', $event)"
    />
  </aside>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.settings__label {
  margin-bottom: 6px;
}

.swatch {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.swatch__dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid;
}
</style>
