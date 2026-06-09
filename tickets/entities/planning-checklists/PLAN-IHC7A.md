---
id: PLAN-IHC7A
type: planning-checklist
title: 'Planning: Per-channel debounce + checkbox-toggle to useAutoSave'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-IHC7A ticket body.

**Acceptance Criteria:**

1. `useAutoSave` gains `fieldDebounceMs?: number` and `contentDebounceMs?: number` options. Legacy `debounceMs` continues to work as an alias that sets both. Per-channel options override the legacy alias when both are set. **Test must use distinct values** to verify precedence (RR-FA1D): one case with `fieldDebounceMs: 100, debounceMs: 300` asserting field fires at ~100ms not 300ms; one inverse case.
2. `useAutoSave` gains `initialServerSnapshot?: Record<string, unknown>` option. When set, `lastSeenServer` is seeded atomically in the constructor. **Jsdoc must state** (RR-FA1H): "Equivalent to calling `recordServerSnapshot(entity)` immediately after construction. Any later `recordServerSnapshot` call fully replaces this seed."
3. `useAutoSave` gains `disablePropertyChannel?: boolean`, `disableContentChannel?: boolean`, `disableRelationsChannel?: boolean` options. **Semantics (amended per RR-FA1A and RR-FA1B):**
   - When a channel is disabled, its `schedule*` function throws with a clear named message: `` `useAutoSave: ${channelName} channel is disabled; remove the disable${ChannelName}Channel flag or stop calling ${methodName}` `` (RR-FA1E). Test asserts the channel name appears in the message.
   - `mergeServerResponse ` **skips the callback invocation** (`applyServerProperty ` / `applyServerContent `) and disappeared-key cleanup for the disabled channel, but **still updates `lastSeenServer `/`lastSeenContent `** from the server response (preserving the baseline for forward-compat).
   - Re-enabling a channel mid-instance-lifetime is explicitly **not supported** (jsdoc note).
   - Required callbacks (`applyServerProperty `, `applyServerContent `, `buildRelationsBody `) stay **required at the type level**; they may be passed as no-op closures when their channel is disabled. A runtime assertion in `mergeServerResponse ` throws as defence-in-depth if `applyServerProperty ` is invoked while `disablePropertyChannel ` is true (should never fire under correct internal logic).
   - `commitImmediately ` requires **no explicit guards** — disabled channels' state is naturally empty (e.g., `timers ` Map is empty because `scheduleFieldSave ` throws before scheduling), so the existing per-channel checks early-exit correctly (RR-FA1G). Test asserts `commitImmediately ` returns `{ settled: true } ` immediately on a fully-disabled instance.
