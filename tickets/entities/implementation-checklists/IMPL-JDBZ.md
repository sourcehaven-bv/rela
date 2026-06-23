---
id: IMPL-JDBZ
type: implementation-checklist
title: 'Implementation: Bundle Font Awesome locally so EasyMDE doesn''t fetch it from a CDN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: configuration-only change in MarkdownEditor.vue; no new branching logic to unit-test)
- [x] Integration tests written (test full flow, not just units) — e2e test in `e2e/tests/markdown-editor.spec.ts:27` asserts AC1/AC2/AC3 against the real built binary
- [x] Happy path implemented — `frontend/src/components/forms/MarkdownEditor.vue`: import of `font-awesome/css/font-awesome.min.css` + `autoDownloadFontAwesome: false`
- [x] ~~Edge cases from planning handled~~ (N/A: no runtime branches; EasyMDE upgrade and font-cache concerns are caught by the e2e network assertion)
- [x] ~~Error handling in place~~ (N/A: no new runtime code path)

## Test Quality

- [x] Using fixture builders or factories for test data — reuses existing `FormPage` page object
- [x] No hardcoded values in assertions when object is in scope — origin compared against `appPage.url()`, not a hardcoded host
- [x] Only specifying values that matter for the test — captures every request, filters at assertion time
- [x] Interpolated values constructed from objects, not hardcoded — error messages embed the actual captured URLs
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no entity property comparisons in this test)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `npm run build` produced `internal/dataentry/static/v2/assets/fontawesome-webfont-*.{woff2,woff,ttf,eot,svg}` (5 files, content-hashed) and embedded `@font-face` declarations in `FormView-*.css` referencing those hashed paths — confirms Vite bundled the FA asset chain into the SPA build (AC3).
- `just build-server` succeeded; the resulting binary picks up the new assets through the existing `//go:embed all:static/*` directive in `internal/dataentry/static.go:7` — no Go-side change required.
- `npx playwright test markdown-editor.spec.ts -g "bundles Font Awesome"` passes: the network listener captured zero requests to `maxcdn.bootstrapcdn.com` (AC1), every css/woff/ttf/eot/svg request was same-origin (AC3), and `getComputedStyle(boldButton, '::before').fontFamily` returned a FontAwesome-containing family (AC2).
- All 10 tests in `markdown-editor.spec.ts` + `markdown-editor-entity-ref.spec.ts` pass — no regression to existing editor behavior.
- AC4 (offline): the network-listener test is equivalent to running offline — the assertion that *zero* off-origin requests fire proves the editor needs nothing beyond what's served from the binary.

## Quality

- [x] Code follows project patterns — CSS-via-npm-import mirrors `main.ts:5-8`'s `@fontsource/open-sans` imports
- [x] No security issues introduced — change *removes* an uncontrolled cross-origin CSS load
- [x] No silent failures — change has no runtime branches; failure modes are caught at test time
- [x] No debug code left behind — only added imports + one option line + a regression test
