---
id: PLAN-P6G4XV
type: planning-checklist
title: 'Planning: Scheduler for_each: expand one occurrence into recipient-scoped jobs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Strict `for_each` config, bounded selection, expansion and child job
kinds, recipient-principal resolution, and ACL-scoped child execution. No
mail/address logic, broadcast,
attenuation, worker pool controls, or legacy scheduler-ladder rewrite.

**Acceptance Criteria:** The eleven numbered ticket criteria are each covered by
config table tests, scheduler unit tests, or one
appbuild end-to-end test with a real store, ACL policy, queue, and Lua action.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** TKT-YOED3R/PLAN-0DHUD4 contains the queue/backend research;
this ticket is its first fan-out consumer and adds no library.

**Existing Solutions:**
- `internal/jobs`: narrow client, JSON payload contract, bounded retry, pending
idempotency, memory/PostgreSQL backends and conformance posture.
- `internal/scheduler/jobs.go`: handler registration, run tokens and job payload
parsing; extend rather than introduce a second dispatcher.
- `internal/schedulerstate`: intentionally only declaration cadence; derived
retry state belongs to jobs after #1444.
- `acl.Declarative.ResolvePrincipal` and `acl.RequestResolver.ForPrincipal`:
existing identity mapping and request construction.
- `ScheduledLuaWriteDeps`: existing row/field-visible scheduled read seam.
- Pending queue keys collapse concurrent work. Once a child completes the key
  expires, so a later expansion retry can recreate it; this is an explicit
  at-least-once boundary rather than a reason to add a second persistence seam.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Implement strict `ForEachConfig`; enqueue an expansion
job instead of the existing action job when present; query a deterministic
bounded set; enqueue bounded-retry child jobs under a stable pending identity;
reconstruct
the selected principal and ACL request in the child; invoke the existing action
handler. Preserve the entire non-for_each path.

Rejected: expanding inline in the scheduler (blocks cadence), one job containing
all recipients (coupled retry and wrong ACL scope), trusting a serialized ACL
request (stale authority), and adding a second pool/table that still cannot make
SMTP or other external effects exactly-once.

**Files to modify:**
- `internal/scheduler/{config,jobs,foreach}.go` plus tests
- `internal/appbuild` queue/scheduler wiring and end-to-end tests
- `.go-arch-lint.yml`, `.testcoverage.yml`
- `docs/scheduled-tasks.md`, changelog

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Config keys are strict; entity type must exist;
filters parse through `filter.ParseAll`; limit is 1..10000; `run_as` xor
`for_each`. Durable payloads are decoded by typed helpers and reject
missing/wrong types. Entity/principal is reloaded at execution; payload
authority is ignored.

**Security-Sensitive Operations:** Child execution can export graph data or
mutate it. The handler installs a fresh ACL request for the resolved recipient
before creating read dependencies; field redaction remains active. Diagnostics
log IDs and counts, never entity
properties or payload bodies.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Config tests cover AC4/5; scheduler/job tests
AC1/2/6/7/9/10/11; appbuild end-to-end covers AC2/3
with real Lua and ACL.

**Edge Cases:** Empty selection; exactly limit and limit+1; interval schedule
rejection; duplicate IDs; entity removed after expansion; blank/list principal
property; expansion retry after mixed child completion; enqueue failure; two
concurrent expanders; cancellation; JSON numeric decoding; calendar
occurrence IDs; old tasks without for_each; Unicode and separator characters in
IDs.

**Negative Tests:** Unknown keys/type, malformed filter, invalid limit, run_as
conflict, malformed payload, and store/query failure return named errors.
Unresolvable recipients are safe skips with warnings, never
identity fallback.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** ACL exfiltration (fresh per-child request + E2E denial test);
duplicate expansion (stable pending keys, with the post-completion replay window
documented); amplification (hard bound); authority staleness (reload/resolve);
external exactly-once gap (document honestly). Effort remains L.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/scheduled-tasks.md`: syntax, execution, ACL, bounds and failures
- [x] changelog: new behavior and compatibility
- [x] ~~`CLAUDE.md`~~ (N/A: no new repository-wide convention)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-MAILI1 (critical interval occurrence identity),
RR-MAILI2 (critical durability mismatch), RR-MAILI3 (significant stale payload
authority). All addressed in the ticket and technical approach.
