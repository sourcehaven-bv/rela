---
id: RR-2HDB
type: review-response
title: 'C4: handleV1Documents accepted unsanitised entityID into the cache filename, then silently swallowed write errors'
finding: internal/dataentry/api_v1.go handleV1Documents extracted `docName, entityID := parts[0], parts[1]` from the URL with no validation, then passed both into workspace/document.go which builds a cache filename `documents/{entityID}-{hash}.html`. After my new validateCacheFilename rejection, an entity ID containing `/` or `..` would silently fail the cache write (the call site uses `_ = w.WriteCacheFile(...)`), causing every subsequent render to re-execute the (potentially expensive) shell command — a performance and DoS amplification.
severity: critical
resolution: Added isSafePathSegment helper to middleware_security.go (alphanumerics + hyphen + underscore + dot, no leading dot, no `.`/`..`). handleV1Documents now rejects unsafe docName or entityID with 400 before any filesystem work. Changed workspace/document.go doRenderDocument to log cache write failures via log.Printf instead of swallowing them, so any future regression is visible immediately.
status: addressed
---
