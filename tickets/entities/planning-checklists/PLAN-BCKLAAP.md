---
id: PLAN-BCKLAAP
type: planning-checklist
title: 'Planning: Automation trigger on entity delete'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: an `on.deleted` automation trigger that runs background `lua_file` actions
once per deleted face, after commit, for hard, soft, face, owned cascade and
script deletes. Out: firing on purge, restore or undo; foreground actions;
`validate:`.

**Acceptance Criteria:**

1. A delete of a note runs the `on.deleted` script with the last state
(`TestDeletedTrigger_EndToEnd`, foreground and queued).
2. The job sees the state only as its identity may read it
(`TestDeletedTrigger_EndToEnd` no-grant and `visible:` cases).
3. Invalid combinations are load errors (`TestValidateBackgroundActions`,
`TestNewEngine_RelatedRefusedAtLoad`).
4. Each delete path notifies once per face; purge and restore do not
(`TestDeleteNotifier`).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small change on an existing seam)
- [x] ~~Searched for existing libraries~~ (N/A: internal trigger plumbing)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows the `on.updated` background pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The `on.updated` background trigger (TKT-2Q4UFI) and the
`CreateNotifier` optional hook on `AliasRewriter`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `entitymanager.DeleteNotifier`, an optional half of
`AliasRewriter` like `CreateNotifier`, receives each deleted face's raw state
after commit. The automation job service implements it, runs the engine with
`EventEntityDeleted` and schedules the matching background actions with the last
state in the payload. The job gates that state with `ScriptReader.GateDeleted`
(row gate plus redaction for the job identity). Rejected: reading the state from
version history (not on every backend) and gating before the delete (the job
identity is not known to the manager).

**Files to modify:** metamodel (types, automationjob), automation (types,
engine), entitymanager (alias_hook, manager, owning, softdelete), visibility
(deleted.go), appbuild (automationjobs, aliasfanout), configure UI label,
GUIDE-metamodel.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** schema.yaml (operator); the job payload, whose
deleted state must name the payload's own id and face.

**Security-Sensitive Operations:** the last state reaches a script that runs as
another identity. It is row-gated and redacted for that identity; a grant that
needs the stored row fails closed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** nil state; a face the job cannot read; two deletes of one face
(no coalescing); a payload whose state names another face.

**Negative Tests:** foreground action, `validate:`, combined trigger keys and
`related(...)` under `on.deleted` are refused at load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a hidden value reaching the script (mitigated by `GateDeleted` and
its tests). Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-metamodel (Triggers table and a new section on
`on.deleted`).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change; covered by the code review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
