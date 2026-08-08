---
id: RR-BEMW01
type: review-response
title: Nested create adds a second awaited dry-run loop; modal open blocks on a POST
finding: DynamicForm awaits refreshStagedAffordances() on mount (DynamicForm.vue:1187-1190) specifically so stagedAffordancesReady flips before interaction (avoiding a hidden-field flash, F19), and re-runs it debounced at 400ms per keystroke via a real dry-run POST. A nested create form gets its own loop, so opening the modal blocks on a server round trip with no spinner specified, and if the modal is kept alive via v-show rather than v-if the loop keeps POSTing behind a closed modal. The plan never mentions the dry-run interaction at all.
severity: significant
resolution: 'Modal renders its own loading state while the nested form''s awaited mount dry-run resolves, preserving the F19 no-flash guarantee instead of working around it. DynamicForm is mounted under v-if (never v-show), stated explicitly so it isn''t later changed for transitions — unmount is what fires the existing stagedUnmounted / stagedDryRunController abort. Two concurrent dry-run loops accepted: both debounced, abortable, read-shaped.'
status: addressed
---

## Resolution

Both halves specified:

- **Mount latency**: the modal renders its own loading state while the nested
form's awaited dry-run is in flight, so the modal paints immediately and the
form body appears when ready. This preserves the F19 no-flash guarantee rather
than working around it.
- **Teardown**: the modal mounts `DynamicForm` under **`v-if`**, never
`v-show`, so closing unmounts and the existing `stagedUnmounted` /
`stagedDryRunController` abort path (RR-2PZB) fires. Stated explicitly in the
approach so it isn't "tidied" into `v-show` later for transition smoothness.

Two concurrent dry-run loops (parent + nested) are accepted: each is debounced,
abortable and read-shaped (never takes `writeMu`, emits no audit row per
`write_handler.go:199-217`), so the added load is one extra in-flight POST while
a modal is open.
