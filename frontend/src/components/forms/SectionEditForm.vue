<script setup lang="ts">
// SectionEditForm — small useAutoSave host for one properties section.
//
// Owns: iteration over `fields`, widget resolution (schema or hint),
// per-cell writability gating, an `useAutoSave` instance with the
// content + relations channels disabled, and the verdict-flip watcher.
// Does NOT own: the section heading, layout placement of
// AutoSaveIndicator, the spread-clone write-back to the host's
// viewData (that's `onPropertyApplied`'s job).
//
// The host (EntityDetail) is responsible for `:key`-driven remount on
// entity-id change so this component's lifecycle handles route
// navigation cleanly (RR-FB1D + RR-FB2A).

import { computed, onBeforeUnmount, reactive, ref, watch, type Ref } from 'vue'
import type { Component } from 'vue'
import type { FieldAffordance, PropertyDef, Entity, AttachmentInfo } from '@/types'
import type { WidgetRoutingHint } from '@/widgets/types'
import { defaultRegistry } from '@/widgets/registry'
import { useAutoSave, type AutoSaveErrorInfo } from '@/composables/useAutoSave'
import { isFieldWritable, optionVerdictsFor } from '@/utils/affordances'
import { isClearedForType } from '@/utils/formValue'
import FieldShell from './FieldShell.vue'
import AutoSaveIndicator from './AutoSaveIndicator.vue'

// Discriminated union: each field resolves its widget via either the
// real schema entry (form-side) or a routing hint (view-side). Exactly
// one of these shapes per field; no bang-casts (RR-FB1H).
export type SectionEditField = {
  property: string
  label: string
  verdict?: FieldAffordance
} & (
  | { kind: 'schema'; propertyDef: PropertyDef }
  | { kind: 'hint'; routingHint: WidgetRoutingHint }
)

const props = defineProps<{
  entityType: string
  entityId: string
  initialValues: Record<string, unknown>
  fields: SectionEditField[]
  // Per-`file`-property attachment LISTS for the entity, so the file
  // widget can show the current files and drive upload/remove on the
  // inline-edit path. Keyed by property name.
  attachments?: Record<string, AttachmentInfo[]>
  // Owner identity captured at construction and forwarded to every
  // callback so the host can reject stale responses arriving after
  // a :key-driven remount targeted a different entity (RR-FB2A).
  onPropertyApplied: (prop: string, value: unknown, owner: { type: string; id: string }) => void
  onError: (msg: string, info?: AutoSaveErrorInfo) => void
  onVerdictFlip?: (prop: string, label: string) => void
  // Called after the file widget uploads/removes an attachment so the
  // host can refresh the entity (property value + _attachments changed).
  onAttachmentChanged?: () => void
}>()

// Owner identity is frozen for the instance's lifetime. When the host
// rekeys this component (entity-id change), a new instance is mounted
// with a fresh owner.
const owner = { type: props.entityType, id: props.entityId }

// Local mirror of the section's properties. Spread independent of the
// initialServerSnapshot baseline so widget edits don't leak into
// useAutoSave's lastSeenServer (RR-FB2D NEW-10).
const formData = reactive<Record<string, unknown>>({ ...props.initialValues })

// Spread again for the autosave baseline; this is the value compared
// against future emits for no-op suppression.
const initialSnapshot = {
  id: props.entityId,
  type: props.entityType,
  properties: { ...props.initialValues },
} satisfies Partial<Entity>

// Adapter ref for AutoSaveOptions.formData — the composable only reads
// shape (never writes) so a computed view is sufficient.
const formDataRef = computed(() => formData) as unknown as Ref<Record<string, unknown>>

const autoSave = useAutoSave({
  getEntityType: () => owner.type,
  getEntityId: () => owner.id,
  initialServerSnapshot: initialSnapshot,
  disableContentChannel: true,
  disableRelationsChannel: true,
  formData: formDataRef,
  // No-op closures for the disabled channels (RR-FB2D NEW-9).
  contentRef: ref(''),
  inverseToCanonical: new Map(),
  buildRelationsBody: () => null,
  applyServerContent: () => {},
  applyServerProperty: (prop, value) => {
    // Mirror DynamicForm's undefined-as-delete semantics
    // (RR-FB2D NEW-5; DynamicForm L923-929 equivalent).
    if (value === undefined) {
      delete formData[prop]
    } else {
      formData[prop] = value
    }
    try {
      props.onPropertyApplied(prop, value, owner)
    } catch (e) {
      // RR-UE3D: never roll back the local formData on host-callback
      // failure. The server-confirmed value IS the truth; the host's
      // job is to fix its reconciler.
      console.error('SectionEditForm: onPropertyApplied threw', e)
    }
  },
  onError: (msg, info) => props.onError(msg, info),
})

// Precompute one widget per field. Stable across renders that don't
// add or reorder fields (PropertyDisplay L42-64 pattern).
interface WidgetRow {
  field: SectionEditField
  widget: Component
  writable: boolean
  optionVerdicts?: Record<string, boolean>
}

