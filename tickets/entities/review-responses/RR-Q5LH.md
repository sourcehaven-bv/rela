---
id: RR-Q5LH
type: review-response
title: TKT-VMD8 Prerequisite section claims false dependencies on TKT-VQGN APIs
finding: 'TKT-VMD8''s Prerequisite lists Request.Visible and store.GraphQuery.WhereIDs as required, but nothing in PR 2''s documented scope uses them. scopedSortedEntities, sidebarCounts.countWithFilters, _position (via resolveScope), and _actions.create only need ReadQuery + existing GraphQueryer (GraphQuery + GraphCount) — all present on PR 911. Middleware fail-loud invariant also not strictly required: attachACLRequest already exists on feat/acl-v1-wiring. Fix: rewrite Prerequisite to list only Request.ReadQuery and store.GraphQueryer.{GraphQuery, GraphCount}, both present on PR 911. PR 2 stacks on PR 911 directly. Removes the cross-PR rebase cliff and lets PR 2 land independently if PR 1 slips. If you want to keep a soft ''land in order'' dependency, say so explicitly — don''t fabricate API dependencies.'
severity: critical
status: open
---
