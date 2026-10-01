---
id: RR-YNNN5G
type: review-response
title: Sync could apply old renames to a reused name
finding: Applying every historic rename when old key exists moves a newly added property that reuses an old name.
severity: significant
resolution: Sync resolves chains in timestamp order and moves a label only when the old name is absent from the current schema and the final name is present; edge case listed.
status: addressed
---