const widgetRows = computed<WidgetRow[]>(() =>
  props.fields.map((field) => {
    const widget =
      field.kind === 'schema'
        ? defaultRegistry.resolve(undefined, field.propertyDef)
        : defaultRegistry.resolveFromHint(field.routingHint)
    return {
      field,
      widget,
      writable: isFieldWritable(field.verdict),
      optionVerdicts: optionVerdictsFor(field.verdict),
    }
  }),
)

function onFieldUpdate(field: SectionEditField, value: unknown) {
  const def = field.kind === 'schema' ? field.propertyDef : undefined
  if (isClearedForType(value, def)) {
    autoSave.scheduleUnset(field.property)
  } else {
    autoSave.scheduleFieldSave(field.property, value)
  }
}

// Verdict-flip watcher (RR-FB1M + RR-FB2C). When a property's writable
// flag goes true → false, drop the pending edit and surface a
// dedicated notification — NOT through `onError`, to avoid the host's
// 403 refetch path (RR-FB2C). The inverse direction (false → true,
// permission restored) is intentionally silent (round-3 N-R3-1): the
// cell becomes editable again with no destructive UX consequence to
// warn about.
watch(
  () => props.fields,
  (next, prev) => {
    if (!prev) return
    const prevByProp = new Map(prev.map((f) => [f.property, f]))
    for (const nextField of next) {
      const prevField = prevByProp.get(nextField.property)
      if (!prevField) continue
      const wasWritable = isFieldWritable(prevField.verdict)
      const nowWritable = isFieldWritable(nextField.verdict)
      if (wasWritable && !nowWritable) {
        autoSave.revertField(nextField.property)
        props.onVerdictFlip?.(nextField.property, nextField.label)
      }
    }
  },
)

onBeforeUnmount(() => {
  // Flush any pending PATCH against this instance's frozen owner so
  // navigating away mid-debounce doesn't silently drop the edit. The
  // identity guard in handlePropertyApplied prevents the response
  // from leaking into the new entity's view (RR-FB2A).
  void autoSave.commitImmediately()
})

defineExpose({
  // Exposed for component-level tests; not part of the public API.
  status: autoSave.status,
  fieldErrors: autoSave.fieldErrors,
})
</script>

<template>
  <div class="section-edit-form">
    <!--
      Indicator slot (TKT-IHC7C / RR-FC1D + RR-FC2A): scope props `status`
      and `error` so a host can render the indicator anywhere (e.g. via
      Vue `<Teleport>` into a card header). Default preserves IHC7B
      behaviour: an inline-positioned AutoSaveIndicator inside the form.
    -->
    <slot name="indicator" :status="autoSave.status.value" :error="autoSave.lastError.value">
      <div class="section-edit-form-indicator">
        <AutoSaveIndicator :status="autoSave.status.value" :error="autoSave.lastError.value" />
      </div>
    </slot>
    <dl class="properties-list">
      <div
        v-for="row in widgetRows"
        :key="row.field.property"
        class="property-item"
      >
        <dt>{{ row.field.label }}</dt>
        <dd>
          <FieldShell
            v-if="row.writable"
            :field-id="`section-edit-${row.field.property}`"
            :error="autoSave.fieldErrors.value[row.field.property]"
          >
            <component
              :is="row.widget"
              :id="`section-edit-${row.field.property}`"
              mode="edit"
              :model-value="formData[row.field.property]"
              :property-name="row.field.property"
              :property-def="row.field.kind === 'schema' ? row.field.propertyDef : undefined"
              :option-verdicts="row.optionVerdicts"
              :attachments="props.attachments?.[row.field.property]"
              :max="row.field.kind === 'schema' ? row.field.propertyDef?.max : undefined"
              :entity-type="entityType"
              :entity-id="entityId"
              @update:model-value="(v: unknown) => onFieldUpdate(row.field, v)"
              @attachment-changed="onAttachmentChanged?.()"
            />
          </FieldShell>
          <component
            :is="row.widget"
            v-else
            mode="display"
            :model-value="formData[row.field.property]"
            :property-name="row.field.property"
            :property-def="row.field.kind === 'schema' ? row.field.propertyDef : undefined"
            :attachments="props.attachments?.[row.field.property]"
            :max="row.field.kind === 'schema' ? row.field.propertyDef?.max : undefined"
          />
        </dd>
      </div>
    </dl>
  </div>
</template>

<style scoped>
.section-edit-form {
  position: relative;
}

.section-edit-form-indicator {
  position: absolute;
  top: -28px;
  right: 0;
}

.properties-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 32px;
  margin: 0;
}

.property-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 200px;
}

.property-item dt {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--muted-text);
  margin: 0;
}

.property-item dd {
  margin: 0;
  font-size: 14px;
  color: var(--text-color);
  line-height: 1.5;
}
</style>
