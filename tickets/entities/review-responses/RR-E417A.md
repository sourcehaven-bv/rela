---
id: RR-E417A
type: review-response
title: pageContainsAny has inconsistent case semantics
finding: String[] path lowercases both sides; RegExp path runs as-is. Two behaviours, one name. Split into pageContainsAnyText and pageMatches.
severity: significant
reason: pageContainsAny is used in two places in dashboard.spec.ts. The inconsistent semantics would matter if callers expected either-way behaviour. Current callers work by accident (regex path uses /i flag, string path is lower-cased). Can be split when a third caller disagrees.
status: deferred
---
