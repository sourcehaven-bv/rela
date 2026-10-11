---
id: PLAN-ONVLC0
type: planning-checklist
title: 'Planning: Comments name capabilities, not vendor libraries or backends'
started: "2026-10-10"
completed: "2026-10-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: reword comments that name a vendor library or storage backend the
package does not depend on (archplan glossary leaks, thread th-58000adf); rename
`store.FaceSelection.Lowered` to `FlattenedWhenUnranked` (th-fd62b77f). Out:
other identifier renames; comments in a package that wraps or imports the
vendor; build-tagged files; arch-lint grant text; string literals.

**Acceptance Criteria:**
1. Only comments change, plus the one rename. Test: diff check over added lines.
2. All three builds compile. Test: `go build` with no tag, `-tags postgres`, `-tags sqlite`.
3. Comment gates pass. Test: `just comment-lint`.
4. No `Lowered` identifier remains. Test: grep.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: comment rewording)
- [x] ~~Searched for existing libraries~~ (N/A: no new code)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: comment rewording)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The archplan glossary leak list (63 comment-only hits)
is the input; kept matches follow the rule "a package may name what it imports".

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Replace the vendor name by the capability ("the markdown
renderer", "the file watcher", "the database backends"). Rename `Lowered` with
its 6 call sites, 2 doc references and its test. Alternative rejected: a lint
rule for vendor names; too many legitimate hits in wrapping packages.

**Files to modify:** 74 files, comments only, across internal/ and cmd/;
internal/store/faceselect.go and the pgstore and sqlitestore callers for the
rename.

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: no behaviour change)
- [x] ~~Input validation approach defined~~ (N/A: no behaviour change)
- [x] ~~Security-sensitive operations identified~~ (N/A: no behaviour change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no behaviour change)

**Input Sources & Validation:** N/A **Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: no behaviour change)
- [x] ~~Negative test cases defined~~ (N/A: no behaviour change)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; plus `just test` for the rename.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed~~ (N/A: comments only)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A reworded comment loses a useful pointer. Mitigation: the uncertain
hits were reviewed by hand. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal comments)
- [x] ~~Docs-checklist will be created~~ (N/A: chore, no user docs)

**Documentation Impact:** N/A

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: comment rewording and one rename; no design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
