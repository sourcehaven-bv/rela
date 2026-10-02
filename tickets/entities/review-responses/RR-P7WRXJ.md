---
id: RR-P7WRXJ
type: review-response
title: Unparseable resume key restarts paging forever
finding: Internal iterators rendered the keyset key as a string and parsed it back with ParseStateRef/splitRelationKey, which run ValidateID. A legacy id (e.g. containing --) at a page boundary fails to parse, the keyset is dropped, and the listing restarts on every page without end.
severity: significant
resolution: Keys are now typed (stateKey, relationKey, visibleKey) and never parsed back; only external page cursors are parsed. pagedSeq also errors when a page makes no progress. Pinned by TestIteratorPaging_LegacyIDAtPageBoundary.
status: addressed
---
