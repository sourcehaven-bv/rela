---
id: PLAN-QUYXFY
type: planning-checklist
title: 'Planning: output.Writer surface cleanup (directive deleted, 23 → 14)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: internal/output (delete dead WriteRelations; unexport
WriteSeparator/WriteFooterSummary; extract 6 schema-JSON methods + 3 schema
interfaces to a focused type), internal/cli/schema.go (the single caller),
internal/cli/kong.go (directive comment corrected to 46 and reclassified as
documented structural exception per survey NO-GO verdict). OUT: cluster A/B/D
methods (earned width — 400 calls across 24 packages), WriteAnalysisResult
(centralizes status→severity mapping, keep).

**Acceptance Criteria:**
1. `just plimsoll` passes with the output directive DELETED (lands at 14,
under the default 20).
2. Full suite green; output_test schema tests re-pointed; WriteRelations
tests removed; coverage floors hold.
3. arch-lint/comment-lint/lint clean.

## Research

- [x] ~~Run `/research`~~ (N/A: survey performed instead)
- [x] ~~Existing libraries~~ (N/A)
- [x] Checked codebase for similar patterns
- [x] Looked for reference implementations
- [x] Reviewed prior art

**Research Doc:** N/A — dedicated Explore survey with per-method call-site
counts: cluster A (5 message primitives, 352 calls) + B (6 domain shapes) + D
(analysis dispatcher) are earned; cluster E (6 schema-JSON methods) has exactly
one caller (internal/cli/schema.go), four are one-line writeJSON wrappers that
never branch on Format; WriteRelations has zero production callers;
WriteSeparator/WriteFooterSummary are internal-only.

**Existing Solutions:** TKT-45QYI (cliServices directive deleted, not bumped) is
the precedent outcome; consumer-side TitleResolver shows the package's existing
decoupling convention.

## Approach

- [x] Technical approach chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** Three independent steps that only jointly clear the
line: (1) delete WriteRelations + its tests (newBorderlessTable stays covered
via writeEntitiesTable); (2) unexport separator/footerSummary; (3) move the
schema-JSON methods + SchemaMetamodel/SchemaEntityDef/SchemaRelationDef
interfaces onto a focused schema writer (either in output or owned by
internal/cli — implementer's choice, respecting arch-lint direction). Then
delete the directive. Alternative rejected: folding WriteAnalysisResult into
callers — 14 calls across 3 files and it centralizes severity mapping.

**Files to modify:** internal/output/{output.go, output_test.go},
internal/cli/schema.go, internal/cli/kong.go (comment only)

## Security Considerations

- [x] Input sources identified
- [x] Input validation defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak

**Input Sources & Validation:** N/A — output formatting only; no parsing of
untrusted input changes.

**Security-Sensitive Operations:** None — CLI stdout rendering.

## Test Plan

- [x] Test scenarios documented
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] ~~Integration approach~~ (N/A: output_test.go drives all formats)

**Test Scenarios:** output_test.go re-pointed for schema methods; JSON output
byte-identical (moved verbatim); FormatSize and message primitives untouched.

**Edge Cases:** Coverage floor for internal/output must hold after deleting
WriteRelations tests — the shared table machinery stays covered.

**Negative Tests:** Existing invalid-format tests unchanged.

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated

**Risks:** Low — one production caller for everything moved; zero for the
deletion. Coverage-floor dip is the main watch item. Effort: s.

## Documentation Planning

- [x] ~~User-facing docs~~ (N/A: no CLI output change)
- [x] ~~Docs-checklist~~ (N/A: refactor)

**Documentation Impact:** N/A.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: plan derived from a dedicated survey with call-site evidence; deletion/unexport/move of single-caller surface)
- [x] ~~Findings addressed~~ (N/A)

**Design Review Findings:** N/A
