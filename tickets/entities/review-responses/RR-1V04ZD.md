---
id: RR-1V04ZD
type: review-response
title: Missed fourth v1.SectionField conversion site (api_v1.go:2321, the cards/list row path)
finding: 'The plan and ticket both enumerate three `v1.SectionField(f)` conversion sites (views_handler.go:147, 161, 562). There is a fourth: internal/dataentry/api_v1.go:2321, inside `sectionEntityToV1` — the cards/list row path, which is the one the plan''s cards/list acceptance criteria depend on. Verified by `grep -rn ''v1\.SectionField('' internal/`. The unnamed struct conversion makes an omission a compile error, so this cannot ship broken, but the plan''s surface analysis was wrong on the load-bearing path.'
severity: significant
resolution: Plan and ticket corrected to enumerate all four sites, with api_v1.go:2321 identified as the cards/list row path.
status: addressed
---

Verified against source. `grep -rn "v1\.SectionField(" internal/` returns four
hits:

```
internal/dataentry/views_handler.go:147   side panel section fields
internal/dataentry/views_handler.go:161   side panel entity fields
internal/dataentry/views_handler.go:562   view section fields
internal/dataentry/api_v1.go:2321         cards/list row fields  <- missed
```

`api_v1.go:2321` sits in `sectionEntityToV1(e SectionEntityData) v1.ViewEntity`,
which builds the per-row payload consumed by `rowShouldRouteToInlineEdit`.
