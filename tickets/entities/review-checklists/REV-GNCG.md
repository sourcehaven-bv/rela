---
id: REV-GNCG
type: review-checklist
title: 'Review: affordances: migrate resolver to *acl.Declarative; has_role consults ancestor-conferred roles'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full tree green; race-clean for
affordances/dataentry packages.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Coverage maintained (`just coverage-check`) — 74.3%, package
floors PASS.
- [x] arch-lint clean (`just arch-lint`).

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~
(N/A: per the split plan, the reference branch `feat/acl-v1-tkt-svxl` already
underwent two cranky-code-reviewer passes producing 22 review-response findings.
This PR ports the already-reviewed code; this checklist audits the PR-3-scope
findings.)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses (audit of source-branch findings applicable to PR-3 scope):**

| Finding ref | Issue | Verified in PR-3 code |
|-------------|-------|----------------------|
| RR-WTLD (significant) | `affordances.New` takes `*acl.Declarative`; drops policy arg | `internal/affordances/resolver.go` `New(meta, lookup, *acl.Declarative)`. Resolver reads policy via `declarative.Policy()`. |
| RR-JRPZ (significant) | `has_role` consults `entityRoles` (sees ancestor-conferred) | `internal/affordances/hostfuncs.go` `hasRole` reads from `bindingContext.entityRoles`. `holdsLocalRole` deleted. `TestFeature_HasRole_AncestorConferred` covers. |
| RR-Y6Y9 (minor) | AC8 parity test rewritten discriminating | `TestFeature_AC8_WriteAffordanceParity` in `features_test.go` uses a scenario where only a correct resolver agrees with the write path. |
| RR-K7CT (minor) | `TestResolver_ReusesRequestFromContext` premise is fatal on stub failure | `resolver_test.go` test uses a counting graph stub; asserts `require.Equal(t, 0, calls)` after reuse (fatal). |
| RR-JJYW (significant, dataentry hook part) | Resolver consults `acl.FromContext(ctx)` | `resolveViaDeclarative` checks `acl.FromContext(ctx)` first; falls back to fresh Request. PR 4 wires the middleware that attaches the Request to ctx. |

**Deferred to PR 4 (out of PR-3 scope):**

- RR-72OJ (critical) — appbuild fail-loud on malformed acl.yaml
- RR-FGJR (significant) — `Collaborators.Declarative` + `appbuildtest.WithDeclarative`
- RR-36UL (significant) — `WithACL` auto-detect Declarative
- RR-8ZGO (minor) — middleware respects upstream Request
- RR-7O6Q (nit) — member-of docs in docs/security.md
- The `Services.ACLDeclarative()` accessor change in
`ResolverServices` — kept on `ACLPolicy()` here so the PR-3 compile is minimal;
PR 4 swaps the interface alongside the proper StoreGraph wiring.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 6 acceptance criteria from PLAN-EUTI verified PASS — see IMPL-64E4 "AC
mapping" for test names + results.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor; user-facing docs land with PR 4 wiring)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — internal refactor.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** *to be filled after `gh pr create`*
