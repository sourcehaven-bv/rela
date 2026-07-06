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
- [x] Self-reviewed the diff for unrelated changes — only the intended source files + docs.

**Review Responses:** RR-B6NDXK (significant, addressed), RR-GW5I5R (minor,
addressed), RR-R1LAKE (nit, addressed), RR-PUIE0H (minor, addressed), RR-RFRS5L
(nit, wont-fix — not a regression, cross-cutting SPA-link concern out of scope).

## Acceptance Verification

**Acceptance Status:**
1. Header markdown renders above filter row — **PASS**.
2. Footer markdown renders below table/pagination — **PASS**.
3. Sanitized (no injection) — **PASS** (DOMPurify via renderMarkdown).
4. No header/footer → renders as before, no layout shift — **PASS** (`v-if` + omitempty).
5. Legacy `description` still works, `header` wins when both set — **PASS** (precedence + trim, 16 unit tests).

## Documentation (enhancements only)

- [x] User-facing documentation updated — `docs/data-entry.md`.
- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: single inline doc change verified above.)

## Final Checks

- [x] Commit message explains the why, not just what.
- [x] No TODOs/FIXMEs left.
- [x] Ready for another developer to use.

## Pull Request

- [x] PR created; CI monitored.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1091
