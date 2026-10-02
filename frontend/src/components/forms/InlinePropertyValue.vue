<script setup lang="ts">
/**
 * One property value on the detail view, changeable where it sits.
 *
 * The widget registry still decides how a value looks and which control edits
 * it. This component only decides how the two meet, following the library's
 * detail-field rule:
 *
 * - A choice from a fixed list (an enum, an enum list, a state-machine
 *   status) is a live control the whole time, drawn bare until hovered.
 *   Picking is the edit.
 * - A toggle or a file control is live too: it has no read form worth
 *   swapping away from.
 * - Everything else reads as its display widget and swaps to the edit widget
 *   on click. The edit is held as a draft, kept on Enter or leaving, and
 *   dropped on Escape; only a kept edit reaches `update`.
 *
 * `writable` false renders the display widget with no affordance at all.
 */
import { computed, ref, toRaw, type Component } from 'vue'
import type { AttachmentInfo, PropertyDef, TransitionOption } from '@/types'
import RlInlineEdit from 'rela-components/components/common/RlInlineEdit.vue'
import RlOptionSelect from 'rela-components/components/form/RlOptionSelect.vue'
import RlMultiSelect from 'rela-components/components/form/RlMultiSelect.vue'
import Badge from '@/components/common/Badge.vue'
import SelectWidget from '@/widgets/SelectWidget.vue'
import MultiSelectWidget from '@/widgets/MultiSelectWidget.vue'
import CheckboxWidget from '@/widgets/CheckboxWidget.vue'
import FileWidget from '@/widgets/FileWidget.vue'
import TextareaWidget from '@/widgets/TextareaWidget.vue'
import DateWidget from '@/widgets/DateWidget.vue'
import DatetimeWidget from '@/widgets/DatetimeWidget.vue'
import StatusControl from './StatusControl.vue'
import { useSchemaStore } from '@/stores/schema'

const props = defineProps<{
  property: string
  label: string
  widget: Component
  value: unknown
  writable: boolean
  propertyDef?: PropertyDef
  /** Present (even empty) for a state-machine field; see SectionEditField. */
  transitions?: TransitionOption[]
  optionVerdicts?: Record<string, boolean>
  attachments?: AttachmentInfo[]
  entityType: string
  entityId: string
  error?: string
}>()

const emit = defineEmits<{
  update: [value: unknown]
  'attachment-changed': []
}>()

type Treatment = 'status' | 'pick' | 'multi' | 'live' | 'swap'

// Compared by identity, so a widget handed over inside reactive state (a proxy
// of the component) must be unwrapped first.
const widget = computed(() => toRaw(props.widget))

const treatment = computed<Treatment>(() => {
  if (props.transitions !== undefined) return 'status'
  const hasValues = (props.propertyDef?.values?.length ?? 0) > 0
  if (widget.value === SelectWidget && hasValues) return 'pick'
  // A list with no fixed values takes free text, so it stays a swap.
  if (widget.value === MultiSelectWidget && hasValues) return 'multi'
  if (widget.value === CheckboxWidget || widget.value === FileWidget) return 'live'
  return 'swap'
})

const fieldId = computed(() => `inline-${props.property}`)

// ─── Pick: an enum drawn as its badge ────────────────────────────────────

// '' stands for "no value" and is offered only when the property may be empty.
const pickOptions = computed(() => {
  const values = props.propertyDef?.values ?? []
  return props.propertyDef?.required ? values : ['', ...values]
})

function isOptionDisabled(option: string) {
  return option !== '' && props.optionVerdicts?.[option] === false
}

function pick(option: string) {
  if (option === stringValue.value) return
  emit('update', option === '' ? undefined : option)
}

const stringValue = computed(() => (props.value == null ? '' : String(props.value)))

// ─── Multi: an enum list, each value drawn as its badge ──────────────────

const schemaStore = useSchemaStore()

const multiOptions = computed(() => {
  const labels = schemaStore.resolveOptionLabels(
    props.propertyDef,
    props.property,
    props.entityType
  )
  return (props.propertyDef?.values ?? []).map((v) => ({ value: v, label: labels[v] ?? v }))
})

const arrayValue = computed(() => {
  if (Array.isArray(props.value)) return props.value.map(String)
  return props.value ? [String(props.value)] : []
})

// Each toggle saves; the auto-save debounce batches a quick run of them.
function pickMany(values: string[]) {
  emit('update', values)
}

// ─── Swap: display widget, then the edit widget on a draft ────────────────

const draft = ref<unknown>(undefined)

const isEmpty = computed(() => {
  const v = props.value
  return v == null || v === '' || (Array.isArray(v) && v.length === 0)
})

