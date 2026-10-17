---
id: REV-4QETIL
type: review-checklist
title: 'Review: archplan security declarations and routes suite'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

## Automated Checks

- [x] All tests pass (touched packages: `go test ./internal/acl/ ./internal/jwtauth/ ./internal/state/ ./internal/store/pgstore/ ./internal/dataentry/`; the full `go test -race ./...` hangs locally in `TestClassificationCmd_SyncThenLint`, unrelated, left to CI)
- [x] Lint clean (golangci-lint on the touched packages: 0 issues; gofmt clean)
- [x] Coverage maintained (comments, a demo script and docs only; no Go code changed)

## Code Review

- [x] Code review run (cranky-code-reviewer on the diff)
- [x] All critical review findings addressed (none)
- [x] All significant review findings addressed:
  - `ValidatedKV.Delete` is now a write sink too.
  - The demo no longer sends requests to `/webhooks/idp` and `/hooks/x`; the demo configures neither, so they only reached the SPA catch-all.
  - The `ResolvePrincipal` reason now says it maps an identity that the JWT gate or the trusted proxy supplied, and verifies nothing itself.
  - `auth=required` on the `/api/` handlers stays: it declares the intended policy, which the detectors check against traces. In header mode a request without identity reaches the ACL as unknown; the routes suite traces exactly that.
- [x] Self-reviewed the diff for unrelated changes

## Acceptance Verification

- [x] Markers parse: archplan reads every declaration from this checkout.
- [x] `SEQTRACE_SUITE=routes just seqtrace-demo` writes one diagram per route request.

## Final Checks

- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created (#1906)
