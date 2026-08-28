---
id: RR-N7IX9Z
type: review-response
title: 'No Cache-Control decided for /_custom/*: un-versioned URL is the classic stale-asset trap'
finding: 'The plan identifies the Content-Length/ETag risk for the rewritten shell and mitigates it, and
  it notes noCacheMiddleware is /api/-only with TestNewRouterStaticFilesNoCacheHeader pinning the SPA
  as having no Cache-Control — then stops without deciding what /_custom/* itself sends. No Cache-Control
  means heuristic caching: browsers may cache /_custom/custom.css for an arbitrary interval based on Last-Modified.
  Unlike every other asset in the SPA these files are NOT content-hashed — the URL is permanently /_custom/custom.css.
  That is the textbook un-versioned-asset cache trap, and it arrives through exactly the ''operator errors
  look like rela bugs'' channel the ticket already worries about.'
severity: significant
status: addressed
resolution: 'Cache-Control: no-cache set on /_custom/* in handleCustomAsset, asserted in TestHandleCustomAsset.
  Verified against a live server: the header is present on a real request.'
---

## Failure scenario

Operator edits `custom.css` to fix a broken layout, reloads, sees no change.
Hard-refreshes — works. Tells their users; half still see the broken version for
an unknowable duration. They file a bug against rela.

## Recommended resolution

Decide explicitly. `Cache-Control: no-cache` (revalidate every time, cheap with
`ETag` / `Last-Modified`) is the right default for an operator-edited file at a
stable URL. Add it to AC1 and assert it in the handler test.
