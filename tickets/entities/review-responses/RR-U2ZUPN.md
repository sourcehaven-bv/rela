---
id: RR-U2ZUPN
type: review-response
title: pg StopWithoutStart covers only a nil receiver
finding: The unstarted non-nil case is untested on postgres.
severity: minor
resolution: Added the unstarted non-nil case over newScopedPool.
status: addressed
---
