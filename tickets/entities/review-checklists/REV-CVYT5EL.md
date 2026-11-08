---
id: REV-CVYT5EL
type: review-checklist
title: 'Review: Declarative peer pull: peers.yaml apply side'
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

**Review Responses:** cranky-code-reviewer and rela-security-reviewer.
Significant, addressed: RR-1VE2UX3 (relation meta validated), RR-I7QWPZZ
(endpoint and cardinality refusals are rejections), RR-NPUU8R4 (held rows go
last), RR-JNRM0IC (credential through HostConfig). Minor and nit, addressed:
RR-VLML0MH, RR-3ZLQD31, RR-JB84BEO, RR-MH8C7G5, RR-T3SJG6D, RR-NWX1L42,
RR-VC0JS6R, RR-RYIWD8S, RR-3YI72U9, RR-SJ1HOKN. Wont-fix: RR-2ECU3H3.
Deferred to TKT-6GRZHXD: RR-3QG82IE, RR-AV17OW3, RR-GE6WC2T, RR-TOJ0G92. No
critical findings.

Checks: `just coverage-check` ran the full suite, all packages pass, total
85.9%. golangci-lint on every changed package: 0 issues. arch-lint, plimsoll,
comment-lint gate and archguard pass. The full `just ci` runs again after the
merge of develop, before the push.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- End-to-end with two instances, entities and relations, local additions
survive an upstream update: PASS. `TestPeerPull_EndToEnd` pulls from a real
data-entry router through its peer key edge: 2 entities and their relation,
the hidden type is not pulled, a second pull is a no-op, an upstream rename is
applied and a field added locally through PatchEntity survives it.
`TestRun_MergesOwnedFieldsAndKeepsLocalOnes` covers the same at unit level.
- Validation of peers.yaml at load: PASS. `peersync.Parse` reports every
problem (`TestParse_Errors`); `rela validate` shows a peers line
(`TestValidateWithFS_PeersStandAlone`); the scheduler refuses an invalid file
(`TestLoadConfig_Peers`).
- A run over the threshold applies nothing and reports why: PASS.
`TestRun_GoneGuard` (ledger unchanged, `--allow-gone N` named),
`TestRun_EmptiedTypeIsRefused`, `TestRun_DeleteWindow`,
`TestRun_DeniedFailsWithoutWriting`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-4SUTT5W

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after the ticket is done, per TKT-UFV01M)

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
