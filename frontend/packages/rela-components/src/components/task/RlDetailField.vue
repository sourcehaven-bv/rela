<script setup lang="ts">
/**
 * One label/value row in a task detail.
 *
 * With `editable` the value becomes changeable in place, and each type gets
 * the treatment that suits it.
 *
 * A value chosen from a list — a status, a priority — is a live control the
 * whole time, drawn bare until hovered. Nothing is swapped in on click: the
 * thing you hover is the thing that opens, so there is no moment where the
 * control has to be created and focused by hand, and no guessing about when
 * an edit is finished.
 *
 * Free text has no such control, since an always-mounted input would read as
 * a form. That one swaps, through `RlInlineEdit`.
 *
 * The built-in types are the common ones, not the whole world. An app with
 * its own registry of property widgets can take over the value column with
 * the `value` slot and keep the row: the label column, the height, the
 * wrapping, and what the row does when the screen is narrow.
 */
import { computed, ref } from 'vue'
import type { DetailField, StatusOption, Tag } from '../../types'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlTag from '../common/RlTag.vue'
import RlFieldLabel from '../common/RlFieldLabel.vue'
import RlText from '../common/RlText.vue'
import RlInlineEdit from '../common/RlInlineEdit.vue'
import RlOptionSelect from '../form/RlOptionSelect.vue'
import RlDateField from '../form/RlDateField.vue'
import { toISODate, fromISODate } from './detailFieldDate'

const props = withDefaults(
  defineProps<{
    field: DetailField
    /** Turns the value into a control that can be changed in place. */
    editable?: boolean
    /**
     * Choices for a status field. Each carries the colour its dot takes once
     * chosen, so picking a status changes the dot as well as the words.
     */
    options?: StatusOption[]
    /**
     * Choices for a single-tag field such as a priority. Each is a whole tag,
     * so the list shows the badge rather than its label alone.
     */
    tagOptions?: Tag[]
    /**
     * Puts the value under its label rather than beside it, across the full
     * width of the row. For a value too long to read in the value column: a
     * paragraph of text, or a list of tags that would otherwise wrap several
     * times against the label.
     */
    stacked?: boolean
  }>(),
  { editable: false, options: () => [], tagOptions: () => [], stacked: false },
)

const emit = defineEmits<{
  /** The value changed. */
  'update:field': [field: DetailField]
}>()

const isTagField = computed(() => props.field.type === 'tag' || props.field.type === 'tags')

/** A picker: the value comes from a list, so the control is always live. */
const picks = computed(() => {
  if (!props.editable) return false
  if (props.field.type === 'status') return props.options.length > 0
  if (props.field.type === 'tag') return props.tagOptions.length > 0
  return false
})

/**
 * A swap: there is no always-on control that reads as plain text, so the
 * value stands in for one until it is clicked. Multi-tag fields are neither,
 * because picking a single value would silently drop the rest.
 */
const swaps = computed(() => props.editable && !picks.value && props.field.type !== 'tags')

const isEmpty = computed(() =>
  isTagField.value ? !props.field.tags?.length : !props.field.value,
)

const statusValues = computed(() => props.options.map((option) => option.value))
const tagLabels = computed(() => props.tagOptions.map((tag) => tag.label))

const colorOf = (value: string) =>
  props.options.find((option) => option.value === value)?.status

const tagOf = (label: string) => props.tagOptions.find((tag) => tag.label === label)

/*
 * A status option's value is its label, so the picker round-trips without the
 * caller keeping a second id, and the colour comes from the matching option.
 */
function setStatus(value: string) {
  if (value === props.field.value) return
  emit('update:field', { ...props.field, value, status: colorOf(value) ?? props.field.status })
}

function setTag(label: string) {
  const tag = tagOf(label)
  if (!tag || tag.id === props.field.tags?.[0]?.id) return
  emit('update:field', { ...props.field, tags: [tag] })
}

/*
 * The swapped edit is held here until it is kept. Writing straight through on
 * every keystroke would make Escape meaningless: by the time it arrives the
 * parent has already taken the value.
 */
const draft = ref('')

/** A date edits as ISO and is stored the way it reads. */
const draftDate = computed({
  get: () => toISODate(draft.value),
  set: (iso: string) => {
    draft.value = fromISODate(iso) || draft.value
  },
})

function onEdit() {
  draft.value = props.field.value ?? ''
}

function commit() {
  if (draft.value === (props.field.value ?? '')) return
  emit('update:field', { ...props.field, value: draft.value })
}
</script>

