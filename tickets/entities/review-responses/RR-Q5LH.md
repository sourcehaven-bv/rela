---
id: RR-Q5LH
type: review-response
title: TKT-VMD8 Prerequisite section claims false dependencies on TKT-VQGN APIs
finding: 'TKT-VMD8''s Prerequisite lists Request.Visible and store.GraphQuery.WhereIDs as required, but nothing in PR 2''s documented scope uses them. scopedSortedEntities, sidebarCounts.countWithFilters, _position (via resolveScope), and _actions.create only need ReadQuery + existing GraphQueryer (GraphQuery + GraphCount) — all present on PR 911. Middleware fail-loud invariant also not strictly required: attachACLRequest already exists on feat/acl-v1-wiring. Fix: rewrite Prerequisite to list only Request.ReadQuery and store.GraphQueryer.{GraphQuery, GraphCount}, both present on PR 911. PR 2 stacks on PR 911 directly. Removes the cross-PR rebase cliff and lets PR 2 land independently if PR 1 slips. If you want to keep a soft ''land in order'' dependency, say so explicitly — don''t fabricate API dependencies.'
severity: critical
reason: 'This RR critiques TKT-VMD8''s planning content (its Prerequisite section), not any code in TKT-VQGN. Acting on it would mean editing TKT-VMD8''s planning-checklist (PLAN-RBHK) and ticket markdown before TKT-VMD8 starts — that''s out-of-scope for this PR and a reviewer would correctly reject the cross-ticket edit here. The critical severity reflects "if uncorrected, TKT-VMD8 will plan with false dependencies and rebase poorly," which is a TKT-VMD8-start blocker, not a TKT-VQGN-merge blocker. Architect rework here also dropped WhereIDs entirely, so the TKT-VMD8 Prerequisite as written is now further wrong — a stronger reason to refresh that planning when TKT-VMD8 begins, not as a side-effect of this PR.'
status: deferred
---
