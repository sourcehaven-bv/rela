---
id: RR-26VAT
type: review-response
title: rela.url.form overloading is structural discriminator masquerading as polymorphism
finding: 'internal/lua/urls.go luaURLForm picks edit vs create mode based on whether the 2nd arg table has an "id" string field. rela.url.form("x", {id="prefill-x"}) silently picks edit mode when the author meant "create with prefill id=...". The ''query'' sub-key inside an entity-table (urls.go:144) signals the shape is muddled — create has three shapes, edit has two. Fix: split into rela.url.form_edit(name, entity) + rela.url.form_create(name, opts). Update tests (internal/lua/urls_test.go:247-311 split cleanly), docs (GUIDE-data-entry.md), VWS scripts, prototype.'
severity: significant
resolution: Split rela.url.form(name, arg?) into rela.url.form_edit(name, entity) and rela.url.form_create(name, opts?). form_edit takes an entity-shaped table (requires string id); form_create takes the full opts table with relations/properties/query. No more structural discriminator on opts.id — author's intent is explicit in the helper name. Updated 12 test cases in urls_test.go, the VWS scripts (applicatie_overview.lua + build_markdown.lua), the prototype (category_report.lua), and GUIDE-data-entry.md. Added note to the guide about why the split exists.
status: addressed
---
