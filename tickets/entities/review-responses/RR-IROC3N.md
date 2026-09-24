---
id: RR-IROC3N
type: review-response
title: LowerScope index claim is false for ad-hoc and unsorted lists
finding: scopelower.go said a lowered scope never probes an unindexed shape; the index exists only for the scope a sorted list names.
severity: minor
resolution: 'Comment now says a ?query_scope= choice or an unsorted list may scan: a time cost only.'
status: addressed
---
