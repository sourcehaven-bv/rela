---
id: RR-OBXBWL
type: review-response
title: Sidebar re-resolves ReadQuery + GraphQuery per nav item without (type, filters) memo
finding: Two sidebar lists over the same entity type with identical filters each recompute ReadQuery and run a full GraphQuery; filterCache keys on list/kanban id, not (type, filters). Missed reuse only — the request-scoped cache itself is correct per RR-BZ4M.
severity: nit
resolution: 'Documented in the countWithFilters godoc: per-nav-item recompute is accepted (the member-of walk is already memoized on the request-scoped acl.Request), and a (type, filters)-keyed memo is named as the obvious upgrade if sidebar latency ever warrants it. No code change — typical sidebars have a handful of nav items. Commit 622b6cf7.'
status: addressed
---
