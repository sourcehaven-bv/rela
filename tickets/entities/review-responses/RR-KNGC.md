---
id: RR-KNGC
type: review-response
title: Pagination/sidebar leak surface broader than AC4/AC6 capture
finding: 'AC4 asserts `total: 5` for 5 visible + 5 hidden but doesn''t enumerate the other leak surfaces the list handler emits: X-Total-Count header (api_v1.go:462), X-Page, X-Per-Page, and the Link header from addPaginationLinks (RFC 5988 — last-page link depends on total). Each must reflect the post-filter count or each is its own enumeration channel. AC6 pins listCount but not kanbanCount (plan modifies both per technical approach). Sidebar menu structure also reveals metamodel shape but not data shape — decide and pin whether menu items for types the principal has zero `read` grant on are visible (current intent: yes, menu == metamodel; tighten in a future ticket if needed). Fix: widen AC4 to enumerate all five leak surfaces (data length, meta.total, X-Total-Count, X-Page links, header timing). Widen AC6 to cover kanbanCount and pin the menu-vs-data decision.'
severity: significant
reason: Moved to TKT-VMD8. AC3 there pins all five pagination leak surfaces (data.length, meta.total, X-Total-Count, Link header, X-Page) and AC5 covers kanbanCount. Menu-vs-data decision documented there too.
status: deferred
---
