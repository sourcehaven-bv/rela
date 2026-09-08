---
id: REV-BEY81R
type: review-checklist
title: 'Review: Audit history:read-redacted reveals'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues (after fixing two `misspell` findings — `modelling` and
`artefact` — in new comments); `just arch-lint` clean; `just comment-lint` no
unresolvable doc links across 11464 comments; `just plimsoll` clean; `just
lint-md` 0 issues.

Two lint findings surfaced during implementation and were FIXED rather than
suppressed, because both were pointing at something real:

- `unparam` on the test helper `getHistoryVersion`: my new tests made every
call pass the same `typeName`. Rather than vary it artificially, the parameter
was removed and replaced by a `historyEntityType` constant — every scenario in
that file is expressed against the one type `historyRedactionACL` grants on, so
a parameter was always a fiction.
- `plimsoll`: `recordHistoryReveal` as a method pushed `App` to 87 methods,
one over the load line of 86. Made a free function taking an `audit.Audit`. That
is the better shape anyway — it needs exactly one field of `App`, and taking the
dependency makes it testable without one.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-KBD2T2 (critical), addressed.

The critical finding is worth reading before merge. The first implementation
audited every history read in an UNCONFIGURED deployment: under NopACL /
ReadOnlyACL no read gate is attached, so `readGateFromContext` returns
`nopReadGate`, whose `HoldsPermission` is true for every permission — meaning
the reveal arm is taken by every reader, with nothing redacted to reveal. That
would have filled the audit log of every unconfigured deployment with
meaningless rows, and trained operators to ignore exactly the row this ticket
adds. Fixed with `revealIsPrivileged`, a closed switch on the ACL implementation
matching the existing `permitsGatedUIElement`.

It also exposed a latent inaccuracy in the shared test harness: `buildPolicyApp`
built a Declarative resolver but left `app.acl` as NopACL, so a test handing it
a policy modeled a configured deployment for field redaction and an unconfigured
one for anything switching on the ACL implementation. Fixed there rather than
worked around here.

Two `misspell` findings (British spellings in new comments) were fixed.

Self-review verified the two claims this change rests on, by reading the code
rather than trusting the plan:

0. **The reveal arm is not the same as a privileged reveal.** Under NopACL and
ReadOnlyACL every reader takes it, because the permit-all gate answers true to
every permission probe. This is the RR-KBD2T2 finding; it is listed first
because it is the one the plan got wrong.
1. **No reveal escapes the audit.** `forWireHistoricalReveal` has exactly ONE
call site in the tree (`history_handler.go:255`), and the emit sits directly
beside it. Grepping `PermHistoryReadRedacted` across `internal/` confirms no
other handler consults the permission.
2. **No false records.** Every error path in `serveHistoryVersion` — bad
version string, `version < 1`, `ErrNotFound`, gate error, and the
snapshot-type/URL-type mismatch that exists to prevent a cross-type leak —
returns BEFORE the reveal branch. The emit is unreachable without a successful
reveal.
3. **The recorded version matches the served one.** Both the audit record and
the response body use `snap.Version` (`history_handler.go:267`), so the trail
cannot point at a different snapshot than the one disclosed.
4. **Restore is out of scope, correctly.** `POST .../{version}/restore` is a
write: it gates the changing fields through `validateFieldWrite` and applies via
the entitymanager, which re-authorizes, validates and audits. It is not an
unaudited disclosure path.
5. **Relation history needs nothing.** Verified against the doc comment on
`serveRelationHistoryVersion`: there is deliberately no `history:read-redacted`
reveal there — a deleted relation's meta is served to nobody, and a live one is
redacted against today's policy.

No unrelated changes in the diff.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | reveal emits exactly one record, op `history-reveal` | PASS | `TestHistoryReveal_Audited` |
| 2 | record names WHAT — type, id, version | PASS | same test: Subject kind/type/id + `version=1` in Summary |
| 3 | record names WHO, real principal | PASS | same test, reading as `dana` |
| 4 | ordinary read emits NOTHING | PASS | `TestHistoryReveal_OrdinaryReadNotAudited` |
| 5 | response body unchanged | PASS | the pre-existing `TestHistoryRedaction_*` tests still pass unmodified |
| 6 | no-ACL reads not recorded | PASS | `TestHistoryReveal_NoACL_NotAudited` (added from RR-KBD2T2) |

AC6 was not in the original plan — it comes from the review finding. Its absence
from planning is the honest lesson here: the plan reasoned about who HOLDS the
permission and never asked what the permission check RETURNS when no policy is
configured.

AC3 is verified with a principal (`dana`) that no other test in the file uses.
Reading as `alice` — the file's default — would have let the assertion pass on
an implementation that hardcoded a user; confirmed by mutating
`principal.From(ctx)` to a literal `alice`, which reddens it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-EBV2HD (done).

`docs/acl-security.md` gains a paragraph inside the existing
`history:read-redacted` section — with the permission it documents, not in a
separate log-format appendix that would drift. It states what an operator needs
in order to USE the row, including the `op == "history-reveal"` query that
isolates reveals: a row nobody knows how to query for is barely better than no
row.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
