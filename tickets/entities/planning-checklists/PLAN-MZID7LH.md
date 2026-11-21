---
id: PLAN-MZID7LH
type: planning-checklist
title: 'Planning: Rendered diff for the History view'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a rendered, block-aware body diff component in rela-components,
used by HistoryView and RelationHistoryView. Out: the property diff, a
side-by-side view, restore flow.

**Acceptance Criteria:**
1. Body changes render as formatted text with changed words highlighted (unit test on `diffBlocks`, story).
2. Lists and tables diff per item and per cell (unit tests).
3. Unchanged runs fold and the stepper moves between changes (`collapseUnchanged` tests, story check).
4. Raw HTML is escaped and unsafe links are dropped (unit tests).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: GitHub PR file diffs used as the model)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: UI only)

**Research Doc:** N/A

**Existing Solutions:** `marked` is already a dependency of the SPA
(`src/utils/markdown.ts`). The old line diff is `utils/lineDiff`, kept for the
"anything changed" check. Diff libraries were not needed: a small LCS covers
blocks, lines and words.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Lex both bodies with `marked` into top-level blocks and
match them by source with an LCS. Pair a run of removed and added blocks in
order. For paired paragraphs, headings, lists, quotes and tables, diff by line,
then by word, and wrap changes in private-use markers that survive `marked` and
become `<ins>`/`<del>`. Fold long unchanged runs. A stepper walks the changes.
Alternative rejected: diffing the ProseMirror document, which is far more work
for the same result.

**Files to modify:**
`packages/rela-components/src/components/doc/{renderedDiff.ts,RlRenderedDiff.vue,*.stories.ts,renderedDiff.test.ts}`,
`src/composables/{useMessages,messages.nl}.ts`, `src/index.ts`, `package.json`,
`scripts/check-purity.mjs`, `vite.config.ts`;
`src/views/{HistoryView,RelationHistoryView}.vue`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Entity bodies from the history API are untrusted
and injected with `v-html`. The renderer escapes raw HTML and allows only
http(s), mailto and relative link and image URLs.

**Security-Sensitive Operations:** None beyond `v-html`; the bodies are the ones
the reader may already see (the history API gates them).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Vitest unit tests in the library for `diffBlocks` and
`collapseUnchanged`; existing HistoryView tests; Storybook stories checked in a
browser.

**Edge Cases:** Empty sides, identical documents, a single unchanged block,
removed text containing markdown punctuation, trailing whitespace in highlights.

**Negative Tests:** `<img onerror>` and `javascript:` links render as text.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A sentence rewrite gives an interleaved word diff (accepted,
readable). Very large bodies make the O(n*m) LCS slow; bodies are small and the
work is per block and line.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - no user-facing docs name the history diff format

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: UI-only change, already built and shown in Storybook)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
