---
id: REV-T499IDT
type: review-checklist
title: 'Review: Peer identity and credentials'
started: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:**

- Design review: RR-9SWQMGV, RR-T28C0OO, RR-3HVBDE2, RR-WIXLJP4, RR-0JR6KXB,
  RR-N119JGG, RR-E6GGM4G, RR-YK0WZLL, RR-0IM7OHE, RR-3UJVP3H, RR-DP0EAT7
- Code review (cranky-code-reviewer and rela-security-reviewer): RR-J0MY0JC,
  RR-299TVAV, RR-0I1BQA2, RR-RVA9DJ5, RR-ALBR6PB, RR-XSJ15RY, RR-ZJ3KPSA,
  RR-DS7TCYI, RR-P3132VJ, RR-S0J9ZEE, RR-QUMMHHZ, RR-7IIXBS5, RR-N2MAEJU,
  RR-LDCFCHI
- Both significant code findings (RR-J0MY0JC, RR-299TVAV) are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: a peer reads only its role's rows through the list endpoints and
  `_reconcile` (`TestPeerKey_*` in `internal/dataentry/peerkey_edge_test.go`;
  manual run: secrets list empty, SEC 404).
- AC2 PASS: a bad, revoked or malformed key answers 401 in every mode, with no
  fall-through (`TestPeerKey_BadKeysAnswer`, `TestPeerKey_JWTMode`, `TestSource_ReloadsAfterInterval` and
  `TestSource_HungReloadFailsClosed`; manual revocation took effect in 11 s).
- AC3 PASS: every write and every underscore sub-resource answers 403
  `peer_route_refused` (`TestPeerKey_CannotWrite`, `TestPeerRouteAllowed`; manual POST and PATCH 403).
- AC4 PASS: a `peer:` name is refused from the header, the environment and a
  JWT (`TestPeerKey_NameCannotBeAsserted`, `principal` reserved-name tests).
- AC5 PASS: only the digest is stored, the key is never logged, and a key
  placed in the identity header is refused unlogged
  (`TestPeerKey_MisplacedInIdentityHeader`, log capture assertions).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-K7Y67GE

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (runs after the ticket is done; see the note below)

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
