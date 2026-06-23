import type { FieldAffordance } from '@/types'

// isFieldWritable: the rendered field is writable unless EITHER the
// static config marks it readonly OR the server's `_fields` verdict
// reports `writable === false`. Both signals are honored — the server
// verdict is authoritative on the wire, but a form config can still
// pin a field readonly for static reasons (e.g. ID display).
//
// `fieldReadonly` is optional so view-side hosts (SectionEditForm)
// that have no static-readonly concept can omit it; passing
// `undefined` falls through cleanly to the verdict check.
export function isFieldWritable(
  verdict: FieldAffordance | undefined,
  fieldReadonly?: boolean,
): boolean {
  if (fieldReadonly) return false
  return verdict?.writable !== false
}

// optionVerdictsFor: pulls the per-option allow-map from a server
// `_fields` verdict. Sparse — only `false` entries appear. Returns
// `undefined` when no verdict exists for this field (all options
// allowed by default).
export function optionVerdictsFor(
  verdict: FieldAffordance | undefined,
): Record<string, boolean> | undefined {
  return verdict?.options
}
