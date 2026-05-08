---
id: RR-QS266
type: review-response
title: Stale `text` and `inlines` together is silently ignored
finding: writeInlinesOrFallback prefers inlines over text. If a script sets both during a half-finished migration, text is silently dropped. No diagnostic.
severity: minor
reason: Migration shim by design. Document removal-target deferred to follow-up.
status: deferred
---
