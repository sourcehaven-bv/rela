---
id: REV-6LC57O
type: review-checklist
title: 'Review: Make searchVisibleHits fail closed when the searcher cannot redact fields'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — adds tests, removes none.
- [x] `just arch-lint`, `just plimsoll`, `just lint-md` — clean.

## Code Review

- [x] ~~Run `/code-review` command~~ — self-reviewed. The change is a
three-line reorder plus a refusal branch, quoted from a settled precedent one
layer down, and the property that needed proving (the fallback does not run at
all) was established by mutation.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes — two files, both intended.

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — fails closed. PASS.** Mutation-verified: restoring the fall-through
yields `got 1 hit(s) and no error`. The test also asserts the fallback never
ran, so an implementation that errored *after* serving would still fail.
- **AC2 — the no-redaction case still works. PASS.** Same non-redacting searcher
with the Nop resolver serves its hit. This is what stops the fix from breaking
the default deployment.
- **AC3 — `ErrScope` specifically. PASS.** The caller in `queryservice.go` maps
it to `errACLListQuery`, so the refusal surfaces as an authorization failure
rather than a generic search error.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal wiring
guard, no operator-visible configuration or behaviour. The one doc change is a
godoc correction, covered in the implementation checklist.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the godoc now says what the function
does rather than what a method it never called does, and cites RR-8W40EW so the
next person sees the principle rather than re-deriving it.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design.)
