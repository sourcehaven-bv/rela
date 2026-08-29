---
id: REV-S5R0EH
type: review-checklist
title: 'Review: Extract typeResolver + trace/export handlers off mcp.Server (plimsoll ratchet 49 → 38)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full suite + `-race ./internal/mcp/...`; CI green except the Rela Tickets gate, resolved by this ticket's files landing on the branch)
- [x] Linters pass (golangci-lint 0 issues, plimsoll at 38, arch-lint OK, comment-lint clean across 11071 comments)
- [x] Coverage floors hold

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, independent verification of the 11c02c1b..b013fb44 diff)
- [x] All critical/significant findings addressed (none — reviewer's normalized-receiver diff showed the moved bodies are byte-identical modulo field renames)
- [x] Minor finding addressed: [[RR-OSJQWC]] — handler-field assignment hazard, closed structurally in the very next arc slice via the embedded `handlerSet`

## Verification

- [x] Identity/ACL invariant verified: principalMiddleware still a Server method registered at SDK level before registerTools; zero principal references outside server.go; no handler struct carries a principal field
- [x] Narrow-reader invariant verified end-to-end: handlers hold `GraphReader`, not `store.Store`; the wiring site passes `reads.Reader` from `GatedReads()`, so a networked wiring substitutes a visibility-gated reader without touching a handler
- [x] toolGetSchema/toolGetMetamodel aliasing confirmed intact
- [x] Method count independently verified at 38, matching the directive with no slack
- [x] Tests: call-site re-points only; TestACL_Trace_HiddenRootIsNotAnOracle (the hidden-vs-absent oracle guard) survives and passes
- [x] Doc-drift fix verified accurate against the current code

**Note:** the RR-FTJUUE existence-oracle comment in tools_trace.go was correctly
updated to track the new field — the reviewer specifically checked for stale
prose here and found none.
