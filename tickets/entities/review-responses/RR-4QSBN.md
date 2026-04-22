---
id: RR-4QSBN
type: review-response
title: Singleflight key must include docConfigID
finding: documentService.Render keys singleflight on entryID alone. With multi-doc support (frontend already allows this via availableDocuments), two concurrent requests for the same entry but different document configs will collapse and the second caller receives the first caller's HTML. Must be fixed in-ticket; plan's own example encourages multi-doc setups.
severity: critical
resolution: Singleflight key now keyed on entryID + '|' + cfg.ConfigID (plan approach §4). Verified in AC8.
status: addressed
---

From design-review on PLAN-78HJO.

`document.go:104` uses `s.group.Do(entryID, ...)`. Required change: key on
`entryID + "|" + configID` (and possibly renderer type).

Interaction with finding on disk-cache read path (RR for #8): both the cache key
and singleflight key must incorporate the config identity.
