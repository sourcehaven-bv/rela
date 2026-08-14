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

- [x] Run `/code-review` — the cranky-code-reviewer agent was launched but died on an API error (connection closed mid-response) before producing findings. A focused manual security/correctness review was completed in its place (see below).
- [x] ~~All critical review-responses addressed~~ (none found)
- [x] ~~All significant review-responses addressed~~ (none found)
- [x] Self-reviewed the diff for unrelated changes (only the .golangci.yml contextcheck exclusion + cmd/rela-server email adapter are outside internal/; both directly required)

**Review Responses:** none new. Design findings RR-28SCW3 (addressed) and
RR-VI9XMY (addressed — seam corrected) carried from design review.

**Manual review coverage (seven risk areas, all clear):**
1. Re-stamp seam reaches manager AND every response-shaping read on all paths (`r = h.enterWrite(r)` reassigns the request so gateRead/reader/serializer see the rebuilt gate).
2. Flag-clear (`WithMatchedVerified`) applied ONLY on the fully-successful provision path; every error branch returns the original unmatched ctx.
3. Fail-closed: all provision errors fall back to the UNMATCHED principal, never a forged/elevated one.
4. Concurrency: `ErrEntityAlreadyExists` caught → re-resolve; serialized under the held writeMu; 8-way concurrent test → exactly one stub.
5. Bare-stub containment intact (system:provisioner create-user-only; cascade cannot author edges, RR-28SCW3).
6. Sync path: `syncContext` re-stamps on the PROVISIONED ctx, preserving resolved User + the ACL-request/read-gate ctx values.
7. Email trust boundary: `WithEmail` reachable only from a verified source; webhook path is off /api/ and correctly excluded from the seam.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS, AC2 PASS, AC3 PASS, AC4 PASS, AC5 PASS, AC6
PASS (see IMPL-YYMOSR for the test mapping).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-KBKMZF)
- [x] User-facing documentation updated (GUIDE-acl-security.md `provision` section + operator-responsibility notes; `docs/acl-security.md` regenerated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-KBKMZF (done)

## Final Checks

- [x] Commit message explains the why (feat + docs commits; design captured on ticket/RRs)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** in progress — see below.
