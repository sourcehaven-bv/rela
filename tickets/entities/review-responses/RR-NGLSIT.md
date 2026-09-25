---
id: RR-NGLSIT
type: review-response
title: Scope resolved over every type even when the query names one
finding: gatedSearcher computed ReadQuery for every metamodel type per call.
severity: minor
resolution: Scope is limited to q.Types when set. Pinned by the denied-type case in TestGatedReads_SearcherHitShape.
status: addressed
---
