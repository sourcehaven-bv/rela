---
id: RR-8EOIXR2
type: review-response
title: A failed load is never retried for the same page
finding: loadedKey was latched before the request, so after an error loadOnce declined until the reader left the page.
severity: minor
resolution: The latch is cleared on failure. Pinned by a test.
status: addressed
---
