<script setup lang="ts">
/**
 * FormFieldList — renders an ordered list of form fields (property fields,
 * relation cards, relation pickers), dispatching each to the right widget.
 *
 * Extracted from DynamicForm so the single-page body, and each wizard step,
 * render fields through one code path instead of duplicated template blocks.
 * The parent owns all state and validation; this component is presentational
 * and re-emits every edit so the parent's handlers stay authoritative.
 */
import type { FormFieldOrRelation, PropertyDef, RelationAffordance, AttachmentInfo } from '@/types'
import FieldRenderer from './FieldRenderer.vue'
import RelationCards from './RelationCards.vue'
import RelationPicker from './RelationPicker.vue'
import type { RelationCardState } from './RelationCards.vue'
import type { RelationPickerIncomingState } from './RelationPicker.vue'

defineProps<{
  fields: FormFieldOrRelation[]
  entityType: string
  entityId?: string
  formData: Record<string, unknown>
  relations: Record<string, string[]>
  errors: Record<string, string>
  relationAffordances: Record<string, RelationAffordance>
  attachments: Record<string, AttachmentInfo[]>
  saveGeneration: number
  getPropertyDef: (name: string) => PropertyDef | undefined
  isFieldReadonly: (field: FormFieldOrRelation) => boolean
  optionVerdictsFor: (field: FormFieldOrRelation) => Record<string, boolean> | undefined
}>()

const emit = defineEmits<{
  (e: 'update-field', property: string, value: unknown): void
  (e: 'attachment-changed', ...args: unknown[]): void
  (e: 'update-relation', relation: string, value: string[]): void
  (e: 'update-relation-types', relation: string, types: Map<string, string>): void
  (e: 'incoming-changed', relation: string, state: RelationPickerIncomingState): void
  (e: 'cards-changed', key: string, state: RelationCardState): void
}>()
</script>

<template>
  <template
    v-for="(field, fieldIdx) in fields"
    :key="`${fieldIdx}-${field.property || field.relation}`"
  >
    <FieldRenderer
      v-if="field.property && !field.hidden"
      :field="field"
      :property-def="getPropertyDef(field.property)"
      :value="formData[field.property]"
      :error="errors[field.property]"
      :readonly="isFieldReadonly(field)"
      :option-verdicts="optionVerdictsFor(field)"
      :entity-type="entityType"
      :entity-id="entityId"
      :attachments="attachments[field.property]"
      :max="getPropertyDef(field.property)?.max"
      @update="emit('update-field', field.property!, $event)"
      @attachment-changed="emit('attachment-changed', $event)"
    />
    <RelationCards
      v-else-if="field.relation && field.widget === 'cards' && entityId"
      :key="`cards-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
      :field="field"
      :entity-type="entityType"
      :entity-id="entityId"
      :verdict="relationAffordances[field.relation!]"
      @cards-changed="
        (state) =>
          emit('cards-changed', `${field.relation}-${field.direction || 'outgoing'}`, state)
      "
    />
    <RelationPicker
      v-else-if="field.relation"
      :key="`picker-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
      :field="field"
      :entity-type="entityType"
      :entity-id="entityId"
      :value="relations[field.relation] || []"
      :verdict="relationAffordances[field.relation!]"
      @update="emit('update-relation', field.relation!, $event)"
      @update:types="(types) => emit('update-relation-types', field.relation!, types)"
      @incoming-changed="(state) => emit('incoming-changed', field.relation!, state)"
    />
  </template>
</template>
