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
import type { FieldAffordance, PropertyDef, Entity, AttachmentInfo, TransitionOption } from '@/types'
import type { WidgetRoutingHint } from '@/widgets/types'
import { defaultRegistry } from '@/widgets/registry'
import { useAutoSave, type AutoSaveErrorInfo } from '@/composables/useAutoSave'
import { isFieldWritable, optionVerdictsFor } from '@/utils/affordances'
import { isClearedForType } from '@/utils/formValue'
import { fieldSpanStyle } from '@/utils/fieldSpan'
import FieldShell from './FieldShell.vue'
import StatusControl from './StatusControl.vue'
import AutoSaveIndicator from './AutoSaveIndicator.vue'

// Discriminated union: each field resolves its widget via either the
// real schema entry (form-side) or a routing hint (view-side). Exactly
// one of these shapes per field; no bang-casts (RR-FB1H).
export type SectionEditField = {
  property: string
  label: string
  verdict?: FieldAffordance
  // Authored width on the 12-column layout grid (TKT-5V8704). Undefined /
  // 0 means full width, which is the default for every auto-generated view.
  span?: number
  // Machine-aware status control (TKT-3G93B8): when present (even empty), the
  // field is a state machine and renders as a StatusControl instead of its
  // resolved widget. Undefined = not a machine field (or a surface without
  // `_transitions`, e.g. cards/list rows) → the ordinary widget.
  transitions?: TransitionOption[]
} & (
  | { kind: 'schema'; propertyDef: PropertyDef }
  | { kind: 'hint'; routingHint: WidgetRoutingHint }
)

const props = defineProps<{
  // Optional section heading. When set, the form renders its own header
  // row with the heading on the left and the auto-save indicator on the
  // right (TKT-U62DVR) — so the two are genuine flex siblings on one line
  // with no positioning tricks. When omitted, the host owns the heading
  // and typically supplies an explicit `#indicator` slot (cards/list).
  heading?: string
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
      Heading row (TKT-U62DVR): when `heading` is provided the form owns the
      section heading so the heading and the auto-save indicator are flex
      siblings on one line — no absolute positioning, no template refs. The
      indicator hides itself when idle and fades out after a save.
    -->
    <div v-if="heading" class="section-edit-form-header">
      <h2 class="section-heading">{{ heading }}</h2>
      <slot name="indicator" :status="autoSave.status.value" :error="autoSave.lastError.value">
        <AutoSaveIndicator :status="autoSave.status.value" :error="autoSave.lastError.value" />
      </slot>
    </div>
    <!--
      Headless indicator slot (TKT-IHC7C / RR-FC1D + RR-FC2A): when no
      `heading` is given the host owns placement (e.g. inline in a card or
      list header) and supplies its own `#indicator`. Scope props `status`
      and `error` are exposed either way.
    -->
    <slot
      v-if="!heading"
      name="indicator"
      :status="autoSave.status.value"
      :error="autoSave.lastError.value"
    >
      <AutoSaveIndicator :status="autoSave.status.value" :error="autoSave.lastError.value" />
    </slot>
    <dl class="properties-list">
      <div
        v-for="row in widgetRows"
        :key="row.field.property"
        class="property-item"
        :style="fieldSpanStyle(row.field.span)"
      >
        <dt>{{ row.field.label }}</dt>
        <dd>
          <StatusControl
            v-if="row.field.transitions !== undefined"
            :model-value="formData[row.field.property] == null ? '' : String(formData[row.field.property])"
            :property="row.field.property"
            :entity-type="entityType"
            :transitions="row.field.transitions"
            :disabled="!row.writable"
            @update:model-value="(v: string) => onFieldUpdate(row.field, v)"
          />
          <FieldShell
            v-else-if="row.writable"
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
/* Heading row: heading left, auto-save indicator right, on one line.
   Carries the section-heading border so it spans the full width beneath
   both. Mirrors EntityDetail's `.section-heading` look (TKT-U62DVR). */
.section-edit-form-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  margin: 0 0 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color);
}

/* KEEP IN SYNC with EntityDetail.vue `.section-heading` (RR-ZE29PY). Scoped
   styles don't cross components, so this deliberately duplicates that rule
   (font, margin, border via the row) — the Properties heading must match
   every sibling section heading on the page. */
.section-edit-form-header .section-heading {
  font-size: var(--font-size-lg);
  font-weight: 600;
  margin: 0;
  color: var(--text-color);
}

/* .properties-list / .property-item now live in styles/properties-list.css,
 * shared with PropertyDisplay and SidePanel. Do not redefine them here — the
 * three scoped copies drifting apart is what this ticket removed. */
</style>
