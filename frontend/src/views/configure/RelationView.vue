<script setup lang="ts">
import { computed } from 'vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlMultiSelect from 'rela-components/components/form/RlMultiSelect.vue'
import RlNumberField from 'rela-components/components/form/RlNumberField.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import {
  entityTypeLabel,
  entityTypeNames,
  relationModel,
  relationSentence,
} from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { isEditable } from '@/configure/paths'
import { get, getIn, isMap, newMap, set, setOrRemove, type TreeValue } from '@/configure/tree'

/**
 * A relation, written as the sentence it produces in both directions. The
 * sentence is what an operator checks; the fields are how they change it.
 */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()

const model = computed(() => relationModel(draft.currentSchema, props.name))
const path = computed(() => ['relations', props.name])
const typeOptions = computed(() =>
  entityTypeNames(draft.currentSchema).map((t) => ({
    value: t,
    label: entityTypeLabel(draft.currentSchema, t),
  }))
)

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('schema', path.value, key, value)
}

/** The inverse is a name, or a mapping with an id and a label; keep its shape. */
function setInverse(value: string) {
  draft.edit('schema', (tree) => {
    const def = getIn(tree, path.value)
    if (!isMap(def)) return
    const inverse = get(def, 'inverse')
    if (isMap(inverse)) setOrRemove(inverse, 'label', value)
    else if (value) {
      // A plain inverse is its id; keep it, and add the label beside it.
      const id = typeof inverse === 'string' && inverse ? inverse : value.replace(/\s+/g, '_')
      set(def, 'inverse', id === value ? value : newMap({ id, label: value }))
    } else setOrRemove(def, 'inverse', undefined)
  })
}

/**
 * Whether the inverse can change in its current form. An inverse written as a
 * plain name may be read-only when the server allows only `id` and `label`.
 */
const inverseEditable = computed(() => {
  const inverse = get(getIn(draft.base?.schema, path.value), 'inverse')
  if (inverse === undefined || isMap(inverse)) return true
  return isEditable(draft.editable, 'schema.yaml', [...path.value, 'inverse'])
})

const required = computed(() => (model.value?.minOutgoing ?? 0) > 0)
const fromLabel = computed(
  () =>
    (model.value?.from ?? []).map((t) => entityTypeLabel(draft.currentSchema, t)).join(' or ') ||
    'record'
)
</script>

<template>
  <ConfigurePage
    :title="model?.label ?? name"
    back-label="Relations"
    :back-to="configureRoute.relations()"
  >
    <RlEmptyState v-if="!model" title="No such relation" />
    <ConfigSplit v-else>
      <ConfigSection title="Relation">
        <ConfigPreview subject="Reads as">
          <span>{{ relationSentence(draft.currentSchema, model) }}.</span>
          <span>{{ relationSentence(draft.currentSchema, model, true) }}.</span>
        </ConfigPreview>
        <div class="grid">
          <RlMultiSelect
            :model-value="model.from"
            label="From"
            :options="typeOptions"
            required
            @update:model-value="setting('from', $event)"
          />
          <RlMultiSelect
            :model-value="model.to"
            label="To"
            :options="typeOptions"
            required
            @update:model-value="setting('to', $event)"
          />
          <RlTextField
            :model-value="model.label"
            label="Name"
            required
            @update:model-value="setting('label', $event)"
          />
          <RlTextField
            :model-value="model.inverse"
            label="Name the other way round"
            :hint="
              inverseEditable
                ? 'Shown on the target\'s page.'
                : 'This name is changed in the project\'s files.'
            "
            :disabled="!inverseEditable"
            @update:model-value="setInverse"
          />
        </div>
        <RlCheckbox
          :model-value="required"
          :label="`Every ${fromLabel} needs one`"
          @update:model-value="setting('min_outgoing', $event ? 1 : undefined)"
        />
        <RlNumberField
          v-if="required"
          :model-value="model.minOutgoing"
          label="At least"
          :min="1"
          class="narrow"
          @update:model-value="setting('min_outgoing', $event)"
        />
        <RlNumberField
          :model-value="model.maxOutgoing"
          label="At most"
          :min="1"
          hint="Leave empty for no limit."
          class="narrow"
          @update:model-value="setting('max_outgoing', $event)"
        />
        <RlTextarea
          :model-value="model.description"
          label="Description"
          :rows="2"
          @update:model-value="setting('description', $event)"
        />
      </ConfigSection>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--rl-space-4);
}

.narrow {
  max-width: 200px;
}
</style>
