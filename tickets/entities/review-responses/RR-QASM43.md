---
id: RR-QASM43
type: review-response
title: Post-commit byte cleanup uses the request context
finding: Unreferenced-byte deletes after a committed write ran on the request context, so a cancelled request skipped them.
severity: minor
resolution: Cleanup runs on context.WithoutCancel plus a timeout (cleanupContext and dropUnreferenced).
status: addressed
---