4. `EntityDetail.handleCheckboxToggle ` rewritten to use a content-only `useAutoSave ` instance with `contentDebounceMs: 100, disablePropertyChannel: true, disableRelationsChannel: true `. **EntityDetail passes** `contentRef: computed(() => entry.value?.content ?? '') ` — safe because the composable never mutates `contentRef ` (RR-FA1C; jsdoc on `AutoSaveOptions.contentRef ` clarifies read-only-shape contract). `applyServerProperty: () => {} ` and `buildRelationsBody: () => null ` are no-op closures.
5. `togglingIndices ` Set removed from `EntityDetail `.
6. Existing `useAutoSave ` tests pass unchanged.
7. Existing `DynamicForm ` tests pass unchanged.
8. Existing `e2e/tests/checkboxes.spec.ts ` passes unchanged. **Note** (RR-FA1F): the e2e uses 2–5s poll timeouts; 100ms debounce is comfortably within tolerance.
9. New unit tests for each of the three useAutoSave API additions (per-channel debounce with precedence, initial snapshot, channel disable + named-error, lastSeenServer baseline preservation under disable).
10. New unit test for `EntityDetail `'s content-channel instance: click → ~100ms PATCH → viewData.entry.content updated AND the entry-content section's `.content ` mirror is updated alongside (preserving today's L230-233 splice shape; RR's prior #7 concern).

## Research

- [x] ~~For larger features: run `/research `~~ (N/A: small extension)
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- `useAutoSave ` (`composables/useAutoSave.ts `, ~540 lines): the composable being extended. Already exposes `debounceMs `, `recordServerSnapshot `, `mergeServerResponse `, `commitImmediately `, `scheduleFieldSave `, `scheduleContentSave `, `scheduleRelationsChange `.
- `DynamicForm.vue ` L916: the canonical caller. Uses the legacy `debounceMs ` (defaults to 800). Must continue to work unchanged.
- `EntityDetail.vue ` L184-240: the migration target — bespoke `handleCheckboxToggle ` with `togglingIndices ` Set.
- TKT-MZSIJ shipped the widget registry (#848); TKT-UD7YR (PR #906, pending) shipped view-side delegation. Neither is a dependency for IHC7A — this work only touches `useAutoSave ` and `EntityDetail.handleCheckboxToggle `.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

1. **`useAutoSave ` API extension** (`frontend/src/composables/useAutoSave.ts `):
   - Add to `AutoSaveOptions `: `fieldDebounceMs?: number `, `contentDebounceMs?: number `, `initialServerSnapshot?: Record<string, unknown> `, `disablePropertyChannel?: boolean `, `disableContentChannel?: boolean `, `disableRelationsChannel?: boolean `.
   - In the composable body:
     - Resolve effective per-channel debounce: `fieldDebounceMs ?? debounceMs ?? 800 ` and `contentDebounceMs ?? debounceMs ?? 800 `.
     - Replace the single internal `debounceMs ` use with per-channel where applicable (field timer uses `fieldDebounceMs `; content timer uses `contentDebounceMs `).
     - If `initialServerSnapshot ` is set, `lastSeenServer.value = { ...initialServerSnapshot } ` during construction (before any `schedule* ` is callable).
     - If `disablePropertyChannel `: `scheduleFieldSave ` and `scheduleUnset ` throw; `mergeServerResponse ` skips the properties iteration; `commitImmediately ` skips its fire-fields step.
     - If `disableContentChannel `: `scheduleContentSave ` throws; `mergeServerResponse ` skips the content iteration; `commitImmediately ` skips its fire-content step.
     - If `disableRelationsChannel `: `scheduleRelationsChange ` throws; `attachRelations ` and `fireRelations ` short-circuit.
2. **`EntityDetail ` checkbox refactor**:
   - Instantiate a content-only `useAutoSave ` instance with `contentDebounceMs: 100, disablePropertyChannel: true, disableRelationsChannel: true `.
   - `applyServerContent ` writes back to `viewData.value ` preserving the entry-content section's `.content ` mirror (same shape as today's L230-233 splice).
   - `applyServerProperty ` and `buildRelationsBody ` are no-op closures (or omitted when channel is disabled).
   - `handleCheckboxToggle ` becomes ~10 lines (toggler-throws guard + `scheduleContentSave ` call).
   - Remove `togglingIndices ` Set.
3. **Tests**:
   - `useAutoSave.test.ts `: extend with per-channel debounce, initial snapshot, channel disable cases.
   - `EntityDetail ` content-channel test: existing tests in `EntityDetail.test.ts ` (if any cover checkbox toggle) updated; new test for the 100ms timing.
   - `e2e/tests/checkboxes.spec.ts `: read first; should pass unchanged.

**Alternatives considered:**

- *Rename `debounceMs ` to `fieldDebounceMs `.* Rejected — breaks DynamicForm tests; back-compat alias is the right answer (RR-UE3E).
- *Don't extend `useAutoSave `; write a smaller composable for content-only.* Rejected — adds a parallel surface to maintain.
- *Keep `togglingIndices `.* Rejected — the FIFO chain is strictly stronger.

**Files to modify:**

- `frontend/src/composables/useAutoSave.ts ` (API extension)
- `frontend/src/composables/useAutoSave.test.ts ` (new tests)
- `frontend/src/components/entity/EntityDetail.vue ` (refactor handleCheckboxToggle; remove togglingIndices)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Content from `toggleCheckboxInSource `*: same as today. The toggler is narrower than the renderer; throws are caught and surfaced as toast.
- *PATCH payload*: typed via existing `EntityPatch `. Server validates.
- *Disabled-channel `schedule* ` calls*: throw at the dev level (developer-error), not server-facing. Mistake during refactor is caught at test time.

**Security-Sensitive Operations:** None new. Reuses existing PATCH path through
`entitiesStore.update `.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

| AC | Test |
|---|---|
| Per-channel debounce | Construct with `fieldDebounceMs: 200, contentDebounceMs: 500 `; schedule both; assert each fires after the correct delay. |
| Legacy `debounceMs ` alias | Construct with `debounceMs: 300 `; both channels use 300ms. |
| Per-channel overrides alias | Construct with `debounceMs: 300, contentDebounceMs: 100 `; field uses 300, content uses 100. |
| `initialServerSnapshot ` | Construct with `initialServerSnapshot: { foo: 'a' } `; schedule `foo = 'a' `; assert no PATCH. |
| Channel disable: properties | Construct with `disablePropertyChannel: true `; assert `scheduleFieldSave ` throws. |
| Channel disable: content | Construct with `disableContentChannel: true `; assert `scheduleContentSave ` throws. |
| Channel disable: relations | Construct with `disableRelationsChannel: true `; assert `scheduleRelationsChange ` throws AND `buildRelationsBody ` closure is never called. |
| `mergeServerResponse ` respects disable | Construct with `disablePropertyChannel: true `; trigger `mergeServerResponse ` with property updates; assert `applyServerProperty ` is never called. |
| EntityDetail checkbox flow | Mock `updateEntity `; click checkbox; assert ~100ms wait then PATCH; assert `viewData.entry.content ` updated from response. |
| `togglingIndices ` gone | Rapid click on different checkboxes; assert only one PATCH in flight at a time (FIFO chain serialization). |
| Existing e2e | `e2e/tests/checkboxes.spec.ts ` passes. |

**Edge Cases:**

- Toggler throws (unsupported bullet) → toast; no PATCH; no channel state change.
- Click during in-flight PATCH → queued via FIFO chain; second click's content overwrites the first's pending value.
- `useAutoSave ` instantiated with `disablePropertyChannel + disableContentChannel + disableRelationsChannel: true ` → constructs OK; all `schedule* ` throw; effectively unusable. Allow but document.
- `useAutoSave ` instantiated with `initialServerSnapshot ` AND a `recordServerSnapshot ` call after → second call overrides the first. Document precedence.

**Negative Tests:**

- `scheduleFieldSave ` on a disabled property channel throws with a clear error message.
- Setting both `debounceMs ` and `fieldDebounceMs ` → per-channel wins (test asserts).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated — `s `

**Risks:**

| Risk | Mitigation |
|---|---|
| Existing checkbox e2e test asserts a tight timing window | Read the test BEFORE refactor. If 100ms is too long, tune to 50ms or accept the small change. |
| Back-compat alias for `debounceMs ` introduces subtle precedence bugs | Explicit unit tests for precedence (per-channel wins). |
| `useAutoSave ` internal code already uses `debounceMs ` in multiple places | Audit during implementation; refactor each use to consult the per-channel resolver. |
| Channel disable flags interact with `commitImmediately ` in unexpected ways | Test `commitImmediately ` returns `{ settled: true } ` for disabled channels (they have nothing to fire). |

## Documentation Planning

- [x] User-facing docs identified — N/A
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: code-internal refactor + composable extension, no user-facing docs or guides affected)

**Documentation Impact:**

- N/A — internal API extension + internal refactor. No user-facing surface.

## Design Review

- [x] Run `/design-review ` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 10 findings captured as RR-FA1A through RR-FA1J. 0
critical, 3 significant (all addressed), 5 minor (all addressed), 2 nit (1
addressed, 1 deferred).

- **Significant** (3, all addressed in ACs above): RR-FA1A (required callbacks + runtime assert), RR-FA1B (lastSeenServer baseline preserved under disable), RR-FA1C (contentRef read-only-shape confirmed)
- **Minor** (5): RR-FA1D (precedence test), RR-FA1E (named error message), RR-FA1F (e2e timing note), RR-FA1G (commitImmediately no-guard), RR-FA1H (jsdoc on initialServerSnapshot)
- **Nit** (1 addressed, 1 deferred): RR-FA1I (commit-message lineage line — see below), RR-FA1J (contentRef cleanup — deferred)

**Implementation commit message note** (RR-FA1I): include a final line `"Split
from TKT-IHCY7; sibling slices TKT-IHC7B (properties inline edit) and TKT-IHC7C
(cards/list)." ` to preserve lineage from `git log `.
