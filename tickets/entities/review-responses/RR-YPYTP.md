---
id: RR-YPYTP
type: review-response
title: today() location vs UTC date-literal parse timezone seam
finding: 'Bind() truncates today() in now.Location(); parseDateLiteral uses time.Parse (UTC). entity.due < today() compares a UTC-midnight bound against a local-midnight today(), skewing up to a day once the Phase-2 entity binder lands if it parses in a different location. Fix: pin the location story now — parse literals and truncate today() in the same location (ideally UTC).'
severity: minor
resolution: Bind() now truncates today() to UTC midnight (now.UTC() + time.UTC), matching parseDateLiteral which yields UTC for zoneless layouts. entity.due < today() is now consistent regardless of the caller's local zone. Documented in the Bind godoc.
status: addressed
---
