---
id: PLAN-JQTWDF
type: planning-checklist
title: 'Planning: Audit rela acl who-can queries (CONTROL-8-15)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `rela acl who-can <verb> <entity>` produces a confidentiality
attestation — who may act on an entity, with the roles, groups, ancestor folders
and raw email addresses by which each grant was acquired — and nothing records
that the query was run. GitHub issue #1145 (IB-review rela#1141), CONTROL-8-15.

**Notable:** `FEAT-RCQ6SJ` already carries a `requires → audit-log` relation in
the tickets project. The gap was declared in the ticket graph before it was
found in review, and nothing satisfied it.

**Scope — IN:** one audit record per `who-can` execution, carrying principal,
time, verb and entity.

**Scope — OUT:**

- Recording the RESULT. The answer *is* the disclosure; copying it into the
audit log would duplicate it rather than record it.
- `rela acl map` / `can`. Same family, but `who-can` is what the issue names and
what produces the per-entity attestation. Widening now would blur why this
record exists.

**Acceptance Criteria:**

1. Running `who-can` emits one record naming principal, time, verb and entity.
2. The record does **not** contain the result set.
3. A missing audit sink does not turn the command into an error.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — one record on one command.

**Existing Solutions:** `audit.OpACLBypassRead` is the precedent and settles the
hardest question — what NOT to log:

> Subject is deliberately EMPTY and no entity data is recorded — the read set is
> unbounded, and logging it would copy ACL-protected content into the audit log,
> a wider disclosure than the read itself.

That reasoning transfers exactly. `who-can`'s output is a list of principals and
their access routes; recording it would make the audit log a second copy of the
access map.

The version-purge commands (`history_purge.go`) are the wiring precedent: a
command that is not a write in the ordinary sense but takes `*writeServices`
purely to reach the audit sink.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** a new `audit.OpACLQuery`, and a `recordACLQuery` helper
called from `ACLWhoCanCmd.Run` **before** the lookup. `Run` moves from
`*readServices` to `*writeServices` to reach the sink, passing
`&svc.readServices` on to `buildACLEngine`.

*Recorded before the answer, deliberately.* An attempt to enumerate access for a
non-existent id is as interesting as a successful one; recording only successes
would let a prober stay out of the log.

*Not recorded on the no-policy path.* That branch returns before any attestation
is produced — there is nothing to attest to.

**Files to modify:** `internal/audit/audit.go`, `internal/cli/acl_whocan.go`,
plus tests.

**Alternatives considered:**

1. *Reuse `OpACLBypassRead`.* Rejected — that op means "an elevated closure read
past the gate", a different event. Overloading it would make both unfilterable.
2. *Log via `slog` instead.* Rejected — CONTROL-8-15 is about the audit log, and
a server log is rotated and unstructured.
3. *Record the result too.* Rejected — see Scope and the `OpACLBypassRead` quote.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the verb is a kong `enum` (`read|create|update|
delete`), so it cannot be arbitrary. The entity id is caller-supplied and is
recorded — it is an identifier the caller already knows, and the record is a
JSONL field, so it cannot break framing.

**Security-Sensitive Operations:** this is the whole ticket, and the sharp edge
is the temptation to log *more*. The record answers "who asked, when, about
what" and stops there. Logging the answer would mean an operator who can read
the audit log obtains the full access map without running the command — a wider
disclosure created by the control meant to observe it.

The trust boundary is unchanged: `who-can` is operator-shell, like the rest of
the CLI. This adds observability, not a gate.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- **AC1** — unit-test the helper for the record's fields, AND a separate test
that runs the real `ACLWhoCanCmd.Run` and asserts the record appears. The second
is the one that matters: the helper could be correct and never called, which is
the exact shape of the gap being closed.
- **AC2** — asserted implicitly by the record's shape (Subject is the queried id;
Summary is the verb) — there is no field in which a result could be carried.
- **AC3** — a nil-sink call must not panic or error.

**Edge Cases:** the no-policy early return (no attestation ⇒ no record); a
lookup that fails with entity-not-found (record still written, deliberately).

**Negative Tests:** the nil-sink case; and mutation — removing the call must
redden the wiring test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| The helper is right but never called | A separate test runs the real command, and mutation-verifies it |
| A fixture without an audit sink breaks the command | Nil-safe, with its own test |
| Someone later "improves" the record by adding the result | The op's godoc states why not, citing the `OpACLBypassRead` reasoning |
| Changing `Run`'s signature breaks existing tests | Four call sites, mechanically wrapped; behaviour unchanged |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `audit.OpACLQuery`'s godoc — the canonical place the op's meaning and its
deliberate omission live.
- [x] `docs/audit-log.md` — a NEW op belongs in the guide's op list, unlike the
earlier attachment ticket which only added an occurrence of an existing one.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one record on
one command, with the "what not to log" question already settled by
`OpACLBypassRead` and quoted above. The judgement calls — new op vs. reuse,
record before the answer, skip the no-policy path — are under Alternatives and
Approach.)
- [x] All critical/significant findings addressed in plan — none raised.

**Design Review Findings:** N/A.
