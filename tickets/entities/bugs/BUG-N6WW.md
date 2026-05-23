---
id: BUG-N6WW
type: bug
title: Markdown checkboxes in entity content are no longer clickable
description: |-
    GFM task-list checkboxes rendered inside an entity's content body no longer toggle when clicked. The breakage was three layered bugs that combined to silently no-op every click. The fix lands in `frontend/src/utils/markdown.ts`, `frontend/src/components/entity/EntityDetail.vue`, and `frontend/src/api/entities.ts`.

    1. **Brittle regex post-process** in `markdown.ts` looked for `<input type="checkbox" ...>` to inject `data-cb-idx`. Marked v17 emits `<input disabled="" type="checkbox">` (disabled first), so the regex never matched and the attribute was never added; the click-handler installer in `EntityDetail.vue` queried `input[type="checkbox"][data-cb-idx]` and found nothing.
    2. **Disabled input** in marked's default checkbox HTML meant that even with a JS listener attached, the browser would not always dispatch the click — clicks on a disabled input are silently swallowed.
    3. **Multipart POST body** in `toggleCheckbox` used `FormData`, producing `multipart/form-data`. The server handler calls `r.ParseForm()` first which sets `r.Form` to URL params only, so the lazy `ParseMultipartForm` inside `r.FormValue` never fires and `entity_id`/`index` are empty — the handler returns 400 "Invalid checkbox index".

    Fix:
    - Replace the regex with a `marked.Renderer.checkbox` hook that emits `<input data-cb-idx="N" type="checkbox"[ checked=""]>` (no `disabled`). The counter is closed over the renderer instance, so each `renderMarkdown` call gets its own index sequence.
    - Switch `EntityDetail.vue` to delegated click handling on `contentRef` via `@click="contentClick"`. The previous `setupCheckboxHandlers` re-attach pattern raced the v-html DOM update (the watch's `nextTick` fired before the section's `v-if` rendered) and silently attached zero handlers on first mount.
    - Send `URLSearchParams` from `toggleCheckbox` so the server's `r.FormValue` reads `entity_id` and `index` from the body.

    Verification: existing `e2e/tests/checkboxes.spec.ts` had a `test.skip` describing this as test-harness-only; that diagnosis was wrong. Unskipping shows real product breakage — and with the three fixes above the test passes.
priority: medium
why1: Clicks on rendered checkboxes never fire the toggle handler, so no API call is made.
why2: EntityDetail.setupCheckboxHandlers selects only `input[type="checkbox"][data-cb-idx]`; no element matched, so no listener was attached.
why3: renderMarkdown's regex for injecting `data-cb-idx` requires `type="checkbox"` as the first attribute, but marked v17 now emits `disabled="" type="checkbox"` (disabled first), so the regex never matched.
why4: The marked v17 attribute-order change made the regex stop matching, and marked's default `disabled` attribute on rendered checkboxes was always blocking real clicks even before the regex broke. The handler attachment also raced the v-html DOM update.
why5: The toggle code defended against future fragility (regex post-process, imperative handler attachment, multipart body) without any test that exercised the full click→API→reload path end-to-end. The e2e test that would have caught this was explicitly `test.skip`-ed with a misdiagnosis ("test-harness only"). Verification gates that don't exercise the bug condition cannot detect drift.
prevention: '1) The un-skipped e2e test (now asserting both server-side content AND rendered checkbox state) closes the regression surface. 2) Replaced the brittle regex with a documented `marked.Renderer.checkbox` hook — marked''s published extension contract, not string parsing of its output. 3) Replaced imperative DOM-handler attachment (which raced render timing) with template-level `@click` delegation — Vue handles the timing. 4) `renderMarkdown` now has an explicit `interactive` flag, so non-toggle call sites can''t accidentally render fake-interactive checkboxes if a future caller forgets to pass it. 5) Lesson: when a test is `.skip`-ed citing ''test-infra only'', the diagnosis itself needs verification — the original BUG-9RANL skip masked a real product regression for an unknown duration.'
status: done
---
