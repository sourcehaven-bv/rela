---
id: REV-LN3QJM
type: review-checklist
title: 'Review: Permission-based navigation filtering (UX: hide menu entries a user cannot use)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — all pass
- [x] `just lint` — 0 issues
- [x] `just lint-md` — 0 issues
- [x] `just arch-lint` — no warnings
- [x] `just plimsoll` — no warnings
- [x] `just coverage-check` — PASS, total 77.2%
- [x] frontend `npm run test:run` — 1429 passed
- [x] frontend `npm run typecheck` — clean

## Code Review

Run with the cranky-code-reviewer agent, asked explicitly to attack the "UX not
security" framing. **The framing held** — the reviewer patched `handleV1Config`
to filter navigation per principal and confirmed
`TestNavPermission_ConfigUnfiltered` fails loudly, i.e. that guard is real.

Two findings landed, both mutation-verified by the reviewer:

| ID | Severity | Status |
|----|----------|--------|
| RR-XYO03L | critical | addressed — ReadOnlyACL arm hid entries a read-only principal can use |
| RR-KCM8R0 | significant | addressed — the presentation-only guard test was theatre |
| RR-2KZEXF | significant | **deferred** → TKT-E5EM3N (permission names unvalidated) |
| RR-ABO495 | nit | addressed — docs table documented a nonexistent `graph:` field |

No open critical or significant responses.

**RR-XYO03L is the one that mattered.** I copied `authorizeCommand`'s
ReadOnlyACL deny arm without checking whether its reasoning transferred. It
doesn't: `authorizeCommand` gates shell execution (write-shaped, correctly
denied under `--read-only`), while this gates menu links to *read* surfaces, and
`acl.ReadOnlyACL` only implements `AuthorizeWrite`. My godoc asserted "the
principal cannot act on anything", which is simply false. And since
`ReadOnlyACL` carries no identity, the arm hid gated entries from *everyone* —
`permission:` changed meaning based on a process-wide flag about writes. The
audit-log entry my own docs use as the example disappeared in exactly the
forensic mode `ReadOnlyACL` documents as a use case.

Fixed by grouping ReadOnlyACL with NopACL (neither carries a policy, so neither
has a permission model) while keeping the arm explicit, because RR-CWWJGW is
real: falling through to the read gate would reach the same answer *by
accident*. `TestNavPermission_ReadOnlyArmIsExplicit` pins that distinction.

**RR-KCM8R0** is a lesson worth keeping: the test I described in the plan as
"the assertion that keeps this honest" asserted that two unconnected subsystems
are unconnected, and could not fail from any change to the feature.

## Mutation testing

Every guard on this branch was verified by breaking the code and confirming the
right test fails:

| Mutation | Expected to fail | Result |
|---|---|---|
| `permitsNavEntry` → always `true` | NonHolderFiltered, NilACLHides, FilterIsPresentationOnly | all three failed ✓ |
| Remove the ReadOnlyACL arm | ReadOnlyShowsEverything (both forms), ReadOnlyArmIsExplicit | all failed ✓ |
| (reviewer) filter `handleV1Config` | ConfigUnfiltered | failed ✓ |

Restored and green after each.

## Acceptance Verification

Verified against a live `rela-server` on `prototypes/data-entry/project` with a
gated `Admin Only` entry granted to alice only.

| AC | Result | Evidence |
|----|--------|----------|
| 1 | PASS | Every pre-existing sidebar test green, unmodified |
| 2 | PASS | No `acl.yaml` → entry shown (browser-verified in the sidebar) |
| 3 | PASS | alice (holder) → present |
| 4 | PASS | bob (non-holder) → absent, ungated entries intact |
| 5 | **CHANGED** | `--read-only` now **shows** gated entries — see RR-XYO03L; verified live |
| 6 | PASS | Fully-gated group dropped, not rendered as a bare heading |
| 7 | PASS | Partly-gated group retained with only its permitted item |
| 8 | PASS | bob's `/api/v1/tickets` returns **rows**, not just 200 (rewritten per RR-KCM8R0) |
| 9 | PASS | `/_config` byte-identical (md5) for alice and bob; still lists the gated entry |
| 10 | PASS | `permission:` on a group is a config error |

AC5's expected outcome was inverted by the review. The plan asserted a rationale
I had not checked against `acl.ReadOnlyACL`'s actual contract.

## Structural guard added

`TestNavFilterStaysPresentational` (a grep test, following the `translateVerb`
precedent in `lint_test.go`) forbids calling `permitsNavEntry` outside
`views_handler.go`. Suggested by the reviewer, and worth it: a prose rule in
CLAUDE.md already failed to prevent this exact drift once, in TKT-M1AX6P.

## Follow-ups filed

- **TKT-E5EM3N** — warn on `permission:` values no role grants, across
commands/documents/navigation (from RR-2KZEXF)
