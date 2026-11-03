import type { FormConfig, FormFieldOrRelation, PropertyDef } from '@/types'
import { naturalCompare } from '@/utils/natsort'

/** A form field that names a property. */
export type PropertyField = FormFieldOrRelation & { property: string }

/** The property fields the conversion dialog asks for, split by need. */
export interface ConversionFields {
  /** Required target properties the mapping does not fill. */
  required: PropertyField[]
  /** The other target properties the caller may set. */
  optional: PropertyField[]
  /**
   * File properties. The dialog cannot stage uploads for an entity that
   * does not exist yet, so these are set after the conversion.
   */
  files: string[]
}

/**
 * Picks the form a new entity of `entityType` is created with: the first
 * form for the type that is not an edit form, else the first edit form.
 * Mirrors `createFormForType` on the server, including its natural sort of
 * form ids, so the dialog shows the fields the create form shows.
 */
export function createFormFor(
  forms: Map<string, FormConfig>,
  entityType: string
): FormConfig | undefined {
  const ids = [...forms.keys()].sort(naturalCompare)
  let fallback: FormConfig | undefined
  for (const id of ids) {
    const form = forms.get(id)
    if (form?.entity !== entityType) continue
    if (form.mode !== 'edit') return form
    fallback ??= form
  }
  return fallback
}

function formPropertyFields(form: FormConfig | undefined): PropertyField[] {
  if (!form) return []
  const all = [
    ...(form.steps ?? []).flatMap((step) => [...(step.fields ?? []), ...(step.relations ?? [])]),
    ...((form.fields ?? []) as FormFieldOrRelation[]),
  ]
  return all.filter((field): field is PropertyField => typeof field.property === 'string')
}

/**
 * Returns the target properties the caller can supply in a conversion, as
 * form fields. A property the form configures keeps its widget, label and
 * order; a property the form leaves out gets a bare field, so a required
 * property is never left without an input. A field the form hides is left
 * out unless it is required, for the same reason. Mapped properties are left
 * out: the mapping sets them, and the server refuses a value for one. File
 * properties are listed apart, by name.
 */
export function conversionFields(
  properties: Record<string, PropertyDef>,
  form: FormConfig | undefined,
  mapped: Set<string>
): ConversionFields {
  const settable = (name: string) => name in properties && !mapped.has(name)
  const isRequired = (name: string) => properties[name].required === true
  const fields: PropertyField[] = []
  const files: string[] = []
  const seen = new Set<string>()
  const take = (field: PropertyField) => {
    const name = field.property
    if (seen.has(name) || !settable(name)) return
    seen.add(name)
    if (properties[name].type === 'file') files.push(name)
    else if (!field.hidden) fields.push(field)
    else if (isRequired(name)) fields.push(shown(field))
  }
  for (const field of formPropertyFields(form)) take(field)
  for (const name of Object.keys(properties)) take({ property: name })
  return {
    required: fields.filter((field) => isRequired(field.property)),
    optional: fields.filter((field) => !isRequired(field.property)),
    files,
  }
}

// shown returns field without its hidden flag.
function shown(field: PropertyField): PropertyField {
  const { hidden: _hidden, ...rest } = field
  return rest
}

/** Reports whether a field value counts as not supplied. */
export function isEmptyValue(value: unknown): boolean {
  return (
    value === undefined ||
    value === null ||
    value === '' ||
    (Array.isArray(value) && value.length === 0)
  )
}
