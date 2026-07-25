---
id: REV-LY0JXC
type: review-checklist
title: 'Review: acl: migrate a scheduler read grant into an existing acl.yaml (new FileTypeACL)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings
- [x] `just plimsoll` — clean
- [x] Tests pass
- [x] Coverage within floors
- [x] `just docs` regenerated and committed

The five `internal/docscapture` browser tests fail in this environment; verified
failing identically on a clean `develop` checkout (they need Chrome + a built
SPA), so they are pre-existing and unrelated.

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] All critical findings addressed
- [x] All significant findings addressed

**Findings:**

| ID | Severity | Status |
|---|---|---|
| RR-KG2FCX | critical | addressed |
| RR-2ZIGX3 | significant | addressed |

The reviewer drove the real `Detect`/`Apply` through the real `acl.Policy`
loader rather than reading the code, and found two shapes where the migration
reported success while granting nothing (`roles:` / `assignments:` present but
null). Both were reproduced locally before fixing and re-verified after.

Two earlier design-review findings (RR-SVQ5HE, RR-1USMEZ) were raised and
resolved during planning — both reversed decisions in the original ticket, and
both were confirmed by probe rather than argument.

## Verification

- [x] Acceptance criteria met
- [x] Manual verification performed
- [x] No regressions introduced

**Mutation testing** — every fix is independently guarded:

| Mutation | Tests failing |
|---|---|
| Scheduler default reverted to `SystemUser()` | 3 |
| `acl.yaml` unwired from migrate file list | 2 |
| `EnsureMapping` stops repairing null in place | 6 subtests |
| `Detect` reverted to bare key-existence | 5 |
| Dead-role repair removed | 2 |
| `Apply` postcondition removed | 1 |

The postcondition mutation initially SURVIVED — the guard was added only after
that run showed the test suite could not detect its removal.

## PR

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1203
- [x] CI monitored

## Sign-off

- [x] Ready to merge

Behaviour change flagged for release notes: scheduled writes now record
`principal.user: "system:scheduler"` instead of the OS account that started the
scheduler. Documented in the scheduled-tasks guide.
