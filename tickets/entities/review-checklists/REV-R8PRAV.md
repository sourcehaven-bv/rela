---
id: REV-R8PRAV
type: review-checklist
title: 'Review: Ship a generated operator handbook for the demo tracker (dogfood rela-docs)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — 75 pkgs, 0 fail; docs seam tests green
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained — docs 78.2%; `just coverage-check` PASS (total 76.4%)
- [x] `go vet` clean; markdownlint on handbook + README → 0 issues

## Code Review

- [x] cranky-code-reviewer reviewed the renderer seam fix
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed (RR-L5GWLU)
- [x] Self-reviewed the diff

**Review Responses:** RR-L5GWLU (significant, addressed) — the first fix only
covered the island side of a seam; replaced with a `seamWriter` covering both
sides + mid-line echo + literal-interior preservation, with tests pinning all
cases.

## Acceptance Verification

- [x] Each acceptance criterion tested

**Acceptance Status:**
1. Handbook builds via `rela-docs build --project prototypes/data-entry/project` → committed `docs/examples/ticket-tracker-manual.md` + PNG — **PASS**.
2. Renders field table + mermaid lifecycle (transition labels) + editor/viewer roles matrix + annotated screenshot — **PASS** (verified visually).
3. README links it; markdown lint 0 issues — **PASS**.
4. rela's own intro intact (README still about rela; handbook is a linked example) — **PASS**.

## Documentation

- [x] The deliverable IS documentation; README links the handbook; `docs/rela-docs.md` guide unchanged.

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs
- [x] Ready to use — `just docs-example` regenerates it

## Pull Request

- [x] Work committed on the branch; will ride with the rela-docs arc PR (#1187) since it depends on the rela-docs binary.
- [x] CI: covered by #1187's pipeline once pushed.

**PR:** rides with #1187 (rela-docs arc); this ticket depends on the rela-docs
binary landing.
