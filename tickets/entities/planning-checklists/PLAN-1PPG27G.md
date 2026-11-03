---
id: PLAN-1PPG27G
type: planning-checklist
title: 'Planning: Property test: two clients autosaving one entity'
started: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: two `useAutoSave` instances (two browser tabs) editing one note through
the verified fake API, under `fc.scheduler()`. Channels: properties (`title`,
`summary`) and the body. Commands: set a property, edit one slot of the body,
deliver one scheduled step, move the clock, flush a client
(`commitImmediately`), and fail the next request with a 503.

Out: relations (the fake answers 501 for relations writes), unsets (an unset
value cannot be told apart per client, so writes could not be attributed),
live refresh between the clients (no `entity:changed` handling; each client
only learns of the other's writes from its own responses and 412s), and
EntityDetail/DynamicForm (TKT-60R64HZ covers DynamicForm).

**Acceptance Criteria:**

1. No invented values: after every run, each property and each body slot on
   the server is its initial value or a value one of the clients set.
2. No blind overwrite: every write the server accepts changes a field
   (property or body slot) only to its sender's own value, and only from a
   value the sender was shown: its seed, a value applied to its form, its
   own value, or the value its conflict refetch returned before the
   conflict was reported. The sender of each request is known
   (AsyncLocalStorage in `runWrite`).
3. No silent loss: for each tab and each field it edited, its last value is
   on the server at the end, or the server accepted it at some point, or the
   tab still shows it with an error on that field.
4. No false conflict: a reported conflict has the other tab's value, or a
   write whose answer was lost, on the server side of a field that really
   conflicted (base read back from the precondition token).
5. Deliberate faults are caught: `mergeProperty` or `mergeText` patched to
   always write ours fails the property (permanent tests), and one-off
   mutations of the base rules in `useAutoSave.ts` fail it too.
6. The runs do something: accepted writes, 412s, reported conflicts, merged
   writes, faults that hit a save and saves out of attempts each pass a
   floor.

Scope change after implementation: the property found real bugs, filed as
BUG-SJURXLT and fixed on this branch (unsaved value replaced on screen, false
body conflict with the tab's own merged write, wrong merge base on a second
412 in one save, also for relations).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: a test of existing code, using the harness from TKT-7LTLHW3)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the pattern is fast-check's scheduler, already used in this repo)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- fast-check `fc.scheduler()` and `fakeAdapter(server, scheduler, { fault })`
  from TKT-7LTLHW3 (`src/test/fakeApi/adapter.ts`).
- `useEntitySession.prop.test.ts`: collect violations, assert at the end,
  count effective steps with a floor.
- `EntityDetail.switching.prop.test.ts`: fake clock (`setTimeout`, `Date`
  only), `settle()` loop that drains the scheduler and timers.
- Merge rules: `autoSaveMerge.ts`, and the autosave section of
  `frontend/CLAUDE.md`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- One `FakeServer` seeded with one note: `title`, `summary` and a body of
  three slots separated by fixed lines (`s0\n--\ns1\n--\ns2`), so edits to
  different slots merge cleanly and edits to one slot conflict.
- Two clients A and B. Each has its own Pinia (as two tabs have), its own
  `formData`/`contentRef`, and an autosave seeded from a GET. Its
  `applyServerProperty` and `applyServerContent` record what the form was
  shown. `runWrite` records the server step at which each save started, so a
  conflict report can be matched to the values the field held meanwhile.
- Every value a client sets is unique (`A3`, `B7`), so the author of each
  accepted write is the owner of the new value.
- The server's `handle` is wrapped to log every accepted PATCH: per field,
  the value before and after. Criterion 2 is checked there, when the write
  lands, against what its author had been shown so far.
- After the commands, both clients are flushed and the scheduler and clock
  are drained; then criteria 1 and 3 are checked.
- The fault check mocks `./autoSaveMerge` with pass-through spies; one test
  overrides them to always write ours and expects `fc.assert` to fail.

Alternatives rejected: driving EntityDetail twice (two mounted pages share
module state such as the event stream, and DOM driving adds noise unrelated
to merging); checking only that writes carry preconditions (a merge that
always writes ours still sends a fresh, valid precondition).

**Files to modify:**

- `frontend/src/composables/useAutoSave.twoClients.prop.test.ts` (new)
- `frontend/src/composables/useAutoSave.ts` and
  `useAutoSave.conflicts.test.ts` (BUG-SJURXLT)
- `frontend/CLAUDE.md` (one line naming the property under the autosave
  rules)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

None: a test-only file. Inputs are generated by fast-check.

**Security-Sensitive Operations:**

None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1-3: one `asyncProperty` over a scheduler and up to 30 commands,
  300 runs by default (`PROP_RUNS`).
- AC4: two tests that patch `mergeProperty` or `mergeText` to always write
  ours and expect the property to fail.
- AC5: counters asserted after the run.

**Edge cases:**

- Both clients edit the same field at once (conflict, or `same` when equal;
  unique values rule out `same` except for a client's own echo).
- A client keeps typing while its save is in flight (held field).
- A 412 with an empty conflict set (a write landed between check and write).
- A 503 on a save or on the conflict refetch (error kept, edit retried on
  the next edit).
- Three 412s in a row (`MAX_CONFLICT_ATTEMPTS`), so the save fails.

**Negative Tests:**

AC4: a merge that writes ours blindly must make the property fail.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The test finds a real autosave bug. Then file it as a bug with the
  counterexample and decide with the user whether to fix it here.
- `backoff` uses `setTimeout` and `Math.random`; the fake clock covers the
  timer and the draining loop advances it.
- Criterion 2 is too strict for a legitimate path and gives false
  counterexamples. Mitigation: every counterexample is read before the rule
  is changed, and any loosening is written down in the test.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: test-only, kind=test)

**Documentation Impact:**

- [x] CLAUDE.md - one line in frontend/CLAUDE.md naming the property

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-H4E5X9H, RR-UN9B0H7 (critical); RR-LXG3D4T,
RR-ZM1SGWM, RR-Q73WATI, RR-WBGKG1P, RR-5OG51BQ, RR-29VGUD0 (significant);
RR-C5LYNQA, RR-Q3SDLY6, RR-SBZU58L, RR-MX18W6R, RR-S9CWDQT (minor). All
addressed.
