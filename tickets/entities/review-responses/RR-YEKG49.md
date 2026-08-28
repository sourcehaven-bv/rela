---
id: RR-YEKG49
type: review-response
title: Invalid config could trigger destructive orphan drops
finding: Treating an unreadable or partly invalid data-entry config as an empty desired query-index set would classify still-desired owned indexes as orphans and drop them.
severity: significant
resolution: Plan requires all-or-nothing desired-set loading. Present invalid config aborts reconciliation before DDL.
status: addressed
---
