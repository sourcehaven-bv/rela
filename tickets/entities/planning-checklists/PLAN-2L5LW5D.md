---
id: PLAN-2L5LW5D
type: planning-checklist
title: 'Planning: Body task counts as a roll-up (rollup body_tasks)'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `rollup: { body_tasks: done|open|total }` on integer properties,
served wherever roll-ups are served (data-entry lists and GET, MCP, CLI). Out:
filter/sort on it (refused), storing counts, object-valued properties, frontend
changes.

**Acceptance Criteria:** See the ticket body; each maps to a test below.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: reuses the roll-up mechanism)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: line-level task detection needs no markdown parser dependency)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the task-list syntax itself is the reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `internal/rollup` (Column, Compile),
`visibility.Rollups.Values`, record roll-ups (TKT-VMSWSWP) as the precedent for
a non-relation source; `dataentry/rowcontent.go loadRows` for the per-face batch
read.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `metamodel.RollupDef` gains `BodyTasks` (`body_tasks`): done, open or total. The loader rejects it with via, type, where, records, window or another function, and requires type integer.
- `rollup.Compile` builds a column with no related type; `rollup.CountTasks` is the pure parser (list markers `-`, `*`, `+`, `1.`, `1)`; boxes `[ ]`, `[x]`, `[X]`; any indentation; fenced blocks skipped).
- `visibility.Rollups.Values` reads the served rows' bodies once per distinct face for all body columns of a type. A row whose body is marked inaccessible has no value; the existing `visible:` gate on the property applies.
- Filter and sort on a body-task roll-up are refused with 400: they would read every body of the type before paging.

**Threat model:** The asset is body text; the count discloses part of it. The
count is served only where the body is: rows arrive row-gated, the
inaccessible-body marker withholds the count, and a `visible:` grant on the
property hides it. Bodies are not `visible:`-redactable today (PolicyReader
body-redaction TODO); that TODO now names this read so a future body gate covers
it. DoS: parsing is linear and only served rows are read. No new network input.

**Alternatives:** store counts at write time (needs a stored column on four
backends and a migration; rejected for v1); one object-valued property (needs
new frontend rendering, sort and filter rules; two integers reuse them).

**Files to modify:** internal/metamodel/rollup.go,
internal/rollup/{rollup,tasks}.go, internal/visibility/rollup.go,
internal/visibility/policyreader.go, internal/dataentry/rollups.go, docs-project
GUIDE-metamodel.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** schema (operator config; allowlist
done/open/total, integer type); entity bodies (parsed only).

**Security-Sensitive Operations:** reading bodies of already row-gated rows; see
the threat model.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** table-driven `TestCountTasks`; `TestCompile_BodyTasks`;
`TestParse_BodyTasksShape`; a visibility test for the deny-all row gate, an
inaccessible body and a gated property; a budget test with
`storetest.AssertScaleFree`; a data-entry list test for the column and the 400
on filter and sort.

**Edge Cases:** empty body, CRLF, unclosed fence, tilde vs backtick fences, a
box glued to text, numbered items, tabs.

**Negative Tests:** invalid declarations rejected by the loader; filter and sort
refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a future body redaction must also gate the count; mitigated by the
note on the body-redaction TODO. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-metamodel (roll-up properties).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** (significant) the body read must use each served
row's face, or a faced row would count another face's body: one read per face.
(significant) filter and sort would silently read every body of the type:
refused with 400. (minor) four-space indented code blocks are not skipped, since
under a list item the same indentation is a nested task: documented. (minor) MCP
and CLI use the same `Rollups.Values`, so they get the same gate.
