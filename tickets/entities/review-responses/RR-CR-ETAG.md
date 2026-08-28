---
id: RR-CR-ETAG
type: review-response
title: 'The shell handler drops ETag/Last-Modified that http.FileServer previously provided'
finding: 'http.FileServer served the shell with Last-Modified from the embed FS and honoured If-Modified-Since, so a repeat visitor got a 304 with no body. The new handler sets neither, so every shell request transfers the full ~3.4KB and there is no conditional-request path. Range requests are also ignored (200 with the full body where FileServer would 206). Cosmetic at 3.4KB, and nothing requests ranges on an HTML shell.'
severity: minor
status: addressed
resolution: "DEFERRED, not fixed. The reviewer's suggested remedy (hash each precomputed variant, serve via http.ServeContent) is sound and would restore conditional requests, HEAD and Range in one call. It is deliberately out of scope here: the shell is ~3.4KB and uncacheable-by-design anyway (it must reflect current custom.css/custom.js presence per request), so the win is small and the change touches the hot path. Recorded so it is a decision rather than an oversight."
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