// Prose takes the column rather than hugging its text, so the box does not
// resize between reading and editing.
const isBlock = computed(() => widget.value === TextareaWidget)

const opensPicker = computed(() => widget.value === DateWidget || widget.value === DatetimeWidget)

function commitDraft() {
  if (JSON.stringify(draft.value ?? null) === JSON.stringify(props.value ?? null)) return
  emit('update', draft.value)
}

const controlRef = ref<HTMLElement | null>(null)

/**
 * The tag picker renders its dropdown at the end of the body, so a press in it
 * would otherwise read as leaving the edit. Only this field's own dropdown
 * counts: slim-select gives the picker and its dropdown the same `data-id`.
 */
function floatingPanels(): Element[] {
  const pickers = controlRef.value?.querySelectorAll<HTMLElement>('.ss-main[data-id]') ?? []
  return Array.from(pickers).flatMap((picker) =>
    Array.from(
      document.querySelectorAll(`.ss-content[data-id="${CSS.escape(picker.dataset.id ?? '')}"]`)
    )
  )
}
</script>

<template>
  <div class="inline-property-value">
    <template v-if="!writable">
      <component
        :is="widget"
        mode="display"
        :model-value="value"
        :property-name="property"
        :property-def="propertyDef"
        :attachments="attachments"
        :max="propertyDef?.max"
      />
    </template>

    <StatusControl
      v-else-if="treatment === 'status'"
      :model-value="stringValue"
      :property="property"
      :entity-type="entityType"
      :transitions="transitions ?? []"
      @update:model-value="(v: string) => emit('update', v)"
    />

    <RlOptionSelect
      v-else-if="treatment === 'pick'"
      variant="inline"
      :model-value="stringValue"
      :options="pickOptions"
      :label="label"
      placeholder="None"
      :is-option-disabled="isOptionDisabled"
      @update:model-value="pick"
    >
      <template #option="{ value: option }">
        <Badge v-if="option" :value="option" :property="property" :entity-type="entityType" />
        <span v-else class="inline-property-none">None</span>
      </template>
    </RlOptionSelect>

    <RlMultiSelect
      v-else-if="treatment === 'multi'"
      variant="inline"
      :model-value="arrayValue"
      :options="multiOptions"
      :label="label"
      placeholder="None"
      :is-option-disabled="isOptionDisabled"
      @update:model-value="pickMany"
    >
      <template #option="{ value: option }">
        <Badge :value="option" :property="property" :entity-type="entityType" />
      </template>
    </RlMultiSelect>

    <component
      :is="widget"
      v-else-if="treatment === 'live'"
      :id="fieldId"
      mode="edit"
      :model-value="value"
      :property-name="property"
      :property-def="propertyDef"
      :attachments="attachments"
      :max="propertyDef?.max"
      :entity-type="entityType"
      :entity-id="entityId"
      @update:model-value="(v: unknown) => emit('update', v)"
      @attachment-changed="emit('attachment-changed')"
    />

    <RlInlineEdit
      v-else
      :label="label"
      :empty="isEmpty"
      :block="isBlock"
      :auto-open-picker="opensPicker"
      :keep-open-within="floatingPanels"
      @edit="draft = value"
      @commit="commitDraft"
    >
      <template #read>
        <component
          :is="widget"
          mode="display"
          :model-value="value"
          :property-name="property"
          :property-def="propertyDef"
          :max="propertyDef?.max"
        />
      </template>
      <template #edit>
        <div ref="controlRef" class="inline-property-control">
          <component
            :is="widget"
            :id="fieldId"
            mode="edit"
            :model-value="draft"
            :property-name="property"
            :property-def="propertyDef"
            :option-verdicts="optionVerdicts"
            :max="propertyDef?.max"
            :entity-type="entityType"
            :entity-id="entityId"
            @update:model-value="(v: unknown) => (draft = v)"
          />
        </div>
      </template>
    </RlInlineEdit>

    <p v-if="error" class="inline-property-error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
/* Fills the value column, so a value lays out against the row's width rather
   than shrinking to the narrowest box that holds it. */
.inline-property-value {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-2xs);
  flex: 1;
  min-width: 0;
}

.inline-property-error {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--rl-color-danger);
}

.inline-property-none {
  color: var(--rl-color-text-subtle);
}

/* Sized to the row rather than the form grid, like the library's own detail
   field control. A wrapper, because some widgets render their visible control
   beside their root (the tag picker hides its <select> and draws a sibling).
   Capped at the width of the library's value column (its container), so a
   narrow grid cell does not spill into its neighbour while editing. */
.inline-property-control {
  min-width: min(240px, 100cqi);
}
</style>
