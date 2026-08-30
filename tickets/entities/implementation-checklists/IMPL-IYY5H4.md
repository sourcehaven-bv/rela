---
id: IMPL-IYY5H4
type: implementation-checklist
title: 'Implementation: Audit rela acl who-can queries (CONTROL-8-15)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestRecordACLQuery` (fields) and
`TestRecordACLQuery_NilSinkIsSafe`.
- [x] Integration tests written — `TestACLWhoCan_RunEmitsAuditRecord` runs the
real `ACLWhoCanCmd.Run` against a seeded graph and a real policy.
- [x] Happy path implemented
- [x] Edge cases from planning handled — nil sink; the no-policy early return;
a failing lookup still records.
- [x] Error handling in place — a missing sink is a no-op, never an error.

## Test Quality

- [x] Using fixture builders or factories — the package's existing
`aclTestServices`, `seedWhoCanGraph`, `whoCanMeta`, `whoCanPolicy`.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Two tests, and the second is the one that matters.* The helper could be
entirely correct and never called — which is precisely the shape of the gap this
ticket closes, since `FEAT-RCQ6SJ` already declared a `requires → audit-log`
relation that nothing satisfied. So a separate test drives the real command.

*Mutation-verified.* Removing the call from `Run`:

```
--- FAIL: TestACLWhoCan_RunEmitsAuditRecord
    running who-can emitted no "acl-query" record; got 0 record(s)
```

*Recorded before the answer, deliberately.* An attempt to enumerate access for a
non-existent id is as interesting as a successful one; recording only successes
would let a prober stay out of the log entirely.

*Not recorded on the no-policy path.* That branch returns before any attestation
exists — logging "someone asked about access in a project with no policy" would
be noise.

*Verified the command still works:* `rela --project tickets acl who-can read
TKT-ZZL53L` behaves unchanged.

**Gates:** `go test ./...` exit 0; `just lint` 0 issues (caught a `perfsprint`
`Sprintf`-to-concat); `lint-md` clean after two fixes; arch-lint, comment-lint,
plimsoll clean.

## Quality

- [x] Code follows project patterns — new op alongside the others in
`audit.go` with a godoc in the same voice; wiring follows the version-purge
commands, which likewise take `*writeServices` purely to reach the sink.
- [x] Checked for DRY opportunities — one helper rather than an inline literal,
so the "what we do and don't record" decision has one home.
- [x] No security issues introduced — this **adds** a security-log record. The
sharp edge was the temptation to log *more*: the record answers "who asked,
when, about what" and stops. Logging the answer would let anyone who can read
the audit log obtain the full access map without running the command — a wider
disclosure created by the control meant to observe it. The op's godoc states
this, citing the same reasoning `OpACLBypassRead` already applies.
- [x] No silent failures — the whole ticket.
- [x] No debug code left behind.

**Signature change, noted for review.** `ACLWhoCanCmd.Run` moves from
`*readServices` to `*writeServices` to reach the audit sink — the same shape the
purge commands use. Four existing test call sites were mechanically wrapped
(`&writeServices{readServices: *svc}`); no assertion changed, and the command's
behaviour is identical.
