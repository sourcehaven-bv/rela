// useSectionAutoSave — the autosave host for one entity edited in place on a
// view: a properties section, a cards or list row, or an editable table row
// (TKT-4Q2EIVZ). Extracted from SectionEditForm so every host keeps the same
// invariants instead of a copy of them:
//
// - the owner is frozen at construction, so a response arriving after the
//   host rekeyed to another entity is rejected by the host (RR-FB2A);
// - the autosave baseline is a separate snapshot from the editable values,
//   so an edit never leaks into lastSeenServer (RR-FB2D NEW-10);
// - a reload merges the way a save response does, keeping pending edits;
// - a field whose write verdict flips to read-only drops its pending edit;
// - a pending save is flushed when the host unmounts.
//
// Must be called from a component's setup, which owns the unmount hook.

import { computed, onBeforeUnmount, reactive, ref, watch, type Ref } from 'vue'
import type { Entity, FieldAffordance, FieldVersions, PropertyDef } from '@/types'
import { useAutoSave, type AutoSaveErrorInfo } from '@/composables/useAutoSave'
import { isFieldWritable } from '@/utils/affordances'
import { isClearedForType } from '@/utils/formValue'

export interface AutoSaveOwner {
  type: string
  id: string
}

// The part of a field the host logic reads.
export interface AutoSaveField {
  property: string
  label: string
  verdict?: FieldAffordance
}

export interface SectionAutoSaveOptions {
  entityType: string
  entityId: string
  initialValues: () => Record<string, unknown>
  // Version tokens served with the values (TKT-2VDVHF). When absent, the first
  // save of each field goes out without a precondition.
  initialVersions?: FieldVersions
  fields: () => AutoSaveField[]
  onPropertyApplied: (prop: string, value: unknown, owner: AutoSaveOwner) => void
  onError: (msg: string, info?: AutoSaveErrorInfo) => void
  onVerdictFlip?: (prop: string, label: string) => void
  // Runs each save, so the host can order its reads against the writes
  // (useEntitySession.write). Without it a save simply runs.
  runWrite?: <T>(send: () => Promise<T>) => Promise<T>
}

export function useSectionAutoSave(opts: SectionAutoSaveOptions) {
  const owner: AutoSaveOwner = { type: opts.entityType, id: opts.entityId }
  const initial = opts.initialValues()

  // Local mirror of the values the host renders.
  const formData = reactive<Record<string, unknown>>({ ...initial })

  // Spread again for the autosave baseline; compared against future emits
  // for no-op suppression.
  const initialSnapshot = {
    id: opts.entityId,
    type: opts.entityType,
    properties: { ...initial },
    _versions: opts.initialVersions,
  } satisfies Partial<Entity>

  // The composable only reads the shape, so a computed view is enough.
  const formDataRef = computed(() => formData) as unknown as Ref<Record<string, unknown>>

  const autoSave = useAutoSave({
    target: owner,
    initialServerSnapshot: initialSnapshot,
    runWrite: opts.runWrite,
    disableContentChannel: true,
    disableRelationsChannel: true,
    formData: formDataRef,
    // No-op closures for the disabled channels (RR-FB2D NEW-9).
    contentRef: ref(''),
    inverseToCanonical: new Map(),
    buildRelationsBody: () => null,
    applyServerContent: () => {},
    applyServerProperty: (prop, value) => {
      // DynamicForm's undefined-as-delete semantics (RR-FB2D NEW-5).
      if (value === undefined) {
        delete formData[prop]
      } else {
        formData[prop] = value
      }
      try {
        opts.onPropertyApplied(prop, value, owner)
      } catch (e) {
        // RR-UE3D: never roll back the local value on a host-callback
        // failure. The server-confirmed value IS the truth.
        console.error('useSectionAutoSave: onPropertyApplied threw', e)
      }
    },
    onError: (msg, info) => opts.onError(msg, info),
  })

  // The value shows at once. A field reads as its value between edits, so
  // waiting for the save to answer would flash the old one back first.
  function updateField(property: string, value: unknown, propertyDef?: PropertyDef) {
    if (isClearedForType(value, propertyDef)) {
      delete formData[property]
      autoSave.scheduleUnset(property)
    } else {
      formData[property] = value
      autoSave.scheduleFieldSave(property, value)
    }
  }

  // Verdict-flip watcher (RR-FB1M + RR-FB2C). When a field's writable flag
  // goes true → false, drop the pending edit and notify through a dedicated
  // callback, NOT onError, to avoid the host's 403 refetch path. The inverse
  // direction is silent: nothing destructive to warn about.
  watch(opts.fields, (next, prev) => {
    if (!prev) return
    const prevByProp = new Map(prev.map((f) => [f.property, f]))
    for (const nextField of next) {
      const prevField = prevByProp.get(nextField.property)
      if (!prevField) continue
      if (isFieldWritable(prevField.verdict) && !isFieldWritable(nextField.verdict)) {
        autoSave.revertField(nextField.property)
        opts.onVerdictFlip?.(nextField.property, nextField.label)
      }
    }
  })

  // The values can change under the host: the title edited in the heading, a
  // live update from another session, a reload after an error. Take them the
  // way a save response is taken, so a field with an edit still pending keeps
  // it. The host mirrors every applied value back into its values, so only a
  // real difference may start a merge, or the two would feed each other.
  watch(opts.initialValues, (next) => {
    const differs = opts
      .fields()
      .some((f) => JSON.stringify(next[f.property] ?? null) !== JSON.stringify(formData[f.property] ?? null))
    if (!differs) return
    autoSave.mergeServerResponse({
      id: opts.entityId,
      type: opts.entityType,
      properties: next,
    } as Entity)
  })

  onBeforeUnmount(() => {
    // Flush any pending PATCH against the frozen owner so navigating away
    // mid-debounce does not drop the edit.
    void autoSave.commitImmediately()
  })

  return { owner, formData, autoSave, updateField }
}
