---
id: RR-3YH5TJ
type: review-response
title: local_changes and remote_ahead are pre-filter counts served to every principal
finding: GET /_git/status returns working-tree counts that include hidden entities, a weak timing channel (no id, also counts visible writes). It predates this change. (rela-security-reviewer)
severity: minor
resolution: Deferred to the BUG-T6HNKS review.
reason: Pre-existing and out of scope for a missing-permission fix. It belongs with the review of conflict and status disclosure in BUG-T6HNKS.
status: deferred
---
