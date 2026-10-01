---
id: RR-58ENT7
type: review-response
title: pg visible search shape has no EXPLAIN pin
finding: Moving the visibility clause inside DISTINCT ON evaluates the ACL EXISTS chain per candidate face row; no EXPLAIN or budget test pins the new plan.
severity: minor
reason: 'PR 11 pinned the list and static-query shapes (flat-world EXPLAIN on both backends). The ranked visible-search shape stays unpinned: its cost is the per-face ACL clause the design requires inside DISTINCT ON, and no plan alternative exists to pin against.'
status: deferred
---
