---
id: REV-7N0L8X
type: review-checklist
title: 'Review: Render admin-authored header/footer markdown on list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend `npm run test:run`: 72 files, 1170 tests pass. Go `go test ./internal/dataentryconfig/ ./internal/dataentry/`: pass. `go build ./...` clean.
- [x] Lint clean — `eslint` on changed files: 0 errors (pre-existing `max-lines` warning on EntityList.vue only, unrelated). `golangci-lint ./internal/dataentryconfig/...`: 0 issues. `npm run typecheck`: clean. `rela --project tickets validate`: data-entry.yaml valid.
- [x] Coverage maintained — change adds tests (Go round-trip + 16 frontend helper tests); `dataentryconfig` floor (70%) only improves.

## Code Review

- [x] Ran cranky-code-reviewer on the diff — no critical findings.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — RR-B6NDXK (dead markdown-content class) → removed.
- [x] Self-reviewed the diff for unrelated changes — only the 6 intended source files + docs.

**Review Responses:** RR-B6NDXK (significant, addressed), RR-GW5I5R (minor,
addressed), RR-R1LAKE (nit, addressed), RR-PUIE0H (minor, addressed), RR-RFRS5L
(nit, wont-fix — not a regression, cross-cutting SPA-link concern out of scope).

## Acceptance Verification

**Acceptance Status:**
1. Header markdown renders above filter row — **PASS**: `/_config` serves `all_ideas.header`; region rendered via `v-if="headerHtml"` after `</header>`; `list-info--top` in built CSS.
2. Footer markdown renders below table/pagination — **PASS**: `all_ideas.footer` served; region after `<Pagination>`; `list-info--bottom` in built CSS.
3. Sanitized (no injection) — **PASS**: reuses `renderMarkdown` (DOMPurify); `markdown.test.ts` covers `<script>`/`onerror`.
4. No header/footer → renders as before, no layout shift — **PASS**: `v-if` guards + omitempty; unit tests assert `''` when unset.
5. Legacy `description` still works, `header` wins when both set — **PASS**: `listHeaderMarkdown` precedence + trim; 16 unit tests incl. whitespace-only fallback.

## Documentation (enhancements only)

- [x] User-facing documentation updated — `docs/data-entry.md`: List Fields table gains `header`/`footer`/`description`; new "Header and footer info regions" subsection with example + sanitization/precedence notes.
- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs updated inline within this ticket; the single doc change is verified above.)

## Final Checks

- [x] Commit message will explain the why (in-context list guidance) not just what.
- [x] No TODOs/FIXMEs left.
- [x] Ready for another developer to use — documented, example config in-tree.

## Pull Request

- [x] PR created on branch `feat/list-info-regions`; CI monitored.

**PR:** see branch `feat/list-info-regions`
