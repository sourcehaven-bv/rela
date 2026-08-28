---
id: REV-9OT5Z2
type: review-checklist
title: 'Review: Enable gosec G704 (SSRF) and annotate operator-configured HTTP destinations'
status: done
---

## Automated Checks

- [x] All tests pass — `go test ./internal/ai/... ./internal/cli/...` green
- [x] Lint clean — `golangci-lint run ./...` 0 issues with G704 enabled
- [x] ~~Coverage maintained~~ (N/A: no behaviour change, so no new code to cover)

## Code Review

- [x] Run `/code-review`
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

This PR annotates rather than changes, so the review question is whether each
annotation names a boundary that actually holds. Both do, and both were verified
by tracing provenance to the source rather than by inspection of the sink. The
strongest evidence is the two-layer negative on the AI endpoint: an override
cannot exist even in principle, because the request type has no URL field.

The hostile-remote-server angle is the non-obvious one and was checked rather
than dismissed: a malicious server influences path segments only, never scheme or
host.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-ZRW35Z
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- G704 enabled repo-wide — PASS (removed from `gosec.excludes`)
- Both findings resolved with narrow, per-site annotations — PASS
- AI endpoint provenance is local config, not request data — PASS (traced end to
end; confirmed negative at two layers for Lua)
- Sync remote is operator-supplied via flag/env — PASS
- Hostile remote server cannot redirect the destination — PASS (`JoinPath`
influences path only; cross-origin redirects not followed)
- No security control weakened — PASS (`Validate` constraints and redirect
refusal both retained)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked~~ (N/A: internal security-lint work,
`kind=refactor`, no user-facing behaviour change)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — full matrix green; the `Rela Tickets` gate is resolved
by this done-transition
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1250
