---
id: PLAN-8GCKM5
type: planning-checklist
title: 'Planning: seqtrace: text call trees and flow diffs for agents'
started: "2026-10-03"
completed: "2026-10-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: a step model shared by the Mermaid and text renderers; `.txt` call trees and
a `steps.json` per diagram directory; `seqtrace diff`; demo support for a git
ref; `just seqtrace-compare`. Out: CI integration (the demo needs postgres and
minutes per run), HTML rendering of diffs.

**Acceptance Criteria:**

1. Mermaid output is unchanged by the refactor (existing `TestRender*`).
2. The text tree shows one line per cross-package call with values, folds
repeats as `N×`, and marks async calls (`TestRenderText`).
3. `diff` reports added, removed and changed scenarios, call-count and
per-edge deltas, and a line diff of call shapes (`TestDiff`).
4. `just seqtrace-compare develop` runs both demos and prints the diff
(manual run; a no-op change shows every scenario unchanged).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: extends an existing tool)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Line diff: `sergi/go-diff` is only an indirect dependency and is
character-oriented; a small LCS over trimmed line lists avoids a new direct
dependency.
- Reuses the diagram package's folding and collapse rules (TKT-79WJ6G).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`diagram.Build` turns a call tree into a `Step` tree (from, to, fn, via, args,
results, repeat, children), applying collapse, depth and folding once. `Render`
(Mermaid) and `RenderText` read steps. The CLI writes `.txt` files and
`steps.json`; `diff` loads two `steps.json` files, matches scenarios by name,
and compares value-free shape lines. Diffing the `.txt` files directly was
rejected: stripping values from rendered text is fragile.

`run.sh` gains `SEQTRACE_REF` (populate the copy with `git archive`, then
overlay the current `tools/seqtrace`) and `SEQTRACE_LABEL` (output
subdirectory).

**Files to modify:**

`tools/seqtrace/diagram/*`, `tools/seqtrace/cmd/seqtrace/*`,
`tools/seqtrace/demo/run.sh`, `tools/seqtrace/README.md`, `justfile`,
`CLAUDE.md` (one line under Commands).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Operator flags and a git ref passed to `git archive` as one argument (no shell
interpolation). `steps.json` is produced by the same tool.

**Security-Sensitive Operations:**

Text trees contain the same summaries as the diagrams; the README's handling
guidance applies to them.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** scenarios only on one side; unnamed scenarios (matched by
handler and position); very long trees (diff falls back to a summary).

**Negative Tests:** a missing `steps.json` is an error naming the directory.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a refactor regression in Mermaid output; mitigated by keeping the
existing render tests unchanged.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `tools/seqtrace/README.md`; one line in `CLAUDE.md`
pointing agents at `just seqtrace-compare`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small extension of a reviewed tool; `/code-review` covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A
