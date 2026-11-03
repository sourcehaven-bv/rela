<script setup lang="ts">
/**
 * The cells of one editable `display: table` section row (TKT-4Q2EIVZ).
 *
 * Renders the row's `<td>`s itself (a fragment, so it sits inside the host's
 * `<tr>`, which may be a reorderable row). A column with `render: input` whose
 * `_fields` verdict allows the write is an inline value, saved through one
 * autosave per row; every other column renders through the `cell` slot, the
 * same read-only content a plain row shows. The `actions` slot receives the
 * row's save status for its indicator.
 *
 * An editable cell reads the row's `_props`, never the stringified
 * `cells[].values`, so a saved value shows as typed until the next view load.
 * A read-only cell shows the server's `cells[].values`, as a plain row does.
 */
import { computed, type Component } from 'vue'
import type { ViewCell, ViewColumn, ViewRow } from '@/api/views'
import type { FieldAffordance, PropertyDef } from '@/types'
import type { AutoSaveErrorInfo, SaveStatus } from '@/composables/useAutoSave'
import { useSectionAutoSave, type AutoSaveOwner } from '@/composables/useSectionAutoSave'
import { useSchemaStore } from '@/stores'
import { defaultRegistry } from '@/widgets/registry'
import { isFieldWritable, optionVerdictsFor } from '@/utils/affordances'
import { entityRef } from '@/utils/entityRef'
import InlinePropertyValue from '@/components/forms/InlinePropertyValue.vue'

const props = defineProps<{
  row: ViewRow
  columns: ViewColumn[]
  onPropertyApplied: (prop: string, value: unknown, owner: AutoSaveOwner) => void
  onError: (msg: string, info?: AutoSaveErrorInfo) => void
  onVerdictFlip?: (prop: string, label: string) => void
  runWrite?: <T>(send: () => Promise<T>) => Promise<T>
}>()

defineSlots<{
  cell(props: { cell: ViewCell; index: number }): unknown
  actions(props: { status: SaveStatus; error: string | null | undefined }): unknown
}>()

const schemaStore = useSchemaStore()

interface EditColumn {
  property: string
  label: string
  propertyDef: PropertyDef
  widget: Component
  numeric: boolean
}

// One widget resolution per column, not per cell. A column edits only when
// it opts in and the property is declared: an undeclared property has no
// definition the server could have checked a widget against.
const editColumns = computed<(EditColumn | undefined)[]>(() =>
  props.columns.map((col) => {
    if (col.render !== 'input' || !col.property) return undefined
    const propertyDef = schemaStore.getEntityType(props.row.entityType)?.properties?.[col.property]
    // A file has no attachment data in a row; config load refuses it too.
    if (!propertyDef || propertyDef.type === 'file') return undefined
    return {
      property: col.property,
      label: col.label || propertyDef.label || col.property,
      propertyDef,
      widget: defaultRegistry.resolve(undefined, propertyDef),
      numeric: propertyDef.type === 'integer' || propertyDef.type === 'decimal',
    }
  })
)

function verdictOf(property: string): FieldAffordance | undefined {
  return props.row._fields?.[property]
}

const { formData, autoSave, updateField } = useSectionAutoSave({
  entityType: props.row.entityType,
  // The row's own address, so the edit lands on the face on screen.
  entityId: entityRef({ id: props.row.entityId, _self: props.row._self }),
  initialValues: () => props.row._props ?? {},
  fields: () =>
    editColumns.value.flatMap((c) =>
      c ? [{ property: c.property, label: c.label, verdict: verdictOf(c.property) }] : []
    ),
  onPropertyApplied: (prop, value, owner) => props.onPropertyApplied(prop, value, owner),
  onError: (msg, info) => props.onError(msg, info),
  onVerdictFlip: (prop, label) => props.onVerdictFlip?.(prop, label),
  runWrite: props.runWrite,
})

const ownerId = entityRef({ id: props.row.entityId, _self: props.row._self })
</script>

<template>
  <td
    v-for="(cell, idx) in row.cells"
    :key="idx"
    :class="{ 'data-table-cell--numeric': editColumns[idx]?.numeric }"
  >
    <InlinePropertyValue
      v-if="editColumns[idx] && isFieldWritable(verdictOf(editColumns[idx]!.property))"
      :property="editColumns[idx]!.property"
      :label="editColumns[idx]!.label"
      :widget="editColumns[idx]!.widget"
      :value="formData[editColumns[idx]!.property]"
      :writable="true"
      :numeric="editColumns[idx]!.numeric"
      :property-def="editColumns[idx]!.propertyDef"
      :option-verdicts="optionVerdictsFor(verdictOf(editColumns[idx]!.property))"
      :entity-type="row.entityType"
      :entity-id="ownerId"
      :error="autoSave.fieldErrors.value[editColumns[idx]!.property]"
      @update="
        (v: unknown) => updateField(editColumns[idx]!.property, v, editColumns[idx]!.propertyDef)
      "
    />
    <slot v-else name="cell" :cell="cell" :index="idx" />
  </td>
  <slot name="actions" :status="autoSave.status.value" :error="autoSave.lastError.value" />
</template>
