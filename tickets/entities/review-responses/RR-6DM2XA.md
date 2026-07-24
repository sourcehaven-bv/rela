---
id: RR-6DM2XA
type: review-response
title: ExtraReadOnly bind paths not validated (empty/relative become silent no-ops)
finding: spec.ExtraReadOnly entries went straight into --ro-bind-try p p. An empty or relative path from a malformed scan_sockets metamodel entry became a silently-ignored bind (swallowed by -try), leaving the operator with an unreachable scanner and no error to debug.
severity: minor
resolution: WithExtraReadOnly now filters to absolute paths only, warning and skipping non-absolute/empty entries so the misconfiguration is visible in the log. Pinned by TestWithExtraReadOnlySkipsNonAbsolute.
status: addressed
---
