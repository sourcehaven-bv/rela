---
id: RR-W2RINP
type: review-response
title: Bind failure swallowed; reader built per call
finding: ScriptReader.bind falls back to an unbound ctx on failure, and lateGatedReader builds a reader per call, so each scan has its own ACL scope.
severity: minor
reason: Both are pre-existing ScriptReader and lateGatedReader behavior shared by every read. An unbound ctx is still gated and only slower. Per-call construction honors policy reloads by design. Tracked in TKT-EFOQCX.
status: deferred
---
