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
