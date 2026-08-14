---
id: RR-DR-ETAG
type: review-response
title: The ticket wrongly asserts RR-CR-ETAG 'is unaffected and stays deferred' - its rationale does not
  survive the new asset class
finding: 'RR-CR-ETAG was deferred on the explicit grounds that ''the shell is ~3.4KB and uncacheable-by-design
  anyway (it must reflect current custom.css/custom.js presence per request)''. That rationale does NOT
  transfer to a 200KB webfont or a hero image, which are exactly the assets browsers cache hardest and
  which are static. Under the current handler (w.Write, Cache-Control: no-cache, no ETag/Last-Modified/Range)
  every such asset re-transfers in full on every navigation. The ticket''s Related section claims ETAG
  is unaffected; that is wrong.'
severity: minor
status: addressed
resolution: Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Either re-open RR-CR-ETAG for
  the asset path specifically (http.ServeContent gives ETag, conditional requests and Range in one call),
  or explicitly re-justify the deferral against the new asset class rather than inheriting a rationale
  that was written about a 3.4KB shell. Correct the ticket's Related section either way.
---

Raised by `/design-review` of TKT-IWMETE before implementation.
