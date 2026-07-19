import type { FieldAffordance } from '@/types'

// Staged-entity sentinel for the create form (TKT-3I5U).
//
// The create form models "create" as editing a staged (uncommitted)
// entity so it can reuse edit mode's affordance-driven field filtering.
// STAGED_ID is a FORM-ONLY sentinel: it identifies the in-progress
// entity in component state but MUST never be sent to the server. The
// dry-run / commit requests send `{type, properties}` with no ID; the
// server treats "no ID" as the create signal. isStaged() guards any
// code path that would otherwise round-trip the sentinel.
export const STAGED_ID = '++new++'

export function isStaged(id: string | undefined): boolean {
  return id === STAGED_ID
}

// adoptLockedFieldValues copies the server's authoritative value into `formData`
// for every field the create dry-run reported as read-only (`writable === false`),
// mutating formData in place (TKT-3G93B8 / BUG-X1C7S).
//
// The motivating case is a state-machine field the server locks to its initial
// value: the control must DISPLAY that initial, not whatever the user typed
// before it locked. The rule is deliberately scoped to *read-only* fields, which
// covers both machine-locked and any policy-read-only field — a read-only field's
// value is server-owned, so adopting the server's echo is correct in general and
// never clobbers a user-editable field. On create the echo usually equals the
// submitted value (a harmless self-copy); it differs only when the server
// overrides (the lock case). The commit filter still omits writable=false keys,
// so this only keeps the *displayed* value honest.
//
// A field is adopted only when the server actually sent a value for it (present
// in `properties`) — an absent key is left untouched.
export function adoptLockedFieldValues(
  fields: Record<string, FieldAffordance> | undefined,
  properties: Record<string, unknown> | undefined,
  formData: Record<string, unknown>
): void {
  if (!fields || !properties) return
  for (const [name, verdict] of Object.entries(fields)) {
    if (verdict.writable === false && name in properties) {
      formData[name] = properties[name]
    }
  }
}
