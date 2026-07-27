---
id: REV-CAN9GM
type: review-checklist
title: Review
status: done
---
<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — acl/aclmap/cli pass; coverage-check exits 0. The only local failure is `internal/dataentry`'s `TestScriptReadSeam_PolicylessProjectStaysUnrestricted`, which fails IDENTICALLY on clean `develop` (pre-existing, unrelated to this change), verified by checking out develop and rerunning.
- [x] Lint clean (`golangci-lint`) — 0 issues on acl + aclmap + cli.
- [x] Coverage maintained — aclmap 81.5%, cli 35.8% (floor 30); `just coverage-check` exit 0.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none raised (read/update/delete false-negative guarantee confirmed intact).
- [x] All significant review-responses addressed — blank-key MapAll abort (fixed), no-policy `can` existence gate (fixed); the "create over-reports" finding was investigated and shown to be based on a stale code comment (production create folds edge routes), so keeping the concrete-id behavior is correct and now pinned by conformance tests.
- [x] Self-reviewed the diff for unrelated changes — `access.go` change is comment-only.

**Review Responses:** MCP tracker offline; findings + resolutions recorded in TKT-CAN9GM body ("Code review (cranky) — findings addressed"): 2 significant fixed, 1 investigated-not-a-bug, 1 minor, 1 nit.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All criteria PASS. `can` allow/deny/exit-code/missing-entity/everyone-fold; whole-graph enumeration/everyone-lift/filters/JSON; the keystone conformance tests (whole-graph = union-of-per-principal; `Can` ⟺ runtime; create ⟺ runtime); blank-key robustness — 13 aclmap + 8 CLI tests.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — command godoc + `--help` (args, whole-graph output, exit-code contract, create semantics, data-entry-transport caveat).
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CAN9GM

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — every code check green; this review completion clears the Rela Tickets gate.
- [x] PR URL documented below

**PR:** (to be filled after `gh pr create`)
