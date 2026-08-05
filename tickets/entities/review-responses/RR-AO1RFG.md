---
id: RR-AO1RFG
type: review-response
title: Relation restore raw-read invariant unpinned by any test
finding: restoreRelationHistoryVersion correctly reads snap.Properties raw (never through visibleRelationMeta), but no test pinned the 'never redact a read that feeds a write' rule for relations — the entity side has TestScriptReads_UpdatePreservesHiddenProperties, relations had no equivalent. A future 'tidy the two GetRelationVersion calls into one helper' refactor could silently reintroduce the data-destruction bug (a redacted read-modify-write erases the caller's hidden meta on save).
severity: significant
resolution: 'Added TestRelationHistoryRestore_ReadsRawMeta_PreservesHidden: restores v1 of a relation whose reason meta is redacted on the display path, as a non-history:read-redacted principal, then asserts the restored LIVE edge still carries reason=blocked — proving restore read the raw frozen meta.'
status: addressed
---
