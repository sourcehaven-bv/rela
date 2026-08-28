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
  fieldReadonly?: boolean
): boolean {
  if (fieldReadonly) return false
  return verdict?.writable !== false
}

// isPropertyRedacted: does the server say this property's value was
// withheld by field-level ACL on this response? (DEC-T0XIWQ)
//
// This is the ONLY sound way to ask. The tempting alternative — "the key is
// missing from `properties`, so it must be hidden" — is wrong, because a key
// is equally absent when it was simply never set. That inference is what made
// every unset property unreachable in the edit form (BUG-MLT9DE).
//
// `redacted` is the entity's `_redacted` list. Undefined means the server did
// not report one (a list row, or a shape carrying no write affordances); we
// answer `false`, i.e. render the field. That is the safe direction: the field
// renders, and the server's write gate remains the actual boundary. Answering
// `true` would resurrect the silent-hiding bug on every such shape.
export function isPropertyRedacted(property: string, redacted: string[] | undefined): boolean {
  return redacted?.includes(property) ?? false
}

// optionVerdictsFor: pulls the per-option allow-map from a server
// `_fields` verdict. Sparse — only `false` entries appear. Returns
// `undefined` when no verdict exists for this field (all options
// allowed by default).
export function optionVerdictsFor(
  verdict: FieldAffordance | undefined
): Record<string, boolean> | undefined {
  return verdict?.options
}
