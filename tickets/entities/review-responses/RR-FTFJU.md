---
id: RR-FTFJU
type: review-response
title: Use rela.document.entry_id, not rela.params.entry_id
finding: 'Plan injects entry_id via rela.params, conflating system-injected identity with author-configured YAML params. A util library checking rela.params.entry_id may see stale values from a different context. Cleaner: rela.document.entry_id sits next to rela.document.id — same grouping, clearer ownership.'
severity: significant
resolution: entry_id moved to rela.document.entry_id (sits next to rela.document.id). AC3 updated. WithDocumentMode now takes (documentID, entryID). rela.params stays clean for author-configured params.
status: addressed
---

From design-review on PLAN-78HJO.

The earlier `/ticket` chat specifically chose rela.params for "no new API
surface". Reviewer's point is stronger: consistency within `rela.document.*` is
worth the one extra field.

Change AC3 to assert `rela.document.entry_id == <entryID>` rather than
`rela.params.entry_id`.
