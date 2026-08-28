---
id: REV-HIH8K8
type: review-checklist
title: 'Review: Client attenuation: principal_type baselines + scope re-openings as an ACL ceiling below the acting user'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — all pass
- [x] `just lint` — 0 issues
- [x] `just coverage-check` — 77.2%, package and total thresholds pass
- [x] `just arch-lint` — no warnings
- [x] `just plimsoll` — no type over a load line
- [x] `just lint-md` — 0 issues
- [x] `just docs-check` — docs regenerated and committed

## Code Review

- [x] `cranky-code-reviewer` run over the full branch diff

Six findings, all triaged and linked via `has-review-response`:

| ID | Severity | Status |
|---|---|---|
| RR-B6DRAG | critical | addressed |
| RR-48B6MT | significant | addressed |
| RR-UYB99D | minor | addressed |
| RR-U41F3D | minor | addressed |
| RR-MUJUWS | minor | deferred |
| RR-9K9NSJ | nit | wont-fix |

**The critical finding was a genuine fail-open I missed.** `applyClientCeiling`
returned early when ctx carried no `acl.Request`, so a stamped-but-unbound
principal kept its user's full field visibility — while `resolveViaDeclarative`,
twenty lines away in the same file, opened its own Request. Within one
`FieldVerdicts` call, roles resolved and the ceiling did not. Every test in the
suite bound via a helper, so the whole suite passed over the hole.

I reproduced it with a failing test before fixing, then fixed it by mirroring
the sibling's fallback. Not reachable in production (`attachACLRequest` and
`ScriptReader.bind` both bind), but `visibility.PolicyRedactor` forwards
whatever ctx it is handed and binds nothing, so the guarantee rested on wiring
rather than structure.

The reviewer also explicitly could NOT substantiate: policy mutation across
requests, aliasing, an un-predicated consumer of the clamped wildcard lists, or
gaps in relation writes / transitions / `HoldsPermissionForEntity`. I
independently re-probed the mutation and aliasing claims and agree.

## Acceptance Verification

Each AC from TKT-IAC8TX, with the test that pins it:

| AC | Verdict | Evidence |
|---|---|---|
| 1 — claims reach the Principal | PASS | `jwtauth` + `dataentry` round-trip tests; absent claims yield empty, no error |
| 2 — overlapping `applies_to` is a startup error | PASS | `TestClientBaselines_DisjointAppliesTo`, plus a determinism test so the reported pair is stable |
| 3 — unmatched principal_type is unrestricted | PASS | pinned at three layers: compiler, affordances, aclmap |
| 4 — `redact` open-world / `visible` closed-world | PASS | `TestCeiling_RedactIsOpenWorld`, `TestCeiling_VisibleIsClosedWorld` |
| 5 — both spellings for one type is a load error | PASS | `TestRestriction_OneSpellingPerAxis` (4 sub-cases) |
| 6 — `deny_write: ["*"]` covers three verbs | PASS | `TestDenyWrite_ExpandsToThreeVerbs` + an ordering regression test |
| 7 — a ceiling never grants past the user | PASS | `TestCeiling_NeverGrantsPastTheUser` (entitymanager), `TestClamp_NeverGrants` (acl) |
| 8 — `deny_permissions` withholds | PASS | both entry points, incl. the entity-scoped one backing transition guards |
| 9 — row denial is indistinguishable from 404 | PASS | `TestCeilingE2E_DenyReadIsIndistinguishableFrom404` + list-pruning test |
| 10 — `acl map --as` renders the ceiling | PASS | 5 aclmap tests + verified against a real project |
| 11 — audit flags inert/undeclared config | PASS | `internal/aclaudit/ceiling_test.go`; verified on a real project |
| 12 — deny names the ceiling | PASS | `RuleKind="client-ceiling"`, `RuleID`=baseline name |
| 13 — NopACL/ReadOnlyACL unchanged | PASS | existing suites green; ceiling is inert without policy config |

## Scope honesty

Two limitations are documented rather than silently left:

- **Relation meta fields are not attenuated** — a separate grant vocabulary
with no ceiling equivalent. Row-level `deny_read` still covers both endpoints.
- **stdio MCP is not addressable** — no authentication, so no verified claim to
key a baseline on. This ticket makes the policy expressible; wiring
`internal/mcp` through the read gate is TKT-G3PPD, deferred at the user's
explicit request ("i will fix mcp later; let's first make the code available").

## PR

Not yet raised — branch `tkt-iac8tx-client-attenuation`, 8 commits.
