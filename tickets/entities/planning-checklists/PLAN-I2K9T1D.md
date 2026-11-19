---
id: PLAN-I2K9T1D
type: planning-checklist
title: 'Planning: Flows: start runs on create'
started: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `on: create` starts for every create through the entity
manager, `when:`, `per_subject`, run_as attribution, rollback, settle after
commit, a bound on chains. Out: `on: predicate`, `on: schedule`, `change`
events (TKT-GYXOTQC); raw `rela import`, `rela db load`, conversion, copy
and restore.

**Acceptance Criteria:**
1. A manager create of a hired person starts one settled onboarding run on
   fs, sqlite and postgres (`TestFlowCreateStart`).
2. A new person starts none (`TestFlowCreateStart/when_fails`).
3. A Lua `rela.create_entity` starts one (`TestFlowCreateStart_Lua`).
4. A rolled-back batch leaves no run (`TestFlowCreateStart_RollbackStartsNoRun`).
5. A due round starts no second run (`TestFlowCreateStart`).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the design follows the existing flow engine and the manager transaction seam)
- [x] ~~Searched for existing libraries~~ (N/A: internal engine change)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal engine change)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `flowengine.Engine.Start` (manual start), the
in-transaction run store join (`joinFlowRunsFor`), `auditTx` and
`CreateLinked`'s `BeforeWrite`, and the job hop guard in
`appbuild/automationjobs.go`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** A consumer-side `entitymanager.CreateHook` runs in
the create's transaction. The flow runtime's hook writes the header and the
run there (run store joined to the transaction on pg and sqlite) and queues
a settle for after the commit. Rejected: starting the run only after commit
(a crash between commit and start loses the run) and a store event observer
(fires on raw writes too, and outside the transaction).

**Files to modify:** `internal/entitymanager/{create_hook,core,audittx,linked,manager}.go`,
`internal/flowengine/{engine,oncreate}.go`, `internal/appbuild/{flowcreatestarts,appbuild,piles,channelguard}.go`,
`internal/store/store.go`, the flows guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** flows.yaml (operator config, validated at
load, including that run_as may write every output); the created entity,
read through `internal/predicate` for `when:`.

**Security-Sensitive Operations:** The create is authorized as the caller.
The header is written as `system:flow` and the outputs as `run_as`, both
through the audited manager. Endpoint reads in a nested transaction keep
the read gate. A `when:` error is logged without its message at warn level.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria; all run under the default,
`sqlite` and `postgres` tags.

**Edge Cases:** busy subject with `per_subject: one`, a `when:` that fails
to evaluate, a settle that fails, a chain of create starts, a nested
linked create's follow-ups.

**Negative Tests:** a caller without create starts nothing; a run_as that
may not write an output refuses the boot; a hidden endpoint created in the
same transaction is refused like a missing one.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Nested writes in one transaction could wait on their own lock
(sqlite): follow-ups are deferred to the commit. Runaway chains: bounded at
8 hops while settling synchronously; a worker retry resets the count
(documented). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project GUIDE-flows (Status note, Starting
on create, Running flows).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Reviewed after a first implementation by an
independent PR-guidance review and the automated security review; the
findings (authorization negative tests, log content, doc mismatch, chain
bound scope) are addressed in the code and docs.
