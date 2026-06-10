---
id: RR-4KBP7W
type: review-response
title: 'AllowAll vs Query store-error asymmetry: silent truncation on the AllowAll list path'
finding: scopedSortedEntities propagated mid-stream GraphQuery errors (Query path → 500) but the AllowAll branch used listFromStoreByTypes, which swallows iterator errors into a partial slice — under the same backend fault an AllowAll principal got a silently-truncated 200 with authoritative-looking pagination while a Query principal got a clean 500.
severity: significant
resolution: AllowAll branch now iterates Store.ListEntities inline and wraps mid-stream errors in the new errListLoad sentinel; both pipeline consumers (handleV1ListEntities, handleV1EntityPosition) route errors through the shared writeListPipelineError helper (errACLListQuery → writeGateError, errListLoad → 500 list_load_failed, else search_failed). Pinned by TestACLList_AllowAllLoadErrorSurfaces. listFromStoreByTypes itself is unchanged — its other consumers (views, commands, mentions) keep their historical degrade behavior. Commit 622b6cf7.
status: addressed
---
