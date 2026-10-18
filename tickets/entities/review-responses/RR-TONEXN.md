---
id: RR-TONEXN
type: review-response
title: Purge reads retention and policy from the current config, not the config at tombstone time
finding: A3 step 1 takes policyGrace and lifecycle.delete.retention from the current snapshot and treats undeclared types as DetachedRetention, and RunRetention still purges when snap.Config is nil. Removing a channel type or realtime.yaml, or shortening retention, therefore erases tombstoned channels held under a 7y retention after 30 days. Capture policy and retention-until on the batch at tombstone time, or never purge an undeclared or config-less batch.
severity: significant
resolution: The tombstone batch records its policy and purge-after time when it is filed. Purge uses the recorded value. A batch without a recorded value (filed before T4) is purged only when the current config declares its type, and never under a nil or degraded config.
status: addressed
---
