---
id: IMPL-MNPA55
type: implementation-checklist
title: 'Implementation: Render admin-authored header/footer markdown on list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: rendering is `v-if` + `v-html` of a unit-tested computed; a full EntityList mount would require heavy Pinia-Colada/router/SSE mocking to test framework glue. Covered by helper unit tests + live `/_config` verification + built-bundle check instead.)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (renderMarkdown returns '' for empty; `v-if` guards absent regions)

## Test Quality

- [x] Using a `list(over)` fixture builder for ListConfig test data
- [x] No hardcoded values where an object is in scope
- [x] Only specifying values that matter per test

## Manual Verification

- [x] Feature verified end-to-end (API + build)
- [x] Each acceptance criterion verified
- [x] Edge cases verified

**Verification Evidence:**

Changes:
- Go: `List` gains `Header`/`Footer` (`internal/dataentryconfig/config.go`), both `omitempty`. Frontend does the `header || description` precedence, so no Go alias mutation of shared config.
- TS: `ListConfig` gains `header?`/`footer?`; `listHeaderMarkdown`/`listFooterMarkdown` helpers (`config.ts`) resolve precedence.
- Vue: `EntityList.vue` renders two `v-if`-guarded `.list-info` regions via `renderMarkdown()` (sanitized), with scoped CSS.
- Example: `tickets/data-entry.yaml` `all_ideas` list now uses `header`/`footer`.

Evidence:
1. **AC1/AC2 (header/footer served):** `curl -H "Origin: …" /api/v1/_config` on the tickets project returns `all_ideas.header` = "Browse all captured ideas… weekly review." and `all_ideas.footer` = "*Tip: use the filters…*". `description` = null.
2. **AC3 (sanitization):** reuses `renderMarkdown` (DOMPurify); existing `markdown.test.ts` covers `<script>`/`onerror` stripping.
3. **AC4/AC5 (absent → no region; precedence):** `config.test.ts` — 14 pass, incl. header-wins-over-description, empty-header→description fallback, undefined→'' for both slots. Go `config_test.go` — YAML/JSON round-trip + omitempty-when-unset pass.
4. **Build:** `npm run build` → `static/v2/assets/ListView-*.{js,css}` contains `list-info--top/--bottom` classes + scoped `.list-info` CSS rules; render code compiled in.
5. **Config validity:** `rela --project tickets validate` → data-entry.yaml valid.

Browser screenshot verification was attempted but the Claude-in-Chrome extension
was not connected; substituted API + built-bundle verification.

## Quality

- [x] Follows project patterns (mirrors EntityDetail.vue's `markdown-content` + eslint-disable `vue/no-v-html`; helper co-located with `getEditFormId` in config.ts)
- [x] DRY: precedence extracted to `listHeaderMarkdown`/`listFooterMarkdown` (also makes it unit-testable without mounting)
- [x] No security issues (only admin YAML input, still DOMPurify-sanitized; no new egress)
- [x] No silent failures
- [x] No debug code left behind
- [x] `npm run typecheck` clean; `eslint` clean (pre-existing `max-lines` warning on EntityList only); Go `go test ./internal/dataentryconfig/ ./internal/dataentry/` pass
