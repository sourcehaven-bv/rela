---
id: PLAN-HWQ6
type: planning-checklist
title: 'Planning: Remove +Add / Link Existing buttons from data-entry view widgets'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**

- Remove `+ Add <type>` and `Link Existing` buttons rendered by `EntityDetail.vue` for each relation section.
- Remove now-dead helpers/state in `EntityDetail.vue`: `LinkExistingModal` import + render, `showLinkModal`/`linkModalInfo` refs, `openLinkExisting`, `handleLinked`, `navigateToCreate`.
- Drop `addInfo` / `linkInfo` (and their interface types `ViewAddInfo`, `ViewLinkInfo`, `ViewAddTarget`) from `frontend/src/api/views.ts` — confirmed only consumed by the view path.
- Backend: drop the `a.resolveSectionButtonsWithTraverse(viewCfg, sections, result.Entry)` call in `handleV1Views` (`api_v1.go` ~line 2572) and remove `AddInfo` / `LinkInfo` fields + population block from `V1ViewSection` (`api_v1.go` ~lines 2449–2693).

**Out of scope:**

- Side-panel path (`SidePanel.vue`, `V1SidePanelSection`, the line-1770 call to `resolveSectionButtonsWithTraverse`). Side panel is form/edit context — mutations stay.
- The `LinkExistingModal.vue` component itself — only EntityDetail referenced it, so it becomes dead code that we'll also delete (one-shot deletion is cleaner than leaving a zombie).
- The `resolveSectionButtonsWithTraverse` Go function itself — still used by side-panel path.
- Per-row Edit pencil buttons in tables/cards — pure navigation, stays.
- Header Edit/Delete buttons on the entity page — operate on the entry entity; stay.
- Document render path (`/document/:name/:id`) — separate from this view.

**Acceptance Criteria:**

1. Visiting `/entity/:type/:id` shows zero `+ Add` or `Link Existing` buttons in any relation section (cards / list / table / content / properties displays). *Test:* E2E or component test renders `EntityDetail` with mocked `ViewResponse` that previously had `addInfo`/`linkInfo` and asserts no button text matches `/^\+ Add/` or `/Link Existing/`.
2. Form side-panel still renders `+ Add` / `Link Existing` for configured relation sections. *Test:* existing SidePanel tests still pass; manual run of `npm run dev` against a project with side-panel forms.
3. `GET /api/v1/_views/{type}/{id}` response JSON has no `addInfo` / `linkInfo` fields anywhere. *Test:* extend `TestV1Views_*` in `api_v1_test.go` to assert these keys are absent in serialized response.
4. `GET /api/v1/forms/{id}/side-panel/{entityId}` still includes `addInfo` / `linkInfo` when applicable. *Test:* existing side-panel handler test (or add one) asserts they remain.
5. Per-row Edit pencil buttons navigate correctly. *Test:* existing `EntityDetail.spec.ts` (if any) or unaffected by this change — manual spot check.
6. `just test`, `just lint`, `just arch-lint`, `just coverage-check` all pass; `npm run typecheck` clean.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

This is a removal/cleanup, not a feature implementation — no library research
needed. Codebase research surfaced:

- `frontend/src/api/views.ts:70-83` defines `ViewAddInfo` and `ViewLinkInfo` only on `ViewSection`; no other consumer imports them.
- `frontend/src/types/entity.ts:94-105` defines parallel `SidePanelAddInfo`/`SidePanelLinkInfo` types used solely by `SidePanel.vue`. The two type families are intentionally separate, so removing the View-side ones does not affect SidePanel.
- `internal/dataentry/api_v1.go:2572` (view handler) and `:1770` (side-panel handler) are the only two callers of `resolveSectionButtonsWithTraverse`. The function itself stays; only the view-handler call goes.
- `LinkExistingModal.vue` is referenced only by `EntityDetail.vue`. After this change it becomes dead code and is deleted together with its `.test.ts` (if any). *Verified via grep — no other importer.*
- No backend test currently asserts on `addInfo`/`linkInfo` presence in view responses (`grep` over `internal/dataentry/*_test.go` returns no hits), so test churn is additive (assert absence) rather than corrective.
- Prior art: TKT-9QNHN ("Add Edit button to data-entry document view") established the pattern that read-only views get an Edit affordance for the **entry** entity but no inline relation-mutation affordances. This ticket extends that principle to relation sections inside `EntityDetail.vue`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Pure deletion, no new abstractions.