<template>
  <div class="rl-detail-field" :class="{ 'rl-detail-field--stacked': stacked }">
    <RlFieldLabel class="rl-detail-field__label">
      {{ field.label }}
      <!-- After the label text, on the label's own baseline: a comment
           marker or another per-field affordance belongs to the field, not
           to its value, and should not move when the value does. -->
      <slot name="label-trailing" :field="field" />
    </RlFieldLabel>

    <RlText as="dd" class="rl-detail-field__value">
      <!--
        The whole value column, handed over. Everything below is the built-in
        rendering, which is what a caller without its own widgets gets.
      -->
      <slot name="value" :field="field" :editable="editable">
        <!--
          A drawn list rather than a native select: a native one holds only
          text, so the dot and the badge disappear exactly when the user is
          choosing between them.
        -->
        <RlOptionSelect
          v-if="picks && field.type === 'status'"
          variant="inline"
          :model-value="field.value"
          :options="statusValues"
          :label="field.label"
          @update:model-value="setStatus"
        >
          <template #option="{ value }">
            <RlStatusDot :color="colorOf(value)" />
            <span class="rl-detail-field__nowrap">{{ value }}</span>
          </template>
        </RlOptionSelect>

        <RlOptionSelect
          v-else-if="picks"
          variant="inline"
          :model-value="field.tags?.[0]?.label ?? ''"
          :options="tagLabels"
          :label="field.label"
          @update:model-value="setTag"
        >
          <template #option="{ value }">
            <RlTag :label="value" :color="tagOf(value)?.color" />
          </template>
        </RlOptionSelect>

        <RlInlineEdit
          v-else-if="swaps"
          :label="field.label"
          :empty="isEmpty"
          :auto-open-picker="field.type === 'date'"
          @edit="onEdit"
          @commit="commit"
        >
          <template #read>
            <span class="rl-detail-field__nowrap">{{ field.value }}</span>
          </template>

          <template #edit>
            <!-- The real picker. The display string is translated around it. -->
            <RlDateField
              v-if="field.type === 'date'"
              v-model="draftDate"
              :label="field.label"
              label-hidden
              class="rl-detail-field__control"
            />

            <input
              v-else
              v-model="draft"
              type="text"
              class="rl-control rl-control--sm rl-detail-field__control"
              :aria-label="field.label"
            />
          </template>
        </RlInlineEdit>

        <template v-else>
          <template v-if="field.type === 'status'">
            <RlStatusDot :color="field.status" />
            {{ field.value }}
          </template>

          <template v-else-if="isTagField">
            <RlTag v-for="tag in field.tags" :key="tag.id" :label="tag.label" :color="tag.color" />
          </template>

          <template v-else>{{ field.value }}</template>
        </template>
      </slot>
    </RlText>
  </div>
</template>

<style scoped>
.rl-detail-field {
  display: flex;
  align-items: center;
  gap: var(--rl-space-4);
  min-height: 30px;
}

.rl-detail-field__label {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 120px;
}

/* Takes the rest of the row, so a value has room to show itself rather than
   shrinking to the width of its own text and clipping. */
.rl-detail-field__value {
  display: flex;
  align-items: center;
  flex: 1;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
  min-width: 0;
}

/* Stacked: the label keeps its own line and the value takes the width below
   it, so long content is read across the row rather than down a column. */
.rl-detail-field--stacked {
  flex-direction: column;
  align-items: stretch;
  gap: var(--rl-space-1);
}

.rl-detail-field--stacked .rl-detail-field__label { width: auto; }

/*
 * Stacked exists for content that has to wrap, so the value fills the row
 * rather than sizing to its own text. An inline edit inside it is
 * `max-content` by default, which is right for a value on a shared row and
 * wrong here: prose would run past the edge instead of breaking.
 */
.rl-detail-field--stacked .rl-detail-field__value > :deep(.rl-inline-edit),
.rl-detail-field--stacked .rl-detail-field__value > :deep(.rl-inline-edit) .rl-inline-edit__trigger {
  width: 100%;
  max-width: none;
}

/* A status or a date is two or three words that read as one value: breaking
   them across lines turns a one-line row into a two-line one. */
.rl-detail-field__nowrap { white-space: nowrap; }

/* Except when stacked, which is chosen precisely for content long enough to
   need more than one line. */
.rl-detail-field--stacked .rl-detail-field__nowrap { white-space: normal; }

/* Sized to the row rather than the form grid: a detail value is not a form
   field and should not push the row taller while it is being edited. */
.rl-detail-field__control { min-width: 180px; }

@media (max-width: 767px) {
  .rl-detail-field { align-items: flex-start; }
  .rl-detail-field__label { width: 96px; }
}
</style>
