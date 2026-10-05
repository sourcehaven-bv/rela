---
id: RR-2NNKG6
type: review-response
title: Injection constant named for delete only
finding: The rename test reused injectBeforeStoreDelete, which read wrongly.
severity: nit
resolution: Renamed to injectBeforeStoreWrite with an updated comment.
status: addressed
---