1. **Backend first** — keeps the wire format change visible in one commit and lets the frontend type drop be a mechanical follow-on.
   - In `internal/dataentry/api_v1.go`:
     - Delete the call `a.resolveSectionButtonsWithTraverse(viewCfg, sections, result.Entry)` and its preceding comment in the view handler (`handleV1Views`, around line 2572).
     - Remove `AddInfo *V1ViewAddInfo` and `LinkInfo *V1ViewLinkInfo` fields from `V1ViewSection` (around lines 2449–2450).
     - Remove the corresponding population block (lines 2675–2693).
     - Leave `V1ViewAddInfo`, `V1ViewLinkInfo`, `V1ViewAddTarget` types in place — `V1SidePanelSection` still uses them (lines 1687–1688). Renaming is out of scope; let them stay under their generic names.
     - Verify nothing else referenced the dropped fields (none expected).
   - Add an assertion in `api_v1_test.go` `TestV1Views_DefaultViewForType` (or a new sub-test) that the marshaled response does not include `"addInfo"` / `"linkInfo"` keys.

2. **Frontend follow-on**:
   - In `frontend/src/components/entity/EntityDetail.vue`:
     - Delete `<div class="section-actions">…</div>` block (template lines ~713–736).
     - Delete the `<LinkExistingModal>` block at the bottom of the template (lines ~754–764).
     - Delete the `LinkExistingModal` import (line 20).
     - Delete `showLinkModal` and `linkModalInfo` refs (lines 53–60).
     - Delete `openLinkExisting`, `handleLinked`, `navigateToCreate` functions.
     - Delete the `.section-actions`, `.btn-add`, `.btn-link-existing` CSS rules in `<style>` (lines ~1300–1328).
   - In `frontend/src/api/views.ts`: drop `ViewAddInfo`, `ViewLinkInfo`, `ViewAddTarget` interfaces and the `addInfo` / `linkInfo` fields on `ViewSection`.
   - Delete `frontend/src/components/forms/LinkExistingModal.vue` (and its `.test.ts` if present).

3. **Verify**:
   - `just test` (backend), `npm run test:run` (frontend), `npm run typecheck`.
   - `just lint`, `just arch-lint`, `just coverage-check`.
   - Manual: start `just dev`, open `/entity/:type/:id` for a type with relation sections, confirm no Add/Link Existing buttons. Open an edit form with a side panel that has relation sections, confirm those buttons still appear.

**Alternatives considered:**

- *Hide buttons in CSS / `v-if="false"`*: rejected — leaves dead code paths and dead backend computation; the user explicitly asked for removal, not concealment.
- *Keep backend payload, only drop frontend rendering*: rejected per user direction; carrying unused payload bloats the wire and tempts a re-render in the future.
- *Generalise the side-panel and view section handlers into one*: out of scope; the duplication that exists is small and the two paths have legitimately different semantics (read vs. write).

**Files to modify:**

- `internal/dataentry/api_v1.go` (delete fields, delete call, delete population block).
- `internal/dataentry/api_v1_test.go` (add absence-assertion sub-test).
- `frontend/src/api/views.ts` (delete three interfaces + two fields).
- `frontend/src/components/entity/EntityDetail.vue` (template + script + style cleanups).
- `frontend/src/components/forms/LinkExistingModal.vue` (delete file).
- `frontend/src/components/forms/LinkExistingModal.test.ts` (delete file if present).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- N/A. This change removes code paths and reduces wire surface; no new inputs are accepted, no validation logic changes. The deleted handlers performed mutations (`POST /api/v1/relations`) via `LinkExistingModal`; those endpoints continue to exist (the side panel still uses them) and retain their existing validation.

