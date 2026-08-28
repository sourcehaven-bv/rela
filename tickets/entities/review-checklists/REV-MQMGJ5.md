---
id: REV-MQMGJ5
type: review-checklist
title: 'Review: Provision a stub user entity for an unmatched verified principal (unmatched_principal: provision)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green; `-race` clean on dataentry/acl)
- [x] Lint clean (`golangci-lint run ./...` → 0 issues; arch-lint OK; plimsoll OK)
- [x] Coverage maintained (`just coverage-check` exit 0, no floor violations)

## Code Review

- [x] Run `/code-review` — the cranky-code-reviewer agent died on an API error mid-run; a focused manual security/correctness review was completed in its place (see below).
- [x] ~~All critical review-responses addressed~~ (none found)
- [x] ~~All significant review-responses addressed~~ (none found)
- [x] Self-reviewed the diff for unrelated changes (only the .golangci.yml contextcheck exclusion + cmd/rela-server email adapter are outside internal/; both directly required)

**Review Responses:** none new. Design findings RR-28SCW3 (addressed) and
RR-VI9XMY (addressed — seam corrected) carried from design review.

**Manual review coverage (seven risk areas, all clear):**
1. Re-stamp seam reaches manager AND every response-shaping read on all paths.
2. Flag-clear (`WithMatchedVerified`) applied ONLY on the fully-successful provision path.
3. Fail-closed: all provision errors fall back to the UNMATCHED principal.
4. Concurrency: `ErrEntityAlreadyExists` caught → re-resolve; 8-way concurrent test → exactly one stub.
5. Bare-stub containment intact (system:provisioner create-user-only, RR-28SCW3).
6. Sync path: `syncContext` re-stamps on the PROVISIONED ctx, preserving resolved User + gate.
7. Email trust boundary: `WithEmail` only from a verified source; webhook off /api/ correctly excluded.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1–AC6 all PASS (see IMPL-YYMOSR for the test mapping).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-KBKMZF)
- [x] User-facing documentation updated (GUIDE-acl-security.md `provision` section; `docs/acl-security.md` regenerated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-KBKMZF (done)

## Final Checks

- [x] Commit message explains the why (feat + docs commits; design captured on ticket/RRs)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (local `just ci` exit 0; remote CI monitored to green)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1332
