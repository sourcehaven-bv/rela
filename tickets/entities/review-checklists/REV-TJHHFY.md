---
id: REV-TJHHFY
type: review-checklist
title: 'Review: Enum values support a display label/title for better UX on snake_case values'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — Go: all `internal/...` packages ok; Frontend: 1195 vitest tests pass
- [x] Lint clean (`just lint`) — golangci-lint 0 issues on changed packages; ESLint 0 errors in changed files; `just arch-lint` OK
- [x] Coverage maintained (`just coverage-check`) — package floor + total (77.2%) PASS

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer) — 2 design-review rounds + 1 code-review round
- [x] All critical review-responses addressed (RR-TRMD4O)
- [x] All significant review-responses addressed (RR-L6XI6S, RR-UZ6Q3G, RR-UAV796, RR-I57OBI, RR-OAQMDM, RR-BPFGG6)
- [x] Self-reviewed the diff for unrelated changes — no unrelated changes; frontend build artifacts gitignored

**Review Responses:**

- Design review (plan): RR-TRMD4O (critical), RR-L6XI6S, RR-UZ6Q3G, RR-UAV796, RR-I57OBI (significant), RR-O2JF23, RR-KQ1DXS (minor) — all addressed
- Code review (impl): RR-OAQMDM, RR-BPFGG6 (significant, addressed), RR-20371R (minor, addressed), RR-6R4P6A (nit, addressed), RR-30SVO6 (nit, deferred — documented known limitation)

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist (IMPL-MFE4M0)

**Acceptance Status:**

1. Named custom type labels → **PASS** (`TestV1SchemaEnumLabels`, live `_schema`; `Badge.test.ts` custom-type resolution).
2. Inline enum labels → **PASS** (`TestParse_EnumLabels`, `TestV1SchemaEnumLabels`, `widgets.test.ts`).
3. Badge shows label, color keyed on value → **PASS** (`Badge.test.ts` 'keeps color styling keyed on the raw value').
4. No-label enum unchanged → **PASS** (`Badge.test.ts` fallback; `TestV1SchemaEnumLabelsOmittedWhenAbsent`).
5. Existing string-list metamodels load, no migration → **PASS** (`TestParse_EnumLabels` nil-Labels; omitempty; no migration added).
6. Validation value-based → **PASS** (validation.go untouched; live API stored raw values).
7. Kanban/filter/edit surfaces show labels, raw value preserved → **PASS** (`KanbanView.test.ts`, `AdHocFilterMenu.test.ts`).
Reach guard (OpenAPI enum value-only) → **PASS**
(`TestGenerator_EnumLabelsDoNotLeakIntoEnum`, live `_openapi.json`).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-WZ3NAD)
- [x] User-facing documentation updated (`docs/metamodel.md` enum Display Labels section)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-WZ3NAD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
