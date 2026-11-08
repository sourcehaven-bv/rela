---
id: RR-ZBI8HUE
type: review-response
title: Symlinked audit directory reported as removed
finding: os.RemoveAll removes only the link, and AuditRemoved was still true.
severity: minor
resolution: A symlinked audit directory is refused with an error. Pinned by TestErase_AuditFailures/symlink.
status: addressed
---
