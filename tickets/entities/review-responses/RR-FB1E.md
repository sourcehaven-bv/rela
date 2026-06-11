---
id: RR-FB1E
type: review-response
title: 'C5: scheduleUnset routing missing — clears become "set to empty"'
finding: |
  DynamicForm routes cleared values to `scheduleUnset` via `isClearedForType(value, def)`. SectionEditForm per the plan calls only `scheduleFieldSave`. Result: clearing a field sends `{ properties: { x: "" } }` instead of `{ properties_unset: ["x"] }` — semantic regression at the server.
severity: critical
status: addressed
resolution: |
  PLAN AC 3 amended: SectionEditForm mirrors DynamicForm's routing via the shared helper. Either (a) extract `isClearedForType` into `frontend/src/utils/formValue.ts` and import from both DynamicForm and SectionEditForm, or (b) leave it duplicated for now (3-line helper). Pick (a) — it sharpens the contract. Test plan adds: "clearing a writable text field schedules scheduleUnset, not scheduleFieldSave".
---
