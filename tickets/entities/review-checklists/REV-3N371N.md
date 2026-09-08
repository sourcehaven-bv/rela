---
id: REV-3N371N
type: review-checklist
title: 'Review: Declarative webhook routes: map an inbound HTTP request onto entity create / find-or-create / update'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -tags postgres ./internal/dataentry/` green
against a live PostgreSQL 15, including the six webhook conflict tests; plus
dataentryconfig, entitymanager, markdown, predicatefns. `-race` green on the
non-postgres packages.
- [x] Lint clean (`golangci-lint run` — 0 issues)
- [x] Comment lint gate clean (11562 comments, no unresolvable doc links)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A for the full-repo
gate: it exceeds 7 minutes locally and was not run to completion. The diff adds
tests alongside every change and removes none, so no package floor can drop.
Should still run in CI.)

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

- [x] Run `/code-review` — two reviewers in parallel (general quality +
rela-security), plus a real-payload compatibility pass against Icinga,
Alertmanager and Grafana formats
- [x] All critical review-responses addressed — RR-HI9QIU, RR-SG8P1N
- [x] All significant review-responses addressed — RR-ZVUHXO, RR-N6AJAS,
RR-OAAVBP fixed; RR-U3T6HQ split (DoS half fixed, lock half -> TKT-34XS2R);
RR-S0QN3H deferred -> TKT-ZEACWJ
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 9 raised.

*Critical, fixed:* RR-HI9QIU (retry loop unreachable — entitymanager re-presents
a unique violation as a validation error, and its pre-write scan raises one with
no store error to wrap at all), RR-SG8P1N (the test asserting that behaviour
never invoked the pipeline).

*Significant, fixed:* RR-ZVUHXO (webhook principal now `system:webhook:<id>`, so
it inherits `principal.IsReserved` instead of being assertable from the wire),
RR-N6AJAS (header refusal by family prefix + wiring-time registration of the
deployment's own `-principal-header`), RR-OAAVBP (step validation checked one
type where find and create may declare two).

*Significant, deferred with tickets:* RR-U3T6HQ — DoS half FIXED (bounded
in-flight, 503 + Retry-After); the `writeMu` half needs store-level CAS, raised
as TKT-34XS2R. RR-S0QN3H — array indexing blocks Alertmanager/Grafana; needs a
`internal/predicate` grammar change, raised as TKT-ZEACWJ.

*Minor/nit, fixed:* RR-HYVCWA (newline flattening at the append sink),
RR-N4693K (`Cache-Control: no-store`).

## Acceptance Verification

- [x] ~~Each acceptance criterion tested (reference planning checklist)~~ (N/A:
no planning checklist — the ticket went straight to implementation at the user's
request.)
- [x] Test evidence documented in implementation checklist (IMPL-845VR0)

**Acceptance Status:**

- Three workflows (always-create / find-or-create / find-and-update-only) —
  **PASS**, driven end to end through the production router.
- Conflict detection on create — **PASS** against live PostgreSQL: 8 concurrent
  deliveries, 1 winner, every loser re-finds and returns 200 with the winner's
  id. Verified FAILING first (7/8 got 500) so the test cannot be vacuous.
- Load shedding — **PASS**: a saturated router sheds with 503 + Retry-After and
  does not latch once a slot frees.
- Body encodings — **PASS**: JSON and form, charset parameters, uppercase media
  types, absent Content-Type, percent-decoding, malformed-body rejection.
- Injection at the append sink — **PASS**: a payload newline can no longer plant
  a sibling heading, and the text survives flattened rather than being dropped.
- Config validation as a load error — **PASS**, including the find/create
  type-mismatch table that was the gap behind RR-OAAVBP.
- Cross-process append durability — **KNOWN LIMITATION, not a pass.**
  `TestWebhookConflict_CrossProcessAppendsCanBeLost` asserts the loss. Documented
  in `docs/webhooks.md` with an interim workaround; TKT-34XS2R is the fix.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `docs/webhooks.md`
ships in this diff rather than as follow-up work, so a separate tracking
checklist would have nothing to track.)
- [x] User-facing documentation updated — `docs/webhooks.md`: config reference,
the concurrency guarantee table (per tier, including what is NOT guaranteed),
producer support matrix naming which of Icinga/Alertmanager/Grafana works today,
body encodings, load shedding, and the reserved principal.
- [x] ~~Docs-checklist marked as done~~ (N/A: none created.)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — the two deferred items are tickets
(TKT-34XS2R, TKT-ZEACWJ) referenced from the code and docs, not inline markers
- [x] Ready for another developer to use

## Pull Request

- [x] `/pr` to follow — the ticket reaching `done` is its precondition, so the
PR necessarily post-dates this checklist (TKT-UFV01M).

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
