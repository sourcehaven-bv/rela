---
id: RR-P054UY
type: review-response
title: internal/git/clone_windows_test.go is compiled by nothing
finding: 'The guard deliberately skips GOOS tags, because `go vet -tags windows` type-checks against the HOST platform and collides with the real GOOS. Correct for this script, but it leaves internal/git/clone_windows_test.go covered by no CI step — rotting by the same mechanism as e2e_test.go. The comment claimed such files "are covered by GOOS=<x> go vet", which is an aspiration: no CI job does that today.'
severity: minor
resolution: Deferred, not silently dropped. `GOOS=windows go vet ./internal/git/` passes today, so nothing is broken. Adding a cross-GOOS vet matrix is a distinct concern from opt-in tags and would widen this ticket past its scope; the comment no longer claims coverage that does not exist. Worth its own ticket.
reason: 'Deferred to its own ticket, not dropped. `GOOS=windows go vet ./internal/git/` passes today, so nothing is broken right now. Cross-GOOS vetting is a different mechanism from opt-in build tags — it needs a GOOS matrix rather than a -tags loop — and folding it in would widen this ticket well past the gap it was filed for. The misleading comment claiming such files are already covered has been corrected, so the hole is documented rather than hidden.'
status: deferred
---
