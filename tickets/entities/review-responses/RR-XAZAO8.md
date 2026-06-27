---
id: RR-XAZAO8
type: review-response
title: No caching on 372KB editor bundle + font
finding: _rela-editor.js (372KB) and _rela-editor.woff2 (77KB) were served with no Cache-Control/ETag, so they were re-transferred in full on every iframe (re)load (iframes reload on host navigation/v-if/remount).
severity: critical
resolution: 'Added serveCachedAsset: ETag (sha256 of bytes) + Cache-Control: public, max-age=0, must-revalidate, with an If-None-Match 304 path. Not ''immutable'' because the URL is unversioned (a new build changes the bytes->ETag, so caches revalidate and pick up new builds without staleness). Covered by ''editor assets carry an ETag and 304'' test.'
status: addressed
---
