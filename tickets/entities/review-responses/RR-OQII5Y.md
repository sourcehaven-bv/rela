---
id: RR-OQII5Y
type: review-response
title: Monotonicity test misses the real trigger and uses the function under test
finding: TestRoleUnion_Monotonic never confers a role via an edge, has no everyone role, no client ceiling, no wildcard and no relation meta, and its oracle calls acl.GrantsVerbOnFace, the function under test.
severity: significant
resolution: TestRoleUnion_Monotonic now confers four of eight roles through watches/edits/reviews/drafts edges per mask; adds an everyone role with its own block; a `*` read and update grant; a relation meta block (cites); and an app-client variant under a ceiling that denies ticket update and narrows visibility. The oracle is a hand-written role -> (verb, type, face) table. Disabling the field union fails create/update for user and app (3680 errors); disabling the relation meta union fails meta:reason (672). History passes either way because it was closed-world before the fix; TestRoleUnion_History pins it.
status: addressed
---