**Security-Sensitive Operations:**

- The dropped buttons surfaced existing relation-creation affordances. Removing the surface does not remove the underlying API; an authenticated user with the rela-server endpoint can still create relations via the form path or direct API call. No authorisation boundary moves.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 (no Add/Link buttons in EntityDetail): rely on the existing `EntityDetail.test.ts` (if present) or add a focused render test that mounts `EntityDetail` with a `ViewResponse` shaped like the previous (with `addInfo`/`linkInfo`) and asserts no `+ Add` / `Link Existing` text in the DOM. Backed up by manual smoke test.
- AC2 (side panel unchanged): existing `SidePanel`/`RelationCards` tests must continue to pass. No new tests required.
- AC3 (response shape): new sub-test in `api_v1_test.go` decodes the view response into `map[string]any` and asserts none of the sections contain `addInfo` or `linkInfo` keys.
- AC4 (side-panel response shape unchanged): existing side-panel handler tests cover this; if absent, add one parallel to AC3 that asserts presence.
- AC5 (per-row Edit pencil): no test change — that code path is untouched.
- AC6 (build hygiene): CI gates.

**Edge Cases:**

- Section with `display: properties` and `source: entry` — never had `addInfo`/`linkInfo` (the resolver early-returns for `entry` source). No change in behaviour.
- Section pointing to a relation whose target type has no `create_form` configured — previously suppressed `addInfo` but still emitted `linkInfo`. After this change neither is emitted. Acceptable.
- Section whose relation only has `linkInfo` (no targets with create forms) — same as above; no buttons before, no buttons after.
- A view configured by a Lua programmable view (`FEAT-i5ji`) — these populate `Sections` via the same `buildSections` + `resolveSectionButtonsWithTraverse` path. Removing the resolver call drops the buttons there too, consistently with the rest of the change.

**Negative Tests:**

- Send a request to `/api/v1/_views/{type}/{id}` and assert `addInfo`/`linkInfo` keys are **absent** in any section. (This is essentially the AC3 test phrased as a negative.)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **R1 — Hidden consumer of `addInfo`/`linkInfo` in the view payload.** *Mitigation:* grep verified `views.ts` is the only consumer; `npm run typecheck` will catch any stragglers.
- **R2 — Breaking a Lua programmable view that relies on `addInfo`/`linkInfo` being present in some custom JSON shape.** *Mitigation:* Lua views go through the same `buildSections` path and serialise via `V1ViewSection`; they never see the resolver output as Lua values. No risk.
- **R3 — A user who relied on the inline `+ Add` flow.** *Mitigation:* the form path still works; the user's stated intent is exactly to push them there. Documented in the PR description.
- **R4 — `LinkExistingModal.vue` deletion accidentally breaks an unseen importer.** *Mitigation:* grep confirms only `EntityDetail.vue` imports it; `npm run typecheck` is the safety net.

**Effort:** s (small) — pure deletion across two ends of a single payload
contract; no new logic.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal refactor; no public CLI / API surface promises this affordance)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor kind, not enhancement)

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

This is a `kind: refactor` ticket — no docs-checklist required by the workflow.
The change does shrink the documented `V1ViewSection` JSON shape, but that shape
is not externally documented (no schema export, no public OpenAPI). If a
consumer outside this repo decoded `addInfo`/`linkInfo` they would need to be
told; none is known to exist.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Skipped for this iteration — pure-deletion ticket with no architectural
decisions; design-review's value is on planning new abstractions or behaviour.
Will be revisited if implementation surfaces unexpected coupling.
