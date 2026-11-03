---
id: RR-K07ICO
type: review-response
title: File-tier crash window is described incompletely and untested
finding: 'internal/channels/filechannels/file.go:50-56 lists two crash outcomes. It misses a third: across a segment boundary, appendRecords (ops.go) writes the message to segment n and the notice to segment n+1 in separate appendLog calls. A crash between them leaves the session moved and the message stored, with no notice and no audit record. The "never names a wrong cause" claim still holds. Fix: name this case in the package doc. A fault-injecting test in the style of TestTornAuditIsRepaired would pin the claim that the next append reuses the lost seqs without a wrong cause.'
severity: minor
resolution: The filechannels package doc names the cross-segment case; TestSessionCrashBetweenSegments builds that state and checks the session stays moved and nothing names a wrong cause.
status: addressed
---
