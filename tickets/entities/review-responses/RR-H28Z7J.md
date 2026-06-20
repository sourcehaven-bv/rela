---
id: RR-H28Z7J
type: review-response
title: scanApps reads + HTML-parses every app's index.html on every _config request (no cache/cap)
finding: 'scanAppsOrLog runs per _config request (hot path: SPA hits it on mount + after reloads). Each call lists apps/ and, for every app dir, reads up to index.html and runs x/net/html.Parse over it (parseAppMeta). No caching, no watcher, no cap on app count or total scan size. CLAUDE.md accepts ''fine for a handful of apps'' — true — but it degrades silently rather than refusing, and is a self-inflicted hot-path cost if apps ever become numerous. FIX (optional/scoped): memoize the scan keyed on the apps/ dir mtime, or cap the number of apps scanned. Acceptable to defer given the documented scope.'
severity: minor
reason: 'Acceptable as scoped and documented (CLAUDE.md: ''fine for a handful of apps''). The scan is read-only, per-request, over a small expected app count; adding mtime-memoization or a cap is premature optimization that complicates the code for a load profile that doesn''t exist yet. Revisit only if apps become numerous. No correctness or security impact.'
status: deferred
---
