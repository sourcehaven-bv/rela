---
id: RR-NQWJ6G
type: review-response
title: Tests do not pin each gated column or the purge fallback
finding: Weakening the gate (dropping relation properties or from_id, or making the purge fallback always dirty) passed the whole pgstore suite. IdleTickCapturesNothing checked nothing the gate does.
severity: significant
resolution: EditBehindAFullBatch is table-driven over entity properties, entity content, relation content and relation properties. The purge case is covered by UnchangedSaveAfterForceLivePurge. IdleTickCapturesNothing was removed. Trigger behaviour is pinned by TestContentHashTriggers (pg) and TestMigrateToContentHash (sqlite).
status: addressed
---
