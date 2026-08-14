---
id: IMPL-GX8C6A
type: implementation-checklist
title: 'Implementation: Client attenuation: principal_type baselines + scope re-openings as an ACL ceiling below the acting user'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**What was built** (8 commits on `tkt-iac8tx-client-attenuation`):

1. Claims plumbing — `principal_type` + `scope` from `jwtauth.AssertionClaims`
through `dataentry.AssertedIdentity` → `principal.VerifiedFrom`. Added a
`principal.Claims` struct so the constructor stops growing positionally (it was
already at 5 args); `Verified` kept as a wrapper so 27 existing call sites
didn't churn.
2. Policy types — `client_baselines` / `scope_grants`, shared `Restriction`
body carrying both spellings per axis, disjointness + one-spelling validation.
3. Load-time compilation (`ceilingcompile.go`) — a MATCHER, not an expanded
type set.
4. Enforcement — `Request.roleFor` as the single clamp point, plus match-time
predicates for the wildcard case; field axis via `dimension.restrictTo` /
`dimension.redact` in `internal/affordances`.
5. `rela acl audit` rules A11/A12/A13/B8/B9 + `rela acl map --as`.
6. Docs (both guides, regenerated) + a `CLAUDE.md` standing rule.

**Edge cases from planning, all covered:** empty/whitespace scope; `applies_to:
[]`; undeclared type in a ceiling; `"*"` in both spellings; `deny_read` on a
type the user cannot read; a scope re-opening what the user lacks; bounded scope
count; whitespace-padded keys (the RR-IK355A trap); historical subjects.

## Bugs found by the tests during implementation

All three would have failed **open** or silently — worth recording because each
was found by a test written before the code was trusted, not by review:

- `trimAll` collapsed an explicit `read: []` to nil, turning a hard lockout
into no restriction at all.
- Scopes re-opened denials by list subtraction, so `deny_write: ["*"]` plus a
scope left the wildcard intact and re-opened nothing. Fixed with an explicit
`except` carve-out checked ahead of the wildcard.
- `deny_write` expanded before validation ran, so a colliding `deny_update` was
silently discarded rather than rejected.

## Manual verification

- [x] Feature tested end-to-end manually
- [x] Each acceptance criterion verified
- [x] Verification evidence documented

Built a throwaway project (`/tmp/aclx`) with a `person`/`ticket` metamodel and
the worked-example policy, then ran the real CLI:

- `rela acl map --principal alice --as app --scope rela.tickets.write` →
reported read on person+ticket and update on ticket only; the same command
without `--as` reported alice's full create/update/delete. The documented output
in `GUIDE-acl-overview.md` is a verbatim copy of this run (I had written the
route format from memory first and corrected it to match).
- `rela acl audit` on the well-formed policy → no ceiling findings.
- `rela acl audit` on an inert policy → `A11-inert-client-baseline` (medium)
and `A12-scope-reopens-nothing` (low), both with actionable fixes.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)

`just check` (lint, arch-lint, plimsoll, lint-md, test), `just coverage-check`
(77.2%, both thresholds pass) and `just docs-check` all green.

Silent-failure review: the security-critical invariants (`applies_to` overlap,
both-spellings-per-axis, blank keys, deny spellings on a scope grant) are HARD
load errors, not warnings — an operator must not get a policy that looks
restrictive and is not. Drift-class findings (undeclared type/field) are audit
findings rather than boot failures, matching the tolerant-by-design convention
`acl.yaml` already follows.

## Structural guarantees added

- `TestNoDirectRoleLookupInEvaluationPaths` — scans every non-exempt file in
`internal/acl` and fails if an evaluation path reaches `policy.Roles[...]`
directly, bypassing the clamp. Verified it actually fires by injecting a bypass
into `readquery.go` and confirming the failure, then reverting.
- The exemption-list (not inclusion-list) shape means a NEW file fails closed.
