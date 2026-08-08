import type { FieldAffordance } from '@/types'

// isFieldWritable: the rendered field is writable unless the static
// config marks it readonly, the server's `_fields` verdict reports
// `writable === false`, or the field is redacted (`visible === false`).
// All three signals are honored — the server verdict is authoritative
// on the wire, but a form config can still pin a field readonly for
// static reasons (e.g. ID display).
//
// The `visible === false` case matters because a redaction tombstone
// carries ONLY that verdict: the server suppresses `writable` for a
// value the caller cannot read (BUG-FB0LN8), so `writable !== false`
// alone would report a redacted field as writable and render an empty,
// editable widget whose every write the server 403s. Checking it here
// rather than per-consumer is deliberate — the gap this closes was one
// consumer compensating while another did not.
export function isFieldWritable(
  verdict: FieldAffordance | undefined,
  fieldReadonly?: boolean,
): boolean {
  if (fieldReadonly) return false
  if (verdict?.visible === false) return false
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
